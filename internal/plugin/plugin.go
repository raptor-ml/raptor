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

package plugin

import (
	"fmt"
	"github.com/natun-ai/natun/pkg/api"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

type registey[P api.Plugins] map[string]P

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
var FeatureReconciler = make(registey[api.Reconcile])
var Configurers = make(registey[api.BindConfig])
var StateFactories = make(registey[api.StateFactory])
var CollectNotifierFactories = make(registey[api.CollectNotifierFactory])
var WriteNotifierFactories = make(registey[api.WriteNotifierFactory])
var HistoricalWriterFactories = make(registey[api.HistoricalWriterFactory])

// NewState creates a new State for a state provider.
func NewState(provider string, viper *viper.Viper) (api.State, error) {
	if p := StateFactories.Get(provider); p != nil {
		return p(viper)
	}
	return nil, fmt.Errorf("state provider `%s` is not registered", provider)
}

// NewCollectNotifier creates a new api.Notifier[api.CollectNotification] for a notifier provider.
func NewCollectNotifier(provider string, viper *viper.Viper) (api.Notifier[api.CollectNotification], error) {
	if p := CollectNotifierFactories.Get(provider); p != nil {
		return p(viper)
	}
	var n api.Notifier[api.CollectNotification]
	return n, fmt.Errorf("notifier provider `%s` is not registered", provider)
}

// NewWriteNotifier creates a new api.Notifier[api.WriteNotification] for a notifier provider.
func NewWriteNotifier(provider string, viper *viper.Viper) (api.Notifier[api.WriteNotification], error) {
	if p := WriteNotifierFactories.Get(provider); p != nil {
		return p(viper)
	}
	var n api.Notifier[api.WriteNotification]
	return n, fmt.Errorf("notifier provider `%s` is not registered", provider)
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

// NewHistoricalWriter creates a new HistoricalWriter for an historical writer provider.
func NewHistoricalWriter(provider string, viper *viper.Viper) (api.HistoricalWriter, error) {
	if p := HistoricalWriterFactories.Get(provider); p != nil {
		return p(viper)
	}
	return nil, fmt.Errorf("historical writer provider `%s` is not registered", provider)
}
