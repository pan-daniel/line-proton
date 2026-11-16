package application

import (
	"context"
	"time"
)

type IdleWaiter struct {
	activity RunnerActivity
	interval time.Duration
}

func NewIdleWaiter(activity RunnerActivity, interval time.Duration) IdleWaiter {
	return IdleWaiter{activity: activity, interval: interval}
}

func (waiter IdleWaiter) Wait(ctx context.Context, limit time.Duration, bounded bool) error {
	deadline := time.Now().Add(limit)
	finished, err := false, error(nil)
	for !finished {
		finished, err = waiter.attempt(ctx, deadline, bounded)
	}
	return err
}

func (waiter IdleWaiter) attempt(ctx context.Context, deadline time.Time, bounded bool) (bool, error) {
	busy, err := waiter.activity.InUse()
	if err != nil || !busy {
		return true, err
	}
	if bounded && !time.Now().Before(deadline) {
		return true, ErrRunnerStillBusy
	}
	return pause(ctx, waiter.interval)
}

func pause(ctx context.Context, duration time.Duration) (bool, error) {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return true, ctx.Err()
	case <-timer.C:
		return false, nil
	}
}
