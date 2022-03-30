package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-redis/redis/v8"
	"github.com/natun-ai/natun/pkg/api"
)

type notifier struct {
	client redis.UniversalClient
}

func (n *notifier) NotificationChannel(typ api.NotificationType) string {
	return fmt.Sprintf("_natun:notification:%s", typ)
}
func (n *notifier) Notify(ctx context.Context, notification api.Notification, typ api.NotificationType) error {
	switch typ {
	case api.NotificationTypeNone:
		return fmt.Errorf("notification type is not supported")
	case api.NotificationTypeCollect:
		if notification.Value != "" {
			return fmt.Errorf("value is not supported for collect notification")
		}
	case api.NotificationTypeWrite:
		if notification.Value == "" {
			return fmt.Errorf("value is required for write notification")
		}
	}

	msg, err := json.Marshal(notification)
	if err != nil {
		return fmt.Errorf("cannot marshal notification: %w", err)
	}
	ret, err := n.client.Publish(ctx, n.NotificationChannel(typ), msg).Result()
	if err != nil {
		return fmt.Errorf("cannot publish notification: %w", err)
	}
	if ret == 0 {
		return fmt.Errorf("no subscriber available")
	}
	return nil
}
func (n *notifier) Subscribe(ctx context.Context, typ api.NotificationType) (<-chan api.Notification, error) {
	pubsub := n.client.Subscribe(ctx, n.NotificationChannel(typ))
	c := make(chan api.Notification)
	go func() {
		<-ctx.Done()
		pubsub.Close()
	}()
	go func() {
		for msg := range pubsub.Channel() {
			var notification = api.Notification{}
			err := json.Unmarshal([]byte(msg.Payload), &notification)
			if err != nil {
				panic(fmt.Errorf("couldn't unmarshal notification: %w", err))
			}
			c <- notification
		}
	}()

	return c, nil
}
