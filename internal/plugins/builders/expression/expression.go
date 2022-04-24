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

package expression

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/natun-ai/natun/pkg/api"
	"github.com/natun-ai/natun/pkg/plugin"
	"github.com/natun-ai/natun/pkg/pyexp"
)

func init() {
	const name = "expression"
	plugin.FeatureAppliers.Register(name, FeatureApply)
}

// ExprSpec is the specification of the expression plugin.
type ExprSpec struct {
	// +kubebuilder:validation:Required
	Expression string `json:"pyexp"`
}

func FeatureApply(md api.Metadata, builderSpec []byte, api api.FeatureAbstractAPI, engine api.EngineWithConnector) error {
	spec := &ExprSpec{}
	err := json.Unmarshal(builderSpec, spec)
	if err != nil {
		return fmt.Errorf("failed to unmarshal expression spec: %w", err)
	}

	if spec.Expression == "" {
		return fmt.Errorf("expression is empty")
	}

	runtime, err := pyexp.New(spec.Expression, engine)
	if err != nil {
		return fmt.Errorf("failed to create expression runtime: %w", err)
	}
	e := expr{runtime}

	if md.Freshness <= 0 {
		api.AddPreGetMiddleware(0, e.getMiddleware)
	} else {
		api.AddPostGetMiddleware(0, e.getMiddleware)
	}
	return nil
}

type expr struct {
	runtime pyexp.Runtime
}

func (p *expr) getMiddleware(next api.MiddlewareHandler) api.MiddlewareHandler {
	return func(ctx context.Context, md api.Metadata, entityID string, val api.Value) (api.Value, error) {
		cache, cacheOk := ctx.Value(api.ContextKeyFromCache).(bool)
		if cacheOk && cache && val.Fresh && !md.ValidWindow() {
			return next(ctx, md, entityID, val)
		}

		v, ts, _, err := p.runtime.Exec(ctx, pyexp.ExecRequest{
			Headers:   nil,
			Payload:   val.Value,
			EntityID:  entityID,
			Fqn:       md.FQN,
			Timestamp: val.Timestamp,
			Logger:    api.LoggerFromContext(ctx),
		})
		if err != nil {
			return val, err
		}

		if v != nil {
			if ts.IsZero() && !val.Timestamp.IsZero() {
				ts = val.Timestamp
			}
			val = api.Value{
				Value:     v,
				Timestamp: ts,
				Fresh:     true,
			}
		}

		return next(ctx, md, entityID, val)
	}
}
