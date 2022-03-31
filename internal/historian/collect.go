package historian

import (
	"context"
	"fmt"
	"github.com/jellydator/ttlcache/v3"
	"github.com/natun-ai/natun/pkg/api"
	"strings"
	"time"
)

func (h *historian) Collector() LeaderRunnableFunc {
	return func(ctx context.Context) error {
		if h.handledBuckets == nil {
			h.handledBuckets = ttlcache.New[string, struct{}](ttlcache.WithDisableTouchOnHit[string, struct{}]())
			go h.handledBuckets.Start()
		}

		return h.collectTasks.Runnable(h.CollectWorkers)(ctx)
	}
}

// dispatch collect notifications: collect the data and send it to the write queue
func (h *historian) dispatchCollect(ctx context.Context, notification api.CollectNotification) error {
	var md api.Metadata
	if v, ok := h.metadata.Load(notification.FQN); !ok {
		return fmt.Errorf("failed to get metadata for %s", notification.FQN)
	} else if m, ok := v.(api.Metadata); !ok {
		panic(fmt.Sprintf("metadata for %s is not of type api.Metadata", notification.FQN))
	} else {
		md = m
	}

	if md.ValidWindow() {
		return h.dispatchCollectWithWindow(ctx, notification, md)
	}

	v, err := h.State.Get(ctx, md, notification.EntityID)
	if err != nil {
		return fmt.Errorf("failed to get state for %s: %w", notification.FQN, err)
	}
	h.writeTasks.queue.Add(api.WriteNotification{
		FQN:      notification.FQN,
		EntityID: notification.EntityID,
		Value:    v,
	})
	return nil
}

// dispatch collect notifications for windows
func (h *historian) dispatchCollectWithWindow(ctx context.Context, notification api.CollectNotification, md api.Metadata) error {
	switch notification.Bucket {
	case "":
		panic(fmt.Errorf("no bucket specified for %s", notification.FQN)) // irrecoverable
	case "dead":
		return h.dispatchCollectDead(ctx, md)
	}

	deadBuckets := api.DeadWindowBuckets(md.Staleness, md.Freshness)

	buckets, err := h.State.WindowBuckets(ctx, md, notification.EntityID, []string{strings.TrimSuffix(notification.Bucket, AliveMarker)})
	if err != nil {
		return fmt.Errorf("failed to get buckets for %s: %w", notification.FQN, err)
	}
	for _, b := range buckets {
		if !contains(deadBuckets, b.Bucket) {
			b.Bucket = fmt.Sprintf("%s(alive)", b.Bucket)
		} else {
			h.handledBuckets.Set(deadBucketKey(b.FQN, b.Bucket, b.EntityID), struct{}{}, api.DeadGracePeriod+time.Minute)
		}
		h.writeTasks.queue.Add(api.WriteNotification{
			FQN:      b.FQN,
			EntityID: b.EntityID,
			Value: &api.Value{
				Value:     b.Data,
				Timestamp: time.Now(),
			},
			Bucket: b.Bucket,
		})
	}
	return nil
}

func (h *historian) dispatchCollectDead(ctx context.Context, md api.Metadata) error {
	var ignore api.RawBuckets
	for k := range h.handledBuckets.Items() {
		if strings.HasPrefix(k, fmt.Sprintf("%s:", md.FQN)) {
			fqn, bucket, eid := fromDeadBucketKey(k)
			ignore = append(ignore, api.RawBucket{
				FQN:      fqn,
				Bucket:   bucket,
				EntityID: eid,
			})
		}
	}
	buckets, err := h.State.DeadWindowBuckets(ctx, md, ignore)
	if err != nil {
		return fmt.Errorf("failed to get buckets for %s: %w", md.FQN, err)
	}

	for _, b := range buckets {
		h.writeTasks.queue.Add(api.WriteNotification{
			FQN:      b.FQN,
			EntityID: b.EntityID,
			Value: &api.Value{
				Value:     b.Data,
				Timestamp: time.Now(),
			},
			Bucket: b.Bucket,
		})
	}

	//todo calculate next collection

	return nil
}

func contains(s []string, i string) bool {
	for _, a := range s {
		if a == i {
			return true
		}
	}
	return false
}

func deadBucketKey(fqn, bucket, eid string) string {
	return fmt.Sprintf("%s/%s:%s", fqn, bucket, eid)
}
func fromDeadBucketKey(k string) (fqn string, bucket string, eid string) {
	firstSep := strings.Index(k, "/")
	lastColon := strings.LastIndex(k, ":")
	return k[:firstSep], k[firstSep+1 : lastColon], k[lastColon+1:]
}
