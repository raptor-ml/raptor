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
	"errors"
	"github.com/raptor-ml/raptor/api"
	coreApi "go.buf.build/raptor/api-go/raptor/core/raptor/core/v1alpha1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type serviceServer struct {
	coreApi.UnimplementedEngineServiceServer
	engine api.Engine
}

// NewServiceServer creates a new coreApi.EngineServiceServer from api.Engine
func NewServiceServer(engine api.Engine) coreApi.EngineServiceServer {
	return &serviceServer{
		engine: engine,
	}
}

func (s *serviceServer) FeatureDescriptor(ctx context.Context, req *coreApi.FeatureDescriptorRequest) (*coreApi.FeatureDescriptorResponse, error) {
	fd, err := s.engine.FeatureDescriptor(ctx, req.GetFqn())
	if err != nil {
		if errors.Is(err, api.ErrFeatureNotFound) {
			return nil, status.Errorf(codes.NotFound, "feature not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to get FeatureDescriptor: %s", err)
	}
	return &coreApi.FeatureDescriptorResponse{
		Uuid:              req.GetUuid(),
		FeatureDescriptor: ToAPIFeatureDescriptor(fd),
	}, nil
}
func (s *serviceServer) Get(ctx context.Context, req *coreApi.GetRequest) (*coreApi.GetResponse, error) {
	resp, fd, err := s.engine.Get(ctx, req.GetFqn(), req.GetKeys())
	if err != nil {
		if errors.Is(err, api.ErrFeatureNotFound) {
			return nil, status.Errorf(codes.NotFound, "feature not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to get entity: %s", err)
	}

	val := resp.Value
	if r, ok := resp.Value.(api.WindowResultMap); ok {
		if len(fd.Aggr) < 1 {
			return nil, status.Errorf(codes.InvalidArgument, "the feature is windowed, but requested window function not found."+
				"please use s request with FullyQualifiedName with an aggregator i.e. `%s+%s`", req.GetFqn(), fd.Aggr[0])
		}
		if len(fd.Aggr) != 1 {
			return nil, status.Errorf(codes.InvalidArgument, "the feature is windowed, but requested window function not found."+
				"please use s request with FullyQualifiedName with an aggregator i.e. `%s+%s`", req.GetFqn(), fd.Aggr[0])
		}
		val = r[fd.Aggr[0]]
	}

	ret := &coreApi.GetResponse{
		Uuid: req.GetUuid(),
		Value: &coreApi.FeatureValue{
			Fqn:       req.GetFqn(),
			Keys:      req.GetKeys(),
			Value:     ToAPIValue(val),
			Timestamp: timestamppb.New(resp.Timestamp),
		},
		FeatureDescriptor: ToAPIFeatureDescriptor(fd),
	}

	return ret, nil
}

func (s *serviceServer) Set(ctx context.Context, req *coreApi.SetRequest) (*coreApi.SetResponse, error) {
	err := s.engine.Set(ctx, req.GetFqn(), req.GetKeys(), FromValue(req.Value), req.Timestamp.AsTime())
	if err != nil {
		if errors.Is(err, api.ErrFeatureNotFound) {
			return nil, status.Errorf(codes.NotFound, "feature not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to set entity: %s", err)
	}
	return &coreApi.SetResponse{
		Uuid:      req.GetUuid(),
		Timestamp: timestamppb.Now(),
	}, nil
}
func (s *serviceServer) Append(ctx context.Context, req *coreApi.AppendRequest) (*coreApi.AppendResponse, error) {
	err := s.engine.Append(ctx, req.GetFqn(), req.GetKeys(), fromScalar(req.Value), req.Timestamp.AsTime())
	if err != nil {
		if errors.Is(err, api.ErrFeatureNotFound) {
			return nil, status.Errorf(codes.NotFound, "feature not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to append entity: %s", err)
	}
	return &coreApi.AppendResponse{
		Uuid:      req.GetUuid(),
		Timestamp: timestamppb.Now(),
	}, nil
}
func (s *serviceServer) Incr(ctx context.Context, req *coreApi.IncrRequest) (*coreApi.IncrResponse, error) {
	err := s.engine.Incr(ctx, req.GetFqn(), req.GetKeys(), fromScalar(req.Value), req.Timestamp.AsTime())
	if err != nil {
		if errors.Is(err, api.ErrFeatureNotFound) {
			return nil, status.Errorf(codes.NotFound, "feature not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to incr entity: %s", err)
	}
	return &coreApi.IncrResponse{
		Uuid:      req.GetUuid(),
		Timestamp: timestamppb.Now(),
	}, nil
}
func (s *serviceServer) Update(ctx context.Context, req *coreApi.UpdateRequest) (*coreApi.UpdateResponse, error) {
	err := s.engine.Update(ctx, req.GetFqn(), req.GetKeys(), FromValue(req.Value), req.Timestamp.AsTime())
	if err != nil {
		if errors.Is(err, api.ErrFeatureNotFound) {
			return nil, status.Errorf(codes.NotFound, "feature not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to update entity: %s", err)
	}
	return &coreApi.UpdateResponse{
		Uuid:      req.GetUuid(),
		Timestamp: timestamppb.Now(),
	}, nil
}
