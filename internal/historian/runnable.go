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
)

// NoLeaderRunnableFunc implements Runnable using a function that's run on every instance (not only the leader).
// It's very important that the given function block until it's done running.
type NoLeaderRunnableFunc func(context.Context) error

// Start implements Runnable.
func (r NoLeaderRunnableFunc) Start(ctx context.Context) error {
	return r(ctx)
}

// NeedLeaderElection make sure the Runnable will run on every instance
func (r *NoLeaderRunnableFunc) NeedLeaderElection() bool {
	return false
}

// LeaderRunnableFunc implements Runnable using a function that's run on *ONLY* on the leader.
// It's very important that the given function block until it's done running.
type LeaderRunnableFunc func(context.Context) error

// Start implements Runnable.
func (r LeaderRunnableFunc) Start(ctx context.Context) error {
	return r(ctx)
}

// NeedLeaderElection make sure the Runnable will run on every instance
func (r *LeaderRunnableFunc) NeedLeaderElection() bool {
	return true
}
