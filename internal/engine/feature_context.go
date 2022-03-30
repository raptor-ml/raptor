package engine

import (
	"context"
	"github.com/go-logr/logr"
	"github.com/natun-ai/natun/pkg/api"
	"github.com/natun-ai/natun/pkg/errors"
	"time"
)

func (f *Feature) Context(ctx context.Context, logger logr.Logger) (context.Context, context.CancelFunc) {
	ctx = context.WithValue(ctx, api.ContextKeyLogger, logger)

	cancel := func() {}
	if f.Metadata.Timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, time.Duration(float64(f.Metadata.Timeout)*0.98))
	}
	return ctx, cancel
}
func (f *Feature) ContextWithWindowFn(ctx context.Context, fn api.WindowFn) context.Context {
	ctx = context.WithValue(ctx, api.ContextKeyWindowFn, fn)
	return ctx
}

// LoggerFromContext returns the logger from the context.
// If not found it returns a discarded logger.
func LoggerFromContext(ctx context.Context) logr.Logger {
	if logger, ok := ctx.Value(api.ContextKeyLogger).(logr.Logger); ok {
		return logger
	}
	return logr.Logger{}
}
func WindowFnFromContext(ctx context.Context) (api.WindowFn, error) {
	if ctx == nil {
		return api.WindowFnUnknown, errors.ErrInvalidPipelineContext
	}

	if f, ok := ctx.Value(api.ContextKeyWindowFn).(api.WindowFn); ok {
		return f, nil
	}

	return api.WindowFnUnknown, errors.ErrInvalidPipelineContext
}
