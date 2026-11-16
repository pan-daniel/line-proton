package application

import (
	"context"
	"errors"
	"slices"

	"github.com/pan-daniel/line-proton/internal/domain/runner"
	"github.com/pan-daniel/line-proton/internal/domain/signing"
	"github.com/pan-daniel/line-proton/internal/shared/sequence"
)

type RunnerRefresher struct {
	catalog   runner.Catalog
	workshop  runner.Workshop
	activity  RunnerActivity
	inspector signing.Inspector
	signer    signing.Signer
	source    string
}

func NewRunnerRefresher(catalog runner.Catalog, workshop runner.Workshop, activity RunnerActivity, inspector signing.Inspector, signer signing.Signer, source string) RunnerRefresher {
	return RunnerRefresher{catalog: catalog, workshop: workshop, activity: activity, inspector: inspector, signer: signer, source: source}
}

func (refresher RunnerRefresher) Refresh(_ context.Context) (runner.Freshness, error) {
	freshness, source, err := refresher.assess()
	if err != nil || !freshness.RequiresBuild() {
		return freshness, err
	}
	return freshness, refresher.rebuildFrom(source)
}

func (refresher RunnerRefresher) assess() (runner.Freshness, runner.Runner, error) {
	line, err := refresher.catalog.Find(runner.LineRunnerName)
	if err != nil {
		return runner.SourceMissing, runner.Runner{}, err
	}
	source, err := refresher.catalog.Find(refresher.source)
	sourceRunner, _ := source.Runner()
	return runner.Assess(line, source), sourceRunner, err
}

func (refresher RunnerRefresher) rebuildFrom(source runner.Runner) error {
	staging, err := refresher.workshop.Stage(source)
	if err != nil {
		return err
	}
	if err := refresher.signAll(staging); err != nil {
		return errors.Join(err, staging.Discard())
	}
	if err := ensureIdle(refresher.activity); err != nil {
		return errors.Join(err, staging.Discard())
	}
	if err := staging.Publish(); err != nil {
		return errors.Join(err, staging.Discard())
	}
	return nil
}

func (refresher RunnerRefresher) signAll(staging runner.Staging) error {
	binaries, err := staging.Binaries()
	if err != nil {
		return err
	}
	return sequence.ForEach(slices.Values(binaries), refresher.signIfNeeded)
}

func (refresher RunnerRefresher) signIfNeeded(path string) error {
	state, err := refresher.inspector.Inspect(path)
	if err != nil || !state.NeedsSigning() {
		return err
	}
	return refresher.signer.Sign(path)
}

func ensureIdle(activity RunnerActivity) error {
	busy, err := activity.InUse()
	if err != nil {
		return err
	}
	if busy {
		return ErrRunnerStillBusy
	}
	return nil
}
