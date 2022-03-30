package api

import "context"

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

type Middleware func(next MiddlewareHandler) MiddlewareHandler
type MiddlewareHandler func(ctx context.Context, md Metadata, entityID string, val Value) (Value, error)

// FeatureAbstractAPI is the interface that plugins can use to modify the Core's feature abstraction on creation time
type FeatureAbstractAPI interface {
	AddPreGetMiddleware(priority int, fn Middleware)
	AddPostGetMiddleware(priority int, fn Middleware)
	AddPreSetMiddleware(priority int, fn Middleware)
	AddPostSetMiddleware(priority int, fn Middleware)
}
