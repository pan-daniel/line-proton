package application_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/pan-daniel/line-proton/internal/application"
	"github.com/pan-daniel/line-proton/internal/domain/runner"
	"github.com/pan-daniel/line-proton/internal/domain/signing"
)

func runnerNamed(test *testing.T, name, version string) runner.Runner {
	test.Helper()
	parsed, err := runner.ParseVersion(version)
	if err != nil {
		test.Fatal(err)
	}
	built, err := runner.New(name, parsed)
	if err != nil {
		test.Fatal(err)
	}
	return built
}

type refresherFixture struct {
	lineVersion    string
	lineUnreadable error
	sourceGone     bool
	busy           bool
	signFailure    error
	publishError   error
}

func (fixture refresherFixture) build(test *testing.T, events journal) application.RunnerRefresher {
	test.Helper()
	runners := map[string]runner.Runner{}
	if !fixture.sourceGone {
		runners["Proton-CachyOS Latest"] = runnerNamed(test, "Proton-CachyOS Latest", "2 new")
	}
	if fixture.lineVersion != "" {
		runners[runner.LineRunnerName] = runnerNamed(test, runner.LineRunnerName, fixture.lineVersion)
	}
	catalog := fakeCatalog{runners: runners, failures: map[string]error{runner.LineRunnerName: fixture.lineUnreadable}, journal: events}
	workshop := fakeWorkshop{staging: fakeStaging{binaries: []string{"/s/unsigned.dll", "/s/readme.dll"}, publishError: fixture.publishError, journal: events}}
	inspector := fixedInspector{states: map[string]signing.State{"/s/unsigned.dll": signing.Unsigned, "/s/readme.dll": signing.NotSignable}}
	signer := recordingSigner{journal: events, failures: map[string]error{"/s/unsigned.dll": fixture.signFailure}}
	return application.NewRunnerRefresher(catalog, workshop, activityAnswering(fixture.busy), inspector, signer, "Proton-CachyOS Latest")
}

func TestRefresh(test *testing.T) {
	signingBroke, diskFull := errors.New("signing broke"), errors.New("disk full")
	examples := []struct {
		name          string
		fixture       refresherFixture
		wantFreshness runner.Freshness
		wantError     error
		wantJournal   []string
	}{
		{"first build", refresherFixture{}, runner.NeedsBuild, nil, []string{"sign /s/unsigned.dll", "publish"}},
		{"outdated runner is rebuilt signing only what needs it", refresherFixture{lineVersion: "1 old"}, runner.Outdated, nil, []string{"sign /s/unsigned.dll", "publish"}},
		{"current runner is left alone", refresherFixture{lineVersion: "2 new"}, runner.Current, nil, []string{}},
		{"missing source is left alone", refresherFixture{lineVersion: "1 old", sourceGone: true}, runner.SourceMissing, nil, []string{}},
		{"signing failure discards the copy", refresherFixture{lineVersion: "1 old", signFailure: signingBroke}, runner.Outdated, signingBroke, []string{"sign /s/unsigned.dll", "discard"}},
		{"busy runner defers the swap", refresherFixture{lineVersion: "1 old", busy: true}, runner.Outdated, application.ErrRunnerStillBusy, []string{"sign /s/unsigned.dll", "discard"}},
		{"failed publish discards the copy", refresherFixture{lineVersion: "1 old", publishError: diskFull}, runner.Outdated, diskFull, []string{"sign /s/unsigned.dll", "publish", "discard"}},
	}
	for _, example := range examples {
		test.Run(example.name, func(test *testing.T) {
			events := newJournal()
			freshness, err := example.fixture.build(test, events).Refresh(test.Context())
			if freshness != example.wantFreshness || !errors.Is(err, example.wantError) || !slices.Equal(events.list(), example.wantJournal) {
				test.Errorf("Refresh = %v, %v, journal %q; want %v, %v, %q", freshness, err, events.list(), example.wantFreshness, example.wantError, example.wantJournal)
			}
		})
	}
}
