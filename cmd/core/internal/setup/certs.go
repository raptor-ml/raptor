/*
Copyright (c) 2022 RaptorML authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package setup

// +kubebuilder:rbac:groups=cert-manager.io,resources=issuers;certificates,verbs=get;create;update;patch;delete;watch;list
// +kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=get;update;patch;watch;list
// +kubebuilder:rbac:groups=admissionregistration.k8s.io,resources=validatingwebhookconfigurations,verbs=get;update;patch;list;watch
// +kubebuilder:rbac:groups=admissionregistration.k8s.io,resources=mutatingwebhookconfigurations,verbs=get;update;patch;list;watch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;create;update;patch;list;watch

import (
	"context"
	"fmt"
	certApi "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	"github.com/go-logr/logr"
	"github.com/open-policy-agent/cert-controller/pkg/rotator"
	opctrl "github.com/raptor-ml/raptor/internal/operator"
	"github.com/spf13/viper"
	"golang.org/x/sync/errgroup"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"net/http"
	"os"
	"reflect"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"time"
)

func Certs(mgr manager.Manager, certsReady chan struct{}) {
	OrFail(mgr.AddReadyzCheck("certs", func(_ *http.Request) error {
		select {
		case <-certsReady:
			return nil
		default:
			return fmt.Errorf("cert rotation not ready")
		}
	}), "unable to add readyz check for certs")

	ns, err := getInClusterNamespace()
	OrFail(err, "Failed to discover local namespace. Are you running in a cluster?")

	gvk := certApi.SchemeGroupVersion.WithKind(certApi.IssuerKind)
	_, err = mgr.GetClient().RESTMapper().RESTMapping(gvk.GroupKind(), gvk.Version)
	if err == nil {
		setupLog.Info("Cert-manager detected")
		err = mgr.Add(certManagerRunnable(mgr.GetClient(), mgr.GetScheme(), ns, certsReady))
		OrFail(err, "Failed to add cert-manager runnable")
		return
	}

	ce := &certsEnsurer{
		client:        mgr.GetClient(),
		isReady:       certsReady,
		upstreamReady: make(chan struct{}),
		logger:        mgr.GetLogger().WithName("certs-ensurer"),
	}
	OrFail(mgr.Add(ce), "Failed to add cert-ensurer runnable")
	certsWithCertsController(mgr, ns, ce.upstreamReady)
}

const (
	secretName     = "raptor-webhook-server-cert" //nolint:gosec //pragma: allowlist secret
	certName       = "raptor-serving-cert"
	serviceName    = "raptor-webhook-service"
	caName         = "raptor-ca"
	caOrganization = "raptor"
	certDir        = "/tmp/k8s-webhook-server/serving-certs"
	certFileName   = "tls.crt"
)

var webhooks = []rotator.WebhookInfo{
	{
		Type: rotator.Validating,
		Name: opctrl.FeatureWebhookValidateName,
	},
	{
		Type: rotator.Mutating,
		Name: opctrl.FeatureWebhookMutateName,
	},
}

func whToUnstructured(wh rotator.WebhookInfo) (*unstructured.Unstructured, error) {
	resource := &unstructured.Unstructured{}
	switch wh.Type {
	case rotator.Mutating:
		resource.SetGroupVersionKind(schema.GroupVersionKind{Group: "admissionregistration.k8s.io", Version: "v1", Kind: "MutatingWebhookConfiguration"})
	case rotator.Validating:
		resource.SetGroupVersionKind(schema.GroupVersionKind{Group: "admissionregistration.k8s.io", Version: "v1", Kind: "ValidatingWebhookConfiguration"})
	case rotator.APIService:
		resource.SetGroupVersionKind(schema.GroupVersionKind{Group: "apiregistration.k8s.io", Version: "v1", Kind: "APIService"})
	case rotator.CRDConversion:
		resource.SetGroupVersionKind(schema.GroupVersionKind{Group: "apiextensions.k8s.io", Version: "v1", Kind: "CustomResourceDefinition"})
	default:
		return nil, fmt.Errorf("unknown webhook type: %o", wh.Type)
	}
	resource.SetName(wh.Name)
	return resource, nil
}

func dnsName(ns string) string {
	return fmt.Sprintf("%s.%s.svc", serviceName, ns)
}

func certManagerRunnable(client client.Client, scheme *runtime.Scheme, ns string, isReady chan struct{}) manager.RunnableFunc {
	return func(ctx context.Context) error {
		logger := setupLog.WithName("certs-with-cert-manager")
		logger.Info("Setting up certificate with Cert-Manager")

		err := certApi.AddToScheme(scheme)
		if err != nil {
			return fmt.Errorf("unable to add cert-manager api to scheme: %w", err)
		}

		g, ctx := errgroup.WithContext(context.Background())

		g.Go(func() error {
			logger := logger.WithName("setup")
			logger.Info("Create/Update Issuer")

			// Create the CA Issuer
			issuer := &certApi.Issuer{ObjectMeta: metav1.ObjectMeta{
				Name:      caName,
				Namespace: ns,
			}}

			_, err := ctrl.CreateOrUpdate(ctx, client, issuer, func() error {
				issuer.Spec.SelfSigned = &certApi.SelfSignedIssuer{}
				return nil
			})
			if err != nil {
				logger.Error(err, "Failed to create/update Issuer")
				return err
			}

			logger.Info("Create/Update Certificate")

			// Create a self-signed certificate for the webhook server
			cert := &certApi.Certificate{ObjectMeta: metav1.ObjectMeta{
				Name:      certName,
				Namespace: ns,
			}}
			_, err = ctrl.CreateOrUpdate(ctx, client, cert, func() error {
				cert.Spec.CommonName = dnsName(ns)
				cert.Spec.IssuerRef = cmmeta.ObjectReference{
					Name: caName,
					Kind: certApi.IssuerKind,
				}
				cert.Spec.SecretName = secretName //pragma: allowlist secret
				cert.Spec.DNSNames = []string{
					dnsName(ns),
					fmt.Sprintf("%s.cluster.local", dnsName(ns))}
				return nil
			})
			if err != nil {
				logger.Error(err, "Failed to create/update Certificate")
				return err
			}
			return nil
		})

		// Inject the CA certificate
		injectName := fmt.Sprintf("%s/%s", ns, certName)

		for _, wh := range webhooks {
			wh := wh // https://golang.org/doc/faq#closures_and_goroutines
			g.Go(func() error {
				logger.Info("Injecting certificate", "webhook", wh.Name)

				resource, err := whToUnstructured(wh)
				if err != nil {
					return err
				}

				if err := client.Get(ctx, types.NamespacedName{Name: wh.Name}, resource); err != nil {
					logger.Error(err, "Failed to get resource", "webhook", wh.Name)
					return err
				}

				annots := make(map[string]string)
				for v, k := range resource.GetAnnotations() {
					annots[v] = k
				}
				annots["cert-manager.io/inject-ca-from"] = injectName
				annots["csi.cert-manager.io/certificate-file"] = certFileName

				if !reflect.DeepEqual(resource.GetAnnotations(), annots) {
					resource.SetAnnotations(annots)
					err = client.Update(ctx, resource)
					if err != nil {
						logger.Error(err, "Failed to update webhook", "webhook", wh.Name)
						return err
					}
				}
				return nil
			})
		}

		err = g.Wait()
		if err != nil {
			return fmt.Errorf("failed to create Certificate using Cert-Manager: %w", err)
		}

		logger.Info("Certificate created successfully using Cert-Manager")
		close(isReady)
		return nil
	}
}

func certsWithCertsController(mgr manager.Manager, ns string, certsReady chan struct{}) {
	setupLog.Info("Setting up internal cert rotation")

	err := rotator.AddRotator(mgr, &rotator.CertRotator{
		SecretKey: types.NamespacedName{ //pragma: allowlist secret
			Namespace: ns,
			Name:      secretName,
		},
		CertDir:                certDir,
		CAName:                 caName,
		CAOrganization:         caOrganization,
		DNSName:                dnsName(ns),
		IsReady:                certsReady,
		RestartOnSecretRefresh: true, //pragma: allowlist secret
		Webhooks:               webhooks,
		RequireLeaderElection:  viper.GetBool("leader-elect"),
	})
	OrFail(err, "unable to set up cert rotation")
}

// The certs ensurer help us to ensure in non-leaders that the certificates are ready
type certsEnsurer struct {
	client        client.Client
	isReady       chan struct{}
	upstreamReady chan struct{}
	logger        logr.Logger
}

func (ce *certsEnsurer) NeedLeaderElection() bool {
	return false
}
func (ce *certsEnsurer) Start(ctx context.Context) error {
	// Wait for certs to be ready
	checkFn := func() (bool, error) {
		select {
		case <-ce.upstreamReady:
			return true, nil
		default:
		}
		certFile := certDir + "/" + certFileName
		_, err := os.Stat(certFile)
		if err == nil {
			return true, nil
		}
		return false, nil
	}
	if err := wait.ExponentialBackoff(wait.Backoff{
		Duration: 1 * time.Second,
		Factor:   2,
		Jitter:   1,
		Steps:    10,
	}, checkFn); err != nil {
		ce.logger.Error(err, "max retries for checking certs existence")
		return fmt.Errorf("max retries for checking certs existence: %w", err)
	}
	ce.logger.Info(fmt.Sprintf("Certs are ready in %s", certDir))

	// Wait for CA to be injected
	checkFn = func() (bool, error) {
		select {
		case <-ce.upstreamReady:
			return true, nil
		default:
		}

		for _, wh := range webhooks {
			resource, err := whToUnstructured(wh)
			if err != nil {
				return false, fmt.Errorf("failed to convert webhook to unstructured: %w", err)
			}

			if err := ce.client.Get(ctx, types.NamespacedName{Name: wh.Name}, resource); err != nil {
				ce.logger.Error(err, "Failed to get resource", "webhook", wh.Name)
				return false, fmt.Errorf("failed to get resource %s: %w", wh.Name, err)
			}

			switch wh.Type {
			case rotator.Validating, rotator.Mutating:
				whks, found, err := unstructured.NestedSlice(resource.Object, "webhooks")
				if err != nil || !found {
					return false, fmt.Errorf("failed to get webhooks: %w", err)
				}
				for _, w := range whks {
					whk := w.(map[string]interface{})
					_, found, err := unstructured.NestedString(w.(map[string]interface{}), "clientConfig", "caBundle")
					if err != nil || !found {
						return false, fmt.Errorf("failed to get caBundle for webhook %s(%v): %w", wh.Name, whk["name"], err)
					}
				}
			case rotator.CRDConversion:
				_, found, err := unstructured.NestedString(resource.Object, "spec", "conversion", "webhookClientConfig", "caBundle")
				if err != nil || !found {
					return false, fmt.Errorf("failed to get caBundle from webhook %s: %w", wh.Name, err)
				}
			case rotator.APIService:
				_, found, err := unstructured.NestedString(resource.Object, "spec", "caBundle")
				if err != nil || !found {
					return false, fmt.Errorf("failed to get caBundle from webhook %s: %w", wh.Name, err)
				}
			}
		}
		return true, nil
	}
	if err := wait.ExponentialBackoff(wait.Backoff{
		Duration: 1 * time.Second,
		Factor:   2,
		Jitter:   1,
		Steps:    10,
	}, checkFn); err != nil {
		ce.logger.Error(err, "max retries for checking CA Injection")
		return fmt.Errorf("max retries for checking CA Injection: %w", err)
	}
	ce.logger.Info("CA injected")
	close(ce.isReady)
	return nil
}
