package historian

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/natun-ai/natun/pkg/api"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/workqueue"
	"sync"
	"time"
)

func newBaseQueue[T api.Notification](logger logr.Logger, fn NotifyFn[T]) baseQueuer[T] {
	return baseQueuer[T]{
		queue:  workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
		logger: logger,
		fn:     fn,
	}
}

type NotifyFn[T api.Notification] func(ctx context.Context, notification T) error
type baseQueuer[T api.Notification] struct {
	queue     workqueue.RateLimitingInterface
	logger    logr.Logger
	fn        NotifyFn[T]
	finalizer func(ctx context.Context)
}

func (b *baseQueuer[T]) Add(item T) {
	b.AddAfter(item, 0)
}

func (b *baseQueuer[T]) AddAfter(item T, duration time.Duration) {
	b.queue.AddAfter(item, duration)
}

func (b *baseQueuer[T]) Runnable(workers int) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		defer b.queue.ShutDownWithDrain()

		wg := sync.WaitGroup{}
		for i := 0; i < workers; i++ {
			go wait.Until(func() {
				wg.Add(1)
				defer wg.Done()

				for b.processNextItem(ctx) {
				}
				if b.finalizer != nil {
					b.finalizer(ctx)
				}
			}, SyncPeriod, ctx.Done())
		}

		<-ctx.Done()
		b.logger.Info("Shutdown signal received, waiting for all workers to finish")
		wg.Wait()
		b.logger.Info("All workers finished")
		return nil
	}
}
func (b *baseQueuer[T]) processNextItem(ctx context.Context) bool {
	// The queue was drained, wait until the next cycle
	if b.queue.Len() == 0 {
		return false
	}

	// Wait until there is a new item in the working queue
	item, quit := b.queue.Get()
	if quit {
		return false
	}
	defer b.queue.Done(item)

	notification, ok := item.(T)
	if !ok {
		b.logger.Error(fmt.Errorf("casting failure"), "failed to cast item to notification", "item", item)
		b.queue.Forget(item)
		return true
	}

	err := b.fn(ctx, notification)
	if err != nil {
		b.logger.WithValues("notification", notification).Error(err, "Failed to process. Requeing item...")
		b.queue.AddRateLimited(item)
	}

	b.queue.Forget(item)
	return true
}
