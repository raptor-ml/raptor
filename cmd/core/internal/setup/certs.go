/*
Copyright 2022 Natun.

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

import (
	"context"
	"fmt"
	certApi "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	"github.com/go-logr/logr"
	natunApi "github.com/natun-ai/natun/api/v1alpha1"
	"github.com/open-policy-agent/cert-controller/pkg/rotator"
	"github.com/spf13/viper"
	"golang.org/x/sync/errgroup"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"net/http"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"time"
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
	if viper.GetBool("disable-cert-management") {
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
		err := mgr.Add(certManagerRunnable(mgr.GetClient(), ns, upstreamReady))
		OrFail(err, "Failed to add cert-manager runnable")
		return
	}

	certsWithCertsController(mgr, ns, upstreamReady)
}

const (
	secretName     = "natun-webhook-server-cert" //nolint:gosec
	certName       = "natun-serving-cert"
	serviceName    = "natun-webhook-service"
	caName         = "natun-ca"
	caOrganization = "natun"
	vwhName        = "natun-validating-webhook-configuration"
	certDir        = "/tmp/k8s-webhook-server/serving-certs"
	certFileName   = "tls.crt"
)

func dnsName(ns string) string {
	return fmt.Sprintf("%s.%s.svc", serviceName, ns)
}

// +kubebuilder:rbac:groups=cert-manager.io,resources=issuers;certificates,namespace=natun-system,verbs=get;create;update;patch;delete
// +kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=get;update;patch
// +kubebuilder:rbac:groups=admissionregistration.k8s.io,resources=validatingwebhookconfigurations,verbs=get;update;patch;list
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;create;update;patch;list;watch,namespace=natun-system
func certManagerRunnable(client client.Client, ns string, isReady chan struct{}) manager.RunnableFunc {
	return func(ctx context.Context) error {
		logger := setupLog.WithName("certs-with-cert-manager")
		logger.Info("Setting up certificate with Cert-Manager")

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
				cert.Spec.SecretName = secretName
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
		g.Go(func() error {
			logger := logger.WithName("inject-crd")
			logger.Info("Injecting certificate to the CRD")

			crd := apiextensionsv1.CustomResourceDefinition{}
			gk := natunApi.GroupVersion.WithKind("Feature").GroupKind()
			err := client.Get(ctx, types.NamespacedName{Name: gk.String()}, &crd)
			if err != nil {
				logger.Error(err, "Failed to get CRD")
				return err
			}
			if v, ok := crd.Annotations["cert-manager.io/inject-ca-from"]; !ok || v != injectName {
				crd.Annotations["cert-manager.io/inject-ca-from"] = injectName
				crd.Annotations["csi.cert-manager.io/certificate-file"] = injectName
				err = client.Update(ctx, &crd)
				if err != nil {
					logger.Error(err, "Failed to update CRD")
					return err
				}
			}
			return nil
		})

		g.Go(func() error {
			logger := logger.WithName("inject-webhook-conf")
			logger.Info("Injecting certificate to the ValidatingWebhookConfiguration")

			vwc := admissionregistrationv1.ValidatingWebhookConfiguration{}
			err := client.Get(ctx, types.NamespacedName{Name: vwhName, Namespace: ns}, &vwc)
			if err != nil {
				logger.Error(err, "Failed to get validating webhook configuration")
				return err
			}
			if v, ok := vwc.Annotations["cert-manager.io/inject-ca-from"]; !ok || v != injectName {
				vwc.Annotations["cert-manager.io/inject-ca-from"] = injectName
				err = client.Update(ctx, &vwc)
				if err != nil {
					logger.Error(err, "Failed to update ValidatingWebhookConfiguration")
					return err
				}
			}
			return nil
		})

		err := g.Wait()
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
		DNSName:                dnsName(ns),
		IsReady:                certsReady,
		RestartOnSecretRefresh: true,
		Webhooks: []rotator.WebhookInfo{{
			Type: rotator.Validating,
			Name: vwhName,
		}},
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
func (ce *certsEnsurer) Start(ctx context.Context) error {
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
	ce.logger.Info(fmt.Sprintf("certs are ready in %s", certDir))
	close(ce.isReady)
	return nil
}
