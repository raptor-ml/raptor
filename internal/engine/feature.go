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
	"github.com/go-logr/logr"
	"github.com/raptor-ml/raptor/api"
	"sort"
	"time"
)

// FeaturePipeliner is a Core's engine feature abstraction. It contains the FD and the pipelines.
type FeaturePipeliner struct {
	api.FeatureDescriptor

	preGet  mws
	postGet mws
	preSet  mws
	postSet mws
}

// AddPreGetMiddleware adds a pre-get hook to the feature abstraction.
func (f *FeaturePipeliner) AddPreGetMiddleware(priority int, fn api.Middleware) {
	if fn == nil {
		return
	}
	f.preGet = append(f.preGet, mw{fn: fn, priority: priority})
}

// AddPostGetMiddleware adds a post-get hook to the feature abstraction.
func (f *FeaturePipeliner) AddPostGetMiddleware(priority int, fn api.Middleware) {
	if fn == nil {
		return
	}
	f.postGet = append(f.postGet, mw{fn: fn, priority: priority})
}

// AddPreSetMiddleware adds a pre-set hook to the feature abstraction.
func (f *FeaturePipeliner) AddPreSetMiddleware(priority int, fn api.Middleware) {
	if fn == nil {
		return
	}
	f.preSet = append(f.preSet, mw{fn: fn, priority: priority})
}

// AddPostSetMiddleware adds a post-set hook to the feature abstraction.
func (f *FeaturePipeliner) AddPostSetMiddleware(priority int, fn api.Middleware) {
	if fn == nil {
		return
	}
	f.postSet = append(f.postSet, mw{fn: fn, priority: priority})
}

// Context returns a new context with the feature attached.
func (f *FeaturePipeliner) Context(ctx context.Context, selector string, logger logr.Logger) (context.Context, context.CancelFunc, error) {
	ctx = context.WithValue(ctx, api.ContextKeyLogger, logger)

	cancel := func() {}
	if f.FeatureDescriptor.Timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, time.Duration(float64(f.FeatureDescriptor.Timeout)*0.98))
	}

	return api.ContextWithSelector(ctx, selector), cancel, nil
}

type mws []mw
type mw struct {
	fn       api.Middleware
	priority int
}

// Sort sorts the middlewares by priority. The lower the priority the earlier the middleware is called.
func (mws *mws) Sort() mws {
	sort.SliceStable(*mws, func(i, j int) bool {
		return (*mws)[i].priority < (*mws)[j].priority
	})
	return *mws
}
func (mws *mws) Middlewares() Middlewares {
	var fns []api.Middleware
	for _, mw := range mws.Sort() {
		fns = append(fns, mw.fn)
	}
	return fns
}
