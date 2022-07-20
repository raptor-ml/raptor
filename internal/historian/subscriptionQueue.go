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

package historian

import (
	"context"
	"github.com/go-logr/logr"
	"github.com/raptor-ml/raptor/api"
	"k8s.io/apimachinery/pkg/util/wait"
	"time"
)

type HandleFn[T api.Notification] func(ctx context.Context, notification T) error
type FinalizerFunc func(ctx context.Context)

type subscriptionQueue[T api.Notification] struct {
	queue     queue[T]
	finalizer func(ctx context.Context)
	notifier  api.Notifier[T]
	logger    logr.Logger
}

func newSubscriptionQueue[T api.Notification](notifier api.Notifier[T], logger logr.Logger, fn HandleFn[T]) subscriptionQueue[T] {
	return subscriptionQueue[T]{
		queue:    newQueue[T](logger, fn),
		notifier: notifier,
		logger:   logger,
	}
}

func (c *subscriptionQueue[T]) Runnable(ctx context.Context) error {
	defer c.queue.ShutDownWithDrain()

	go func() {
		// wait for initialization of the internal feature state
		time.Sleep(time.Second)

		go wait.UntilWithContext(ctx, func(ctx context.Context) {
			for c.queue.processNextItem(ctx, true) {
			}
			if c.finalizer != nil {
				c.finalizer(ctx)
			}
		}, SyncPeriod)
	}()
	go func() {
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
	return nil
}
