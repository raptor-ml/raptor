package expression

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/natun-ai/natun/internal/plugin"
	"github.com/natun-ai/natun/pkg/api"
	"github.com/natun-ai/natun/pkg/pyexp"
)

func init() {
	const name = "expression"
	plugin.FeatureAppliers.Register(name, FeatureApply)
}

// ExprSpec is the specification of the expression plugin.
type ExprSpec struct {
	// +kubebuilder:validation:Required
	Expression string `json:"expression"`
}

func FeatureApply(md api.Metadata, builderSpec []byte, api api.FeatureAbstractAPI, engine api.Engine) error {
	expSpec := &ExprSpec{}
	err := json.Unmarshal(builderSpec, expSpec)
	if err != nil {
		return fmt.Errorf("failed to unmarshal expression spec: %w", err)
	}

	runtime, err := pyexp.New(md.FQN, expSpec.Expression, engine)
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
		if cacheOk && cache && val.Fresh {
			return next(ctx, md, entityID, val)
		}

		v, ts, _, err := p.runtime.Exec(pyexp.ExecRequest{
			Context:   ctx,
			Headers:   nil,
			Payload:   val.Value,
			EntityID:  entityID,
			Fqn:       md.FQN,
			Timestamp: val.Timestamp,
		})
		if err != nil {
			return val, err
		}

		ret := api.Value{
			Value:     v,
			Timestamp: ts,
			Fresh:     true,
		}

		return next(ctx, md, entityID, ret)
	}
}
