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

package model

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/raptor-ml/raptor/api"
	manifests "github.com/raptor-ml/raptor/api/v1alpha1"
	"github.com/raptor-ml/raptor/pkg/plugins"
	"sync"
)

func init() {
	const name = api.ModelBuilder
	plugins.FeatureAppliers.Register(name, FeatureApply)
}

func FeatureApply(fd api.FeatureDescriptor, builder manifests.FeatureBuilder, faapi api.FeatureAbstractAPI, engine api.ExtendedManager) error {
	spec := manifests.ModelSpec{}
	err := json.Unmarshal(builder.Raw, &spec)
	if err != nil {
		return fmt.Errorf("failed to unmarshal expression spec: %w", err)
	}

	if len(spec.Features) < 2 {
		return fmt.Errorf("model must have at least 2 features")
	}

	ns, _, _, _, _, err := api.ParseFQN(fd.FQN)
	if err != nil {
		return err
	}

	//normalize features
	for i, f := range spec.Features {
		spec.Features[i], err = api.NormalizeFQN(f, ns)
		if err != nil {
			return fmt.Errorf("failed to normalize feature %s in model %s: %w", f, fd.FQN, err)
		}
	}

	fs := &model{engine: engine, features: spec.Features}
	faapi.AddPostGetMiddleware(0, fs.preGetMiddleware)
	faapi.AddPreSetMiddleware(0, fs.preSetMiddleware)
	return nil
}

type model struct {
	features []string
	engine   api.Engine
}

func (fs *model) preGetMiddleware(next api.MiddlewareHandler) api.MiddlewareHandler {
	return func(ctx context.Context, fd api.FeatureDescriptor, keys api.Keys, val api.Value) (api.Value, error) {
		logger := api.LoggerFromContext(ctx)
		wg := &sync.WaitGroup{}
		wg.Add(len(fs.features))

		ret := api.Value{}
		results := make(map[string]api.Value)
		for _, fqn := range fs.features {
			go func(fqn string, wg *sync.WaitGroup) {
				defer wg.Done()
				val, _, err := fs.engine.Get(ctx, fqn, keys)
				if err != nil {
					logger.Error(err, "failed to get feature %s", fqn)
					return
				}
				results[fqn] = val
				if ret.Timestamp.IsZero() || ret.Timestamp.Before(val.Timestamp) {
					ret.Timestamp = val.Timestamp
				}
				if val.Fresh {
					ret.Fresh = true
				}
			}(fqn, wg)
		}
		wg.Wait()
		ret.Value = results

		return ret, nil
	}
}

func (fs *model) preSetMiddleware(next api.MiddlewareHandler) api.MiddlewareHandler {
	return func(ctx context.Context, fd api.FeatureDescriptor, keys api.Keys, val api.Value) (api.Value, error) {
		return val, fmt.Errorf("cannot set model %s", fd.FQN)
	}
}
