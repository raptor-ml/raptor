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
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

type (
	Client interface {
		// AddCollectNotification adds a notification to the collector
		AddCollectNotification(fqn, encodedKeys, bucket string)

		// AddWriteNotification adds a notification to the writer
		AddWriteNotification(fqn, encodedKeys, bucket string, value *api.Value)

		// CollectNotifier is a runnable that notifies the collector of a new collection task
		CollectNotifier() NoLeaderRunnableFunc

		// WriteNotifier is a runnable that notifies the writer of a new writing task
		WriteNotifier() NoLeaderRunnableFunc

		// WithManager adds all the Runnables (CollectNotifier, WriteNotifier) to the manager
		WithManager(mgr manager.Manager) error
	}
)
type ClientConfig struct {
	CollectNotifier            api.Notifier[api.CollectNotification]
	WriteNotifier              api.Notifier[api.WriteNotification]
	CollectNotificationWorkers int
	WriteNotificationWorkers   int
	Logger                     logr.Logger
}
type client struct {
	ClientConfig
	pendingWrite    queue[api.WriteNotification]
	pendingCollects queue[api.CollectNotification]
}

func NewClient(config ClientConfig) Client {
	if config.WriteNotificationWorkers == 0 {
		config.WriteNotificationWorkers = 5
	}
	if config.CollectNotificationWorkers == 0 {
		config.WriteNotificationWorkers = 5
	}
	c := &client{
		ClientConfig: config,
	}
	c.pendingWrite = newQueue[api.WriteNotification](c.Logger.WithName("pendingWrite"), c.queueWrite)
	c.pendingCollects = newQueue[api.CollectNotification](c.Logger.WithName("pendingCollect"), c.queueCollect)
	return c
}

func (c *client) AddCollectNotification(fqn, encodedKeys, bucket string) {
	c.pendingCollects.Add(api.CollectNotification{
		FQN:         fqn,
		EncodedKeys: encodedKeys,
		Bucket:      bucket,
	})
}

func (c *client) AddWriteNotification(fqn, encodedKeys, bucket string, value *api.Value) {
	if value == nil {
		panic(fmt.Errorf("value is nil for NotificationTypeWrite"))
	}
	c.pendingWrite.Add(api.WriteNotification{
		FQN:         fqn,
		EncodedKeys: encodedKeys,
		Value:       value,
		Bucket:      bucket,
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
	return c.ClientConfig.WriteNotifier.Notify(ctx, notification)
}

// send collect notifications to the external queue
func (c *client) queueCollect(ctx context.Context, notification api.CollectNotification) error {
	return c.ClientConfig.CollectNotifier.Notify(ctx, notification)
}
