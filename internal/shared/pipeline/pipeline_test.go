package pipeline_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/pan-daniel/line-proton/internal/shared/pipeline"
)

type announcements struct {
	names *[]string
}

func (recorder announcements) Announce(stepName string) error {
	*recorder.names = append(*recorder.names, stepName)
	return nil
}

func succeeding(name string) pipeline.Step {
	return pipeline.NewStep(name, func(context.Context) error { return nil })
}

func TestRunAnnouncesEveryStepInOrder(test *testing.T) {
	announced := []string{}
	err := pipeline.Run(test.Context(), announcements{names: &announced}, succeeding("first"), succeeding("second"))
	if err != nil || !slices.Equal(announced, []string{"first", "second"}) {
		test.Errorf("announced = %v, err = %v", announced, err)
	}
}

func TestRunStopsAtFirstFailureAndNamesTheStep(test *testing.T) {
	broken := errors.New("broken")
	announced := []string{}
	err := pipeline.Run(test.Context(), announcements{names: &announced},
		pipeline.NewStep("signing prefix", func(context.Context) error { return broken }),
		succeeding("later"),
	)
	if !errors.Is(err, broken) || !strings.HasPrefix(err.Error(), "signing prefix: ") {
		test.Errorf("err = %v, want it wrapped with the step name", err)
	}
	if slices.Contains(announced, "later") {
		test.Error("the step after the err ran")
	}
}
