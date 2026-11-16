package application_test

import (
	"bytes"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/pan-daniel/line-proton/internal/application"
	"github.com/pan-daniel/line-proton/internal/domain/fonts"
	"github.com/pan-daniel/line-proton/internal/domain/hooks"
	"github.com/pan-daniel/line-proton/internal/domain/install"
	"github.com/pan-daniel/line-proton/internal/domain/prefix"
)

type fakeLock struct {
	busy    bool
	journal journal
}

func (lock fakeLock) Acquire() (func() error, error) {
	return lock.release, lock.journal.add("lock")
}

func (lock fakeLock) TryAcquire() (func() error, bool, error) {
	if lock.busy {
		return lock.release, false, nil
	}
	return lock.release, true, lock.journal.add("lock")
}

func (lock fakeLock) release() error {
	return lock.journal.add("unlock")
}

type hookFixture struct {
	lockBusy      bool
	idleAnswers   []bool
	refresher     refresherFixture
	fontsInPlace  bool
	refresherWork journal
	prefixWork    journal
}

func newHookFixture() hookFixture {
	return hookFixture{idleAnswers: []bool{false}, refresher: refresherFixture{lineVersion: "2 new"}, fontsInPlace: true, refresherWork: newJournal(), prefixWork: newJournal()}
}

func (fixture hookFixture) build(test *testing.T, events journal, logger *slog.Logger) application.HookRunner {
	test.Helper()
	installation, err := install.New("/p", "Proton-CachyOS Latest", fonts.Japanese, "/usr/bin/line-proton", install.Standalone)
	if err != nil {
		test.Fatal(err)
	}
	inventory := newFakeInventory(fixture.prefixWork, prefix.NewSystemFile("/p/a.dll", prefix.RegularFile, noon))
	prefixSigner := prefixSignerFor(inventory, nil, fixture.prefixWork, logger)
	return application.NewHookRunner(application.HookRunnerDependencies{
		Lock:         fakeLock{busy: fixture.lockBusy, journal: events},
		Waiter:       application.NewIdleWaiter(activityAnswering(fixture.idleAnswers...), time.Second),
		PrefixSigner: prefixSigner,
		Refresher:    fixture.refresher.build(test, fixture.refresherWork),
		Fonts:        fontProvisionerFor(fixture.prefixWork, fixture.systemRegistry(), fixedFontLibrary{located: []string{"/fonts/cjk.ttc"}}),
		Wine:         recordingWine{journal: events},
		Installation: installation,
		Announcer:    announcedSteps{journal: events},
		Logger:       logger,
	})
}

func (fixture hookFixture) systemRegistry() string {
	if !fixture.fontsInPlace {
		return ""
	}
	return systemRegistryWithJapaneseSubstitutes()
}

func TestPreHookSkipsWhenSomeoneElseHoldsTheLock(test *testing.T) {
	events := newJournal()
	fixture := newHookFixture()
	fixture.lockBusy = true
	if err := fixture.build(test, events, discardingLogger()).Run(test.Context(), hooks.BeforeLaunch); err != nil {
		test.Fatal(err)
	}
	if len(events.list()) != 0 || len(fixture.prefixWork.list()) != 0 {
		test.Errorf("work happened while the lock was busy: %q %q", events.list(), fixture.prefixWork.list())
	}
}

func TestStandaloneLaunchReleasesTheLockWhileLineRuns(test *testing.T) {
	events := newJournal()
	if err := newHookFixture().build(test, events, discardingLogger()).Run(test.Context(), hooks.StandaloneLaunch); err != nil {
		test.Fatal(err)
	}
	want := []string{
		"lock", "refreshing Proton-LINE", "signing prefix", "unlock",
		"running LINE", "run /p/drive_c/users/steamuser/AppData/Local/LINE/bin/LineLauncher.exe", "waiting for Proton-LINE to go idle",
		"lock", "signing prefix", "unlock",
	}
	if got := events.list(); !slices.Equal(got, want) {
		test.Errorf("journal = %q, want %q", got, want)
	}
}

func TestPostHookTimeoutStopsLaterStepsAndIsOnlyAWarning(test *testing.T) {
	synctest.Test(test, func(test *testing.T) {
		events := newJournal()
		var logs bytes.Buffer
		fixture := newHookFixture()
		fixture.idleAnswers = []bool{true}
		if err := fixture.build(test, events, bufferedLogger(&logs)).Run(test.Context(), hooks.AfterLaunch); err != nil {
			test.Fatal(err)
		}
		if got := events.list(); !slices.Equal(got, []string{"lock", "waiting for Proton-LINE to go idle", "unlock"}) {
			test.Errorf("journal = %q", got)
		}
		if len(fixture.prefixWork.list()) != 0 || len(fixture.refresherWork.list()) != 0 {
			test.Errorf("later steps ran: %q %q", fixture.prefixWork.list(), fixture.refresherWork.list())
		}
		if !strings.Contains(logs.String(), "level=WARN") {
			test.Errorf("logs = %q", logs.String())
		}
	})
}

func TestPostHookDefersABusyRebuildAndStillEnsuresFonts(test *testing.T) {
	events := newJournal()
	var logs bytes.Buffer
	fixture := newHookFixture()
	fixture.refresher = refresherFixture{lineVersion: "1 old", busy: true}
	fixture.fontsInPlace = false
	if err := fixture.build(test, events, bufferedLogger(&logs)).Run(test.Context(), hooks.AfterLaunch); err != nil {
		test.Fatal(err)
	}
	if got := fixture.refresherWork.list(); !slices.Equal(got, []string{"sign /s/unsigned.dll", "discard"}) {
		test.Errorf("refresher journal = %q", got)
	}
	if got := fixture.prefixWork.list(); !slices.Contains(got, "import registry") {
		test.Errorf("fonts were not repaired: %q", got)
	}
	if !strings.Contains(logs.String(), "deferred") {
		test.Errorf("logs = %q", logs.String())
	}
}

func TestPostHookLeavesFontsAloneWhenTheyAreStillApplied(test *testing.T) {
	fixture := newHookFixture()
	if err := fixture.build(test, newJournal(), discardingLogger()).Run(test.Context(), hooks.AfterLaunch); err != nil {
		test.Fatal(err)
	}
	if got := fixture.prefixWork.list(); slices.Contains(got, "import registry") || slices.Contains(got, "add fonts") {
		test.Errorf("fonts were reapplied: %q", got)
	}
}

func TestPostHookSurfacesAnUnreadableProtonLine(test *testing.T) {
	unreadable := errors.New("permission denied")
	fixture := newHookFixture()
	fixture.refresher = refresherFixture{lineUnreadable: unreadable}
	if err := fixture.build(test, newJournal(), discardingLogger()).Run(test.Context(), hooks.AfterLaunch); !errors.Is(err, unreadable) {
		test.Errorf("err = %v, want %v", err, unreadable)
	}
}

func TestHookRunnerRefusesAnUnknownMoment(test *testing.T) {
	events := newJournal()
	if err := newHookFixture().build(test, events, discardingLogger()).Run(test.Context(), hooks.Moment(99)); !errors.Is(err, hooks.ErrUnknownMoment) {
		test.Errorf("err = %v, want ErrUnknownMoment", err)
	}
	if len(events.list()) != 0 {
		test.Errorf("work happened for an unknown moment: %q", events.list())
	}
}

func TestManualSignWaitsForTheLockInsteadOfSkipping(test *testing.T) {
	events := newJournal()
	fixture := newHookFixture()
	fixture.lockBusy = true
	if err := fixture.build(test, events, discardingLogger()).Run(test.Context(), hooks.ManualSign); err != nil {
		test.Fatal(err)
	}
	if got := events.list(); !slices.Equal(got, []string{"lock", "signing prefix", "unlock"}) {
		test.Errorf("journal = %q", got)
	}
}
