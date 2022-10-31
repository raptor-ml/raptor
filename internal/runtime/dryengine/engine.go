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

package dryengine

import (
	"context"
	"github.com/raptor-ml/raptor/api"
	"time"
)

type ValueMeta struct {
	api.Value
	api.FeatureDescriptor
}
type DependenciesData map[string]map[string]ValueMeta

type InstructionOp int

const (
	InstructionOpNone InstructionOp = iota
	InstructionOpSet
	InstructionOpAppend
	InstructionOpIncr
	InstructionOpUpdate
)

type Instruction struct {
	Operation InstructionOp
	FQN       string
	EntityID  string
	Timestamp time.Time
	Value     any
}

type dry struct {
	discovery  bool
	discovered map[string]struct{}

	dd           DependenciesData
	instructions []Instruction
}

type DryEngine interface {
	api.Engine
	Instructions() []Instruction
	Dependencies() []string
}

// New returns a new DryEngine.
// The DryEngine is used to simulate the execution of a pipeline without actually
// executing it. It is used to test the pipeline and to generate the instructions
// that will be executed by the real engine.
//
// The DryEngine can be used in two modes:
// - discovery mode: in this mode, the DryEngine will not return any values, but
//   will instead record all the features that are requested. This is useful to
//   discover the dependencies of a pipeline.
// - execution mode: in this mode, the DryEngine will return the values that are
//   requested and will register the side effect instructions. This is useful to test the pipeline.
func New(data DependenciesData, discoveryMode bool) DryEngine {
	if data == nil {
		data = make(DependenciesData)
	}
	return &dry{
		dd:         data,
		discovery:  discoveryMode,
		discovered: make(map[string]struct{}),
	}
}

func (d *dry) FeatureDescriptor(_ context.Context, FQN string) (api.FeatureDescriptor, error) {
	return api.FeatureDescriptor{}, nil
}
func (d *dry) Get(_ context.Context, FQN string, entityID string) (api.Value, api.FeatureDescriptor, error) {
	if d.discovery {
		d.discovered[FQN] = struct{}{}
		return api.Value{}, api.FeatureDescriptor{}, nil
	}
	if f, ok := d.dd[FQN]; ok {
		if v, ok := f[entityID]; ok {
			return v.Value, v.FeatureDescriptor, nil
		}
	}
	return api.Value{}, api.FeatureDescriptor{}, api.ErrFeatureNotFound
}
func (d *dry) Set(_ context.Context, FQN string, entityID string, val any, ts time.Time) error {
	d.instructions = append(d.instructions, Instruction{
		Operation: InstructionOpSet,
		FQN:       FQN,
		EntityID:  entityID,
		Timestamp: ts,
		Value:     val,
	})
	return nil
}
func (d *dry) Append(_ context.Context, FQN string, entityID string, val any, ts time.Time) error {
	d.instructions = append(d.instructions, Instruction{
		Operation: InstructionOpAppend,
		FQN:       FQN,
		EntityID:  entityID,
		Timestamp: ts,
		Value:     val,
	})
	return nil
}
func (d *dry) Incr(_ context.Context, FQN string, entityID string, by any, ts time.Time) error {
	d.instructions = append(d.instructions, Instruction{
		Operation: InstructionOpIncr,
		FQN:       FQN,
		EntityID:  entityID,
		Timestamp: ts,
		Value:     by,
	})
	return nil
}
func (d *dry) Update(_ context.Context, FQN string, entityID string, val any, ts time.Time) error {
	d.instructions = append(d.instructions, Instruction{
		Operation: InstructionOpUpdate,
		FQN:       FQN,
		EntityID:  entityID,
		Timestamp: ts,
		Value:     val,
	})
	return nil
}
func (d *dry) Instructions() []Instruction {
	return d.instructions
}
func (d *dry) Dependencies() []string {
	discovered := make([]string, 0, len(d.discovered))
	for k := range d.discovered {
		discovered = append(discovered, k)
	}
	return discovered
}
