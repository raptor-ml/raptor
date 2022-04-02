package api

import "context"

type Notification interface {
	CollectNotification | WriteNotification
}
type CollectNotification struct {
	FQN      string `json:"fqn"`
	EntityID string `json:"entity_id"`
	Bucket   string `json:"bucket,omitempty"`
}
type WriteNotification struct {
	FQN      string `json:"fqn"`
	EntityID string `json:"entity_id"`
	Bucket   string `json:"bucket,omitempty"`
	Value    *Value `json:"value,omitempty"`
}

// Notifier is the interface to be implemented by plugins that want to provide a Queue implementation
// The Queue is used to sync notifications between instances
type Notifier[T Notification] interface {
	Notify(context.Context, T) error
	Subscribe(context.Context) (<-chan T, error)
}

type HistoricalWriter interface {
	Commit(context.Context, WriteNotification) error
	Flush(ctx context.Context, fqn string) error
	FlushAll(context.Context) error
}

const AliveMarker = "(alive)"
