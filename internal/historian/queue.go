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
	"fmt"
	"github.com/go-logr/logr"
	"github.com/raptor-ml/raptor/api"
	"k8s.io/client-go/util/workqueue"
	"sync"
)

func newQueue[T api.Notification](logger logr.Logger, fn HandleFn[T]) queue[T] {
	return queue[T]{
		RateLimitingInterface: workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
		logger:                logger,
		fn:                    fn,
	}
}

type queue[T api.Notification] struct {
	workqueue.RateLimitingInterface
	logger logr.Logger
	fn     HandleFn[T]
}

func (b *queue[T]) Runnable(workers int) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		defer b.ShutDownWithDrain()

		wg := sync.WaitGroup{}
		wg.Add(workers)
		for i := 0; i < workers; i++ {
			go func() {
				defer wg.Done()
				for b.processNextItem(ctx, false) {
				}
			}()
		}

		<-ctx.Done()
		b.logger.Info("Shutdown signal received, waiting for all workers to finish")
		wg.Wait()
		b.logger.Info("All workers finished")
		return nil
	}
}
func (b *queue[T]) processNextItem(ctx context.Context, stopOnDrained bool) bool {
	select {
	case <-ctx.Done():
		return false
	default:
	}

	if stopOnDrained && b.Len() == 0 {
		// The queue was drained, wait until the next cycle
		return false
	}

	// Wait until there is a new item in the working queue
	item, quit := b.Get()
	if quit {
		return false
	}
	defer b.Done(item)

	notification, ok := item.(T)
	if !ok {
		b.logger.Error(fmt.Errorf("casting failure"), "failed to cast item to notification", "item", item)
		b.Forget(item)
		return true
	}

	err := b.fn(ctx, notification)
	if err != nil {
		b.logger.WithValues("notification", notification).Error(err, "Failed to process. Requeuing item...")
		b.AddRateLimited(item)
	}

	b.Forget(item)
	return true
}
