package application_test

import (
	"github.com/pan-daniel/line-proton/internal/domain/runner"
)

type fakeCatalog struct {
	runners  map[string]runner.Runner
	failures map[string]error
	journal  journal
}

func (catalog fakeCatalog) Installed() ([]runner.Runner, error) {
	installed := []runner.Runner{}
	for _, found := range catalog.runners {
		installed = append(installed, found)
	}
	return installed, nil
}

func (catalog fakeCatalog) Find(name string) (runner.Installed, error) {
	if failure := catalog.failures[name]; failure != nil {
		return runner.Absent(), failure
	}
	found, present := catalog.runners[name]
	if !present {
		return runner.Absent(), nil
	}
	return runner.Present(found), nil
}

func (catalog fakeCatalog) Remove(name string) error {
	return catalog.journal.add("remove runner " + name)
}

type fakeWorkshop struct {
	staging fakeStaging
}

func (workshop fakeWorkshop) Stage(runner.Runner) (runner.Staging, error) {
	return workshop.staging, nil
}

type fakeStaging struct {
	binaries     []string
	publishError error
	journal      journal
}

func (staging fakeStaging) Binaries() ([]string, error) {
	return staging.binaries, nil
}

func (staging fakeStaging) Publish() error {
	_ = staging.journal.add("publish")
	return staging.publishError
}

func (staging fakeStaging) Discard() error {
	return staging.journal.add("discard")
}

type scriptedActivity struct {
	answers *[]bool
}

func activityAnswering(answers ...bool) scriptedActivity {
	return scriptedActivity{answers: &answers}
}

func (activity scriptedActivity) InUse() (bool, error) {
	busy := (*activity.answers)[0]
	*activity.answers = (*activity.answers)[min(1, len(*activity.answers)-1):]
	return busy, nil
}
