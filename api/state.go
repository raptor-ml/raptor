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

package api

import (
	"context"
	"time"
)

// Value is storing a feature value.
type Value struct {
	// Value can be cast to LowLevelValue using the ToLowLevelValue() method
	Value     any       `json:"value"`
	Timestamp time.Time `json:"timestamp"`
	Fresh     bool      `json:"fresh"`
}

// WindowResultMap is a map of AggrFn and their aggregated results
type WindowResultMap map[AggrFn]float64

// RawBucket is the data that is stored in the raw bucket.
type RawBucket struct {
	FQN         string          `json:"FQN"`
	Bucket      string          `json:"bucket"`
	EncodedKeys string          `json:"encoded_keys"`
	Data        WindowResultMap `json:"raw"`
}
type RawBuckets []RawBucket

// LowLevelValue is a low level value that can be cast to any type
type LowLevelValue interface {
	~int | ~string | ~float64 | time.Time | ~[]int | ~[]string | ~[]float64 | ~[]time.Time | WindowResultMap
}

// ToLowLevelValue returns the low level value of the feature
func ToLowLevelValue[T LowLevelValue](v any) T {
	return v.(T)
}

// State is a feature state management layer
type State interface {
	// Get returns the SimpleValue of the feature.
	// If the feature is not available, it returns nil.
	// If the feature is windowed, the returned SimpleValue is a map from window function to SimpleValue.
	// version indicates the previous version of the feature. If version is 0, the latest version is returned.
	Get(ctx context.Context, fd FeatureDescriptor, keys Keys, version uint) (*Value, error)

	// Set sets the SimpleValue of the feature.
	// If the feature's primitive is a List, it replaces the entire list.
	// If the feature is windowed, it is aliased to WindowAdd instead of Set.
	Set(ctx context.Context, fd FeatureDescriptor, keys Keys, val any, timestamp time.Time) error

	// Append appends the SimpleValue to the feature.
	// If the feature's primitive is NOT a List it will throw an error.
	Append(ctx context.Context, fd FeatureDescriptor, keys Keys, val any, ts time.Time) error

	// Incr increments the SimpleValue of the feature.
	// If the feature's primitive is NOT a Scalar it will throw an error.
	// It returns the updated value in the state, and an error if occurred.
	Incr(ctx context.Context, fd FeatureDescriptor, keys Keys, by any, timestamp time.Time) error
	// Update is the common function to update a feature SimpleValue.
	// Under the hood, it utilizes lower-level functions depending on the type of the feature.
	//  - Set for Scalars
	//	- Append for Lists
	//  - WindowAdd for Windows
	Update(ctx context.Context, fd FeatureDescriptor, keys Keys, val any, timestamp time.Time) error

	// WindowAdd adds a Bucket to the window that contains aggregated data internally
	// Later the bucket's aggregations should be aggregated for the whole Window via Get
	//
	// Buckets should last *at least* as long as the feature's staleness time + DeadGracePeriod
	WindowAdd(ctx context.Context, fd FeatureDescriptor, keys Keys, val any, timestamp time.Time) error

	// WindowBuckets returns the list of RawBuckets for the feature and specific Keys.
	WindowBuckets(ctx context.Context, fd FeatureDescriptor, keys Keys, buckets []string) (RawBuckets, error)

	// DeadWindowBuckets returns the list of all the dead feature's RawBuckets of all the entities.
	DeadWindowBuckets(ctx context.Context, fd FeatureDescriptor, ignore RawBuckets) (RawBuckets, error)

	// Ping is a simple keepalive check for the state.
	// It should return an error in case an error occurred, or nil if everything is alright.
	Ping(ctx context.Context) error
}

// StateMethod is a method that can be used with a State.
type StateMethod int

const (
	StateMethodGet StateMethod = iota
	StateMethodSet
	StateMethodAppend
	StateMethodIncr
	StateMethodUpdate
	StateMethodWindowAdd
)

func (s StateMethod) String() string {
	switch s {
	case StateMethodGet:
		return "Get"
	case StateMethodSet:
		return "Set"
	case StateMethodAppend:
		return "Append"
	case StateMethodIncr:
		return "Incr"
	case StateMethodUpdate:
		return "Update"
	case StateMethodWindowAdd:
		return "WindowAdd"
	default:
		panic("unreachable")
	}
}
