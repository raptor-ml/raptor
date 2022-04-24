package engine

import (
	"context"
	"github.com/natun-ai/natun/pkg/api"
	"time"
)

type Dummy struct{}

func (*Dummy) Metadata(ctx context.Context, FQN string) (api.Metadata, error) {
	return api.Metadata{}, nil
}
func (*Dummy) Get(ctx context.Context, FQN string, entityID string) (api.Value, api.Metadata, error) {
	return api.Value{}, api.Metadata{}, nil
}
func (*Dummy) Set(ctx context.Context, FQN string, entityID string, val any, ts time.Time) error {
	return nil
}
func (*Dummy) Append(ctx context.Context, FQN string, entityID string, val any, ts time.Time) error {
	return nil
}
func (*Dummy) Incr(ctx context.Context, FQN string, entityID string, by any, ts time.Time) error {
	return nil
}
func (*Dummy) Update(ctx context.Context, FQN string, entityID string, val any, ts time.Time) error {
	return nil
}
