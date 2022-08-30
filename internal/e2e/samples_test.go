//go:build e2e
// +build e2e

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

package e2e

import (
	"context"
	manifests "github.com/raptor-ml/raptor/api/v1alpha1"
	"os"
	"sigs.k8s.io/e2e-framework/pkg/envfuncs"
	"testing"

	"k8s.io/klog/v2"
	"sigs.k8s.io/e2e-framework/klient/decoder"
	"sigs.k8s.io/e2e-framework/klient/k8s/resources"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

func TestCRDSetup(t *testing.T) {
	namespace := "samples"
	feature := features.New("Install the samples").
		Setup(FeatureEnvFn(envfuncs.CreateNamespace(namespace))).
		Teardown(FeatureEnvFn(envfuncs.DeleteNamespace(namespace))).
		Setup(FeatureEnvFn(SetupCoreFromCtx(namespace))).
		Teardown(FeatureEnvFn(DestroyCore(namespace))).
		Setup(func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
			r, err := resources.New(c.Client().RESTConfig())
			if err != nil {
				t.Errorf("failed to create resources client: %s", err)
				t.Fail()
			}
			r = r.WithNamespace(namespace)

			manifests.AddToScheme(r.GetScheme())
			err = DecodeEachFileWithFiler(
				ctx, os.DirFS("../../config/samples/"), FilterKustomize,
				decoder.CreateHandler(r),
				decoder.MutateNamespace(namespace),
			)
			if err != nil {
				t.Errorf("failed to decode samples: %s", err)
				t.Fail()
			}
			return ctx
		}).
		Assess("Check If Resource created", func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
			r, err := resources.New(c.Client().RESTConfig())
			if err != nil {
				t.Errorf("failed to create resources client: %s", err)
				t.Fail()
			}

			r.WithNamespace(namespace)
			err = manifests.AddToScheme(r.GetScheme())
			if err != nil {
				t.Errorf("failed to add manifests to scheme: %s", err)
				t.Fail()
			}

			ct := &manifests.Feature{}
			err = r.Get(ctx, "hello-world", namespace, ct)
			if err != nil {
				t.Errorf("failed to get hello-world: %s", err)
				t.Fail()
			}

			klog.InfoS("CR Details", "cr", ct)
			return ctx
		}).Feature()

	testEnv.Test(t, feature)
}
