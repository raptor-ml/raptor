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
	"fmt"
	"github.com/raptor-ml/raptor/api"
	manifests "github.com/raptor-ml/raptor/api/v1alpha1"
	"github.com/vladimirvivien/gexe"
	"sigs.k8s.io/e2e-framework/klient/decoder"
	"sigs.k8s.io/e2e-framework/klient/k8s/resources"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/envfuncs"
	"sigs.k8s.io/e2e-framework/pkg/features"
	"strings"
	"testing"
	"time"
)

func TestSamples(t *testing.T) {
	namespace := "samples"
	feature := features.New("Install the sample features").
		Setup(FeatureEnvFn(envfuncs.CreateNamespace(namespace))).
		Teardown(FeatureEnvFn(envfuncs.DeleteNamespace(namespace))).
		Setup(func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
			r, err := resources.New(c.Client().RESTConfig())
			if err != nil {
				t.Errorf("failed to create resources client: %s", err)
				t.Fail()
				return ctx
			}
			r = r.WithNamespace(namespace)

			err = manifests.AddToScheme(r.GetScheme())
			if err != nil {
				t.Errorf("failed to add manifests to scheme: %s", err)
				t.Fail()
				return ctx
			}

			// Use kustomize to preserve the order of the manifests. This is important because the DataSources must be created before the Features.
			rdr := strings.NewReader(gexe.New().RunProc("kustomize build ../../config/samples/").Result())
			err = decoder.DecodeEach(
				ctx,
				rdr,
				decoder.CreateHandler(r),
				decoder.MutateNamespace(namespace),
			)

			if err != nil {
				t.Errorf("failed to decode samples: %s", err)
				t.FailNow()
				return ctx
			}
			// wait for the resources to be created.
			// This is so fast that it's not really required, but it's just safer to do that than rare race-condition failures.
			time.Sleep(time.Second)
			return ctx
		}).
		Assess("Check If Resource created", func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
			r, err := resources.New(c.Client().RESTConfig())
			if err != nil {
				t.Errorf("failed to create resources client: %s", err)
				t.Fail()
				return ctx
			}

			r.WithNamespace(namespace)
			err = manifests.AddToScheme(r.GetScheme())
			if err != nil {
				t.Errorf("failed to add manifests to scheme: %s", err)
				t.Fail()
				return ctx
			}

			cr := &manifests.Feature{}
			err = r.Get(ctx, "hello-world", namespace, cr)
			if err != nil {
				t.Errorf("failed to get hello-world: %s", err)
				t.FailNow()
			}

			sdkClient, err := CreateSDK()
			if err != nil {
				t.Errorf("failed to create sdk client: %s", err)
				t.FailNow()
			}

			keys := api.Keys{"name": "test"}
			v, _, err := sdkClient.Get(ctx, fmt.Sprintf("%s.hello_world", namespace), keys)
			if err != nil {
				t.Errorf("failed to get feature value: %s", err)
				t.FailNow()
				return ctx
			}
			if v.Value != fmt.Sprintf("Hello world %s", keys["name"]) {
				t.Errorf("unexpected value: %v", v)
				t.FailNow()
				return ctx
			}
			t.Log("CR Details", "cr", cr)
			return ctx
		}).Feature()

	testEnv.Test(t, feature)
}
