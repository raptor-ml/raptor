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

package redis

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-redis/redis/v8"
	"github.com/raptor-ml/raptor/api"
	"github.com/raptor-ml/raptor/pkg/plugins"
	"github.com/spf13/viper"
)

func init() {
	plugins.CollectNotifierFactories.Register(pluginName, NotifierFactory[api.CollectNotification])
	plugins.WriteNotifierFactories.Register(pluginName, NotifierFactory[api.WriteNotification])
}

func NotifierFactory[T api.Notification](viper *viper.Viper) (api.Notifier[T], error) {
	rc, err := redisClient(viper)
	if err != nil {
		return nil, fmt.Errorf("failed to create redis client: %w", err)
	}
	return &notifier[T]{
		client: rc,
	}, nil
}

type notifier[T api.Notification] struct {
	client redis.UniversalClient
}

func (n *notifier[T]) NotificationChannel() string {
	var t T
	switch any(t).(type) {
	case api.WriteNotification:
		return "_raptor:notification:write"
	case api.CollectNotification:
		return "_raptor:notification:collect"
	}
	panic("not implemented")
}

func (n *notifier[T]) Notify(ctx context.Context, notification T) error {
	msg, err := json.Marshal(notification)
	if err != nil {
		return fmt.Errorf("cannot marshal notification: %w", err)
	}
	ret, err := n.client.Publish(ctx, n.NotificationChannel(), msg).Result()
	if err != nil {
		return fmt.Errorf("cannot publish notification: %w", err)
	}
	if ret == 0 {
		return fmt.Errorf("no subscriber available")
	}
	return nil
}

func (n *notifier[T]) Subscribe(ctx context.Context) (<-chan T, error) {
	pubsub := n.client.Subscribe(ctx, n.NotificationChannel())
	c := make(chan T)
	go func() {
		<-ctx.Done()
		_ = pubsub.Close()
	}()
	go func() {
		for msg := range pubsub.Channel() {
			var notification T
			err := json.Unmarshal([]byte(msg.Payload), &notification)
			if err != nil {
				panic(fmt.Errorf("couldn't unmarshal notification: %w", err))
			}
			c <- notification
		}
		fmt.Println("subscription closed") //nolint:forbidigo // TODO replace with logging to logger
		close(c)
	}()

	return c, nil
}
