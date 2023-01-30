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
	md := api.ModelDescriptor{}
	err := json.Unmarshal(builder.Raw, &md)
	if err != nil {
		return fmt.Errorf("failed to unmarshal expression spec: %w", err)
	}

	if len(md.Features) < 2 {
		return fmt.Errorf("model must have at least 2 features")
	}

	ns, _, _, _, _, err := api.ParseSelector(fd.FQN)
	if err != nil {
		return err
	}

	//normalize features
	for i, f := range md.Features {
		md.Features[i], err = api.NormalizeSelector(f, ns)
		if err != nil {
			return fmt.Errorf("failed to normalize feature %s in model %s: %w", f, fd.FQN, err)
		}
	}

	fs := &model{engine: engine, md: md}
	faapi.AddPostGetMiddleware(0, fs.preGetMiddleware)
	faapi.AddPreSetMiddleware(0, fs.preSetMiddleware)
	return nil
}

type model struct {
	md     api.ModelDescriptor
	engine api.Engine
}

func (m *model) preGetMiddleware(next api.MiddlewareHandler) api.MiddlewareHandler {
	return func(ctx context.Context, fd api.FeatureDescriptor, keys api.Keys, val api.Value) (api.Value, error) {
		cache, cacheOk := ctx.Value(api.ContextKeyFromCache).(bool)
		if cacheOk && cache && val.Fresh {
			return next(ctx, fd, keys, val)
		}

		logger := api.LoggerFromContext(ctx)
		wg := &sync.WaitGroup{}
		wg.Add(len(m.md.Features))

		ret := api.Value{}
		results := make(map[string]api.Value)
		for _, fqn := range m.md.Features {
			go func(fqn string, wg *sync.WaitGroup) {
				defer wg.Done()
				val, _, err := m.engine.Get(ctx, fqn, keys)
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

		if ms := plugins.ModelServer.Get(m.md.ModelServer); ms != nil {
			val, err := ms.Serve(ctx, fd, m.md, val)
			if err != nil {
				return val, err
			}
			return next(ctx, fd, keys, val)
		}

		return next(ctx, fd, keys, val)
	}
}

func (m *model) preSetMiddleware(next api.MiddlewareHandler) api.MiddlewareHandler {
	return func(ctx context.Context, fd api.FeatureDescriptor, keys api.Keys, val api.Value) (api.Value, error) {
		return val, fmt.Errorf("cannot set data to model %s", fd.FQN)
	}
}
