package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/pan-daniel/line-proton/internal/domain/hooks"
	"github.com/pan-daniel/line-proton/internal/domain/install"
	"github.com/pan-daniel/line-proton/internal/domain/runner"
	"github.com/pan-daniel/line-proton/internal/shared/pipeline"
)

type HookRunnerDependencies struct {
	Lock         Lock
	Waiter       IdleWaiter
	PrefixSigner PrefixSigner
	Refresher    RunnerRefresher
	Fonts        FontProvisioner
	Wine         Wine
	Installation install.Installation
	Announcer    pipeline.Announcer
	Logger       *slog.Logger
}

type HookRunner struct {
	lock         Lock
	waiter       IdleWaiter
	prefixSigner PrefixSigner
	refresher    RunnerRefresher
	fonts        FontProvisioner
	wine         Wine
	installation install.Installation
	announcer    pipeline.Announcer
	logger       *slog.Logger
}

func NewHookRunner(dependencies HookRunnerDependencies) HookRunner {
	return HookRunner{
		lock:         dependencies.Lock,
		waiter:       dependencies.Waiter,
		prefixSigner: dependencies.PrefixSigner,
		refresher:    dependencies.Refresher,
		fonts:        dependencies.Fonts,
		wine:         dependencies.Wine,
		installation: dependencies.Installation,
		announcer:    dependencies.Announcer,
		logger:       dependencies.Logger,
	}
}

func (hookRunner HookRunner) Run(ctx context.Context, moment hooks.Moment) error {
	phases, err := hooks.Phases(moment)
	if err != nil {
		return err
	}
	return hookRunner.deferredWhenBusy(hookRunner.runPhases(ctx, moment, phases))
}

func (hookRunner HookRunner) runPhases(ctx context.Context, moment hooks.Moment, phases []hooks.Phase) error {
	if len(phases) == 0 {
		return nil
	}
	if err := hookRunner.runPhase(ctx, moment, phases[0]); err != nil {
		return err
	}
	return hookRunner.runPhases(ctx, moment, phases[1:])
}

func (hookRunner HookRunner) runPhase(ctx context.Context, moment hooks.Moment, phase hooks.Phase) error {
	steps, err := hookRunner.stepsFor(phase, moment)
	if err != nil {
		return err
	}
	release, acquired, err := hookRunner.acquire(phase.Locking())
	if err != nil {
		return err
	}
	if !acquired {
		return hookRunner.skipBusyPhase(phase)
	}
	return errors.Join(pipeline.Run(ctx, hookRunner.announcer, steps...), release())
}

func (hookRunner HookRunner) acquire(locking hooks.Locking) (func() error, bool, error) {
	switch locking {
	case hooks.SkipWhenBusy:
		return hookRunner.lock.TryAcquire()
	case hooks.WaitWhenBusy:
		return hookRunner.waitForLock()
	case hooks.Unlocked:
		return nothingToRelease, true, nil
	}
	return nothingToRelease, false, fmt.Errorf("%w: %d", hooks.ErrUnknownLocking, int(locking))
}

func (hookRunner HookRunner) waitForLock() (func() error, bool, error) {
	release, err := hookRunner.lock.Acquire()
	return release, err == nil, err
}

func nothingToRelease() error {
	return nil
}

func (hookRunner HookRunner) skipBusyPhase(phase hooks.Phase) error {
	hookRunner.logger.Info("another line-proton run holds the lock, skipping", "phase", phase.String())
	return nil
}

func (hookRunner HookRunner) stepsFor(phase hooks.Phase, moment hooks.Moment) ([]pipeline.Step, error) {
	steps := []pipeline.Step{}
	failures := []error{}
	for _, step := range phase.Steps() {
		built, err := hookRunner.stepFor(step, moment)
		steps = append(steps, built)
		failures = append(failures, err)
	}
	return steps, errors.Join(failures...)
}

func (hookRunner HookRunner) stepFor(step hooks.Step, moment hooks.Moment) (pipeline.Step, error) {
	action, err := hookRunner.actionFor(step, moment)
	if err != nil {
		return pipeline.Step{}, err
	}
	return pipeline.NewStep(step.String(), action), nil
}

func (hookRunner HookRunner) actionFor(step hooks.Step, moment hooks.Moment) (func(context.Context) error, error) {
	switch step {
	case hooks.RefreshRunner:
		return hookRunner.refreshRunner, nil
	case hooks.SignPrefix:
		return hookRunner.prefixSigner.Sign, nil
	case hooks.RunLINE:
		return hookRunner.runLINE, nil
	case hooks.WaitUntilRunnerIdle:
		return hookRunner.waitForIdle(moment), nil
	case hooks.EnsureFonts:
		return hookRunner.fonts.EnsureFonts, nil
	}
	return nil, fmt.Errorf("%w: %d", hooks.ErrUnknownStep, int(step))
}

func (hookRunner HookRunner) refreshRunner(ctx context.Context) error {
	freshness, err := hookRunner.refresher.Refresh(ctx)
	if err != nil {
		return hookRunner.deferredWhenBusy(err)
	}
	if freshness == runner.SourceMissing {
		return hookRunner.keepCurrentRunner()
	}
	return nil
}

func (hookRunner HookRunner) keepCurrentRunner() error {
	hookRunner.logger.Warn("source Proton is gone, keeping the current Proton-LINE", "source", hookRunner.installation.SourceRunner())
	return nil
}

func (hookRunner HookRunner) deferredWhenBusy(err error) error {
	if !errors.Is(err, ErrRunnerStillBusy) {
		return err
	}
	hookRunner.logger.Warn("deferred until Proton-LINE is idle; the next hook will retry", "reason", err.Error())
	return nil
}

func (hookRunner HookRunner) runLINE(ctx context.Context) error {
	return hookRunner.wine.Run(ctx, hookRunner.installation.LauncherExecutable())
}

func (hookRunner HookRunner) waitForIdle(moment hooks.Moment) func(context.Context) error {
	limit, bounded := hooks.IdleLimit(moment)
	return func(ctx context.Context) error {
		return hookRunner.waiter.Wait(ctx, limit, bounded)
	}
}
