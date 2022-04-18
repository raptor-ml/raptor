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

package sdk

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/natun-ai/natun/pkg/api"
	coreApi "go.buf.build/natun/api-go/natun/core/natun/core/v1alpha1"
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

func (e *grpcEngine) Metadata(ctx context.Context, fqn string) (api.Metadata, error) {
	req := &coreApi.MetadataRequest{
		Uuid: uuid.NewString(),
		Fqn:  fqn,
	}
	resp, err := e.client.Metadata(ctx, req)
	if err != nil {
		return api.Metadata{}, fmt.Errorf("failed to get metadata: %w", normalizeError(err))
	}
	if resp.Uuid != req.Uuid {
		return api.Metadata{}, fmt.Errorf("got %s uuid but requested with %s", resp.Uuid, req.Uuid)
	}
	return FromAPIMetadata(resp.Metadata), nil
}
func (e *grpcEngine) Get(ctx context.Context, fqn string, entityID string) (api.Value, api.Metadata, error) {
	req := coreApi.GetRequest{
		Uuid:     uuid.NewString(),
		Fqn:      fqn,
		EntityId: entityID,
	}
	ret := api.Value{}
	resp, err := e.client.Get(ctx, &req)
	if err != nil {
		return ret, api.Metadata{}, fmt.Errorf("failed to get feature: %w", normalizeError(err))
	}

	if resp.Uuid != req.Uuid {
		return ret, api.Metadata{}, fmt.Errorf("got %s uuid but requested with %s", resp.Uuid, req.Uuid)
	}

	ret.Value = FromValue(resp.Value.Value)
	ret.Timestamp = resp.Value.Timestamp.AsTime()
	ret.Fresh = resp.Value.Fresh
	return ret, FromAPIMetadata(resp.Metadata), nil
}
func (e *grpcEngine) Set(ctx context.Context, fqn string, entityID string, val any, ts time.Time) error {
	req := coreApi.SetRequest{
		Uuid:      uuid.NewString(),
		Fqn:       fqn,
		EntityId:  entityID,
		Value:     ToAPIValue(val),
		Timestamp: timestamppb.New(ts),
	}
	resp, err := e.client.Set(ctx, &req)
	if resp.Uuid != req.Uuid {
		return fmt.Errorf("got %s uuid but requested with %s", resp.Uuid, req.Uuid)
	}
	return normalizeError(err)
}
func (e *grpcEngine) Append(ctx context.Context, fqn string, entityID string, val any, ts time.Time) error {
	req := coreApi.AppendRequest{
		Uuid:      uuid.NewString(),
		Fqn:       fqn,
		EntityId:  entityID,
		Value:     ToAPIScalar(val),
		Timestamp: timestamppb.New(ts),
	}
	resp, err := e.client.Append(ctx, &req)
	if resp.Uuid != req.Uuid {
		return fmt.Errorf("got %s uuid but requested with %s", resp.Uuid, req.Uuid)
	}
	return normalizeError(err)
}
func (e *grpcEngine) Incr(ctx context.Context, fqn string, entityID string, by any, ts time.Time) error {
	req := coreApi.IncrRequest{
		Uuid:      uuid.NewString(),
		Fqn:       fqn,
		EntityId:  entityID,
		Value:     ToAPIScalar(by),
		Timestamp: timestamppb.New(ts),
	}
	resp, err := e.client.Incr(ctx, &req)
	if resp.Uuid != req.Uuid {
		return fmt.Errorf("got %s uuid but requested with %s", resp.Uuid, req.Uuid)
	}
	return normalizeError(err)
}
func (e *grpcEngine) Update(ctx context.Context, fqn string, entityID string, val any, ts time.Time) error {
	req := coreApi.UpdateRequest{
		Uuid:      uuid.NewString(),
		Fqn:       fqn,
		EntityId:  entityID,
		Value:     ToAPIValue(val),
		Timestamp: timestamppb.New(ts),
	}
	resp, err := e.client.Update(ctx, &req)
	if resp.Uuid != req.Uuid {
		return fmt.Errorf("got %s uuid but requested with %s", resp.Uuid, req.Uuid)
	}
	return normalizeError(err)
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
		return nil
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
