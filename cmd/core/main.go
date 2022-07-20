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

package main

import (
	"github.com/raptor-ml/raptor/cmd/core/internal/setup"
	"github.com/raptor-ml/raptor/internal/version"
	"github.com/spf13/viper"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"sigs.k8s.io/controller-runtime/pkg/cache"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	_ "github.com/raptor-ml/raptor/internal/plugins"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"

	raptorApi "github.com/raptor-ml/raptor/api/v1alpha1"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(raptorApi.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	setup.InitConfig()
	setupLog.WithValues("version", version.Version).Info("Initializing Core...")

	// Set up a Manager
	mgrOpts := ctrl.Options{
		Scheme:                        scheme,
		MetricsBindAddress:            viper.GetString("metrics-bind-address"),
		Port:                          9443,
		HealthProbeBindAddress:        viper.GetString("health-probe-bind-address"),
		LeaderElection:                viper.GetBool("leader-elect"),
		LeaderElectionResourceLock:    resourcelock.LeasesResourceLock,
		LeaderElectionID:              "core.raptor.ml",
		LeaderElectionReleaseOnCancel: true,
	}
	{
		nss := viper.GetStringSlice("watch-namespaces")
		if len(nss) == 1 {
			mgrOpts.Namespace = nss[0]
		} else if len(nss) > 1 {
			setupLog.Info("manager set up with multiple namespaces", "namespaces", nss)
			// configure cluster-scoped with MultiNamespacedCacheBuilder
			mgrOpts.NewCache = cache.MultiNamespacedCacheBuilder(nss)
		}
	}
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), mgrOpts)
	setup.OrFail(err, "unable to start manager")

	// Set Up certificates for the webhooks
	certsReady := make(chan struct{})
	setup.Certs(mgr, certsReady)

	// Set up the Core
	setup.Core(mgr, certsReady)

	// +kubebuilder:scaffold:builder

	err = mgr.AddHealthzCheck("healthz", setup.HealthCheck)
	setup.OrFail(err, "unable to set up health check")

	// Currently, this is being solved by configuring a `initialDelaySeconds` for the probe
	err = mgr.AddReadyzCheck("readyz", setup.HealthCheck)
	setup.OrFail(err, "unable to set up ready check")

	setupLog.Info("starting manager")
	err = mgr.Start(ctrl.SetupSignalHandler())
	setup.OrFail(err, "problem running manager")
}
