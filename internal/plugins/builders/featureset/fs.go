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

package featureset

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/natun-ai/natun/api"
	"github.com/natun-ai/natun/pkg/plugins"
	"sync"
)

func init() {
	const name = "featureset"
	plugins.FeatureAppliers.Register(name, FeatureApply)
}

func FeatureApply(md api.Metadata, builderSpec []byte, api api.FeatureAbstractAPI, engine api.EngineWithConnector) error {
	fs := &featureset{engine: engine}
	err := json.Unmarshal(builderSpec, &fs.features)
	if err != nil {
		return fmt.Errorf("failed to unmarshal expression spec: %w", err)
	}

	if len(fs.features) < 2 {
		return fmt.Errorf("featureset must have at least 2 features")
	}

	api.AddPostGetMiddleware(0, fs.preGetMiddleware)
	return nil
}

type featureset struct {
	features []string
	engine   api.Engine
}

func (fs *featureset) preGetMiddleware(next api.MiddlewareHandler) api.MiddlewareHandler {
	return func(ctx context.Context, md api.Metadata, entityID string, val api.Value) (api.Value, error) {
		logger := api.LoggerFromContext(ctx)
		wg := &sync.WaitGroup{}
		wg.Add(len(fs.features))

		ret := api.Value{}
		results := make(map[string]api.Value)
		for _, fqn := range fs.features {
			go func(fqn string, wg *sync.WaitGroup) {
				defer wg.Done()
				val, _, err := fs.engine.Get(ctx, fqn, entityID)
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
