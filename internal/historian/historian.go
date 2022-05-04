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
	"github.com/natun-ai/natun/api"
	manifests "github.com/natun-ai/natun/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sync"
	"time"
)

const SyncPeriod = 5 * time.Minute
const DeadRequestMarker = "*dead*"

// Although this is done at compile time, we want to make sure nobody messed with the numbers inappropriately
//goland:noinspection GoBoolExpressions
func init() {
	if api.DeadGracePeriod < (SyncPeriod - (30 * time.Second)) {
		panic(fmt.Sprintf("DeadGracePeriod (%v) must be greater than SyncPeriod (%v)", api.DeadGracePeriod, SyncPeriod))
	}
}

type Server interface {
	api.FeatureManager

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
	writes         uint32
	metadata       sync.Map
	handledBuckets *ttlcache.Cache[string, struct{}]
}

type ServerConfig struct {
	CollectNotifier api.Notifier[api.CollectNotification]
	WriteNotifier   api.Notifier[api.WriteNotification]
	Logger          logr.Logger

	State            api.State
	HistoricalWriter api.HistoricalWriter
}

func NewServer(config ServerConfig) Server {
	h := &historian{
		ServerConfig: config,
	}
	h.collectTasks = newSubscriptionQueue[api.CollectNotification](h.CollectNotifier, h.Logger.WithName("collectTasks"), h.dispatchCollect)
	h.writeTasks = newSubscriptionQueue[api.WriteNotification](h.WriteNotifier, h.Logger.WithName("dispatchWrite"), h.dispatchWrite)
	h.writeTasks.finalizer = h.finalizeWrite
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
	return h.writeTasks.Runnable // must have only one writer
}

func (h *historian) BindFeature(in *manifests.Feature) error {
	md, err := api.MetadataFromManifest(in)
	if err != nil {
		return fmt.Errorf("failed to parse metadata from CR: %w", err)
	}

	if md.Primitive == api.PrimitiveTypeHeadless {
		// Headless features are not stored and not backed up to historical storage
		return nil
	}

	if md.ValidWindow() {
		h.collectTasks.queue.Add(api.CollectNotification{
			FQN:    md.FQN,
			Bucket: DeadRequestMarker,
		})
	}

	h.metadata.Store(in.FQN(), md)
	h.Logger.Info("feature bounded", "feature", in.FQN())
	return nil
}

func (h *historian) UnbindFeature(fqn string) error {
	h.metadata.Delete(fqn)
	h.Logger.Info("feature unbound", "feature", fqn)
	return nil
}

func (h *historian) HasFeature(fqn string) bool {
	_, ok := h.metadata.Load(fqn)
	return ok
}

func timeTillNextBucket(bucketSize time.Duration) time.Duration {
	now := time.Now()
	return time.Duration(float64(api.BucketTime(api.BucketName(now, bucketSize), bucketSize).Add(bucketSize).Sub(now)) * 0.9)
}
