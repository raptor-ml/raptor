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
	"time"

	"github.com/raptor-ml/raptor/api"
)

type ValueMeta struct {
	api.Value
	api.Metadata
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

func (d *dry) Metadata(_ context.Context, fqn string) (api.Metadata, error) {
	return api.Metadata{}, nil
}

func (d *dry) Get(_ context.Context, fqn string, entityID string) (api.Value, api.Metadata, error) {
	if d.discovery {
		d.discovered[fqn] = struct{}{}
		return api.Value{}, api.Metadata{}, nil
	}
	if f, ok := d.dd[fqn]; ok {
		if v, ok := f[entityID]; ok {
			return v.Value, v.Metadata, nil
		}
	}
	return api.Value{}, api.Metadata{}, api.ErrFeatureNotFound
}

func (d *dry) Set(_ context.Context, fqn string, entityID string, val any, ts time.Time) error {
	d.instructions = append(d.instructions, Instruction{
		Operation: InstructionOpSet,
		FQN:       fqn,
		EntityID:  entityID,
		Timestamp: ts,
		Value:     val,
	})
	return nil
}

func (d *dry) Append(_ context.Context, fqn string, entityID string, val any, ts time.Time) error {
	d.instructions = append(d.instructions, Instruction{
		Operation: InstructionOpAppend,
		FQN:       fqn,
		EntityID:  entityID,
		Timestamp: ts,
		Value:     val,
	})
	return nil
}

func (d *dry) Incr(_ context.Context, fqn string, entityID string, by any, ts time.Time) error {
	d.instructions = append(d.instructions, Instruction{
		Operation: InstructionOpIncr,
		FQN:       fqn,
		EntityID:  entityID,
		Timestamp: ts,
		Value:     by,
	})
	return nil
}

func (d *dry) Update(_ context.Context, fqn string, entityID string, val any, ts time.Time) error {
	d.instructions = append(d.instructions, Instruction{
		Operation: InstructionOpUpdate,
		FQN:       fqn,
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
