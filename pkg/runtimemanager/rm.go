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
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"os"
	"path/filepath"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sync"
	"time"
)

// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch

type runtime struct {
	environments map[string]v1.Container
	defaultEnv   string
	conns        sync.Map
}

func New(mgr manager.Manager, namespace string, podname string) (api.RuntimeManager, error) {
	rm := &runtime{
		environments: make(map[string]v1.Container),
		conns:        sync.Map{},
	}

	checked := false
check:
	if mgr == nil || os.Getenv("DEFAULT_RUNTIME") != "" {
		checked = true

		time.Sleep(2 * time.Second) // wait for the runtimes to start
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

		if len(matches) == 0 {
			if checked {
				return nil, fmt.Errorf("no runtimes found")
			}
			time.Sleep(5 * time.Second)
			goto check
		}

		return rm, nil
	}

	k, err := client.New(mgr.GetConfig(), client.Options{
		Scheme: mgr.GetScheme(),
		Mapper: mgr.GetRESTMapper(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	pod := &v1.Pod{}

	if podname == "" {
		// guess from the deployment
		// this is useful for localhost development
		deploy := &appsv1.Deployment{}
		err := k.Get(context.Background(), client.ObjectKey{
			Namespace: namespace,
			Name:      "raptor-controller-core",
		}, deploy)
		if err != nil {
			return nil, fmt.Errorf("failed to get controller deployment: %w", err)
		}
		pod.Spec = deploy.Spec.Template.Spec
	} else {
		err = k.Get(context.Background(), client.ObjectKey{
			Namespace: namespace,
			Name:      podname,
		}, pod)

		if err != nil {
			return nil, fmt.Errorf("failed to get pod: %w", err)
		}
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

	if len(rm.environments) == 0 {
		return nil, fmt.Errorf("no runtimes found")
	}

	return rm, nil
}

func (r *runtime) LoadProgram(env, fqn, program string, packages []string) (*api.ParsedProgram, error) {
	rt, err := r.getRuntime(env)
	if err != nil {
		return nil, fmt.Errorf("failed to get runtime: %w", err)
	}

	req := &runtimeApi.LoadProgramRequest{
		Uuid:     uuid.NewString(),
		Fqn:      fqn,
		Program:  program,
		Packages: packages,
	}
	resp, err := rt.LoadProgram(context.Background(), req)
	if err != nil {
		return nil, fmt.Errorf("failed to load program: %w", err)
	}
	if resp.Uuid != req.Uuid {
		return nil, fmt.Errorf("uuid mismatch")
	}

	pp := &api.ParsedProgram{
		Primitive: sdk.FromAPIPrimitive(resp.Primitive),
	}
	for _, effect := range resp.GetSideEffects() {
		if effect.GetKind() == "get_feature" {
			dFqn := ""
			if v, ok := effect.GetArgs()["fqn"]; ok {
				dFqn = v
			} else {
				if v, ok := effect.GetArgs()["1"]; ok {
					dFqn = v
				} else {
					return nil, fmt.Errorf("failed to get_feature fqn")
				}
			}
			pp.Dependencies = append(pp.Dependencies, dFqn)
		}
	}

	return pp, nil
}

func (r *runtime) ExecuteProgram(env string, fqn string, keys api.Keys, row map[string]any, ts time.Time) (api.Value, api.Keys, error) {
	rt, err := r.getRuntime(env)
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

func (r *runtime) getRuntime(name string) (runtimeApi.RuntimeServiceClient, error) {
	if name == "" {
		name = r.defaultEnv
	}
	if rt, ok := r.conns.Load(name); ok && rt != nil {
		return rt.(runtimeApi.RuntimeServiceClient), nil
	}
	if _, ok := r.environments[name]; !ok {
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

	rt := runtimeApi.NewRuntimeServiceClient(cc)
	r.conns.Store(name, rt)

	return rt, nil
}

func (r *runtime) GetSidecars() []v1.Container {
	// convert map to array
	s := make([]v1.Container, 0, len(r.environments))
	for _, c := range r.environments {
		s = append(s, c)
	}
	return s
}

func (r *runtime) GetDefaultEnv() string {
	return r.defaultEnv
}
