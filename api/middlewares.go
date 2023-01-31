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

package api

import (
	"context"
	"github.com/go-logr/logr"
)

type Middleware func(next MiddlewareHandler) MiddlewareHandler
type MiddlewareHandler func(ctx context.Context, fd FeatureDescriptor, keys Keys, val Value) (Value, error)

// Pipeliner is the interface that plugins can use to modify the Core's feature pipelines on creation time
type Pipeliner interface {
	AddPreGetMiddleware(priority int, fn Middleware)
	AddPostGetMiddleware(priority int, fn Middleware)
	AddPreSetMiddleware(priority int, fn Middleware)
	AddPostSetMiddleware(priority int, fn Middleware)
}

// ContextKey is a key to store data in	context.
type ContextKey int

const (
	// ContextKeyCachePostGet is a key to store the flag to cache in the storage postGet value.
	// If not set it is defaulting to true.
	ContextKeyCachePostGet ContextKey = iota

	// ContextKeyCacheFresh is a key to store the flag that indicate if the result from the cache was fresh.
	ContextKeyCacheFresh

	// ContextKeyFromCache is a key to store the flag to indicate if the value is from the cache.
	ContextKeyFromCache

	// ContextKeyLogger is a key to store a logger.
	ContextKeyLogger

	// ContextKeySelector is a key to store the requested Feature Selector.
	ContextKeySelector
)

// LoggerFromContext returns the logger from the context.
// If not found it returns a discarded logger.
func LoggerFromContext(ctx context.Context) logr.Logger {
	if logger, ok := ctx.Value(ContextKeyLogger).(logr.Logger); ok {
		return logger
	}
	return logr.Logger{}
}

func ContextWithSelector(ctx context.Context, selector string) context.Context {
	return context.WithValue(ctx, ContextKeySelector, selector)
}
func SelectorFromContext(ctx context.Context) (string, error) {
	if ctx == nil {
		return "", ErrInvalidPipelineContext
	}
	if s, ok := ctx.Value(ContextKeySelector).(string); ok {
		return s, nil
	}

	return "", ErrInvalidPipelineContext
}
