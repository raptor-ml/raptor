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

package engine

import (
	"fmt"
	"github.com/natun-ai/natun/internal/stats"
	"github.com/natun-ai/natun/pkg/api"
	manifests "github.com/natun-ai/natun/pkg/api/v1alpha1"
	"github.com/natun-ai/natun/pkg/plugin"
)

// BindFeature converts the k8s Feature CRD to the internal implementation, and adds it to the engine.
func (e *engine) BindFeature(in *manifests.Feature) error {
	md, err := api.MetadataFromManifest(in)
	if err != nil {
		return fmt.Errorf("failed to parse metadata from CR: %w", err)
	}

	ft := Feature{
		Metadata: *md,
	}

	if p := plugin.FeatureAppliers.Get(ft.Builder); p != nil {
		err := p(ft.Metadata, in.Spec.Builder.Raw, &ft, e)
		if err != nil {
			return err
		}
	}

	return e.bindFeature(ft)
}

func (e *engine) UnbindFeature(fqn string) error {
	defer stats.DecNumberOfFeatures()
	e.features.Delete(fqn)
	e.logger.Info("feature unbound", "feature", fqn)
	return nil
}

func (e *engine) bindFeature(f Feature) error {
	defer stats.IncNumberOfFeatures()
	if e.HasFeature(f.FQN) {
		return fmt.Errorf("%w: %s", api.ErrFeatureAlreadyExists, f.FQN)
	}
	e.features.Store(f.FQN, f)
	e.logger.Info("feature bound", "FQN", f.FQN)
	return nil
}

func (e *engine) HasFeature(fqn string) bool {
	_, ok := e.features.Load(fqn)
	return ok
}
