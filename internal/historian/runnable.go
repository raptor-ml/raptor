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
