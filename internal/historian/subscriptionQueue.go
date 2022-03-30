package historian

import (
	"context"
	"github.com/go-logr/logr"
	"github.com/natun-ai/natun/pkg/api"
	"sync"
)

type subscriptionQueue struct {
	queue    baseQueuer
	notifier api.Notifier
	logger   logr.Logger
	typ      api.NotificationType
}

func newSubscriptionQueue(typ api.NotificationType, notifier api.Notifier, logger logr.Logger, fn notifyFn) subscriptionQueue {
	return subscriptionQueue{
		queue:    newBaseQueue(logger.WithName("queue"), fn),
		notifier: notifier,
		logger:   logger,
		typ:      typ,
	}
}

func (c *subscriptionQueue) Runnable(workers int) func(ctx context.Context) error {
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
					subscription, err := c.notifier.Subscribe(ctx, c.typ)
					if err != nil {
						c.logger.Error(err, "failed to subscribe to notifications")
					}
					for notification := range subscription {
						c.queue.Notify(notification)
					}
				}
			}
		}()
		<-ctx.Done()
		wg.Wait()
		return nil
	}
}
