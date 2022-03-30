package engine

import (
	"context"
	"github.com/natun-ai/natun/pkg/api"
)

type Middlewares []api.Middleware
type Pipeline struct {
	Middlewares
	api.Metadata
}

func (p Pipeline) Apply(ctx context.Context, entityID string, first api.Value) (api.Value, error) {
	if len(p.Middlewares) == 0 {
		return first, nil
	}
	// Although this is probably redundant to have two appliers, we prefer to keep it simple when possible,
	// even if it's mainly for documentation purposes.
	if _, ok := ctx.Deadline(); ok {
		return p.applyWithTimeout(ctx, entityID, first)
	}
	return p.apply(ctx, entityID, first)
}

func (p Pipeline) apply(ctx context.Context, entityID string, first api.Value) (api.Value, error) {
	var next api.MiddlewareHandler
	next = func(ctx context.Context, md api.Metadata, entityID string, val api.Value) (api.Value, error) {
		return val, nil
	}
	for i := len(p.Middlewares) - 1; i >= 0; i-- {
		next = p.Middlewares[i](next)
	}
	return next(ctx, p.Metadata, entityID, first)
}

func handlerWithTimeout(next api.MiddlewareHandler, c chan api.Value) api.MiddlewareHandler {
	return func(ctx context.Context, md api.Metadata, entityID string, val api.Value) (api.Value, error) {
		c <- val
		if next == nil {
			return val, nil
		}
		return next(ctx, md, entityID, val)
	}
}

func (p Pipeline) applyWithTimeout(ctx context.Context, entityID string, first api.Value) (api.Value, error) {
	c := make(chan api.Value)

	var next api.MiddlewareHandler
	next = handlerWithTimeout(nil, c)
	for i := len(p.Middlewares) - 1; i >= 0; i-- {
		next = handlerWithTimeout(p.Middlewares[i](next), c)
	}

	var err error
	go func(first api.Value) {
		_, e := next(ctx, p.Metadata, entityID, first)
		if err != nil {
			err = e
		}
		close(c)
	}(first)

	for {
		select {
		case <-ctx.Done():
			return first, ctx.Err()
		case g, ok := <-c:
			if !ok {
				return first, err
			}
			first = g
		}
	}
}
