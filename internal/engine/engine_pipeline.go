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

package engine

import (
	"context"
	"fmt"
	"github.com/natun-ai/natun/api"
	"time"
)

func (e *engine) getValueMiddleware() api.Middleware {
	return func(next api.MiddlewareHandler) api.MiddlewareHandler {
		return func(ctx context.Context, md api.Metadata, entityID string, val api.Value) (api.Value, error) {
			wf, err := api.WindowFnFromContext(ctx)
			if err != nil {
				return val, err
			}

			v, err := e.state.Get(ctx, md, entityID)
			if err != nil {
				return val, err
			}

			if v == nil {
				return next(ctx, md, entityID, val)
			}
			if time.Now().Add(-md.Staleness).After(v.Timestamp) {
				// Ignore expired values.
				return next(ctx, md, entityID, val)
			}

			// Mark the context as from cache.
			ctx = context.WithValue(ctx, api.ContextKeyFromCache, v.Value != nil)
			ctx = context.WithValue(ctx, api.ContextKeyCacheFresh, v.Fresh)

			if md.ValidWindow() && wf != api.WindowFnUnknown {
				val = api.Value{
					Value:     api.ToLowLevelValue[api.WindowResultMap](v.Value)[wf],
					Timestamp: v.Timestamp,
					Fresh:     v.Fresh,
				}
				return next(ctx, md, entityID, val)
			}

			// modify the value to the result from the state
			val = *v

			return next(ctx, md, entityID, val)
		}
	}
}

func (e *engine) cachePostGetMiddleware(f *Feature) api.Middleware {
	return func(next api.MiddlewareHandler) api.MiddlewareHandler {
		return func(ctx context.Context, md api.Metadata, entityID string, val api.Value) (api.Value, error) {
			// If the value is nil, we should not cache the value.
			if val.Value == nil || md.ValidWindow() {
				return next(ctx, md, entityID, val)
			}

			// If the flag `ContextKeyCachePostGet` is disabled, we should not cache the value.
			if cpg, ok := ctx.Value(api.ContextKeyCachePostGet).(bool); ok && !cpg {
				return next(ctx, md, entityID, val)
			}

			// If the data from the cache was freshy, we should not cache the value.
			if fresh, ok := ctx.Value(api.ContextKeyCacheFresh).(bool); ok && fresh {
				return next(ctx, md, entityID, val)
			}

			ctx2 := context.WithValue(context.Background(), api.ContextKeyLogger, api.LoggerFromContext(ctx))
			go func(ctx context.Context, entityID string, val api.Value) {
				_, err := e.writePipeline(f, api.StateMethodSet).Apply(ctx, entityID, val)
				if err != nil {
					logger := api.LoggerFromContext(ctx)
					logger.Error(err, "failed to update the value to cache")
				}
			}(ctx2, entityID, val)

			return next(ctx, md, entityID, val)
		}
	}
}

func (e *engine) readPipeline(f *Feature) Pipeline {
	return Pipeline{
		Middlewares: append(append(f.preGet.Middlewares(), e.getValueMiddleware()), append(f.postGet.Middlewares(), e.cachePostGetMiddleware(f))...),
		Metadata:    f.Metadata,
	}
}
func (e *engine) writePipeline(f *Feature, method api.StateMethod) Pipeline {
	return Pipeline{
		Middlewares: append(append(f.preSet.Middlewares(), e.setMiddleware(method)), f.postSet.Middlewares()...),
		Metadata:    f.Metadata,
	}
}

func (e *engine) setMiddleware(method api.StateMethod) api.Middleware {
	return func(next api.MiddlewareHandler) api.MiddlewareHandler {
		return func(ctx context.Context, md api.Metadata, entityID string, val api.Value) (api.Value, error) {
			if md.Primitive == api.PrimitiveTypeHeadless {
				return next(ctx, md, entityID, val)
			}

			if api.TypeDetect(val.Value) != md.Primitive {
				return val, fmt.Errorf("value mismatch: got value with a different type than the feature type")
			}
			var err error
			switch method {
			case api.StateMethodSet:
				err = e.state.Set(ctx, md, entityID, val.Value, val.Timestamp)
			case api.StateMethodAppend:
				err = e.state.Append(ctx, md, entityID, val.Value, val.Timestamp)
			case api.StateMethodIncr:
				err = e.state.Incr(ctx, md, entityID, val.Value, val.Timestamp)
			case api.StateMethodUpdate:
				err = e.state.Update(ctx, md, entityID, val.Value, val.Timestamp)
			case api.StateMethodWindowAdd:
				err = e.state.WindowAdd(ctx, md, entityID, val.Value, val.Timestamp)
			}
			if err != nil {
				return val, err
			}

			bucket := ""
			if md.ValidWindow() {
				bucket = api.BucketName(val.Timestamp, md.Freshness)
			}
			e.historian.AddCollectNotification(md.FQN, entityID, bucket)

			return next(ctx, md, entityID, val)
		}
	}
}
