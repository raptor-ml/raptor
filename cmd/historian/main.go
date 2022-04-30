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

	"github.com/natun-ai/natun/internal/historian"
	"github.com/natun-ai/natun/internal/version"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	_ "github.com/natun-ai/natun/internal/plugins"
	"github.com/natun-ai/natun/pkg/plugins"

	natunApi "github.com/natun-ai/natun/api/v1alpha1"
	corectrl "github.com/natun-ai/natun/internal/engine/controllers"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(natunApi.AddToScheme(scheme))
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
	pflag.String("historical-writer-provider", "parquet-aws", "The historical writer provider.")

	zapOpts := zap.Options{}
	zapOpts.BindFlags(flag.CommandLine)
	utilruntime.Must(plugins.BindConfig(pflag.CommandLine))

	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Parse()
	utilruntime.Must(viper.BindPFlags(pflag.CommandLine))

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
		LeaderElectionID:              "historian.natun.ai",
		LeaderElectionReleaseOnCancel: true,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

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
		CollectWorkers:   5,
	})
	orFail(hss.WithManager(mgr), "failed to create historian client")

	// Setup Core Controllers
	if err = (&corectrl.FeatureReconciler{
		Reader:         mgr.GetClient(),
		Scheme:         mgr.GetScheme(),
		UpdatesAllowed: !viper.GetBool("production"),
		EngineManager:  hss,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "DataConnector")
		os.Exit(1)
	}

	health := func(r *http.Request) error {
		return state.Ping(r.Context())
	}

	if err := mgr.AddHealthzCheck("healthz", health); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}

	// Currently, this is being solved by configuring a `initialDelaySeconds` for the probe
	if err := mgr.AddReadyzCheck("readyz", health); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func orFail(e error, message string) {
	if e != nil {
		setupLog.Error(e, message)
		os.Exit(1)
	}
}
