package sdk

import (
	"context"
	"fmt"
	"github.com/natun-ai/natun/pkg/api"
	coreApi "github.com/natun-ai/natun/proto/gen/go/natun/core/v1alpha1"
	"google.golang.org/protobuf/types/known/timestamppb"
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

func (e *grpcEngine) Metadata(ctx context.Context, FQN string) (api.Metadata, error) {
	req := &coreApi.MetadataRequest{
		Uuid: newUUID(),
		Fqn:  FQN,
	}
	resp, err := e.client.Metadata(ctx, req)
	if err != nil {
		return api.Metadata{}, fmt.Errorf("failed to get metadata: %w", err)
	}
	if resp.Uuid != req.Uuid {
		return api.Metadata{}, fmt.Errorf("got %s uuid but requested with %s", resp.Uuid, req.Uuid)
	}
	return FromAPIMetadata(resp.Metadata), nil
}
func (e *grpcEngine) Get(ctx context.Context, FQN string, entityID string) (api.Value, api.Metadata, error) {
	req := coreApi.GetRequest{
		Uuid:     newUUID(),
		Fqn:      FQN,
		EntityId: entityID,
	}
	ret := api.Value{}
	resp, err := e.client.Get(ctx, &req)
	if err != nil {
		return ret, api.Metadata{}, fmt.Errorf("failed to get feature: %w", err)
	}

	if resp.Uuid != req.Uuid {
		return ret, api.Metadata{}, fmt.Errorf("got %s uuid but requested with %s", resp.Uuid, req.Uuid)
	}

	ret.Value = FromValue(resp.Value.Value)
	ret.Timestamp = resp.Value.Timestamp.AsTime()
	ret.Fresh = resp.Value.Fresh
	return ret, FromAPIMetadata(resp.Metadata), nil
}
func (e *grpcEngine) Set(ctx context.Context, FQN string, entityID string, val any, ts time.Time) error {
	req := coreApi.SetRequest{
		Uuid:      newUUID(),
		Fqn:       FQN,
		EntityId:  entityID,
		Value:     ToAPIValue(val),
		Timestamp: timestamppb.New(ts),
	}
	resp, err := e.client.Set(ctx, &req)
	if resp.Uuid != req.Uuid {
		return fmt.Errorf("got %s uuid but requested with %s", resp.Uuid, req.Uuid)
	}
	return err
}
func (e *grpcEngine) Append(ctx context.Context, FQN string, entityID string, val any, ts time.Time) error {
	req := coreApi.AppendRequest{
		Uuid:      newUUID(),
		Fqn:       FQN,
		EntityId:  entityID,
		Value:     ToAPIScalar(val),
		Timestamp: timestamppb.New(ts),
	}
	resp, err := e.client.Append(ctx, &req)
	if resp.Uuid != req.Uuid {
		return fmt.Errorf("got %s uuid but requested with %s", resp.Uuid, req.Uuid)
	}
	return err
}
func (e *grpcEngine) Incr(ctx context.Context, FQN string, entityID string, by any, ts time.Time) error {
	req := coreApi.IncrRequest{
		Uuid:      newUUID(),
		Fqn:       FQN,
		EntityId:  entityID,
		Value:     ToAPIScalar(by),
		Timestamp: timestamppb.New(ts),
	}
	resp, err := e.client.Incr(ctx, &req)
	if resp.Uuid != req.Uuid {
		return fmt.Errorf("got %s uuid but requested with %s", resp.Uuid, req.Uuid)
	}
	return err
}
func (e *grpcEngine) Update(ctx context.Context, FQN string, entityID string, val any, ts time.Time) error {
	req := coreApi.UpdateRequest{
		Uuid:      newUUID(),
		Fqn:       FQN,
		EntityId:  entityID,
		Value:     ToAPIValue(val),
		Timestamp: timestamppb.New(ts),
	}
	resp, err := e.client.Update(ctx, &req)
	if resp.Uuid != req.Uuid {
		return fmt.Errorf("got %s uuid but requested with %s", resp.Uuid, req.Uuid)
	}
	return err
}
