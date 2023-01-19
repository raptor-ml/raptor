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

package api

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	manifests "github.com/raptor-ml/raptor/api/v1alpha1"
	v1 "k8s.io/api/core/v1"
	"net/url"
	"strings"
	"time"
)

type Keys map[string]string

func (k *Keys) String() string {
	vals := url.Values{}
	for k, v := range *k {
		vals.Set(k, v)
	}
	return vals.Encode()
}
func (k *Keys) Encode(fd FeatureDescriptor) (string, error) {
	var ret []string
	for _, key := range fd.Keys {
		val, ok := (*k)[key]
		if !ok {
			return "", fmt.Errorf("missing key %q", key)
		}
		ret = append(ret, val)
	}
	return strings.Join(ret, "."), nil
}

func (k *Keys) Decode(encodedKeys string, fd FeatureDescriptor) error {
	vals := strings.Split(encodedKeys, ".")
	if len(vals) != len(fd.Keys) {
		return fmt.Errorf("expected %d keys, got %d", len(fd.Keys), len(vals))
	}
	for i, key := range fd.Keys {
		(*k)[key] = vals[i]
	}
	return nil
}

// Engine is the main engine of the Core
// It is responsible for the low-level operation for the features against the feature store
type Engine interface {
	FeatureDescriptor(ctx context.Context, selector string) (FeatureDescriptor, error)
	Get(ctx context.Context, selector string, keys Keys) (Value, FeatureDescriptor, error)
	Set(ctx context.Context, FQN string, keys Keys, val any, ts time.Time) error
	Append(ctx context.Context, FQN string, keys Keys, val any, ts time.Time) error
	Incr(ctx context.Context, FQN string, keys Keys, by any, ts time.Time) error
	Update(ctx context.Context, FQN string, keys Keys, val any, ts time.Time) error
}
type FeatureDescriptorGetter func(ctx context.Context, FQN string) (FeatureDescriptor, error)

// Logger is a simple interface that returns a Logr.Logger
type Logger interface {
	Logger() logr.Logger
}

// FeatureManager is managing Feature(s) within Core
// It is responsible for managing features as well as operating on them
type FeatureManager interface {
	BindFeature(in *manifests.Feature) error
	UnbindFeature(FQN string) error
	HasFeature(FQN string) bool
}

// DataSourceManager is managing DataSource(s) within Core
// It is responsible for maintaining the DataSource(s) in an internal store
type DataSourceManager interface {
	BindDataSource(fd DataSource) error
	UnbindDataSource(FQN string) error
	HasDataSource(FQN string) bool
}

// DataSourceGetter is a simple interface that returns a DataSource
type DataSourceGetter interface {
	GetDataSource(FQN string) (DataSource, error)
}

type ParsedProgram struct {
	// Primitive is the primitive that this program is returning
	Primitive PrimitiveType

	// Dependencies is a list of FQNs that this program *might* be depended on
	Dependencies []string
}

type RuntimeManager interface {
	// LoadProgram loads a program into the runtime.
	LoadProgram(env, fqn, program string, packages []string) (*ParsedProgram, error)

	// ExecuteProgram executes a program in the runtime.
	ExecuteProgram(ctx context.Context, env string, fqn string, keys Keys, row map[string]any, ts time.Time, dryRun bool) (value Value, keyz Keys, err error)

	// GetSidecars returns the sidecar containers attached to the current container.
	GetSidecars() []v1.Container

	// GetDefaultEnv returns the default environment for the current container.
	GetDefaultEnv() string
}

// ExtendedManager is an Engine that has a DataSource
type ExtendedManager interface {
	Engine
	RuntimeManager
	DataSourceGetter
}

// ManagerEngine is the business-logic implementation of the Core
type ManagerEngine interface {
	Logger
	FeatureManager
	DataSourceManager
	RuntimeManager
	Engine
}
