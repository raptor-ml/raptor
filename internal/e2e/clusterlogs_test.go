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
	"bufio"
	"context"
	"fmt"
	"io"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"time"
)

func CollectNamespaceLogs(ns string, since time.Duration) env.Func {
	return func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
		cs, err := kubernetes.NewForConfig(cfg.Client().RESTConfig())
		if err != nil {
			return nil, err
		}

		pods, err := cs.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, err
		}

		for _, pod := range pods.Items {
			for _, container := range pod.Spec.Containers {
				go func(pod corev1.Pod, container corev1.Container) {
					sinceTime := &metav1.Time{Time: time.Now().Add(-since)}
					if since <= 0 {
						sinceTime.Time = pod.GetCreationTimestamp().Time
					}
					rdr, err := cs.CoreV1().Pods(pod.GetNamespace()).GetLogs(pod.GetName(), &corev1.PodLogOptions{
						Follow:    false,
						Container: container.Name,
						SinceTime: sinceTime,
					}).Stream(ctx)

					logger := klog.Background().V(4).WithValues("pod", pod.GetName(), "container", container.Name)

					if err != nil {
						logger.Error(err, "failed to get logs")
						return
					}

					defer rdr.Close()

					r := bufio.NewReader(rdr)
					for {
						select {
						case <-ctx.Done():
							return
						default:
						}
						text, err := r.ReadBytes('\n')
						if err != nil {
							if err == io.EOF {
								logger.Error(err, "failed to read logs")
							}
							return
						}

						fmt.Printf("%s/%s/%s: %s", pod.GetNamespace(), pod.GetName(), container.Name, text)
					}
				}(pod, container)
			}
		}
		return ctx, nil
	}
}
