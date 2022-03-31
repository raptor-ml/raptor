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
