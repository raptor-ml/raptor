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

package runtimemanager

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	grpcMiddleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpcRetry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	"github.com/raptor-ml/raptor/api"
	"github.com/raptor-ml/raptor/pkg/sdk"
	coreApi "go.buf.build/raptor/api-go/raptor/core/raptor/core/v1alpha1"
	runtimeApi "go.buf.build/raptor/api-go/raptor/core/raptor/runtime/v1alpha1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/local"
	"google.golang.org/protobuf/types/known/timestamppb"
	v1 "k8s.io/api/core/v1"
	"os"
	"path/filepath"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

type manager struct {
	environments map[string]v1.Container
	defaultEnv   string
}

func New(k client.Client, namespace string, podname string) (api.RuntimeManager, error) {
	rm := &manager{
		environments: make(map[string]v1.Container),
	}

	if k == nil && os.Getenv("DEFAULT_RUNTIME") != "" {
		rm.defaultEnv = os.Getenv("DEFAULT_RUNTIME")

		// discover runtimes by unix sockets
		path := "/tmp/raptor/runtime/"
		suffix := ".sock"
		matches, err := filepath.Glob(fmt.Sprintf("%s*%s", path, suffix))
		if err != nil {
			return nil, fmt.Errorf("failed to discover runtimes: %w", err)
		}
		for _, v := range matches {
			rm.environments[v[len(path):len(v)-len(suffix)]] = v1.Container{}
		}

		return rm, nil
	}

	pod := &v1.Pod{}
	err := k.Get(context.Background(), client.ObjectKey{
		Namespace: namespace,
		Name:      podname,
	}, pod)

	if err != nil {
		return nil, fmt.Errorf("failed to get pod: %w", err)
	}

	firstRt := -1
	for i, c := range pod.Spec.Containers {
		name := ""
		for _, e := range c.Env {
			if e.Name == "RUNTIME_NAME" {
				name = e.Value
			}
		}
		if name == "" {
			continue
		}
		if firstRt == -1 {
			firstRt = i
		}
		rm.environments[name] = c
		if name == "default" {
			rm.defaultEnv = name
		}
	}
	if rm.defaultEnv == "" {
		if os.Getenv("DEFAULT_RUNTIME") != "" {
			rm.defaultEnv = os.Getenv("DEFAULT_RUNTIME")
		} else {
			for _, e := range pod.Spec.Containers[firstRt].Env {
				if e.Name == "RUNTIME_NAME" {
					rm.defaultEnv = e.Value
					break
				}
			}
		}
	}

	return rm, nil
}

func (m *manager) LoadProgram(env, fqn, program string, packages []string) error {
	rt, err := m.getRuntime(env)
	if err != nil {
		return fmt.Errorf("failed to get runtime: %w", err)
	}

	req := &runtimeApi.LoadProgramRequest{
		Uuid:     uuid.NewString(),
		Fqn:      fqn,
		Program:  program,
		Packages: packages,
	}
	resp, err := rt.LoadProgram(context.Background(), req)
	if err != nil {
		return fmt.Errorf("failed to load program: %w", err)
	}
	if resp.Uuid != req.Uuid {
		return fmt.Errorf("uuid mismatch")
	}
	return nil
}

func (m *manager) ExecuteProgram(env string, fqn string, keys api.Keys, row map[string]any, ts time.Time) (api.Value, api.Keys, error) {
	rt, err := m.getRuntime(env)
	if err != nil {
		return api.Value{}, keys, fmt.Errorf("failed to get runtime: %w", err)
	}

	data := make(map[string]*coreApi.Value)
	for k, v := range row {
		data[k] = sdk.ToAPIValue(v)
	}
	req := &runtimeApi.ExecuteProgramRequest{
		Uuid:      uuid.NewString(),
		Fqn:       fqn,
		Keys:      keys,
		Data:      data,
		Timestamp: timestamppb.New(ts),
	}
	resp, err := rt.ExecuteProgram(context.Background(), req)
	if err != nil {
		return api.Value{}, keys, fmt.Errorf("failed to execute program: %w", err)
	}
	if resp.Uuid != req.Uuid {
		return api.Value{}, keys, fmt.Errorf("uuid mismatch")
	}

	if resp.Timestamp.CheckValid() == nil && !resp.Timestamp.AsTime().IsZero() {
		ts = resp.Timestamp.AsTime()
	}
	if resp.Keys != nil && len(resp.Keys) > 0 {
		keys = resp.Keys
	}

	return api.Value{
		Value:     sdk.FromValue(resp.Result),
		Timestamp: ts,
		Fresh:     true,
	}, keys, nil
}

func (m *manager) getRuntime(name string) (runtimeApi.RuntimeServiceClient, error) {
	if name == "" {
		name = m.defaultEnv
	}
	if _, ok := m.environments[name]; !ok {
		return nil, fmt.Errorf("runtime %s not found", name)
	}
	socket := fmt.Sprintf("/tmp/raptor/runtime/%s.sock", name)

	// check if the socket exists
	if _, err := os.Stat(socket); os.IsNotExist(err) {
		return nil, fmt.Errorf("socket %s not found", socket)
	}

	cc, err := grpc.Dial(
		fmt.Sprintf("unix://%s", socket),
		grpc.WithStreamInterceptor(grpcMiddleware.ChainStreamClient(
			grpcRetry.StreamClientInterceptor(),
		)),
		grpc.WithUnaryInterceptor(grpcMiddleware.ChainUnaryClient(
			grpcRetry.UnaryClientInterceptor(),
		)),
		grpc.WithTransportCredentials(local.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to dial socket: %w", err)
	}
	return runtimeApi.NewRuntimeServiceClient(cc), nil
}

func (m *manager) GetSidecars() []v1.Container {
	// convert map to array
	s := make([]v1.Container, 0, len(m.environments))
	for _, c := range m.environments {
		s = append(s, c)
	}
	return s
}

func (m *manager) GetDefaultEnv() string {
	return m.defaultEnv
}
