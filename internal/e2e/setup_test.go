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
	"github.com/vladimirvivien/gexe"
	"io/fs"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/klient/decoder"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/klient/k8s/resources"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/envfuncs"
	"sigs.k8s.io/e2e-framework/pkg/features"
	"sigs.k8s.io/e2e-framework/third_party/helm"
	"strings"
	"testing"
	"time"

	_ "github.com/raptor-ml/raptor/internal/plugins"
)

type redisContextKey string
type raptorContextKey string
type extraCfgContextKey int

type extraCfg struct {
	buildTag    string
	imgBasename string
	clusterName string
}

func SetupCfg(c extraCfg) env.Func {
	return func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
		return context.WithValue(ctx, extraCfgContextKey(1), c), nil
	}
}
func getExtraCfg(ctx context.Context) extraCfg {
	return ctx.Value(extraCfgContextKey(1)).(extraCfg)
}

func SetupRedis(name string) env.Func {
	return func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
		ns := envconf.RandomName(name, 32)
		ctx, err := envfuncs.CreateNamespace(ns)(ctx, cfg)
		if err != nil {
			return ctx, fmt.Errorf("failed to create redis namespace: %w", err)
		}

		manager := helm.New(cfg.KubeconfigFile())
		err = manager.RunRepo(helm.WithArgs("add", "bitnami", "https://charts.bitnami.com/bitnami"))
		if err != nil {
			return ctx, fmt.Errorf("failed to add bitnami chart repo: %w", err)
		}
		err = manager.RunRepo(helm.WithArgs("update"))
		if err != nil {
			return ctx, fmt.Errorf("failed to update chart repo: %w", err)
		}
		err = manager.RunInstall(helm.WithName(name),
			helm.WithReleaseName("bitnami/redis"),
			helm.WithNamespace(ns),
			helm.WithArgs(
				"--set", "replica.replicaCount=1",
				"--set", "architecture=standalone",
				"--set", "auth.enabled=false",
			),
		)
		if err != nil {
			return ctx, fmt.Errorf("failed to install redis: %w", err)
		}

		ss := &appsv1.StatefulSet{
			ObjectMeta: v1.ObjectMeta{
				Name:      fmt.Sprintf("%s-master", name),
				Namespace: ns,
			},
		}
		err = wait.For(conditions.New(cfg.Client().Resources()).ResourceScaled(ss, func(object k8s.Object) int32 {
			return object.(*appsv1.StatefulSet).Status.ReadyReplicas
		}, 1), wait.WithTimeout(5*time.Minute))
		if err != nil {
			return ctx, fmt.Errorf("failed to wait for redis to be ready: %w", err)
		}

		return context.WithValue(ctx, redisContextKey(name), ss.ObjectMeta), nil
	}
}

func FeatureEnvFn(fn env.Func) features.Func {
	return func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
		ctx, err := fn(ctx, cfg)
		if err != nil {
			t.Error(err)
			t.Fail()
		}
		return ctx
	}
}

func SetupCoreFromCtx(name string, args ...string) env.Func {
	return func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
		c := getExtraCfg(ctx)
		return SetupCore(name, c.clusterName, c.imgBasename, c.buildTag, args)(ctx, cfg)
	}
}

func FailedCoreLogs(name string) features.Func {
	return func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
		if t.Failed() {
			ns := ctx.Value(raptorContextKey(name))
			if ns == nil {
				t.Log("no raptor-core namespace found")
				return ctx
			}
			ctx, err := CollectNamespaceLogs(ns.(string), -1)(ctx, cfg)
			if err != nil {
				t.Error(err)
			}
			return ctx
		}
		return ctx
	}
}
func SetupCore(name, kindClusterName, imgBasename, buildTag string, args []string) env.Func {
	return func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
		args = append(args, "--usage-reporting=false")

		// namespace will be created via the kustomize scripts
		ns := envconf.RandomName(fmt.Sprintf("%s-raptor-core", name), 32)

		// Create redis
		redisName := fmt.Sprintf("%s-redis", name)
		ctx, err := SetupRedis(redisName)(ctx, cfg)
		if err != nil {
			return ctx, fmt.Errorf("failed to setup redis: %w", err)
		}
		redisSvc := ctx.Value(redisContextKey(redisName)).(v1.ObjectMeta)
		args := append(args, fmt.Sprintf("--redis=%s.%s:6379", redisSvc.Name, redisSvc.Namespace))

		// Upload images to the registry
		coreImg := fmt.Sprintf("%s-core:%s", imgBasename, buildTag)
		ctx, err = envfuncs.LoadDockerImageToCluster(kindClusterName, coreImg)(ctx, cfg)
		if err != nil {
			return ctx, fmt.Errorf("failed to load core image: %w", err)
		}

		historianImg := fmt.Sprintf("%s-historian:%s", imgBasename, buildTag)
		ctx, err = envfuncs.LoadDockerImageToCluster(kindClusterName, historianImg)(ctx, cfg)
		if err != nil {
			return ctx, fmt.Errorf("failed to load core image: %w", err)
		}

		runtimeImg := fmt.Sprintf("%s-runtime:%s", imgBasename, buildTag)
		ctx, err = envfuncs.LoadDockerImageToCluster(kindClusterName, runtimeImg)(ctx, cfg)
		if err != nil {
			return ctx, fmt.Errorf("failed to load core image: %w", err)
		}

		// Create the core
		r, err := resources.New(cfg.Client().RESTConfig())
		if err != nil {
			return ctx, fmt.Errorf("failed to create resources client: %w", err)
		}

		// For some reason reading from .Out() doesn't work :O
		rdr := strings.NewReader(gexe.New().RunProc("kustomize build ../../config/default/base").Result())

		err = decoder.DecodeEach(
			ctx,
			rdr,
			decoder.CreateHandler(r),
			MutateRaptorKustomize(ns, coreImg, historianImg, args...),
		)
		if err != nil {
			return ctx, fmt.Errorf("failed to install Core: %w", err)
		}

		dep := &appsv1.Deployment{
			ObjectMeta: v1.ObjectMeta{
				Name:      "raptor-controller-core",
				Namespace: ns,
			},
		}
		err = wait.For(conditions.New(cfg.Client().Resources()).ResourceScaled(dep, func(object k8s.Object) int32 {
			return object.(*appsv1.Deployment).Status.ReadyReplicas
		}, 3), wait.WithTimeout(5*time.Minute))
		if err != nil {
			return ctx, fmt.Errorf("failed to wait for Core to be ready: %w", err)
		}

		return context.WithValue(ctx, raptorContextKey(name), ns), nil
	}
}

// MutateRaptorKustomize is an optional parameter to decoding functions that will patch objects with the given namespace name
func MutateRaptorKustomize(ns string, coreImg string, historianImg string, args ...string) decoder.DecodeOption {
	return decoder.MutateOption(func(obj k8s.Object) error {
		// rename namespace
		obj.SetNamespace(ns)
		if obj.GetObjectKind().GroupVersionKind().Kind == "Namespace" && obj.GetName() == "raptor-system" {
			obj.SetName(ns)
			return nil
		}
		if obj.GetObjectKind().GroupVersionKind().Kind == "ClusterRoleBinding" {
			crb := obj.(*rbacv1.ClusterRoleBinding)
			for i, ref := range crb.Subjects {
				if ref.Kind == "ServiceAccount" && ref.Namespace == "raptor-system" {
					crb.Subjects[i].Namespace = ns
				}
			}
		}
		if obj.GetObjectKind().GroupVersionKind().Kind == "RoleBinding" {
			crb := obj.(*rbacv1.RoleBinding)
			for i, ref := range crb.Subjects {
				if ref.Kind == "ServiceAccount" && ref.Namespace == "raptor-system" {
					crb.Subjects[i].Namespace = ns
				}
			}
		}
		if obj.GetObjectKind().GroupVersionKind().Kind == "MutatingWebhookConfiguration" {
			mwc := obj.(*admissionregistrationv1.MutatingWebhookConfiguration)
			for i, rule := range mwc.Webhooks {
				if rule.ClientConfig.Service.Namespace == "raptor-system" {
					mwc.Webhooks[i].ClientConfig.Service.Namespace = ns
				}
			}
		}
		if obj.GetObjectKind().GroupVersionKind().Kind == "ValidatingWebhookConfiguration" {
			vwc := obj.(*admissionregistrationv1.ValidatingWebhookConfiguration)
			for i, rule := range vwc.Webhooks {
				if rule.ClientConfig.Service.Namespace == "raptor-system" {
					vwc.Webhooks[i].ClientConfig.Service.Namespace = ns
				}
			}
		}

		if obj.GetObjectKind().GroupVersionKind().Kind == "Deployment" {
			dep := obj.(*appsv1.Deployment)
			if dep.GetName() == "raptor-controller-core" {
				for i, c := range dep.Spec.Template.Spec.Containers {
					if c.Name == "core" {
						dep.Spec.Template.Spec.Containers[i].Image = coreImg
						dep.Spec.Template.Spec.Containers[i].Args = append(c.Args, args...)
					}
				}
			}
			if dep.GetName() == "raptor-historian" {
				for i, c := range dep.Spec.Template.Spec.Containers {
					if c.Name == "historian" {
						dep.Spec.Template.Spec.Containers[i].Image = historianImg
						dep.Spec.Template.Spec.Containers[i].Args = append(c.Args, args...)
					}
				}

				//TODO remove this once we have historian tests
				zero := int32(0)
				dep.Spec.Replicas = &zero
			}
		}
		return nil
	})
}
func DestroyCore(name string) env.Func {
	return func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
		redisNs := ctx.Value(redisContextKey(fmt.Sprintf("%s-redis", name))).(v1.ObjectMeta)
		ctx, err := envfuncs.DeleteNamespace(redisNs.Namespace)(ctx, cfg)

		ns := ctx.Value(raptorContextKey(name)).(string)
		ctx, err = envfuncs.DeleteNamespace(ns)(ctx, cfg)
		if err != nil {
			return ctx, fmt.Errorf("failed to delete Core namespace: %w", err)
		}
		return ctx, nil
	}
}

type filerFunc func(string) bool

func FilterKustomize(f string) bool {
	return !(f == "kustomization.yaml" || f == "kustomization.yml")
}
func DecodeEachFileWithFilter(ctx context.Context, fsys fs.FS, ff filerFunc, handlerFn decoder.HandlerFunc, options ...decoder.DecodeOption) error {
	files, err := fs.Glob(fsys, "*")
	if err != nil {
		return err
	}
	for _, file := range files {
		if !ff(file) {
			continue
		}

		f, err := fsys.Open(file)
		if err != nil {
			return err
		}
		defer f.Close()
		if err := decoder.DecodeEach(ctx, f, handlerFn, options...); err != nil {
			return err
		}
		if err := f.Close(); err != nil {
			return err
		}
	}
	return nil
}
