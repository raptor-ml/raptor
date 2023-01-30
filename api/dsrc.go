/*
 * Copyright (c) 2022 RaptorML authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package api

import (
	"context"
	"fmt"
	manifests "github.com/raptor-ml/raptor/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DataSource is a parsed abstracted representation of a manifests.DataSource
type DataSource struct {
	FQN    string                 `json:"fqn"`
	Kind   string                 `json:"kind"`
	Config manifests.ParsedConfig `json:"config"`
	// todo Schema
}

// DataSourceFromManifest returns a DataSource from a manifests.DataSource
func DataSourceFromManifest(ctx context.Context, src *manifests.DataSource, r client.Reader) (DataSource, error) {
	pc, err := src.ParseConfig(ctx, r)
	if err != nil {
		return DataSource{}, fmt.Errorf("failed to parse config: %w", err)
	}

	return DataSource{
		FQN:    src.FQN(),
		Kind:   src.Spec.Kind,
		Config: pc,
	}, nil
}
