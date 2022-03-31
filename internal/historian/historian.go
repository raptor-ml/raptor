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
const AliveMarker = "(alive)"

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
	Config
	collectTasks   subscriptionQueue[api.CollectNotification]
	writeTasks     subscriptionQueue[api.WriteNotification]
	metadata       sync.Map
	handledBuckets *ttlcache.Cache[string, struct{}]
}

type Config struct {
	CollectNotifier api.Notifier[api.CollectNotification]
	WriteNotifier   api.Notifier[api.WriteNotification]
	State           api.State
	Logger          logr.Logger

	CollectNotificationWorkers int
	CollectWorkers             int
	WriteNotificationWorkers   int
}

func NewServer(config Config) Server {
	if config.CollectWorkers == 0 {
		config.CollectWorkers = 5
	}
	h := &historian{
		Config: config,
	}
	h.collectTasks = newSubscriptionQueue[api.CollectNotification](h.CollectNotifier, h.Logger.WithName("collectTasks"), h.dispatchCollect)
	h.writeTasks = newSubscriptionQueue[api.WriteNotification](h.WriteNotifier, h.Logger.WithName("dispatchWrite"), h.dispatchWrite)
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
		//TODO add collect tasks
		//h.collectTasks.queue.
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
