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

// Feature is a Core's feature abstraction.
type Feature struct {
	api.Metadata

	preGet  mws
	postGet mws
	preSet  mws
	postSet mws
}

// AddPreGetMiddleware adds a pre-get hook to the feature abstraction.
func (f *Feature) AddPreGetMiddleware(priority int, fn api.Middleware) {
	if fn == nil {
		return
	}
	f.preGet = append(f.preGet, mw{fn: fn, priority: priority})
}

// AddPostGetMiddleware adds a post-get hook to the feature abstraction.
func (f *Feature) AddPostGetMiddleware(priority int, fn api.Middleware) {
	if fn == nil {
		return
	}
	f.postGet = append(f.postGet, mw{fn: fn, priority: priority})
}

// AddPreSetMiddleware adds a pre-set hook to the feature abstraction.
func (f *Feature) AddPreSetMiddleware(priority int, fn api.Middleware) {
	if fn == nil {
		return
	}
	f.preSet = append(f.preSet, mw{fn: fn, priority: priority})
}

// AddPostSetMiddleware adds a post-set hook to the feature abstraction.
func (f *Feature) AddPostSetMiddleware(priority int, fn api.Middleware) {
	if fn == nil {
		return
	}
	f.postSet = append(f.postSet, mw{fn: fn, priority: priority})
}

// Context returns a new context with the feature attached.
func (f *Feature) Context(ctx context.Context, logger logr.Logger) (context.Context, context.CancelFunc) {
	ctx = context.WithValue(ctx, api.ContextKeyLogger, logger)

	cancel := func() {}
	if f.Metadata.Timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, time.Duration(float64(f.Metadata.Timeout)*0.98))
	}
	return ctx, cancel
}

type mws []mw
type mw struct {
	fn       api.Middleware
	priority int
}

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
