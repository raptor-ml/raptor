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

package pyexp

import (
	"context"
	"fmt"
	"github.com/natun-ai/natun/pkg/errors"
	"github.com/sourcegraph/starlight/convert"
	sTime "go.starlark.net/lib/time"
	"go.starlark.net/starlark"
	"time"
)

type BasicOp struct {
	thread     *starlark.Thread
	builtin    *starlark.Builtin
	args       starlark.Tuple
	kwargs     []starlark.Tuple
	valueField string
}
type BasicOpResponse struct {
	ctx      context.Context
	fqn      string
	entityID string
	val      any
	ts       time.Time
	err      error
}

func (r *runtime) basicOp(op BasicOp) (res BasicOpResponse) {
	if op.valueField == "" {
		op.valueField = "value"
	}
	var ts = sTime.Time(time.Now())
	res.err = starlark.UnpackArgs(op.builtin.Name(), op.args, op.kwargs, "fqn", &res.fqn, "entity_id", &res.entityID, op.valueField, &res.val, "timestamp?", &ts)
	if res.err != nil {
		return
	}

	if res.val, res.err = starToGo(res.val); res.err != nil {
		return
	}

	var ok bool
	if res.ctx, ok = op.thread.Local(localKeyContext).(context.Context); !ok {
		res.err = errors.ErrInvalidPipelineContext
		return
	}

	res.ts = time.Time(ts)
	return
}

func (r *runtime) SetFeature(t *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	res := r.basicOp(BasicOp{
		thread:  t,
		builtin: b,
		args:    args,
		kwargs:  kwargs,
	})
	if res.err != nil {
		return nil, res.err
	}

	return starlark.None, r.engine.Set(res.ctx, res.fqn, res.entityID, res.val, res.ts)
}
func (r *runtime) Update(t *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	res := r.basicOp(BasicOp{
		thread:  t,
		builtin: b,
		args:    args,
		kwargs:  kwargs,
	})
	if res.err != nil {
		return nil, res.err
	}
	return starlark.None, r.engine.Update(res.ctx, res.fqn, res.entityID, res.val, res.ts)
}
func (r *runtime) Incr(t *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	res := r.basicOp(BasicOp{
		thread:     t,
		builtin:    b,
		args:       args,
		kwargs:     kwargs,
		valueField: "by",
	})
	if res.err != nil {
		return nil, res.err
	}

	return starlark.None, r.engine.Incr(res.ctx, res.fqn, res.entityID, res.val, res.ts)
}

func (r *runtime) AppendFeature(t *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	res := r.basicOp(BasicOp{
		thread:  t,
		builtin: b,
		args:    args,
		kwargs:  kwargs,
	})
	if res.err != nil {
		return nil, res.err
	}
	return starlark.None, r.engine.Append(res.ctx, res.fqn, res.entityID, res.val, res.ts)
}

func (r *runtime) GetFeature(t *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var fqn string
	var entityID string
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "fqn", &fqn, "entity_id", &entityID); err != nil {
		return nil, err
	}

	// TODO protect cyclic fetch
	if t.Name == fqn {
		return nil, fmt.Errorf("cyclic Get: you tried to fetch the feature your'e in")
	}

	ctx, ok := t.Local(localKeyContext).(context.Context)
	if !ok {
		return nil, errors.ErrInvalidPipelineContext
	}

	val, _, err := r.engine.Get(ctx, fqn, entityID)
	if err != nil {
		return nil, err
	}
	if val.Value == nil {
		return nil, fmt.Errorf("feature value not found for this fqn and entity id")
	}

	// Return the value and the timestamp
	starlarkVal, err := convert.ToValue(val.Value)
	if err != nil {
		return nil, err
	}

	return starlark.Tuple{
		starlarkVal,
		sTime.Time(val.Timestamp),
	}, nil

}
