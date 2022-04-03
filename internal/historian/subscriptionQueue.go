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

package historian

import (
	"context"
	"github.com/go-logr/logr"
	"github.com/natun-ai/natun/pkg/api"
	"sync"
)

type subscriptionQueue[T api.Notification] struct {
	queue    baseQueuer[T]
	notifier api.Notifier[T]
	logger   logr.Logger
}

func newSubscriptionQueue[T api.Notification](notifier api.Notifier[T], logger logr.Logger, fn NotifyFn[T]) subscriptionQueue[T] {
	return subscriptionQueue[T]{
		queue:    newBaseQueue(logger.WithName("queue"), fn),
		notifier: notifier,
		logger:   logger,
	}
}

func (c *subscriptionQueue[T]) Runnable(workers int) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		wg := sync.WaitGroup{}
		wg.Add(2)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
					err := c.queue.Runnable(workers)(ctx)
					if err != nil {
						c.logger.Error(err, "failed to run fetch notifications")
					}
				}
			}
		}()
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
					subscription, err := c.notifier.Subscribe(ctx)
					if err != nil {
						c.logger.Error(err, "failed to subscribe to notifications")
					}
					for notification := range subscription {
						c.queue.Add(notification)
					}
				}
			}
		}()
		<-ctx.Done()
		wg.Wait()
		return nil
	}
}
