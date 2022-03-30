package api

import "context"

type Notification struct {
	Fqn      string `json:"fqn"`
	EntityId string `json:"entity_id"`

	// Value indicate the new value of the feature
	// For NotificationTypeCollect it should be always empty
	// For NotificationTypeWrite it should contain the value that the Historian should store in the Historical Storage
	Value any `json:"value,omitempty"`

	// Bucket is the WindowBucket of the value
	// Alive buckets are suffixed with the "(alive)" string (e.g. "q48a(alive)")
	// When the value is "dead" it means that we should collect dead buckets
	Bucket string `json:"bucket,omitempty"`
}

type NotificationType int

const (
	NotificationTypeNone NotificationType = iota
	NotificationTypeCollect
	NotificationTypeWrite
)

func (n NotificationType) String() string {
	switch n {
	case NotificationTypeCollect:
		return "collect"
	case NotificationTypeWrite:
		return "write"
	default:
		return "unknown"
	}
}

// Notifier is the interface to be implemented by plugins that want to provide a Queue implementation
// The Queue is used to sync notifications between instances
type Notifier interface {
	Notify(context.Context, Notification, NotificationType) error
	Subscribe(context.Context, NotificationType) (<-chan Notification, error)
}
