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

package runtime

import (
	"context"
	"errors"
	"fmt"
	ceProto "github.com/cloudevents/sdk-go/binding/format/protobuf/v2"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/types"
	"github.com/go-logr/logr"
	"github.com/raptor-ml/natun/api"
	"github.com/raptor-ml/natun/internal/programregistry"
	"github.com/raptor-ml/natun/pkg/protoregistry"
	"github.com/raptor-ml/natun/pkg/pyexp"
	pbRuntime "go.buf.build/natun/api-go/natun/core/natun/runtime/v1alpha1"
	"go.starlark.net/lib/proto"
	"go.starlark.net/starlark"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"net/url"
	"strings"
)

type runtime struct {
	pbRuntime.UnimplementedRuntimeServiceServer
	engine   api.Engine
	programs programregistry.Registry
	logger   logr.Logger
}

func New(engine api.Engine, programs programregistry.Registry, logger logr.Logger) pbRuntime.RuntimeServiceServer {
	return &runtime{
		engine:   engine,
		programs: programs,
		logger:   logger,
	}
}

func (r *runtime) LoadPyExpProgram(_ context.Context, req *pbRuntime.LoadPyExpProgramRequest) (*pbRuntime.LoadPyExpProgramResponse, error) {
	hash, err := r.programs.Register(req.GetProgram(), req.GetFqn())
	if err != nil && !errors.Is(err, programregistry.ErrAlreadyRegistered) {
		return nil, status.Errorf(codes.Internal, "failed to register program: %v", err)
	}
	return &pbRuntime.LoadPyExpProgramResponse{
		Uuid:        req.GetUuid(),
		ProgramHash: hash,
	}, nil
}
func (r *runtime) RegisterSchema(_ context.Context, req *pbRuntime.RegisterSchemaRequest) (*pbRuntime.RegisterSchemaResponse, error) {
	_, err := protoregistry.Register(req.GetSchema())
	if err != nil && !errors.Is(err, protoregistry.ErrAlreadyRegistered) {
		return nil, status.Errorf(codes.Internal, "failed to register schema: %v", err)
	}
	return &pbRuntime.RegisterSchemaResponse{
		Uuid: req.GetUuid(),
	}, nil
}
func (r *runtime) ExecutePyExp(ctx context.Context, req *pbRuntime.ExecutePyExpRequest) (*pbRuntime.ExecutePyExpResponse, error) {
	programRuntime, err := r.programs.Get(req.GetProgramHash())
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "program not found: %v", err)
	}

	ev := cloudevents.NewEvent()
	req.GetData()
	err = ceProto.Protobuf.Unmarshal(req.GetData().Value, &ev)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "failed to unmarshal event: %v", err)
	}

	payload, err := r.pyExpPayload(&ev)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get payload: %v", err)
	}

	headers := map[string][]string{}
	if h, ok := ev.Extensions()["headers"]; ok {
		u, err := types.ToURL(h)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to get headers: %v", err)
		}
		headers = u.Query()
	}
	headers["X-SOURCE"] = []string{ev.Source()}
	headers["X-SUBJECT"] = []string{ev.Subject()}
	headers["X-ID"] = []string{ev.ID()}

	ret, err := programRuntime.ExecWithEngine(ctx, pyexp.ExecRequest{
		Headers:   headers,
		Payload:   payload,
		EntityID:  req.GetEntityId(),
		Timestamp: ev.Time(),
		Logger:    r.logger.WithName(req.Fqn),
	}, r.engine)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to execute program: %v", err)
	}

	if ret.Value != nil {
		if ret.EntityID == "" && req.GetEntityId() == "" {
			return nil, fmt.Errorf("you must return entity_id is when returning from this expression")
		}
		err := r.engine.Update(ctx, req.GetFqn(), ret.EntityID, ret.Value, ret.Timestamp)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to update entity: %v", err)
		}
	}

	return &pbRuntime.ExecutePyExpResponse{
		Uuid: req.GetUuid(),
	}, nil
}

func (r *runtime) pyExpPayload(ev *cloudevents.Event) (starlark.Value, error) {
	schema := ev.DataSchema()
	if schema == "" {
		return starlark.String(ev.Data()), nil
	}

	u, err := url.Parse(schema)
	if err != nil {
		return nil, fmt.Errorf("failed to parse data schema: %w", err)
	}

	md, err := protoregistry.GetDescriptor(u.Fragment)
	if err != nil {
		if !errors.Is(err, protoregistry.ErrNotFound) {
			return nil, fmt.Errorf("failed to find proto type for message")
		}

		pack, err := protoregistry.Register(schema)
		if err != nil && !errors.Is(err, protoregistry.ErrAlreadyRegistered) {
			return nil, fmt.Errorf("failed to register proto type: %w", err)
		}

		s := u.Fragment
		if strings.Count(s, ".") < 1 {
			s = fmt.Sprintf("%s.%s", pack, u.Fragment)
		}
		md, err = protoregistry.GetDescriptor(s)
		if err != nil {
			panic(fmt.Errorf("failed to get a schema that was just registered: %w", err))
		}
	}

	payload, err := proto.Unmarshal(md, ev.Data())
	if err != nil {
		return nil, fmt.Errorf("failed to parse message to proto: %w", err)
	}
	return payload, nil
}
