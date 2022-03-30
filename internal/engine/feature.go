package engine

import (
	"github.com/natun-ai/natun/pkg/api"
	"sort"
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
