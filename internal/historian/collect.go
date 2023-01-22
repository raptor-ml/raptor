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
	"github.com/jellydator/ttlcache/v3"
	"github.com/raptor-ml/raptor/api"
	"strings"
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
	fd, err := h.FeatureDescriptor(ctx, notification.FQN)
	if err != nil {
		return fmt.Errorf("failed to get FeatureDescriptor for %s", notification.FQN)
	}

	if fd.ValidWindow() {
		return h.dispatchCollectWithWindow(ctx, notification, fd)
	}

	keys := api.Keys{}
	err = keys.Decode(notification.EncodedKeys, fd)
	if err != nil {
		return fmt.Errorf("failed to decode keys: %w", err)
	}

	v, err := h.State.Get(ctx, fd, keys, 0)
	if err != nil {
		return fmt.Errorf("failed to get state for %s: %w", notification.FQN, err)
	}
	h.writeTasks.queue.Add(api.WriteNotification{
		FQN:         notification.FQN,
		EncodedKeys: notification.EncodedKeys,
		Value:       v,
	})
	return nil
}

// dispatch collect notifications for windows
func (h *historian) dispatchCollectWithWindow(ctx context.Context, notification api.CollectNotification, fd api.FeatureDescriptor) error {
	switch notification.Bucket {
	case "":
		panic(fmt.Errorf("no bucket specified for %s", notification.FQN)) // irrecoverable
	case DeadRequestMarker:
		return h.dispatchCollectDead(ctx, fd)
	}

	deadBuckets := api.DeadWindowBuckets(fd.Staleness, fd.Freshness)

	keys := api.Keys{}
	err := keys.Decode(notification.EncodedKeys, fd)
	if err != nil {
		return fmt.Errorf("failed to decode keys: %w", err)
	}

	buckets, err := h.State.WindowBuckets(ctx, fd, keys, []string{notification.Bucket})
	if err != nil {
		return fmt.Errorf("failed to get buckets for %s: %w", notification.FQN, err)
	}
	for _, b := range buckets {
		activeBucket := !contains(deadBuckets, b.Bucket)
		h.writeTasks.queue.Add(api.WriteNotification{
			FQN:         b.FQN,
			EncodedKeys: b.EncodedKeys,
			Value: &api.Value{
				Value:     b.Data,
				Timestamp: api.BucketTime(b.Bucket, fd.Freshness),
			},
			Bucket:       b.Bucket,
			ActiveBucket: activeBucket,
		})
	}
	return nil
}

func (h *historian) dispatchCollectDead(ctx context.Context, fd api.FeatureDescriptor) error {
	var ignore api.RawBuckets
	for k := range h.handledBuckets.Items() {
		if strings.HasPrefix(k, fmt.Sprintf("%s/", fd.FQN)) {
			fqn, bucket, eid := fromDeadBucketKey(k)
			ignore = append(ignore, api.RawBucket{
				FQN:         fqn,
				Bucket:      bucket,
				EncodedKeys: eid,
			})
		}
	}
	buckets, err := h.State.DeadWindowBuckets(ctx, fd, ignore)
	if err != nil {
		return fmt.Errorf("failed to get buckets for %s: %w", fd.FQN, err)
	}

	for _, b := range buckets {
		h.writeTasks.queue.Add(api.WriteNotification{
			FQN:         b.FQN,
			EncodedKeys: b.EncodedKeys,
			Value: &api.Value{
				Value:     b.Data,
				Timestamp: api.BucketTime(b.Bucket, fd.Freshness),
			},
			Bucket:       b.Bucket,
			ActiveBucket: false,
		})
	}

	// Add the next dead collection to the queue
	h.collectTasks.queue.AddAfter(api.CollectNotification{
		FQN:    fd.FQN,
		Bucket: DeadRequestMarker,
	}, timeTillNextBucket(fd.Freshness))

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
