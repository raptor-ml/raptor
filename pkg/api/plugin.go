package api

import (
	"context"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Plugins interface {
	BindConfig | FeatureApply | Reconcile | StateFactory | NotifierFactory
}

// BindConfig adds config flags for the plugin.
type BindConfig func(set *pflag.FlagSet) error

// FeatureApply applies changes on the feature abstraction.
type FeatureApply func(metadata Metadata, builderSpec []byte, api FeatureAbstractAPI, engine Engine) error

// Reconcile is the interface to be implemented by plugins that want to be reconciled in the operator.
// This is useful for plugins that need to spawn external Feature Ingestion.
type Reconcile func(ctx context.Context, client client.Client, metadata Metadata, builderSpec []byte) error

// StateFactory is the interface to be implemented by plugins that implements storage providers.
type StateFactory func(viper *viper.Viper) (State, error)

// NotifierFactory is the interface to be implemented by plugins that implements storage providers.
type NotifierFactory func(viper *viper.Viper) (Notifier, error)
