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

package streaming

import (
	"fmt"

	"github.com/raptor-ml/raptor/api"
	manifests "github.com/raptor-ml/raptor/api/v1alpha1"
	"github.com/raptor-ml/raptor/pkg/plugins"
	"github.com/raptor-ml/raptor/pkg/pyexp"
	"github.com/raptor-ml/raptor/pkg/runner"
)

// These variables are being overwritten by the build process
var (
	Image      = "ghcr.io/raptor-ml/streaming-runner:latest"
	runtimeVer = "latest"
)

const name = "streaming"

func init() {
	baseRunner := runner.BaseRunner{
		Image:          Image,
		RuntimeVersion: runtimeVer,
		Command:        []string{"./runner"},
	}
	reconciler, err := baseRunner.Reconciler()
	if err != nil {
		panic(err)
	}

	// Register the plugin
	plugins.DataConnectorReconciler.Register(name, reconciler)
	plugins.FeatureAppliers.Register(name, FeatureApply)
}

func FeatureApply(md api.Metadata, builder manifests.FeatureBuilder, api api.FeatureAbstractAPI, engine api.EngineWithConnector) error {
	if md.DataConnector == "" {
		return fmt.Errorf("data connector must be set for `%s` builder", name)
	}

	dc, err := engine.GetDataConnector(md.DataConnector)
	if err != nil {
		return fmt.Errorf("failed to get data connector: %w", err)
	}

	if dc.Kind != name {
		return fmt.Errorf("data connector must be of type `%s`. got `%s`", name, dc.Kind)
	}

	if builder.PyExp == "" {
		return fmt.Errorf("expression is empty")
	}

	// make sure the expression is valid
	_, err = pyexp.New(builder.PyExp, md.FQN)
	if err != nil {
		return fmt.Errorf("failed to create expression runtime: %w", err)
	}

	return nil
}
