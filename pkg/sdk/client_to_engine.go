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

package sdk

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/raptor-ml/raptor/api"
	coreApi "go.buf.build/raptor/api-go/raptor/core/raptor/core/v1alpha1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
	"strings"
	"time"
)

type grpcEngine struct {
	client coreApi.EngineServiceClient
}

// NewGRPCEngine creates a new api.Engine from GRPC client
func NewGRPCEngine(client coreApi.EngineServiceClient) api.Engine {
	return &grpcEngine{
		client: client,
	}
}

func (e *grpcEngine) FeatureDescriptor(ctx context.Context, fqn string) (api.FeatureDescriptor, error) {
	req := &coreApi.FeatureDescriptorRequest{
		Uuid: uuid.NewString(),
		Fqn:  fqn,
	}
	resp, err := e.client.FeatureDescriptor(ctx, req)
	if err != nil {
		return api.FeatureDescriptor{}, fmt.Errorf("failed to get FeatureDescriptor: %w", normalizeError(err))
	}
	if resp.Uuid != req.Uuid {
		return api.FeatureDescriptor{}, fmt.Errorf("got %s uuid but requested with %s", resp.Uuid, req.Uuid)
	}
	return FromAPIFeatureDescriptor(resp.FeatureDescriptor), nil
}
func (e *grpcEngine) Get(ctx context.Context, fqn string, keys api.Keys) (api.Value, api.FeatureDescriptor, error) {
	req := coreApi.GetRequest{
		Uuid: uuid.NewString(),
		Fqn:  fqn,
		Keys: keys,
	}
	ret := api.Value{}
	resp, err := e.client.Get(ctx, &req)
	if err != nil {
		return ret, api.FeatureDescriptor{}, fmt.Errorf("failed to get feature: %w", normalizeError(err))
	}

	if resp.Uuid != req.Uuid {
		return ret, api.FeatureDescriptor{}, fmt.Errorf("got %s uuid but requested with %s", resp.Uuid, req.Uuid)
	}

	ret.Value = FromValue(resp.Value.Value)
	ret.Timestamp = resp.Value.Timestamp.AsTime()
	ret.Fresh = resp.Value.Fresh
	return ret, FromAPIFeatureDescriptor(resp.FeatureDescriptor), nil
}
func (e *grpcEngine) Set(ctx context.Context, fqn string, keys api.Keys, val any, ts time.Time) error {
	req := coreApi.SetRequest{
		Uuid:      uuid.NewString(),
		Fqn:       fqn,
		Keys:      keys,
		Value:     ToAPIValue(val),
		Timestamp: timestamppb.New(ts),
	}
	resp, err := e.client.Set(ctx, &req)
	if err != nil {
		return normalizeError(err)
	}
	if resp.Uuid != req.Uuid {
		return fmt.Errorf("got %s uuid but requested with %s", resp.Uuid, req.Uuid)
	}
	return nil
}
func (e *grpcEngine) Append(ctx context.Context, fqn string, keys api.Keys, val any, ts time.Time) error {
	req := coreApi.AppendRequest{
		Uuid:      uuid.NewString(),
		Fqn:       fqn,
		Keys:      keys,
		Value:     ToAPIScalar(val),
		Timestamp: timestamppb.New(ts),
	}
	resp, err := e.client.Append(ctx, &req)
	if err != nil {
		return normalizeError(err)
	}
	if resp.Uuid != req.Uuid {
		return fmt.Errorf("got %s uuid but requested with %s", resp.Uuid, req.Uuid)
	}
	return nil
}
func (e *grpcEngine) Incr(ctx context.Context, fqn string, keys api.Keys, by any, ts time.Time) error {
	req := coreApi.IncrRequest{
		Uuid:      uuid.NewString(),
		Fqn:       fqn,
		Keys:      keys,
		Value:     ToAPIScalar(by),
		Timestamp: timestamppb.New(ts),
	}
	resp, err := e.client.Incr(ctx, &req)
	if err != nil {
		return normalizeError(err)
	}
	if resp.Uuid != req.Uuid {
		return fmt.Errorf("got %s uuid but requested with %s", resp.Uuid, req.Uuid)
	}
	return nil
}
func (e *grpcEngine) Update(ctx context.Context, fqn string, keys api.Keys, val any, ts time.Time) error {
	req := coreApi.UpdateRequest{
		Uuid:      uuid.NewString(),
		Fqn:       fqn,
		Keys:      keys,
		Value:     ToAPIValue(val),
		Timestamp: timestamppb.New(ts),
	}
	resp, err := e.client.Update(ctx, &req)
	if err != nil {
		return normalizeError(err)
	}
	if resp.Uuid != req.Uuid {
		return fmt.Errorf("got %s uuid but requested with %s", resp.Uuid, req.Uuid)
	}
	return nil
}

func normalizeError(err error) error {
	if err == nil {
		return nil
	}
	e, ok := status.FromError(err)
	if !ok {
		return err
	}
	if e.Err() == nil {
		return err
	}

	if e.Code() == codes.NotFound {
		return api.ErrFeatureNotFound
	}
	if strings.HasSuffix(e.Err().Error(), api.ErrUnsupportedPrimitiveError.Error()) {
		return api.ErrUnsupportedPrimitiveError
	}
	if strings.HasSuffix(e.Err().Error(), api.ErrUnsupportedAggrError.Error()) {
		return api.ErrUnsupportedAggrError
	}
	return e.Err()
}
