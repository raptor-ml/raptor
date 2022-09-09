/*
 * Copyright (c) 2022 RaptorML authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package pyexp

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/raptor-ml/raptor/api"
	"go.starlark.net/starlark"
)

const localKeyDependencyGetter = "dependency_getter"

type (
	DependencyGetter func(FQN string, entityID string, timestamp time.Time) (api.Value, error)
	ExecRequest      struct {
		Headers          map[string][]string
		Payload          any
		EntityID         string
		Timestamp        time.Time
		Logger           logr.Logger
		DependencyGetter DependencyGetter
	}
)

type InstructionOp int

const (
	InstructionOpNone InstructionOp = iota
	InstructionOpSet
	InstructionOpAppend
	InstructionOpIncr
	InstructionOpUpdate
)

func (i InstructionOp) String() string {
	switch i {
	case InstructionOpSet:
		return "set"
	case InstructionOpAppend:
		return "append"
	case InstructionOpIncr:
		return "incr"
	case InstructionOpUpdate:
		return "update"
	default:
		return "unknown"
	}
}

type Instruction struct {
	Operation InstructionOp
	FQN       string
	EntityID  string
	Timestamp time.Time
	Value     any
}
type InstructionsBag struct {
	sync.Mutex
	Instructions []Instruction
}

type ExecResponse struct {
	Value        any
	Timestamp    time.Time
	EntityID     string
	Instructions []Instruction
}

const localKeyInstructions = "engine.instructions"

func (r *runtime) Exec(req ExecRequest) (*ExecResponse, error) {
	v, thread, err := r.exec(req, false)
	if err != nil {
		evalErr := new(starlark.EvalError)
		if ok := errors.As(err, &evalErr); ok {
			req.Logger.WithValues("backtrace", evalErr.Backtrace()).Error(evalErr, "execution failed")
			return nil, fmt.Errorf(evalErr.Backtrace())
		} else {
			req.Logger.Error(err, "execution failed")
		}
		return nil, err
	}

	// Convert and validate the returned value
	ret, ts, eid, err := parseHandlerResults(v, thread)
	if err != nil {
		return nil, err
	}
	if req.EntityID != "" && eid != "" && eid != req.EntityID {
		err := fmt.Errorf("execution returned entity id %s, but the request was for entity id %s", eid, req.EntityID)
		return nil, err
	}
	if req.EntityID == "" && eid == "" {
		return nil, fmt.Errorf("this program must return an entity_id along with the value")
	}

	ib, ok := thread.Local(localKeyInstructions).(*InstructionsBag)
	if !ok {
		return nil, fmt.Errorf("failed to cast %v to *InstructionsBag", localKeyInstructions)
	}
	return &ExecResponse{
		Value:        ret,
		Timestamp:    ts,
		EntityID:     eid,
		Instructions: ib.Instructions,
	}, nil
}

func (r *runtime) DiscoverDependencies() ([]string, error) {
	_, thread, err := r.exec(ExecRequest{}, true)
	if err != nil {
		return nil, err
	}
	dd, ok := thread.Local(localKeyDiscoverDependencies).(discoveredDependencies)
	if !ok {
		return nil, fmt.Errorf("failed to cast %v to map[string]struct{}", localKeyDiscoverDependencies)
	}
	deps := make([]string, 0, len(dd))
	for k := range dd {
		deps = append(deps, k)
	}
	return deps, nil
}

func (r *runtime) exec(req ExecRequest, discoveryMode bool) (starlark.Value, *starlark.Thread, error) {
	// Prepare request
	kwargs, err := requestToKwargs(req)
	if err != nil {
		return nil, nil, err
	}

	// Create the globals dict
	predeclared := starlark.StringDict{}
	// Set builtins types
	for k, v := range r.builtins {
		predeclared[k] = v
	}

	// Create a Thread and redefine the behavior of the built-in 'print' function.
	thread := &starlark.Thread{
		Name:  r.fqn,
		Print: func(_ *starlark.Thread, msg string) { req.Logger.WithName("program").Info(msg) },
	}
	thread.SetLocal(localKeyNow, req.Timestamp)
	thread.SetLocal(localKeyInstructions, new(InstructionsBag))
	if discoveryMode {
		thread.SetLocal(localKeyDiscoverDependencies, make(discoveredDependencies))
	} else {
		thread.SetLocal(localKeyDependencyGetter, req.DependencyGetter)
	}

	// Execute the program
	globals, err := r.program.Init(thread, predeclared)
	globals.Freeze()

	if err != nil {
		return nil, thread, err
	}

	// Call the handler
	v, err := starlark.Call(thread, globals[r.handler], nil, kwargs)
	if err != nil {
		return nil, thread, err
	}

	return v, thread, err
}

func (r *runtime) ExecWithEngine(ctx context.Context, req ExecRequest, e api.Engine) (*ExecResponse, error) {
	req.DependencyGetter = func(FQN string, entityID string, timestamp time.Time) (api.Value, error) {
		v, _, err := e.Get(ctx, FQN, entityID)
		return v, err
	}
	ret, err := r.Exec(req)
	if err != nil {
		return nil, err
	}

	for _, i := range ret.Instructions {
		var err error
		switch i.Operation {
		case InstructionOpSet:
			err = e.Set(ctx, i.FQN, i.EntityID, i.Value, i.Timestamp)
		case InstructionOpUpdate:
			err = e.Update(ctx, i.FQN, i.EntityID, i.Value, i.Timestamp)
		case InstructionOpAppend:
			err = e.Append(ctx, i.FQN, i.EntityID, i.Value, i.Timestamp)
		case InstructionOpIncr:
			err = e.Incr(ctx, i.FQN, i.EntityID, i.Value, i.Timestamp)
		default:
			panic("unsupported op")
		}
		if err != nil {
			return nil, fmt.Errorf("failed to execute state instruction %s: %w", i.Operation, err)
		}
	}

	return ret, nil
}
