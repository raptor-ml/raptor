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
	"github.com/go-logr/logr"
	manifests "github.com/raptor-ml/raptor/api/v1alpha1"
	"time"
)

// Engine is the main engine of the Core
// It is responsible for the low-level operation for the features against the feature store
type Engine interface {
	FeatureDescriptor(ctx context.Context, FQN string) (FeatureDescriptor, error)
	Get(ctx context.Context, FQN string, entityID string) (Value, FeatureDescriptor, error)
	Set(ctx context.Context, FQN string, entityID string, val any, ts time.Time) error
	Append(ctx context.Context, FQN string, entityID string, val any, ts time.Time) error
	Incr(ctx context.Context, FQN string, entityID string, by any, ts time.Time) error
	Update(ctx context.Context, FQN string, entityID string, val any, ts time.Time) error
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

// EngineWithSource is an Engine that has a DataSource
type EngineWithSource interface {
	Engine
	DataSourceGetter
}

// ManagerEngine is the business-logic implementation of the Core
type ManagerEngine interface {
	Logger
	FeatureManager
	DataSourceManager
	Engine
}
