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
	goerrors "errors"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/raptor-ml/raptor/api"
	"github.com/raptor-ml/raptor/internal/historian"
	"github.com/raptor-ml/raptor/internal/stats"
	"strings"
	"sync"
	"time"
)

type engine struct {
	features    sync.Map
	dataSources sync.Map
	state       api.State
	historian   historian.Client
	logger      logr.Logger
	api.RuntimeManager
}

// New creates a new engine manager
func New(state api.State, h historian.Client, rm api.RuntimeManager, logger logr.Logger) api.ManagerEngine {
	if state == nil {
		panic("state is nil")
	}
	e := &engine{
		state:          state,
		historian:      h,
		logger:         logger,
		RuntimeManager: rm,
	}
	return e
}

func (e *engine) Append(ctx context.Context, fqn string, keys api.Keys, val any, ts time.Time) error {
	defer stats.IncrFeatureAppends()
	return e.write(ctx, fqn, keys, val, ts, api.StateMethodAppend)
}
func (e *engine) Incr(ctx context.Context, fqn string, keys api.Keys, by any, ts time.Time) error {
	defer stats.IncrFeatureIncrements()
	return e.write(ctx, fqn, keys, by, ts, api.StateMethodIncr)
}
func (e *engine) Set(ctx context.Context, fqn string, keys api.Keys, val any, ts time.Time) error {
	defer stats.IncrFeatureSets()
	return e.write(ctx, fqn, keys, val, ts, api.StateMethodSet)
}
func (e *engine) Update(ctx context.Context, fqn string, keys api.Keys, val any, ts time.Time) error {
	defer stats.IncrFeatureUpdates()
	return e.write(ctx, fqn, keys, val, ts, api.StateMethodUpdate)
}
func (e *engine) write(ctx context.Context, fqn string, keys api.Keys, val any, ts time.Time, method api.StateMethod) error {
	f, ctx, cancel, err := e.featureForRequest(ctx, fqn)
	if err != nil {
		return err
	}
	defer cancel()

	_, err = keys.Encode(f.FeatureDescriptor)
	if err != nil {
		return fmt.Errorf("failed to encode keys: %w", err)
	}

	v := api.Value{Value: val, Timestamp: ts}
	if _, err = e.writePipeline(f, method).Apply(ctx, keys, v); err != nil {
		return fmt.Errorf("failed to %s value for feature %s with keys %s: %w", method, fqn, keys, err)
	}
	return nil
}

func (e *engine) Get(ctx context.Context, selector string, keys api.Keys) (api.Value, api.FeatureDescriptor, error) {
	defer stats.IncrFeatureGets()

	ret := api.Value{Timestamp: time.Now()}
	f, ctx, cancel, err := e.featureForRequest(ctx, selector)
	if err != nil {
		return ret, api.FeatureDescriptor{}, err
	}
	defer cancel()

	ret, err = e.readPipeline(f).Apply(ctx, keys, ret)
	if err != nil && !(goerrors.Is(err, context.DeadlineExceeded) && ret.Value != nil && !ret.Fresh) {
		return ret, f.FeatureDescriptor, fmt.Errorf("failed to GET value for feature %s with keys %s: %w", selector, keys, err)
	}
	return ret, f.FeatureDescriptor, nil
}

func (e *engine) FeatureDescriptor(ctx context.Context, selector string) (api.FeatureDescriptor, error) {
	defer stats.IncrFeatureDescriptorReqs()
	f, _, cancel, err := e.featureForRequest(ctx, selector)
	if err != nil {
		return api.FeatureDescriptor{}, err
	}
	defer cancel()

	return f.FeatureDescriptor, nil
}
func (e *engine) featureForRequest(ctx context.Context, selector string) (*Feature, context.Context, context.CancelFunc, error) {
	fqn, err := api.NormalizeFQN(selector, "undefined-namespace")
	if err != nil {
		return nil, ctx, nil, fmt.Errorf("failed to normalize Feature Selector `%s` as FQN: %w", selector, err)
	}
	if strings.HasPrefix(fqn, "undefined-namespace") {
		return nil, ctx, nil, fmt.Errorf("namespace is required in Feature Selector `%s`", selector)
	}

	_, _, aggrFn, _, _, err := api.ParseSelector(selector)
	if err != nil {
		return nil, ctx, nil, fmt.Errorf("failed to parse selector %s: %w", selector, err)
	}

	if f, ok := e.features.Load(fqn); ok {
		if f, ok := f.(*Feature); ok {
			ctx, cancel := f.Context(ctx, e.Logger())
			ctx = api.ContextWithAggrFn(ctx, api.StringToAggrFn(aggrFn))
			return f, ctx, cancel, nil
		}
	}
	return nil, ctx, nil, fmt.Errorf("%w: %s", api.ErrFeatureNotFound, selector)
}

func (e *engine) Logger() logr.Logger {
	return e.logger
}
