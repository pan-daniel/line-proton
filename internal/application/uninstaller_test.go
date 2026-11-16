package application_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/pan-daniel/line-proton/internal/application"
	"github.com/pan-daniel/line-proton/internal/domain/fonts"
	"github.com/pan-daniel/line-proton/internal/domain/install"
	"github.com/pan-daniel/line-proton/internal/domain/prefix"
	"github.com/pan-daniel/line-proton/internal/domain/runner"
)

type namedRemovable struct {
	name    string
	journal journal
}

func (removable namedRemovable) Remove() error {
	return removable.journal.add("remove " + removable.name)
}

type uninstallFixture struct {
	prefix     string
	traces     []string
	runnerBusy bool
}

func (fixture uninstallFixture) build(test *testing.T, events journal) application.Uninstaller {
	test.Helper()
	installation, err := install.New(fixture.prefix, "Proton-CachyOS Latest", fonts.Japanese, "/usr/bin/line-proton", install.Standalone)
	if err != nil {
		test.Fatal(err)
	}
	removables := []application.Removable{
		application.NewLineRunnerRemoval(fakeCatalog{journal: events}),
		namedRemovable{name: "certificate", journal: events},
	}
	return application.NewUninstaller(removables, fakePrefixDirectory{present: fixture.traces, journal: events}, fakeLock{journal: events}, activityAnswering(fixture.runnerBusy), installation, "/home/u")
}

func TestUninstall(test *testing.T) {
	marker := []string{prefix.MarkerName}
	everythingButThePrefix := []string{"lock", "remove runner " + runner.LineRunnerName, "remove certificate"}
	examples := []struct {
		name    string
		fixture uninstallFixture
		purge   bool
		want    error
		journal []string
	}{
		{"plain uninstall keeps the prefix", uninstallFixture{prefix: "/home/u/.local/share/line-proton/prefix", traces: marker}, false, nil, append(slices.Clone(everythingButThePrefix), "unlock")},
		{"purge removes a signed prefix", uninstallFixture{prefix: "/home/u/.local/share/line-proton/prefix", traces: marker}, true, nil, append(slices.Clone(everythingButThePrefix), "remove prefix", "unlock")},
		{"purge removes a prefix holding LINE", uninstallFixture{prefix: "/home/u/Faugus/line", traces: []string{"drive_c/users/steamuser/AppData/Local/LINE/bin/LineLauncher.exe"}}, true, nil, append(slices.Clone(everythingButThePrefix), "remove prefix", "unlock")},
		{"purge refuses a foreign directory", uninstallFixture{prefix: "/home/u/Games"}, true, install.ErrPrefixNotOurs, []string{}},
		{"purge refuses the home directory", uninstallFixture{prefix: "/home/u", traces: marker}, true, install.ErrPrefixNotOurs, []string{}},
		{"purge refuses the filesystem root", uninstallFixture{prefix: "/", traces: marker}, true, install.ErrPrefixNotOurs, []string{}},
		{"busy Proton-LINE refuses and removes nothing", uninstallFixture{prefix: "/home/u/Faugus/line", traces: marker, runnerBusy: true}, true, application.ErrRunnerStillBusy, []string{"lock", "unlock"}},
	}
	for _, example := range examples {
		test.Run(example.name, func(test *testing.T) {
			events := newJournal()
			err := example.fixture.build(test, events).Uninstall(example.purge)
			if !errors.Is(err, example.want) || !slices.Equal(events.list(), example.journal) {
				test.Errorf("Uninstall = %v, journal %q", err, events.list())
			}
		})
	}
}
