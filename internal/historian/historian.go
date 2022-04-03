/*
Copyright 2022 Natun.

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
	"fmt"
	"github.com/go-logr/logr"
	"github.com/jellydator/ttlcache/v3"
	"github.com/natun-ai/natun/pkg/api"
	manifests "github.com/natun-ai/natun/pkg/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sync"
	"time"
)

const SyncPeriod = 5 * time.Minute
const AliveMarker = api.AliveMarker
const DeadRequestMarker = "*dead*"

// Although this is done at compile time, we want to make sure nobody messed with the numbers inappropriately
//goland:noinspection GoBoolExpressions
func init() {
	if api.DeadGracePeriod < (SyncPeriod - (30 * time.Second)) {
		panic(fmt.Sprintf("DeadGracePeriod (%v) must be greater than SyncPeriod (%v)", api.DeadGracePeriod, SyncPeriod))
	}
}

type Server interface {
	api.Manager

	// Collector is a runnable that collects data from the state and sends a writing notification via the WriteNotifier
	Collector() LeaderRunnableFunc

	// Writer is a runnable that writes data to the Historical Data Storage
	Writer() LeaderRunnableFunc

	// WithManager adds all the Runnables (Collector, Writer) to the manager
	WithManager(manager manager.Manager) error
}
type historian struct {
	ServerConfig
	collectTasks   subscriptionQueue[api.CollectNotification]
	writeTasks     subscriptionQueue[api.WriteNotification]
	metadata       sync.Map
	handledBuckets *ttlcache.Cache[string, struct{}]
}

type ServerConfig struct {
	CollectNotifier api.Notifier[api.CollectNotification]
	WriteNotifier   api.Notifier[api.WriteNotification]
	Logger          logr.Logger

	CollectWorkers   int
	State            api.State
	HistoricalWriter api.HistoricalWriter
}

func NewServer(config ServerConfig) Server {
	if config.CollectWorkers == 0 {
		config.CollectWorkers = 5
	}
	h := &historian{
		ServerConfig: config,
	}
	h.collectTasks = newSubscriptionQueue[api.CollectNotification](h.CollectNotifier, h.Logger.WithName("collectTasks"), h.dispatchCollect)
	h.writeTasks = newSubscriptionQueue[api.WriteNotification](h.WriteNotifier, h.Logger.WithName("dispatchWrite"), h.dispatchWrite)
	h.writeTasks.queue.finalizer = h.finalizeWrite
	return h
}

func (h *historian) WithManager(manager manager.Manager) error {
	if err := manager.Add(h.Collector()); err != nil {
		return err
	}
	if err := manager.Add(h.Writer()); err != nil {
		return err
	}
	return nil
}

func (h *historian) Writer() LeaderRunnableFunc {
	return h.writeTasks.Runnable(1) // must have only one writer
}

func (h *historian) BindFeature(in manifests.Feature) error {
	md, err := api.MetadataFromManifest(in)
	if err != nil {
		return fmt.Errorf("failed to parse metadata from CR: %w", err)
	}

	if md.ValidWindow() {
		h.collectTasks.queue.AddAfter(api.CollectNotification{
			FQN:    md.FQN,
			Bucket: DeadRequestMarker,
		}, timeTillNextBucket(md.Freshness))
	}

	h.metadata.Store(in.FQN(), md)
	return nil
}

func (h *historian) UnbindFeature(FQN string) error {
	h.metadata.Delete(FQN)
	h.Logger.Info("feature unbound", "feature", FQN)
	return nil
}

func (h *historian) HasFeature(FQN string) bool {
	_, ok := h.metadata.Load(FQN)
	return ok
}

func timeTillNextBucket(bucketSize time.Duration) time.Duration {
	now := time.Now()
	return time.Duration(float64(api.BucketTime(api.BucketName(now, bucketSize), bucketSize).Add(bucketSize).Sub(now)) * 0.9)
}
