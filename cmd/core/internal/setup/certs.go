package setup

import (
	"context"
	"fmt"
	certApi "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	"github.com/open-policy-agent/cert-controller/pkg/rotator"
	"github.com/spf13/viper"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"net/http"

	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

func Certs(mgr manager.Manager) {
	if viper.GetBool("disable-cert-management") {
		return
	}

	ns, err := getInClusterNamespace()
	OrFail(err, "Failed to discover local namespace. Are you running in a cluster?")

	gvk := certApi.SchemeGroupVersion.WithKind(certApi.IssuerKind)
	_, err = mgr.GetClient().RESTMapper().RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		certsWithCertManager(mgr, ns)
		return
	}

	certsWithCertsController(mgr, ns)
}

const (
	secretName     = "natun-webhook-server-cert" //nolint:gosec
	serviceName    = "natun-webhook-service"
	caName         = "natun-ca"
	caOrganization = "natun"
	vwhName        = "natun-validating-webhook-configuration"
	certDir        = "/tmp/k8s-webhook-server/serving-certs"
)

func dnsName(ns string) string {
	return fmt.Sprintf("%s.%s.svc", serviceName, ns)
}
func certsWithCertManager(mgr manager.Manager, ns string) {
	setupLog.Info("Setting up certificate with cert-manager")

	// Create the CA Issuer
	issuer := &certApi.Issuer{ObjectMeta: metav1.ObjectMeta{
		Name:      caName,
		Namespace: ns,
	}}

	_, err := ctrl.CreateOrUpdate(context.TODO(), mgr.GetClient(), issuer, func() error {
		issuer.Spec.SelfSigned = &certApi.SelfSignedIssuer{}
		return nil
	})
	OrFail(err, "Failed to create issuer")

	// Create a self-signed certificate for the webhook server
	cert := &certApi.Certificate{ObjectMeta: metav1.ObjectMeta{
		Name:      secretName,
		Namespace: ns,
	}}
	_, err = ctrl.CreateOrUpdate(context.TODO(), mgr.GetClient(), cert, func() error {
		cert.Spec.CommonName = dnsName(ns)
		cert.Spec.IssuerRef = cmmeta.ObjectReference{
			Name: caName,
			Kind: certApi.IssuerKind,
		}
		cert.Spec.SecretName = secretName
		cert.Spec.DNSNames = []string{dnsName(ns), fmt.Sprintf("%s.cluster.local", dnsName(ns))}
		return nil
	})
	OrFail(err, "Failed to create certificate")
}

func certsWithCertsController(mgr manager.Manager, ns string) {
	setupLog.Info("Setting up internal cert rotation")

	// Make sure certs are generated and valid if cert rotation is enabled.
	isReady := make(chan struct{})

	err := rotator.AddRotator(mgr, &rotator.CertRotator{
		SecretKey: types.NamespacedName{
			Namespace: ns,
			Name:      secretName,
		},
		CertDir:        certDir,
		CAName:         caName,
		CAOrganization: caOrganization,
		DNSName:        dnsName(ns),
		IsReady:        isReady,
		Webhooks: []rotator.WebhookInfo{{
			Type: rotator.Validating,
			Name: vwhName,
		}},
	})
	OrFail(err, "unable to set up cert rotation")

	healthChecks = append(healthChecks, func(_ *http.Request) error {
		select {
		case <-isReady:
			return nil
		default:
			return fmt.Errorf("cert rotation not ready")
		}
	})
}
