/*
Copyright (c) 2022 Raptor.

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
	"fmt"
	"github.com/raptor-ml/raptor/api"
	"github.com/sourcegraph/starlight/convert"
	sTime "go.starlark.net/lib/time"
	"go.starlark.net/starlark"
	"strings"
	"time"
)

type BasicOp struct {
	op         InstructionOp
	thread     *starlark.Thread
	builtin    *starlark.Builtin
	args       starlark.Tuple
	kwargs     []starlark.Tuple
	valueField string
}

func (r *runtime) basicOp(op BasicOp) (starlark.Value, error) {
	if op.valueField == "" {
		op.valueField = "value"
	}
	var ts = sTime.Time(nowf(op.thread))
	var fqn, entityID string
	var val any
	err := starlark.UnpackArgs(op.builtin.Name(), op.args, op.kwargs, "fqn", &fqn, "entity_id", &entityID, op.valueField, &val, "timestamp?", &ts)
	if err != nil {
		return nil, err
	}

	ns := op.thread.Name[strings.Index(op.thread.Name, ".")+1:]
	fqn = api.NormalizeFQN(fqn, ns)

	if val, err = starToGo(val); err != nil {
		return nil, err
	}

	ib := op.thread.Local(localKeyInstructions).(*InstructionsBag)
	ib.Lock()
	defer ib.Unlock()
	ib.Instructions = append(ib.Instructions, Instruction{
		Operation: op.op,
		FQN:       fqn,
		EntityID:  entityID,
		Timestamp: time.Time(ts),
		Value:     val,
	})

	return starlark.None, nil
}

func (r *runtime) SetFeature(t *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return r.basicOp(BasicOp{
		op:      InstructionOpSet,
		thread:  t,
		builtin: b,
		args:    args,
		kwargs:  kwargs,
	})
}
func (r *runtime) Update(t *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return r.basicOp(BasicOp{
		op:      InstructionOpUpdate,
		thread:  t,
		builtin: b,
		args:    args,
		kwargs:  kwargs,
	})
}
func (r *runtime) Incr(t *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return r.basicOp(BasicOp{
		op:         InstructionOpIncr,
		thread:     t,
		builtin:    b,
		args:       args,
		kwargs:     kwargs,
		valueField: "by",
	})
}

func (r *runtime) AppendFeature(t *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return r.basicOp(BasicOp{
		op:      InstructionOpAppend,
		thread:  t,
		builtin: b,
		args:    args,
		kwargs:  kwargs,
	})

}

type discoveredDependencies map[string]struct{}

const localKeyDiscoverDependencies = "discover_dependencies"

func (r *runtime) GetFeature(t *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var fqn string
	var entityID string
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "fqn", &fqn, "entity_id", &entityID); err != nil {
		return nil, err
	}

	ns := t.Name[strings.Index(t.Name, ".")+1:]
	fqn = api.NormalizeFQN(fqn, ns)

	// TODO protect cyclic fetch
	if t.Name == fqn {
		return nil, fmt.Errorf("cyclic Get: you tried to fetch the feature your'e in")
	}

	if dd := t.Local(localKeyDiscoverDependencies); dd != nil {
		if d, ok := dd.(discoveredDependencies); ok {
			d[fqn] = struct{}{}
			return starlark.Tuple{
				starlark.None,
				sTime.Time(nowf(t)),
			}, nil
		} else {
			panic("discover_dependencies is not valid")
		}
	}

	getter := t.Local(localKeyDependencyGetter).(DependencyGetter)
	val, err := getter(fqn, entityID, nowf(t))
	if err != nil {
		return nil, fmt.Errorf("couldn't get feature value: %w", err)
	}

	if val.Value == nil {
		return starlark.Tuple{starlark.None, starlark.None}, nil
	}
	starlarkVal, err := convert.ToValue(val.Value)
	if err != nil {
		return nil, err
	}

	// Return the value and the timestamp
	return starlark.Tuple{
		starlarkVal,
		sTime.Time(val.Timestamp),
	}, nil
}

const localKeyNow = "now"

func nowf(thread *starlark.Thread) time.Time {
	now := time.Now
	if n := thread.Local(localKeyNow); n != nil {
		if nv, ok := n.(time.Time); ok {
			now = func() time.Time {
				return nv
			}
		}
	}
	return now()
}

// Allow now function to be overridden per thread
func now(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return sTime.Time(nowf(thread)), nil
}
