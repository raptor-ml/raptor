package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-redis/redis/v8"
	"github.com/natun-ai/natun/internal/plugin"
	"github.com/natun-ai/natun/pkg/api"
	"github.com/spf13/viper"
)

func init() {
	plugin.CollectNotifierFactories.Register(pluginName, NotifierFactory[api.CollectNotification])
	plugin.WriteNotifierFactories.Register(pluginName, NotifierFactory[api.WriteNotification])
}
func NotifierFactory[T api.Notification](viper *viper.Viper) (api.Notifier[T], error) {
	rc, err := redisClient(viper)
	if err != nil {
		return nil, err
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
		return "_natun:notification:write"
	case api.CollectNotification:
		return "_natun:notification:collect"
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
	}()

	return c, nil
}
