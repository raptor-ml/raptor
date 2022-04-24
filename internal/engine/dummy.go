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
	"github.com/natun-ai/natun/pkg/api"
	"time"
)

type Dummy struct {
	DataConnector api.DataConnector
}

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

func (d *Dummy) GetDataConnector(_ string) (api.DataConnector, error) {
	return d.DataConnector, nil
}
