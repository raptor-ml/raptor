/*
Copyright 2022 Natun.

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
	goerrors "errors"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/natun-ai/natun/internal/historian"
	"github.com/natun-ai/natun/pkg/api"
	"github.com/natun-ai/natun/pkg/errors"
	"sync"
	"time"
)

type engine struct {
	features  sync.Map
	state     api.State
	historian historian.Client
	logger    logr.Logger
}

// New creates a new engine manager
func New(state api.State, h historian.Client, logger logr.Logger) api.ManagerEngine {
	if state == nil {
		panic("state is nil")
	}
	e := &engine{
		state:     state,
		historian: h,
		logger:    logger,
	}
	return e
}

func (e *engine) Append(ctx context.Context, FQN string, entityID string, val any, ts time.Time) error {
	return e.write(ctx, FQN, entityID, val, ts, api.StateMethodAppend)
}
func (e *engine) Incr(ctx context.Context, FQN string, entityID string, by any, ts time.Time) error {
	return e.write(ctx, FQN, entityID, by, ts, api.StateMethodIncr)
}
func (e *engine) Set(ctx context.Context, FQN string, entityID string, val any, ts time.Time) error {
	return e.write(ctx, FQN, entityID, val, ts, api.StateMethodSet)
}
func (e *engine) Update(ctx context.Context, FQN string, entityID string, val any, ts time.Time) error {
	return e.write(ctx, FQN, entityID, val, ts, api.StateMethodUpdate)
}
func (e *engine) write(ctx context.Context, FQN string, entityID string, val any, ts time.Time, method api.StateMethod) error {
	f, ctx, cancel, err := e.featureForRequest(ctx, FQN)
	if err != nil {
		return err
	}
	defer cancel()

	v := api.Value{Value: val, Timestamp: ts}
	if _, err = e.writePipeline(f, method).Apply(ctx, entityID, v); err != nil {
		return fmt.Errorf("failed to %s value for feature %s with entity %s: %w", method, FQN, entityID, err)
	}
	return nil
}

func (e *engine) Get(ctx context.Context, FQN string, entityID string) (api.Value, api.Metadata, error) {
	ret := api.Value{Timestamp: time.Now()}
	f, ctx, cancel, err := e.featureForRequest(ctx, FQN)
	if err != nil {
		return ret, api.Metadata{}, err
	}
	defer cancel()

	ret, err = e.readPipeline(f).Apply(ctx, entityID, ret)
	if err != nil && !(goerrors.Is(err, context.DeadlineExceeded) && ret.Value != nil && !ret.Fresh) {
		return ret, f.Metadata, fmt.Errorf("failed to GET value for feature %s with entity %s: %w", FQN, entityID, err)
	}
	return ret, f.Metadata, nil
}

func (e *engine) Metadata(ctx context.Context, FQN string) (api.Metadata, error) {
	f, _, cancel, err := e.featureForRequest(ctx, FQN)
	if err != nil {
		return api.Metadata{}, err
	}
	defer cancel()

	return f.Metadata, nil
}
func (e *engine) featureForRequest(ctx context.Context, FQN string) (*Feature, context.Context, context.CancelFunc, error) {
	FQN, fn := api.FQNToRealFQN(FQN)
	if f, ok := e.features.Load(FQN); ok {
		if f, ok := f.(Feature); ok {
			ctx, cancel := f.Context(ctx, e.Logger())
			ctx = api.ContextWithWindowFn(ctx, fn)
			return &f, ctx, cancel, nil
		}
	}
	return nil, ctx, nil, fmt.Errorf("%w: %s", errors.ErrFeatureNotFound, FQN)
}

func (e *engine) Logger() logr.Logger {
	return e.logger
}
