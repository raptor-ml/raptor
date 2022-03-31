package historian

import (
	"context"
	"fmt"
	"github.com/natun-ai/natun/pkg/api"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

type Client interface {
	// AddCollectNotification adds a notification to the collector
	AddCollectNotification(fqn, entityID, bucket string)

	// AddWriteNotification adds a notification to the writer
	AddWriteNotification(fqn, entityID, bucket string, value *api.Value)

	// CollectNotifier is a runnable that notifies the collector of a new collection task
	CollectNotifier() NoLeaderRunnableFunc

	// WriteNotifier is a runnable that notifies the writer of a new writing task
	WriteNotifier() NoLeaderRunnableFunc

	// WithManager adds all the Runnables (CollectNotifier, WriteNotifier) to the manager
	WithManager(mgr manager.Manager) error
}
type client struct {
	Config
	pendingWrite    baseQueuer[api.WriteNotification]
	pendingCollects baseQueuer[api.CollectNotification]
}

func NewClient(config Config) Client {
	if config.WriteNotificationWorkers == 0 {
		config.WriteNotificationWorkers = 5
	}
	if config.CollectNotificationWorkers == 0 {
		config.WriteNotificationWorkers = 5
	}
	c := &client{
		Config: config,
	}
	c.pendingWrite = newBaseQueue[api.WriteNotification](c.Logger.WithName("pendingWrite"), c.queueWrite)
	c.pendingCollects = newBaseQueue[api.CollectNotification](c.Logger.WithName("pendingCollect"), c.queueCollect)
	return c
}

func (c *client) AddCollectNotification(fqn, entityID, bucket string) {
	c.pendingCollects.Add(api.CollectNotification{
		FQN:      fqn,
		EntityID: entityID,
		Bucket:   bucket,
	})
}

func (c *client) AddWriteNotification(fqn, entityID, bucket string, value *api.Value) {
	if value == nil {
		panic(fmt.Errorf("value is nil for NotificationTypeWrite"))
	}
	c.pendingWrite.Add(api.WriteNotification{
		FQN:      fqn,
		EntityID: entityID,
		Value:    value,
		Bucket:   bucket,
	})
}

func (c *client) CollectNotifier() NoLeaderRunnableFunc {
	return c.pendingCollects.Runnable(c.CollectNotificationWorkers)
}
func (c *client) WriteNotifier() NoLeaderRunnableFunc {
	return c.pendingWrite.Runnable(c.WriteNotificationWorkers)
}

func (c *client) WithManager(manager manager.Manager) error {
	if err := manager.Add(c.CollectNotifier()); err != nil {
		return err
	}
	if err := manager.Add(c.WriteNotifier()); err != nil {
		return err
	}
	return nil
}

// send write notifications to the external queue
func (c *client) queueWrite(ctx context.Context, notification api.WriteNotification) error {
	return c.Config.WriteNotifier.Notify(ctx, notification)
}

// send collect notifications to the external queue
func (c *client) queueCollect(ctx context.Context, notification api.CollectNotification) error {
	return c.Config.CollectNotifier.Notify(ctx, notification)
}
