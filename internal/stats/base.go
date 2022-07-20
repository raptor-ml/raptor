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

package stats

// +kubebuilder:rbac:groups="",resources=nodes,verbs=list;watch;get
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch

import (
	"context"
	"crypto/sha256"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/push"
	"github.com/raptor-ml/raptor/internal/version"
	coreV1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"net/http"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
	"time"
)

const (
	coreSubsystemKey = "core"
	usageAPI         = "https://usage.raptor.ml"
	pushPeriod       = time.Hour
)

var (
	UID        string
	InstanceID string
)

func hash(s string) string {
	h := sha256.New()
	h.Write([]byte(s))
	return fmt.Sprintf("%x", h.Sum(nil))
}

func init() {
	if InstanceID == "" {
		hn, err := os.Hostname()
		if err == nil {
			InstanceID = hash(hn)
		} else {
			InstanceID = uuid.NewString()
		}
	}
}

func Run(cfg *rest.Config, kc client.Client, usageReporting bool, logger logr.Logger) NoLeaderRunnableFunc {
	return func(ctx context.Context) error {
		// Kubernetes server version
		k8sVer, err := k8sVersion(cfg)
		if err != nil {
			return err
		}

		// Kubernetes nodes
		var nl coreV1.NodeList
		err = kc.List(ctx, &nl)
		nodes := -1
		if err == nil {
			nodes = len(nl.Items)
		}

		metrics.Registry.MustRegister(prometheus.NewGaugeFunc(
			prometheus.GaugeOpts{
				Name: "metadata_info",
				Help: "Metadata information includes the version of the application, the instance id and the uid",
				ConstLabels: prometheus.Labels{
					"version":      version.Version,
					"go_version":   version.GoVersion,
					"architecture": version.Architecture,
					"os":           version.OS,
					"k8s_version":  k8sVer,
					"node_count":   fmt.Sprintf("%d", nodes),
				},
			},
			func() float64 { return 1 },
		))

		if !usageReporting {
			return nil
		}

		wait.UntilWithContext(ctx, func(ctx context.Context) {
			pr := push.New(usageAPI, "core").
				Gatherer(metrics.Registry).
				Client(&http.Client{Timeout: 5 * time.Minute}).
				Grouping("instance_id", InstanceID)
			if UID != "" {
				pr.Grouping("uid", UID)
			} else {
				pr.Grouping("anon_id", getAnonID(kc))
			}
			if err := pr.Push(); err != nil {
				logger.V(-1).Info("failed to push metrics to usage server", "err", err)
			}
		}, pushPeriod)
		return nil
	}
}

func k8sVersion(cfg *rest.Config) (string, error) {
	dc, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return "", fmt.Errorf("failed to create discovery client: %w", err)
	}
	ver, err := dc.ServerVersion()
	if err != nil {
		return "", fmt.Errorf("failed to get k8s version: %w", err)
	}
	return fmt.Sprintf("%s.%s (%s) / %s", ver.Major, ver.Minor, ver.GitVersion, ver.Platform), nil
}
