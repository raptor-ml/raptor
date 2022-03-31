package api

import (
	"context"
	"github.com/go-logr/logr"
	"github.com/natun-ai/natun/pkg/errors"
)

type Middleware func(next MiddlewareHandler) MiddlewareHandler
type MiddlewareHandler func(ctx context.Context, md Metadata, entityID string, val Value) (Value, error)

// FeatureAbstractAPI is the interface that plugins can use to modify the Core's feature abstraction on creation time
type FeatureAbstractAPI interface {
	AddPreGetMiddleware(priority int, fn Middleware)
	AddPostGetMiddleware(priority int, fn Middleware)
	AddPreSetMiddleware(priority int, fn Middleware)
	AddPostSetMiddleware(priority int, fn Middleware)
}

// ContextKey is a key to store data in	context.
type ContextKey int

const (
	// ContextKeyWindowFn is a key to store the requested window function in context.
	ContextKeyWindowFn ContextKey = iota

	// ContextKeyCachePostGet is a key to store the flag to cache in the storage postGet value.
	// If not set it is defaulting to true.
	ContextKeyCachePostGet

	// ContextKeyCacheFresh is a key to store the flag that indicate if the result from the cache was fresh.
	ContextKeyCacheFresh

	// ContextKeyFromCache is a key to store the flag to indicate if the value is from the cache.
	ContextKeyFromCache

	// ContextKeyLogger is a key to store a logger.
	ContextKeyLogger
)

// LoggerFromContext returns the logger from the context.
// If not found it returns a discarded logger.
func LoggerFromContext(ctx context.Context) logr.Logger {
	if logger, ok := ctx.Value(ContextKeyLogger).(logr.Logger); ok {
		return logger
	}
	return logr.Logger{}
}
func WindowFnFromContext(ctx context.Context) (WindowFn, error) {
	if ctx == nil {
		return WindowFnUnknown, errors.ErrInvalidPipelineContext
	}

	if f, ok := ctx.Value(ContextKeyWindowFn).(WindowFn); ok {
		return f, nil
	}

	return WindowFnUnknown, errors.ErrInvalidPipelineContext
}
func ContextWithWindowFn(ctx context.Context, fn WindowFn) context.Context {
	ctx = context.WithValue(ctx, ContextKeyWindowFn, fn)
	return ctx
}
