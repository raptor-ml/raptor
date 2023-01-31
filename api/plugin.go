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
	manifests "github.com/raptor-ml/raptor/api/v1alpha1"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Plugins interface {
	BindConfig | FeatureApply | DataSourceReconcile | StateFactory |
		CollectNotifierFactory | WriteNotifierFactory |
		HistoricalWriterFactory
}

// BindConfig adds config flags for the plugin.
type BindConfig func(set *pflag.FlagSet) error

// FeatureApply applies changes on the feature abstraction.
type FeatureApply func(fd FeatureDescriptor, builder manifests.FeatureBuilder, pl Pipeliner, src ExtendedManager) error

// DataSourceReconcileRequest contains metadata for the reconcile.
type DataSourceReconcileRequest struct {
	DataSource     *manifests.DataSource
	RuntimeManager RuntimeManager
	Client         client.Client
	Scheme         *runtime.Scheme
	CoreAddress    string
}

// DataSourceReconcile is the interface to be implemented by plugins that want to be reconciled in the operator.
// This is useful for plugins that need to spawn an external Feature Ingestion.
//
// It returns ture if the reconciliation has changed the object (and therefore the operator should re-queue).
type DataSourceReconcile func(ctx context.Context, rr DataSourceReconcileRequest) (changed bool, err error)

// ModelReconcileRequest contains metadata for the reconcile.
type ModelReconcileRequest struct {
	Model  *manifests.Model
	Client client.Client
	Scheme *runtime.Scheme
}

// ModelServer is the interface to be implemented by plugins that implements a Model Server.
type ModelServer interface {
	Reconcile(ctx context.Context, rr ModelReconcileRequest) (changed bool, err error)
	Owns() []client.Object
	Serve(ctx context.Context, fd FeatureDescriptor, md ModelDescriptor, val Value) (Value, error)
}

// StateFactory is the interface to be implemented by plugins that implements storage providers.
type StateFactory func(viper *viper.Viper) (State, error)

// NotifierFactory is the interface to be implemented by plugins that implements Notifier.
type NotifierFactory[T Notification] func(viper *viper.Viper) (Notifier[T], error)
type CollectNotifierFactory NotifierFactory[CollectNotification]
type WriteNotifierFactory NotifierFactory[WriteNotification]

type HistoricalWriterFactory func(viper *viper.Viper) (HistoricalWriter, error)
