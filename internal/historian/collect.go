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
	"strings"

	"github.com/jellydator/ttlcache/v3"
	"github.com/raptor-ml/raptor/api"
)

func (h *historian) Collector() LeaderRunnableFunc {
	return func(ctx context.Context) error {
		if h.handledBuckets == nil {
			h.handledBuckets = ttlcache.New[string, struct{}](ttlcache.WithDisableTouchOnHit[string, struct{}]())
			go h.handledBuckets.Start()
			go func(ctx context.Context) {
				<-ctx.Done()
				h.handledBuckets.Stop()
			}(ctx)
		}

		return h.collectTasks.Runnable(ctx)
	}
}

// dispatch collect notifications: collect the data and send it to the write queue
func (h *historian) dispatchCollect(ctx context.Context, notification api.CollectNotification) error {
	md, err := h.Metadata(ctx, notification.FQN)
	if err != nil {
		return fmt.Errorf("failed to get metadata for %s", notification.FQN)
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
	case DeadRequestMarker:
		return h.dispatchCollectDead(ctx, md)
	}

	deadBuckets := api.DeadWindowBuckets(md.Staleness, md.Freshness)

	buckets, err := h.State.WindowBuckets(ctx, md, notification.EntityID, []string{notification.Bucket})
	if err != nil {
		return fmt.Errorf("failed to get buckets for %s: %w", notification.FQN, err)
	}
	for _, b := range buckets {
		activeBucket := !contains(deadBuckets, b.Bucket)
		h.writeTasks.queue.Add(api.WriteNotification{
			FQN:      b.FQN,
			EntityID: b.EntityID,
			Value: &api.Value{
				Value:     b.Data,
				Timestamp: api.BucketTime(b.Bucket, md.Freshness),
			},
			Bucket:       b.Bucket,
			ActiveBucket: activeBucket,
		})
	}
	return nil
}

func (h *historian) dispatchCollectDead(ctx context.Context, md api.Metadata) error {
	var ignore api.RawBuckets
	for k := range h.handledBuckets.Items() {
		if strings.HasPrefix(k, fmt.Sprintf("%s/", md.FQN)) {
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
				Timestamp: api.BucketTime(b.Bucket, md.Freshness),
			},
			Bucket:       b.Bucket,
			ActiveBucket: false,
		})
	}

	// Add the next dead collection to the queue
	h.collectTasks.queue.AddAfter(api.CollectNotification{
		FQN:    md.FQN,
		Bucket: DeadRequestMarker,
	}, timeTillNextBucket(md.Freshness))

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

func fromDeadBucketKey(k string) (fqn, bucket, eid string) {
	firstSep := strings.Index(k, "/")
	lastColon := strings.LastIndex(k, ":")
	return k[:firstSep], k[firstSep+1 : lastColon], k[lastColon+1:]
}
