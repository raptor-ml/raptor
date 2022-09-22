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
	"encoding/json"
	"fmt"
	"io"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sync"
	"time"
)

type log struct {
	namespace     string
	podName       string
	containers    int
	containerName string
	log           map[string]any
}

func (l *log) Log(logger klog.Logger) {
	prefix := fmt.Sprintf("%s/%s", l.namespace, l.podName)
	if l.containers > 1 {
		prefix = fmt.Sprintf("%s[%s]", prefix, l.containerName)
	}

	msg := ""
	if m, ok := l.log["message"]; ok {
		msg = m.(string)
		delete(l.log, "message")
	} else if m, ok := l.log["msg"]; ok {
		msg = m.(string)
		delete(l.log, "msg")
	}

	if t, ok := l.log["ts"]; ok {
		logger = logger.WithValues("timestamp", t)
		delete(l.log, "ts")
	} else if t, ok := l.log["time"]; ok {
		logger = logger.WithValues("timestamp", t)
		delete(l.log, "time")
	}

	level := ""
	if lvl, ok := l.log["level"]; ok {
		level = lvl.(string)
		delete(l.log, "level")
	}

	// convert map to pairs
	pairs := make([]any, 0, len(l.log)/2)
	for k, v := range l.log {
		if m, ok := v.(map[any]any); ok && len(m) == 0 {
			continue
		}
		pairs = append(pairs, k, v)
	}

	switch level {
	case "info":
		logger.WithValues(pairs...).WithName(prefix).Info(msg)
	case "error":
		var err error
		if e, ok := l.log["error"]; ok {
			err = fmt.Errorf(e.(string))
			delete(l.log, "error")
		}
		logger.WithValues(pairs...).WithName(prefix).Error(err, msg)
	default:
		logger.WithValues(pairs...).WithName(prefix).Info(msg, pairs...)
	}
}

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

		logger := klog.Background().V(3)

		ch := make(chan log, 500)
		wg := &sync.WaitGroup{}
		for _, pod := range pods.Items {
			for _, container := range pod.Spec.Containers {
				wg.Add(1)
				go func(pod corev1.Pod, container corev1.Container, wg *sync.WaitGroup) {
					defer wg.Done()
					sinceTime := &metav1.Time{Time: time.Now().Add(-since)}
					if since <= 0 {
						sinceTime.Time = pod.GetCreationTimestamp().Time
					}
					rdr, err := cs.CoreV1().Pods(pod.GetNamespace()).GetLogs(pod.GetName(), &corev1.PodLogOptions{
						Follow:    false,
						Container: container.Name,
						SinceTime: sinceTime,
					}).Stream(ctx)

					defer func() {
						if rdr != nil {
							_ = rdr.Close()
						}
					}()
					logger := logger.WithValues("pod", pod.GetName(), "container", container.Name)

					if err != nil {
						logger.Error(err, "failed to get logs")
						return
					}

					r := bufio.NewReader(rdr)
					for {
						select {
						case <-ctx.Done():
							return
						default:
						}
						text, err := r.ReadBytes('\n')
						if err != nil {
							if err != io.EOF {
								logger.Error(err, "failed to read logs")
							}
							return
						}

						l := map[string]any{}
						err = json.Unmarshal(text, &l)
						if err != nil {
							l["msg"] = string(text)
						}

						ch <- log{
							namespace:     pod.GetNamespace(),
							podName:       pod.GetName(),
							containers:    len(pod.Spec.Containers),
							containerName: container.Name,
							log:           l,
						}
					}
				}(pod, container, wg)
			}
		}
		go func() {
			wg.Wait()
			close(ch)
		}()
		for l := range ch {
			l.Log(logger)
		}
		return ctx, nil
	}
}
