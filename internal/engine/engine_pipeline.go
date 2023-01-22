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

package engine

import (
	"context"
	"fmt"
	"github.com/raptor-ml/raptor/api"
	"time"
)

func (e *engine) getValueMiddleware() api.Middleware {
	return func(next api.MiddlewareHandler) api.MiddlewareHandler {
		return func(ctx context.Context, fd api.FeatureDescriptor, keys api.Keys, val api.Value) (api.Value, error) {
			if fd.DataSource == "" {
				return next(ctx, fd, keys, val)
			}

			selector, err := api.SelectorFromContext(ctx)
			if err != nil {
				return val, fmt.Errorf("failed to get selector from context: %w", err)
			}

			_, _, af, ver, _, err := api.ParseSelector(selector)
			if err != nil {
				return val, fmt.Errorf("failed to parse selector: %w", err)
			}

			if ver > 0 {
				if fd.KeepPrevious == nil {
					return val, fmt.Errorf("selector specified a previous version, although the feature "+
						"doesn't support it: %s", selector)
				}
				if ver > fd.KeepPrevious.Versions {
					return val, fmt.Errorf("selector specified a previous version(%d) which is greater than "+
						"the configured keep_previous (%d): %s", ver, fd.KeepPrevious.Versions, selector)
				}
			}

			v, err := e.state.Get(ctx, fd, keys, ver)
			if err != nil {
				return val, err
			}

			if v == nil {
				return next(ctx, fd, keys, val)
			}
			if time.Now().Add(-fd.Staleness).After(v.Timestamp) {
				// Ignore expired values.
				return next(ctx, fd, keys, val)
			}

			// Mark the context as from cache.
			ctx = context.WithValue(ctx, api.ContextKeyFromCache, v.Value != nil)
			ctx = context.WithValue(ctx, api.ContextKeyCacheFresh, v.Fresh)

			if fd.ValidWindow() && af != api.AggrFnUnknown {
				val = api.Value{
					Value:     api.ToLowLevelValue[api.WindowResultMap](v.Value)[af],
					Timestamp: v.Timestamp,
					Fresh:     v.Fresh,
				}
				return next(ctx, fd, keys, val)
			}

			// modify the value to the result from the state
			val = *v

			return next(ctx, fd, keys, val)
		}
	}
}

func (e *engine) cachePostGetMiddleware(f *Feature) api.Middleware {
	return func(next api.MiddlewareHandler) api.MiddlewareHandler {
		return func(ctx context.Context, fd api.FeatureDescriptor, keys api.Keys, val api.Value) (api.Value, error) {
			// If the value is nil, we should not cache the value.
			if val.Value == nil || fd.ValidWindow() || fd.DataSource == "" {
				return next(ctx, fd, keys, val)
			}

			if api.TypeDetect(val.Value) != fd.Primitive {
				return val, fmt.Errorf("value mismatch: got value with a different type than the feature type")
			}

			// If the flag `ContextKeyCachePostGet` is disabled, we should not cache the value.
			if cpg, ok := ctx.Value(api.ContextKeyCachePostGet).(bool); ok && !cpg {
				return next(ctx, fd, keys, val)
			}

			// If the data from the cache was freshy, we should not cache the value.
			if fresh, ok := ctx.Value(api.ContextKeyCacheFresh).(bool); ok && fresh {
				return next(ctx, fd, keys, val)
			}

			ctx2 := context.WithValue(context.Background(), api.ContextKeyLogger, api.LoggerFromContext(ctx))
			go func(ctx context.Context, keys api.Keys, val api.Value) {
				_, err := e.writePipeline(f, api.StateMethodSet).Apply(ctx, keys, val)
				if err != nil {
					logger := api.LoggerFromContext(ctx)
					logger.Error(err, "failed to update the value to cache")
				}
			}(ctx2, keys, val)

			return next(ctx, fd, keys, val)
		}
	}
}

func (e *engine) readPipeline(f *Feature) Pipeline {
	return Pipeline{
		Middlewares:       append(append(f.preGet.Middlewares(), e.getValueMiddleware()), append(f.postGet.Middlewares(), e.cachePostGetMiddleware(f))...),
		FeatureDescriptor: f.FeatureDescriptor,
	}
}
func (e *engine) writePipeline(f *Feature, method api.StateMethod) Pipeline {
	return Pipeline{
		Middlewares:       append(append(f.preSet.Middlewares(), e.setMiddleware(method)), f.postSet.Middlewares()...),
		FeatureDescriptor: f.FeatureDescriptor,
	}
}

func (e *engine) setMiddleware(method api.StateMethod) api.Middleware {
	return func(next api.MiddlewareHandler) api.MiddlewareHandler {
		return func(ctx context.Context, fd api.FeatureDescriptor, keys api.Keys, val api.Value) (api.Value, error) {
			if fd.DataSource == "" {
				return next(ctx, fd, keys, val)
			}

			if api.TypeDetect(val.Value) != fd.Primitive {
				return val, fmt.Errorf("value mismatch: got value with a different type than the feature type")
			}

			encodedKeys, err := keys.Encode(fd)
			if err != nil {
				return val, fmt.Errorf("failed to encode keys: %v", err)
			}

			// (retrospective write): when the value is expired, only write it to the historical storage
			if !fd.ValidWindow() && val.Timestamp.Before(time.Now().Add(-fd.Staleness)) {
				e.historian.AddWriteNotification(fd.FQN, encodedKeys, "", &val)
				return next(ctx, fd, keys, val)
			}

			switch method {
			case api.StateMethodSet:
				err = e.state.Set(ctx, fd, keys, val.Value, val.Timestamp)
			case api.StateMethodAppend:
				err = e.state.Append(ctx, fd, keys, val.Value, val.Timestamp)
			case api.StateMethodIncr:
				err = e.state.Incr(ctx, fd, keys, val.Value, val.Timestamp)
			case api.StateMethodUpdate:
				err = e.state.Update(ctx, fd, keys, val.Value, val.Timestamp)
			case api.StateMethodWindowAdd:
				err = e.state.WindowAdd(ctx, fd, keys, val.Value, val.Timestamp)
			}
			if err != nil {
				return val, err
			}

			if fd.ValidWindow() {
				bucket := api.BucketName(val.Timestamp, fd.Freshness)
				e.historian.AddCollectNotification(fd.FQN, encodedKeys, bucket)
			} else {
				e.historian.AddWriteNotification(fd.FQN, encodedKeys, "", &val)
			}

			return next(ctx, fd, keys, val)
		}
	}
}
