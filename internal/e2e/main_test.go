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
	"flag"
	"k8s.io/klog/v2"
	"os"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/envfuncs"
	"testing"
)

var (
	testEnv env.Environment
)

func TestMain(m *testing.M) {
	buildTag := flag.String("build-tag", "", "The docker image tag that used when testing")
	imgBasename := flag.String("image-basename", "ghcr.io/raptor-ml/raptor", "The base name for docker images")
	cfg, _ := envconf.NewFromFlags()

	if *buildTag == "" {
		klog.Fatal("--build-tag argument is required. (or as environment variable BUILD_TAG)")
	}

	testEnv = env.NewWithConfig(cfg)
	kindClusterName := envconf.RandomName("raptor-test", 16)

	testEnv.Setup(
		SetupCfg(extraCfg{
			buildTag:    *buildTag,
			imgBasename: *imgBasename,
			clusterName: kindClusterName,
		}),
		envfuncs.CreateKindClusterWithConfig(kindClusterName, "''", "./kind-cluster.yaml"),
		envfuncs.SetupCRDs("../../config/crd/bases", "*"),
		SetupCore("system", kindClusterName, *imgBasename, *buildTag),
	)

	testEnv.Finish(
		CollectNamespaceLogsWithNamespaceFn(m, func(ctx context.Context) string {
			return ctx.Value(raptorContextKey("system")).(string)
		}, -1),
		DestroyCore("system"),
		envfuncs.TeardownCRDs("../../config/crd/bases", "*"),
		envfuncs.DestroyKindCluster(kindClusterName),
	)

	os.Exit(testEnv.Run(m))
}
