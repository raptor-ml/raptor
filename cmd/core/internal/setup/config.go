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

package setup

import (
	"flag"
	"github.com/raptor-ml/raptor/pkg/plugins"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"strings"
)

var updatesAllowed = false

func InitConfig() {
	pflag.Bool("leader-elect", false, "Enable leader election for controller manager."+
		"Enabling this will ensure there is only one active controller manager.")
	pflag.String("metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	pflag.String("health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	pflag.StringArray("watch-namespaces", []string{}, "Enable namespace-level only by specify a list of"+
		"namespaces that the operator is watching. If not specify, the operator will run on cluster level.")
	pflag.String("accessor-grpc-address", ":60000", "The address the grpc accessor binds to.")
	pflag.String("accessor-http-address", ":60001", "The address the http accessor binds to.")
	pflag.String("accessor-http-prefix", "/api", "The the http accessor path prefix.")
	pflag.String("accessor-service", "", "The the accessor service URL (that points the this application).")
	pflag.Bool("production", true, "Set as production")
	pflag.Bool("usage-reporting", true, "Allow us to anonymously report usage statistics to improve Raptor 🪄")
	pflag.String("usage-reporting-uid", "", "Usage reporting Unique Identifier. "+
		"You can use this to set a unique identifier for your cluster.")
	pflag.String("state-provider", "redis", "The state provider.")
	pflag.String("notifier-provider", "redis", "The notifier provider.")
	pflag.Bool("disable-cert-management", false, "Setting this flag will disable the automatically "+
		"certificate binding to the K8s API webhooks.")
	pflag.Bool("no-webhooks", false, "Setting this flag will disable the K8s API webhook.")

	zapOpts := zap.Options{}
	zapOpts.BindFlags(flag.CommandLine)
	OrFail(plugins.BindConfig(pflag.CommandLine), "Failed to bind plugins' config")

	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Parse()
	OrFail(viper.BindPFlags(pflag.CommandLine), "Failed to bind flags")

	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_", ".", "_"))
	viper.AutomaticEnv()

	zapOpts.Development = !viper.GetBool("production")
	logger := zap.New(zap.UseFlagOptions(&zapOpts))
	ctrl.SetLogger(logger)

	updatesAllowed = !viper.GetBool("production")
}
