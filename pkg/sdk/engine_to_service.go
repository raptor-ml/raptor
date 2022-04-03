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
	"github.com/natun-ai/natun/pkg/api"
	coreApi "github.com/natun-ai/natun/proto/gen/go/natun/core/v1alpha1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type serviceServer struct {
	engine api.Engine
	coreApi.UnimplementedEngineServiceServer
}

// NewServiceServer creates a new coreApi.EngineServiceServer from api.Engine
func NewServiceServer(engine api.Engine) coreApi.EngineServiceServer {
	return &serviceServer{
		engine: engine,
	}
}

func (s *serviceServer) Metadata(ctx context.Context, req *coreApi.MetadataRequest) (*coreApi.MetadataResponse, error) {
	md, err := s.engine.Metadata(ctx, req.GetFqn())
	if err != nil {
		return nil, fmt.Errorf("failed to get metadata: %w", err)
	}
	return &coreApi.MetadataResponse{
		Uuid:     req.GetUuid(),
		Metadata: ToAPIMetadata(md),
	}, nil
}
func (s *serviceServer) Get(ctx context.Context, req *coreApi.GetRequest) (*coreApi.GetResponse, error) {
	resp, md, err := s.engine.Get(ctx, req.GetFqn(), req.GetEntityId())
	if err != nil {
		return nil, err
	}

	val := resp.Value
	if r, ok := resp.Value.(api.WindowResultMap); ok {
		if len(md.Aggr) > 1 {
			return nil, fmt.Errorf("the feature is windowed, but requested window function not found."+
				"pleasue use s request with FullyQualifiedName with an aggregator i.e. `%s[%s]`", req.GetFqn(), md.Aggr[0])
		}
		val = r[md.Aggr[0]]
	}

	ret := &coreApi.GetResponse{
		Uuid: req.GetUuid(),
		Value: &coreApi.FeatureValue{
			Fqn:       req.GetFqn(),
			EntityId:  req.GetEntityId(),
			Value:     ToAPIValue(val),
			Timestamp: timestamppb.New(resp.Timestamp),
		},
		Metadata: ToAPIMetadata(md),
	}

	return ret, nil
}

func (s *serviceServer) HistoricalGet(ctx context.Context, req *coreApi.HistoricalGetRequest) (*coreApi.HistoricalGetResponse, error) {
	_, _ = ctx, req
	return nil, nil // TODO
}
func (s *serviceServer) Set(ctx context.Context, req *coreApi.SetRequest) (*coreApi.SetResponse, error) {
	err := s.engine.Set(ctx, req.GetFqn(), req.GetEntityId(), FromValue(req.Value), req.Timestamp.AsTime())
	if err != nil {
		return nil, err
	}
	return &coreApi.SetResponse{
		Uuid:      req.GetUuid(),
		Timestamp: timestamppb.Now(),
	}, nil
}
func (s *serviceServer) Append(ctx context.Context, req *coreApi.AppendRequest) (*coreApi.AppendResponse, error) {
	err := s.engine.Append(ctx, req.GetFqn(), req.GetEntityId(), fromScalar(req.Value), req.Timestamp.AsTime())
	if err != nil {
		return nil, err
	}
	return &coreApi.AppendResponse{
		Uuid:      req.GetUuid(),
		Timestamp: timestamppb.Now(),
	}, nil
}
func (s *serviceServer) Incr(ctx context.Context, req *coreApi.IncrRequest) (*coreApi.IncrResponse, error) {
	err := s.engine.Incr(ctx, req.GetFqn(), req.GetEntityId(), fromScalar(req.Value), req.Timestamp.AsTime())
	if err != nil {
		return nil, err
	}
	return &coreApi.IncrResponse{
		Uuid:      req.GetUuid(),
		Timestamp: timestamppb.Now(),
	}, nil
}
func (s *serviceServer) Update(ctx context.Context, req *coreApi.UpdateRequest) (*coreApi.UpdateResponse, error) {
	err := s.engine.Update(ctx, req.GetFqn(), req.GetEntityId(), FromValue(req.Value), req.Timestamp.AsTime())
	if err != nil {
		return nil, err
	}
	return &coreApi.UpdateResponse{
		Uuid:      req.GetUuid(),
		Timestamp: timestamppb.Now(),
	}, nil
}
