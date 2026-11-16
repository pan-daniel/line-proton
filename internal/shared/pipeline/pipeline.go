package pipeline

import (
	"context"
	"fmt"
)

type Announcer interface {
	Announce(stepName string) error
}

type Step struct {
	name    string
	perform func(context.Context) error
}

func NewStep(name string, perform func(context.Context) error) Step {
	return Step{name: name, perform: perform}
}

func (step Step) Name() string {
	return step.name
}

func Run(ctx context.Context, announcer Announcer, steps ...Step) error {
	if len(steps) == 0 {
		return nil
	}
	if err := steps[0].runAnnounced(ctx, announcer); err != nil {
		return err
	}
	return Run(ctx, announcer, steps[1:]...)
}

func (step Step) runAnnounced(ctx context.Context, announcer Announcer) error {
	if err := announcer.Announce(step.name); err != nil {
		return err
	}
	if err := step.perform(ctx); err != nil {
		return fmt.Errorf("%s: %w", step.name, err)
	}
	return nil
}
