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

import (
	"fmt"
	"github.com/raptor-ml/raptor/api"
	"github.com/raptor-ml/raptor/internal/accessor"
	"github.com/raptor-ml/raptor/internal/engine"
	corectrl "github.com/raptor-ml/raptor/internal/engine/controllers"
	"github.com/raptor-ml/raptor/internal/historian"
	opctrl "github.com/raptor-ml/raptor/internal/operator"
	"github.com/raptor-ml/raptor/internal/stats"
	"github.com/raptor-ml/raptor/pkg/plugins"
	"github.com/spf13/viper"
	"net/http"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

func setupStats(mgr manager.Manager) {
	// Setup usage reports
	stats.UID = viper.GetString("usage-reporting-uid")
	OrFail(mgr.Add(stats.Run(
		mgr.GetConfig(),
		mgr.GetClient(),
		viper.GetBool("usage-reporting"),
		ctrl.Log.WithName("stats"),
	)), "unable to add stats")
}

func historianClient(mgr manager.Manager) historian.Client {
	// Create Notifiers
	collectNotifier, err := plugins.NewCollectNotifier(viper.GetString("notifier-provider"), viper.GetViper())
	OrFail(err, "failed to create collect notifier")
	writeNotifier, err := plugins.NewWriteNotifier(viper.GetString("notifier-provider"), viper.GetViper())
	OrFail(err, "failed to create collect notifier")

	// Create a Historian Client
	hsc := historian.NewClient(historian.ClientConfig{
		CollectNotifier:            collectNotifier,
		WriteNotifier:              writeNotifier,
		Logger:                     ctrl.Log.WithName("historian"),
		CollectNotificationWorkers: 5,
		WriteNotificationWorkers:   5,
	})
	OrFail(hsc.WithManager(mgr), "failed to create historian client")

	return hsc
}

func coreControllers(mgr manager.Manager, eng api.ManagerEngine) {
	var err error

	// Setup Core Controllers
	err = (&corectrl.DataConnectorReconciler{
		Reader:        mgr.GetClient(),
		Scheme:        mgr.GetScheme(),
		EngineManager: eng,
	}).SetupWithManager(mgr)
	OrFail(err, "unable to create core controller", "controller", "DataConnector")

	err = (&corectrl.FeatureReconciler{
		Reader:         mgr.GetClient(),
		Scheme:         mgr.GetScheme(),
		UpdatesAllowed: updatesAllowed,
		EngineManager:  eng,
	}).SetupWithManager(mgr)
	OrFail(err, "unable to create core controller", "controller", "Feature")

	err = (&corectrl.FeatureSetReconciler{
		Reader:        mgr.GetClient(),
		Scheme:        mgr.GetScheme(),
		EngineManager: eng,
	}).SetupWithManager(mgr)
	OrFail(err, "unable to create core controller", "controller", "FeatureSet")
}

func operatorControllers(mgr manager.Manager) {
	var err error

	coreAddr := viper.GetString("accessor-service")
	if coreAddr == "" {
		ns, err := getInClusterNamespace()
		OrFail(err, "unable to get in-cluster namespace. Please set the accessor-service flag")
		coreAddr = fmt.Sprintf("raptor-core-service.%s.svc", ns)
	}

	err = (&opctrl.DataConnectorReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		CoreAddr: coreAddr,
	}).SetupWithManager(mgr)
	OrFail(err, "unable to create controller", "operator", "DataConnector")

	err = (&opctrl.FeatureSetReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr)
	OrFail(err, "unable to create controller", "operator", "FeatureSet")

	err = (&opctrl.FeatureReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr)
	OrFail(err, "unable to create controller", "operator", "Feature")

	if !viper.GetBool("no-webhooks") {
		opctrl.SetupFeatureWebhook(mgr, updatesAllowed)
	}
}

func Core(mgr manager.Manager, certsReady chan struct{}) {
	// Setup usage reporting
	setupStats(mgr)

	// Create a Historian Client
	hsc := historianClient(mgr)

	// Create the state
	state, err := plugins.NewState(viper.GetString("state-provider"), viper.GetViper())
	OrFail(err, fmt.Sprintf("failed to create state for provider %s", viper.GetString("state-provider")))

	err = mgr.AddHealthzCheck("state", func(req *http.Request) error {
		return state.Ping(req.Context())
	})
	OrFail(err, "unable to add health check for state")
	err = mgr.AddReadyzCheck("state", func(req *http.Request) error {
		return state.Ping(req.Context())
	})
	OrFail(err, "unable to add ready check for state")

	// Create a new Core engine
	eng := engine.New(state, hsc, ctrl.Log.WithName("engine"))

	// Create a new Accessor
	acc := accessor.New(eng, ctrl.Log.WithName("accessor"))
	OrFail(mgr.Add(acc.GRPC(viper.GetString("accessor-grpc-address"))), "unable to start gRPC accessor")
	OrFail(mgr.Add(acc.GrpcUds()), "unable to start gRPC UDS accessor")
	OrFail(
		mgr.Add(acc.HTTP(viper.GetString("accessor-http-address"), viper.GetString("accessor-http-prefix"))),
		"unable to start HTTP accessor")

	// The call to mgr.Start will never return, but the certs won't be ready until the manager starts
	// and we can't set up the webhooks without them (the webhook server runnable will try to read the
	// certs, and if those certs don't exist, the entire process will exit). So start a goroutine
	// which will wait until the certs are ready, and then create the rest of the HNC controllers.
	go func() {
		// The controllers won't work until the webhooks are operating, and those won't work until the
		// certs are all in place.
		setupLog.Info("Waiting for certificate generation to complete")
		<-certsReady

		setupLog.Info("Certs ready")

		coreControllers(mgr, eng)
		operatorControllers(mgr)
	}()
}
