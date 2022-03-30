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
	"flag"
	"fmt"
	"github.com/natun-ai/natun/internal/historian"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"net/http"
	"os"
	"strings"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/natun-ai/natun/internal/accessor"
	"github.com/natun-ai/natun/internal/engine"
	"github.com/natun-ai/natun/internal/plugin"
	_ "github.com/natun-ai/natun/internal/plugins"

	corectrl "github.com/natun-ai/natun/internal/engine/controllers"
	opctrl "github.com/natun-ai/natun/internal/operator"
	natunApi "github.com/natun-ai/natun/pkg/api/v1alpha1"
	//+kubebuilder:scaffold:imports
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
	pflag.String("accessor-grpc-address", ":9090", "The address the grpc accessor binds to.")
	pflag.String("accessor-http-address", ":9091", "The address the http accessor binds to.")
	pflag.String("accessor-http-prefix", "/api", "The the http accessor path prefix.")
	pflag.Bool("production", true, "Set as production")

	pflag.String("state-provider", "redis", "The state provider.")
	pflag.String("notifier-provider", "redis", "The notifier provider.")

	zapOpts := zap.Options{}
	zapOpts.BindFlags(flag.CommandLine)
	utilruntime.Must(plugin.BindConfig(pflag.CommandLine))

	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Parse()
	utilruntime.Must(viper.BindPFlags(pflag.CommandLine))

	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv()

	zapOpts.Development = !viper.GetBool("production")
	logger := zap.New(zap.UseFlagOptions(&zapOpts))
	ctrl.SetLogger(logger)

	// Set up a Manager
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     viper.GetString("metrics-bind-address"),
		Port:                   9443,
		HealthProbeBindAddress: viper.GetString("health-probe-bind-address"),
		LeaderElection:         viper.GetBool("leader-elect"),
		LeaderElectionID:       "c2eb2c1b.natun.ai",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// Create the state
	state, err := plugin.NewState(viper.GetString("state-provider"), viper.GetViper())
	orFail(err, fmt.Sprintf("failed to create state for provider %s", viper.GetString("provider")))

	// Create a Notifier
	notifier, err := plugin.NewNotifier(viper.GetString("notifier-provider"), viper.GetViper())
	orFail(err, "failed to create notifier")

	// Create an Historian
	historian := historian.New(historian.Config{
		Notifier:                   notifier,
		State:                      state,
		Logger:                     logger.WithName("historian"),
		CollectNotificationWorkers: 5,
		CollectWorkers:             5,
		WriteNotificationWorkers:   5,
	})

	// Create a new Core engine
	eng := engine.New(state, historian, logger.WithName("engine"))

	// Create a new Accessor
	acc := accessor.New(eng)
	orFail(mgr.Add(acc.Grpc(viper.GetString("accessor-grpc-address"))), "unable to start gRPC accessor")
	orFail(mgr.Add(acc.Http(viper.GetString("accessor-http-address"), viper.GetString("accessor-http-prefix"))), "unable to start HTTP accessor")

	// Setup Operator Controllers
	if err = (&opctrl.FeatureReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Feature")
		os.Exit(1)
	}
	if err = (&opctrl.DataConnectorReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "DataConnector")
		os.Exit(1)
	}

	// Setup Core Controllers
	if err = (&corectrl.FeatureReconciler{
		Reader:         mgr.GetClient(),
		Scheme:         mgr.GetScheme(),
		UpdatesAllowed: !viper.GetBool("production"),
		Engine:         eng,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "DataConnector")
		os.Exit(1)
	}
	//+kubebuilder:scaffold:builder

	health := func(r *http.Request) error {
		return state.Ping(r.Context())
	}

	if err := mgr.AddHealthzCheck("healthz", health); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}

	// TODO READINESS PROBE
	//Currently this is being solved by configuring a `initialDelaySeconds` for the probe
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
