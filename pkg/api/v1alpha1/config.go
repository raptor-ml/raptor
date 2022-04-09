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

package v1alpha1

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/mitchellh/mapstructure"
	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
	"net/url"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

// +kubebuilder:object:generate=false

// ParsedConfig is a parsed configuration
type ParsedConfig map[string]string

// Unmarshal is unmarshalling the config into a Struct. Make sure that the tags
// on the fields of the structure are properly set using the `mapstructure` tag.
func (cfg *ParsedConfig) Unmarshal(output any) error {
	c := &mapstructure.DecoderConfig{
		Metadata:         nil,
		Result:           output,
		WeaklyTypedInput: true,
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			mapstructure.StringToTimeDurationHookFunc(),
			mapstructure.StringToTimeHookFunc(time.RFC3339),
			mapstructure.StringToSliceHookFunc(","),
			stringToURLHookFunc,
		),
	}
	decoder, err := mapstructure.NewDecoder(c)
	if err != nil {
		return err
	}

	return decoder.Decode(cfg)
}

func stringToURLHookFunc(f reflect.Type, t reflect.Type, data interface{}) (interface{}, error) {
	if f.Kind() != reflect.String {
		return data, nil
	}
	if t != reflect.TypeOf(&url.URL{}) {
		return data, nil
	}

	dataVal := reflect.ValueOf(data)
	return url.Parse(dataVal.String())
}

// ParseConfig parses the config, and extracts the secrets, into a map of key-value pairs
func (in *DataConnector) ParseConfig(ctx context.Context, rdr client.Reader) (ParsedConfig, error) {
	cfg := make(ParsedConfig)
	cfg["_fqn"] = fmt.Sprintf("%s.%s", in.GetName(), in.GetNamespace())

	g, ctx := errgroup.WithContext(ctx)
	for _, cv := range in.Spec.Config {
		if cv.Name == "" {
			continue
		}
		if cv.Value != "" {
			cfg[cv.Name] = cv.Value
			continue
		}
		if cv.SecretKeyRef == nil {
			continue
		}
		g.Go(func(cv ConfigVar) func() error {
			return func() error {
				secret := &corev1.Secret{}
				err := rdr.Get(ctx, client.ObjectKey{
					Namespace: in.GetNamespace(),
					Name:      cv.SecretKeyRef.Name,
				}, secret)
				if err != nil {
					return fmt.Errorf("failed to get secret %s: %w", cv.SecretKeyRef.Name, err)
				}

				val, ok := secret.Data[cv.SecretKeyRef.Key]
				if !ok {
					return fmt.Errorf("secret %s does not have key %s", cv.SecretKeyRef.Name, cv.SecretKeyRef.Key)
				}
				cfg[cv.Name] = base64.StdEncoding.EncodeToString(val)
				return nil
			}
		}(cv))
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}
	return cfg, nil
}
