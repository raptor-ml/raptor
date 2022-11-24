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

package expression

import (
	"context"
	"fmt"
	"github.com/raptor-ml/raptor/api"
	manifests "github.com/raptor-ml/raptor/api/v1alpha1"
	"github.com/raptor-ml/raptor/pkg/plugins"
	"github.com/raptor-ml/raptor/pkg/pyexp"
)

func init() {
	const name = "expression"
	plugins.FeatureAppliers.Register(name, FeatureApply)
}

func FeatureApply(fd api.FeatureDescriptor, builder manifests.FeatureBuilder, api api.FeatureAbstractAPI, engine api.EngineWithSource) error {
	if builder.PyExp == "" {
		return fmt.Errorf("pyexp is empty")
	}

	runtime, err := pyexp.New(builder.PyExp, fd.FQN)
	if err != nil {
		return fmt.Errorf("failed to create expression runtime: %w", err)
	}
	e := expr{runtime: runtime, engine: engine}

	if fd.Freshness <= 0 {
		api.AddPreGetMiddleware(0, e.getMiddleware)
	} else {
		api.AddPostGetMiddleware(0, e.getMiddleware)
	}
	return nil
}

type expr struct {
	runtime pyexp.Runtime
	engine  api.Engine
}

func (p *expr) getMiddleware(next api.MiddlewareHandler) api.MiddlewareHandler {
	return func(ctx context.Context, fd api.FeatureDescriptor, entityID string, val api.Value) (api.Value, error) {
		cache, cacheOk := ctx.Value(api.ContextKeyFromCache).(bool)
		if cacheOk && cache && val.Fresh && !fd.ValidWindow() {
			return next(ctx, fd, entityID, val)
		}

		ret, err := p.runtime.ExecWithEngine(ctx, pyexp.ExecRequest{
			Headers:   nil,
			Payload:   val.Value,
			EntityID:  entityID,
			Timestamp: val.Timestamp,
			Logger:    api.LoggerFromContext(ctx),
		}, p.engine)
		if err != nil {
			return val, err
		}

		if ret.Value != nil {
			if ret.Timestamp.IsZero() && !val.Timestamp.IsZero() {
				ret.Timestamp = val.Timestamp
			}
			val = api.Value{
				Value:     ret.Value,
				Timestamp: ret.Timestamp,
				Fresh:     true,
			}
		}

		return next(ctx, fd, entityID, val)
	}
}
