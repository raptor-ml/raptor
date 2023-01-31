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

package sourceless

import (
	"context"
	"fmt"
	"github.com/raptor-ml/raptor/api"
	manifests "github.com/raptor-ml/raptor/api/v1alpha1"
	"github.com/raptor-ml/raptor/pkg/plugins"
)

func init() {
	const name = "sourceless"
	plugins.FeatureAppliers.Register(name, FeatureApply)
}

func FeatureApply(fd api.FeatureDescriptor, builder manifests.FeatureBuilder, api api.FeatureAbstractAPI, engine api.ExtendedManager) error {
	e := mw{engine}
	if fd.Freshness <= 0 {
		api.AddPreGetMiddleware(0, e.getMiddleware)
	} else {
		api.AddPostGetMiddleware(0, e.getMiddleware)
	}
	return nil
}

type mw struct {
	api.RuntimeManager
}

func (p *mw) getMiddleware(next api.MiddlewareHandler) api.MiddlewareHandler {
	return func(ctx context.Context, fd api.FeatureDescriptor, keys api.Keys, val api.Value) (api.Value, error) {
		cache, cacheOk := ctx.Value(api.ContextKeyFromCache).(bool)
		if cacheOk && cache && val.Fresh && !fd.ValidWindow() {
			return next(ctx, fd, keys, val)
		}

		val, keys, err := p.ExecuteProgram(ctx, fd.RuntimeEnv, fd.FQN, keys, nil, val.Timestamp, true)
		if err != nil {
			return val, fmt.Errorf("failed to execute python program: %w", err)
		}
		return next(ctx, fd, keys, val)
	}
}
