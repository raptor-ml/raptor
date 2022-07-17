/*
Copyright (c) 2022 Raptor.

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
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"k8s.io/client-go/tools/leaderelection/resourcelock"

	"github.com/raptor-ml/raptor/internal/historian"
	"github.com/raptor-ml/raptor/internal/version"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	_ "github.com/raptor-ml/raptor/internal/plugins"
	"github.com/raptor-ml/raptor/pkg/plugins"

	raptorApi "github.com/raptor-ml/raptor/api/v1alpha1"
	corectrl "github.com/raptor-ml/raptor/internal/engine/controllers"
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
	pflag.Bool("leader-elect", false, "Enable leader election for controller manager."+
		"Enabling this will ensure there is only one active controller manager.")
	pflag.String("metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	pflag.String("health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	pflag.Bool("production", true, "Set as production")

	pflag.String("state-provider", "redis", "The state provider.")
	pflag.String("notifier-provider", "redis", "The notifier provider.")
	pflag.String("historical-writer-provider", "s3-parquet", "The historical writer provider.")

	zapOpts := zap.Options{}
	zapOpts.BindFlags(flag.CommandLine)
	orFail(plugins.BindConfig(pflag.CommandLine), "failed to bind plugins' config")

	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Parse()
	orFail(viper.BindPFlags(pflag.CommandLine), "failed to bind flags")

	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_", ".", "_"))
	viper.AutomaticEnv()

	zapOpts.Development = !viper.GetBool("production")
	logger := zap.New(zap.UseFlagOptions(&zapOpts))
	ctrl.SetLogger(logger)

	setupLog.WithValues("version", version.Version).Info("Initializing Historian...")

	// Set up a Manager
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                        scheme,
		MetricsBindAddress:            viper.GetString("metrics-bind-address"),
		Port:                          9443,
		HealthProbeBindAddress:        viper.GetString("health-probe-bind-address"),
		LeaderElection:                viper.GetBool("leader-elect"),
		LeaderElectionResourceLock:    resourcelock.LeasesResourceLock,
		LeaderElectionID:              "historian.raptor.ml",
		LeaderElectionReleaseOnCancel: true,
	})
	orFail(err, "unable to start manager")

	// Create the state
	state, err := plugins.NewState(viper.GetString("state-provider"), viper.GetViper())
	orFail(err, fmt.Sprintf("failed to create state for provider %s", viper.GetString("provider")))

	// Create Notifiers
	collectNotifier, err := plugins.NewCollectNotifier(viper.GetString("notifier-provider"), viper.GetViper())
	orFail(err, "failed to create collect notifier")
	writeNotifier, err := plugins.NewWriteNotifier(viper.GetString("notifier-provider"), viper.GetViper())
	orFail(err, "failed to create collect notifier")

	// Historical Writer
	historicalWriter, err := plugins.NewHistoricalWriter(viper.GetString("historical-writer-provider"), viper.GetViper())
	orFail(err, "failed to create historical writer")
	defer historicalWriter.Close(context.TODO())

	// Create a Historian Client
	hss := historian.NewServer(historian.ServerConfig{
		CollectNotifier:  collectNotifier,
		WriteNotifier:    writeNotifier,
		State:            state,
		Logger:           logger.WithName("historian"),
		HistoricalWriter: historicalWriter,
	})
	orFail(hss.WithManager(mgr), "failed to create historian client")

	// Setup Core Controllers
	err = (&corectrl.FeatureReconciler{
		Reader:         mgr.GetClient(),
		Scheme:         mgr.GetScheme(),
		UpdatesAllowed: !viper.GetBool("production"),
		EngineManager:  hss,
	}).SetupWithManager(mgr)
	orFail(err, "failed to setup feature contoller")

	err = (&corectrl.FeatureReconciler{
		Reader:         mgr.GetClient(),
		Scheme:         mgr.GetScheme(),
		UpdatesAllowed: !viper.GetBool("production"),
		EngineManager:  hss,
	}).SetupWithManager(mgr)
	orFail(err, "unable to create core controller", "controller", "Feature")

	err = (&corectrl.FeatureSetReconciler{
		Reader:        mgr.GetClient(),
		Scheme:        mgr.GetScheme(),
		EngineManager: hss,
	}).SetupWithManager(mgr)
	orFail(err, "unable to create core controller", "controller", "FeatureSet")

	health := func(r *http.Request) error {
		return state.Ping(r.Context())
	}

	err = mgr.AddHealthzCheck("healthz", health)
	orFail(err, "failed to set up health check")

	// Currently, this is being solved by configuring a `initialDelaySeconds` for the probe
	err = mgr.AddReadyzCheck("readyz", health)
	orFail(err, "failed to set up ready check")

	setupLog.Info("starting manager")
	err = mgr.Start(ctrl.SetupSignalHandler())
	orFail(err, "problem starting manager")
}

func orFail(err error, message string, keyAndValues ...any) {
	if err != nil {
		if setupLog.GetSink() == nil {
			_, _ = fmt.Fprint(os.Stderr, append([]any{"error", err, "message", message}, keyAndValues...)...)
		} else {
			setupLog.Error(err, message, keyAndValues...)
		}
		os.Exit(1)
	}
}
