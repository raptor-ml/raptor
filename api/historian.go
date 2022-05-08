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

package api

import (
	"context"
	manifests "github.com/natun-ai/natun/api/v1alpha1"
)

type Notification interface {
	CollectNotification | WriteNotification
}
type CollectNotification struct {
	FQN      string `json:"fqn"`
	EntityID string `json:"entity_id"`
	Bucket   string `json:"bucket,omitempty"`
}
type WriteNotification struct {
	FQN          string `json:"fqn"`
	EntityID     string `json:"entity_id"`
	Bucket       string `json:"bucket,omitempty"`
	ActiveBucket bool   `json:"active_bucket,omitempty"`
	Value        *Value `json:"value,omitempty"`
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
	Close(ctx context.Context) error
	BindFeature(md *Metadata, fs *manifests.FeatureSetSpec, getter MetadataGetter) error
}
