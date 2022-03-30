package historian

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/jellydator/ttlcache/v3"
	"github.com/natun-ai/natun/pkg/api"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"strings"
	"time"
)

const SyncPeriod = 5 * time.Minute
const AliveMarker = "(alive)"

type Historian interface {
	NotifyCollect(fqn, entityId, bucket string)
	SetMetadataGetter(api.MetadataGetter)

	CollectNotifier() NoLeaderRunnableFunc
	Collector() LeaderRunnableFunc

	WriteNotifier() LeaderRunnableFunc
	Writer() LeaderRunnableFunc

	WithManager(manager manager.Manager) error
}
type historian struct {
	Config
	pendingWrite    baseQueuer
	pendingCollects baseQueuer
	collectTasks    subscriptionQueue
	writeTasks      subscriptionQueue
	metadataGetter  api.MetadataGetter
	handledBuckets  *ttlcache.Cache[string, struct{}]
}

type Config struct {
	Notifier api.Notifier
	State    api.State
	Logger   logr.Logger

	CollectNotificationWorkers int
	CollectWorkers             int
	WriteNotificationWorkers   int
}

func New(config Config) Historian {
	h := &historian{
		Config: config,
	}
	h.pendingWrite = newBaseQueue(h.Logger.WithName("pendingWrite"), h.queueWrite)
	h.pendingCollects = newBaseQueue(h.Logger.WithName("pendingCollect"), h.queueCollect)
	h.collectTasks = newSubscriptionQueue(api.NotificationTypeCollect, h.Notifier, h.Logger.WithName("collectTasks"), h.dispatchCollect)
	h.writeTasks = newSubscriptionQueue(api.NotificationTypeCollect, h.Notifier, h.Logger.WithName("dispatchWrite"), h.dispatchWrite)
	return h
}

func (h *historian) WithManager(manager manager.Manager) error {
	if err := manager.Add(h.CollectNotifier()); err != nil {
		return err
	}
	if err := manager.Add(h.Collector()); err != nil {
		return err
	}
	if err := manager.Add(h.WriteNotifier()); err != nil {
		return err
	}
	if err := manager.Add(h.Writer()); err != nil {
		return err
	}
	return nil
}

func (h *historian) SetMetadataGetter(mg api.MetadataGetter) {
	h.metadataGetter = mg
}

func (h *historian) NotifyCollect(fqn, entityID, bucket string) {
	h.pendingCollects.Notify(api.Notification{
		Fqn:      fqn,
		EntityId: entityID,
		Bucket:   bucket,
	})
}
func (h *historian) CollectNotifier() NoLeaderRunnableFunc {
	return h.pendingCollects.Runnable(h.CollectNotificationWorkers)
}
func (h *historian) WriteNotifier() LeaderRunnableFunc {
	return h.pendingWrite.Runnable(h.WriteNotificationWorkers)
}
func (h *historian) Collector() LeaderRunnableFunc {
	return func(ctx context.Context) error {
		if h.metadataGetter == nil {
			return fmt.Errorf("metadataGetter is required to be set before running the Collect Runnable")
		}

		if h.handledBuckets == nil {
			h.handledBuckets = ttlcache.New[string, struct{}](ttlcache.WithDisableTouchOnHit[string, struct{}]())
			go h.handledBuckets.Start()
		}

		return h.collectTasks.Runnable(h.CollectWorkers)(ctx)
	}
}
func (h *historian) Writer() LeaderRunnableFunc {
	return h.writeTasks.Runnable(1) // must have only one writer
}

// send write notifications to the external queue
func (h *historian) queueWrite(ctx context.Context, notification api.Notification) error {
	return h.Notifier.Notify(ctx, notification, api.NotificationTypeWrite)
}

// send collect notifications to the external queue
func (h *historian) queueCollect(ctx context.Context, notification api.Notification) error {
	return h.Notifier.Notify(ctx, notification, api.NotificationTypeCollect)
}

// dispatch collect notifications: collect the data and send it to the write queue
func (h *historian) dispatchCollect(ctx context.Context, notification api.Notification) error {
	md, err := h.metadataGetter(ctx, notification.Fqn)
	if err != nil {
		return fmt.Errorf("failed to get metadata for %s: %w", notification.Fqn, err)
	}
	if md.ValidWindow() {
		return h.dispatchCollectWithWindow(ctx, notification, md)
	}

	v, err := h.State.Get(ctx, md, notification.EntityId)
	if err != nil {
		return fmt.Errorf("failed to get state for %s: %w", notification.Fqn, err)
	}
	h.writeTasks.queue.Notify(api.Notification{
		Fqn:      notification.Fqn,
		EntityId: notification.EntityId,
		Value:    v,
	})
	return nil
}

// dispatch collect notifications for windows
func (h *historian) dispatchCollectWithWindow(ctx context.Context, notification api.Notification, md api.Metadata) error {
	var bucketNames []string
	dead := false
	switch notification.Bucket {
	case "":
		panic(fmt.Errorf("no bucket specified for %s", notification.Fqn)) //irrecoverable
	case "dead":
		for _, b := range api.DeadWindowBuckets(md.Staleness, md.Freshness) {
			if h.handledBuckets.Get(b) == nil {
				bucketNames = append(bucketNames, b)
			}
		}
		dead = true
	default:
		bucketNames = []string{strings.TrimSuffix(notification.Bucket, AliveMarker)}
	}

	buckets, err := h.State.WindowBuckets(ctx, md, notification.EntityId, bucketNames)
	if err != nil {
		return fmt.Errorf("failed to get buckets for %s: %w", notification.Fqn, err)
	}
	for bn, rawBucket := range buckets {
		if !dead {
			bn = fmt.Sprintf("%s(alive)", bn)
		} else {
			h.handledBuckets.Set(deadBucketKey(md.FQN, notification.EntityId, bn), struct{}{}, api.DeadGracePeriod+time.Minute)
		}
		h.writeTasks.queue.Notify(api.Notification{
			Fqn:      notification.Fqn,
			EntityId: notification.EntityId,
			Value:    rawBucket,
			Bucket:   bn,
		})
	}
	return nil
}
func deadBucketKey(fqn, eid, bucket string) string {
	return fmt.Sprintf("%s:%s:%s", fqn, eid, bucket)
}

func (h *historian) dispatchWrite(ctx context.Context, notification api.Notification) error {
	//TODO write to s3
	return nil
}
