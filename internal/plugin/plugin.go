package plugin

import (
	"fmt"
	"github.com/natun-ai/natun/pkg/api"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

type registey[p api.Plugins] map[string]p

// Register registers a plugin
func (r registey[P]) Register(name string, p P) {
	if _, ok := r[name]; ok {
		panic(fmt.Errorf("plugin `%s` is already registered", name))
	}
	r[name] = p
}

// Get retrieve a plugin
func (r registey[P]) Get(name string) P {
	return r[name]
}

var FeatureAppliers = make(registey[api.FeatureApply])
var FeatureReconciler = make(registey[api.FeatureApply])
var Configurers = make(registey[api.BindConfig])
var StateFactories = make(registey[api.StateFactory])
var NotifierFactories = make(registey[api.NotifierFactory])

// NewState creates a new State for a provider.
func NewState(sp string, viper *viper.Viper) (api.State, error) {
	if p := StateFactories.Get(sp); p != nil {
		return p(viper)
	}
	return nil, fmt.Errorf("state provider `%s` is not registered", sp)
}

// NewNotifier creates a new State for a provider.
func NewNotifier(notifier string, viper *viper.Viper) (api.Notifier, error) {
	if p := NotifierFactories.Get(notifier); p != nil {
		return p(viper)
	}
	return nil, fmt.Errorf("notifier provider `%s` is not registered", notifier)
}

// BindConfig adds config flags for the plugin.
func BindConfig(set *pflag.FlagSet) error {
	for _, p := range Configurers {
		if err := p(set); err != nil {
			return err
		}
	}
	return nil
}
