package application_test

import (
	"errors"
	"testing"
	"testing/synctest"
	"time"

	"github.com/pan-daniel/line-proton/internal/application"
)

func TestWaitReturnsOnceTheRunnerIsIdle(test *testing.T) {
	synctest.Test(test, func(test *testing.T) {
		activity := activityAnswering(true, true, false)
		started := time.Now()
		err := application.NewIdleWaiter(activity, 2*time.Second).Wait(test.Context(), time.Minute, true)
		if err != nil || time.Since(started) != 4*time.Second {
			test.Errorf("Wait = %v after %v", err, time.Since(started))
		}
	})
}

func TestBoundedWaitGivesUp(test *testing.T) {
	synctest.Test(test, func(test *testing.T) {
		activity := activityAnswering(true)
		err := application.NewIdleWaiter(activity, 2*time.Second).Wait(test.Context(), 10*time.Second, true)
		if !errors.Is(err, application.ErrRunnerStillBusy) {
			test.Errorf("err = %v, want ErrRunnerStillBusy", err)
		}
	})
}
