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
	"net/http"
	"os"
	"path"
	"reflect"
	"time"

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
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

func Certs(mgr manager.Manager, certsReady chan struct{}) {
	healthChecks = append(healthChecks, func(_ *http.Request) error {
		select {
		case <-certsReady:
			return nil
		default:
			return fmt.Errorf("cert rotation not ready")
		}
	})
	if viper.GetBool("disable-cert-management") || viper.GetBool("no-webhooks") {
		close(certsReady)
		return
	}

	upstreamReady := make(chan struct{})
	err := mgr.Add(&certsEnsurer{
		isReady:       certsReady,
		upstreamReady: upstreamReady,
		logger:        mgr.GetLogger().WithName("certs-ensurer"),
	})
	OrFail(err, "unable to set up cert ensurer")

	ns, err := getInClusterNamespace()
	OrFail(err, "Failed to discover local namespace. Are you running in a cluster?")

	gvk := certApi.SchemeGroupVersion.WithKind(certApi.IssuerKind)
	_, err = mgr.GetClient().RESTMapper().RESTMapping(gvk.GroupKind(), gvk.Version)
	if err == nil {
		setupLog.Info("Cert-manager detected")
		err := mgr.Add(certManagerRunnable(mgr.GetClient(), mgr.GetScheme(), ns, upstreamReady))
		OrFail(err, "Failed to add cert-manager runnable")
		return
	}

	certsWithCertsController(mgr, ns, upstreamReady)
}

const (
	secretName     = "raptor-webhook-server-cert" //nolint:gosec
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

func dnsNames(ns string) []string {
	const suffix = ".cluster.local"
	localCluster := fmt.Sprintf(serviceName+".%s.svc"+suffix, ns)
	commonName := localCluster[:len(localCluster)-len(suffix)]
	return []string{commonName, localCluster}
}

func certManagerRunnable(client client.Client, scheme *runtime.Scheme, ns string, isReady chan struct{}) manager.RunnableFunc {
	return func(ctx context.Context) error {
		logger := setupLog.WithName("certs-with-cert-manager")
		logger.Info("Setting up certificate with Cert-Manager")

		err := certApi.AddToScheme(scheme)
		if err != nil {
			return fmt.Errorf("unable to add cert-manager api to scheme: %w", err)
		}

		g, ctx := errgroup.WithContext(ctx)

		g.Go(func() error {
			logger := logger.WithName("setup")
			logger.Info("Create/Update Issuer")

			// Create the CA Issuer
			issuer := &certApi.Issuer{ObjectMeta: metav1.ObjectMeta{
				Name:      caName,
				Namespace: ns,
			}}

			_, err := ctrl.CreateOrUpdate(ctx, client, issuer, func() error {
				issuer.Spec.SelfSigned = new(certApi.SelfSignedIssuer)
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
				cert.Spec.DNSNames = dnsNames(ns)
				cert.Spec.CommonName = cert.Spec.DNSNames[0]
				cert.Spec.IssuerRef = cmmeta.ObjectReference{
					Name: caName,
					Kind: certApi.IssuerKind,
				}
				cert.Spec.SecretName = secretName
				return nil
			})
			if err != nil {
				logger.Error(err, "Failed to create/update Certificate")
				return err
			}
			return nil
		})

		// Inject the CA certificate
		injectName := path.Join(ns, certName)

		for _, wh := range webhooks {
			wh := wh // https://golang.org/doc/faq#closures_and_goroutines
			g.Go(func() error {
				logger.Info("Injecting certificate", "webhook", wh.Name)

				resource := new(unstructured.Unstructured)
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
					return fmt.Errorf("unknown webhook type: %o", wh.Type)
				}

				err := client.Get(ctx, types.NamespacedName{Name: wh.Name}, resource)
				if err != nil {
					logger.Error(err, "Failed to get resource", "webhook", wh.Name)
					return err
				}

				annotsOriginal := resource.GetAnnotations()
				annots := make(map[string]string, len(annotsOriginal))
				for v, k := range annotsOriginal {
					annots[v] = k
				}
				annots["cert-manager.io/inject-ca-from"] = injectName
				annots["csi.cert-manager.io/certificate-file"] = certFileName

				if !reflect.DeepEqual(annotsOriginal, annots) {
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
		SecretKey: types.NamespacedName{
			Namespace: ns,
			Name:      secretName,
		},
		CertDir:                certDir,
		CAName:                 caName,
		CAOrganization:         caOrganization,
		DNSName:                dnsNames(ns)[0],
		IsReady:                certsReady,
		RestartOnSecretRefresh: true,
		Webhooks:               webhooks,
	})
	OrFail(err, "unable to set up cert rotation")
}

type certsEnsurer struct {
	isReady       chan struct{}
	upstreamReady chan struct{}
	logger        logr.Logger
}

func (ce *certsEnsurer) NeedLeaderElection() bool {
	return false
}

func (ce *certsEnsurer) Start(_ context.Context) error {
	checkFn := func() (bool, error) {
		select {
		case <-ce.upstreamReady:
			return true, nil
		default:
		}
		certFile := path.Join(certDir, certFileName)
		_, err := os.Stat(certFile)
		if err == nil {
			return true, nil
		}
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err // in case of real error like permissions
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
	ce.logger.Info(fmt.Sprintf("certs are ready in %s", certDir))
	close(ce.isReady)
	return nil
}
