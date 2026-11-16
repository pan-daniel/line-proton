package application_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/pan-daniel/line-proton/internal/application"
	"github.com/pan-daniel/line-proton/internal/domain/fonts"
	"github.com/pan-daniel/line-proton/internal/domain/install"
	"github.com/pan-daniel/line-proton/internal/domain/signing"
)

func installationFor(test *testing.T, kind install.Launcher) install.Installation {
	test.Helper()
	installation, err := install.New("/p", "Proton-CachyOS Latest", fonts.Japanese, "/usr/bin/line-proton", kind)
	if err != nil {
		test.Fatal(err)
	}
	return installation
}

func fontProvisionerFor(events journal, systemRegistry string, library fixedFontLibrary) application.FontProvisioner {
	target := fakeFontTarget{systemRegistry: systemRegistry, journal: events}
	return application.NewFontProvisioner(library, target, recordingRegistry{journal: events}, fonts.PlanFor(fonts.Japanese), "sudo pacman -S noto-fonts-cjk noto-fonts")
}

func setupWith(test *testing.T, events journal, library fixedFontLibrary, refresher application.RunnerRefresher) application.Setup {
	test.Helper()
	installation := installationFor(test, install.Standalone)
	wine := recordingWine{journal: events}
	return application.NewSetup(application.SetupDependencies{
		Installation:  installation,
		Store:         recordingStore{journal: events},
		Refresher:     refresher,
		Creator:       application.NewPrefixCreator(wine, presentFiles{}),
		Fonts:         fontProvisionerFor(events, "", library),
		PrefixSigner:  prefixSignerFor(newFakeInventory(events), nil, events, discardingLogger()),
		LineInstaller: application.NewLineInstaller(wine, presentFiles{}, fakeInstaller{path: "/tmp/LineInst.exe", journal: events}, fixedVerifier{signer: lyCorporation()}, installation),
		Entries:       application.NewLauncherEntries(recordingIconExtractor{journal: events}, "/icons/line.png", fakeFaugus{journal: events}, fakeDesktop{journal: events}, installation, discardingLogger()),
		Announcer:     announcedSteps{journal: newJournal()},
	})
}

func lineInstallerFor(test *testing.T, events journal, present presentFiles, verifier fixedVerifier) application.LineInstaller {
	test.Helper()
	installer := fakeInstaller{path: "/tmp/LineInst.exe", journal: events}
	return application.NewLineInstaller(recordingWine{journal: events}, present, installer, verifier, installationFor(test, install.Standalone))
}

func systemRegistryWithJapaneseSubstitutes() string {
	names := []string{
		"Segoe UI", "Segoe UI Semibold", "Segoe UI Light", "Yu Gothic", "Yu Gothic UI", "Meiryo", "Meiryo UI",
		"MS UI Gothic", "Tahoma", "Helvetica", "Arial Unicode MS", "MS Shell Dlg", "MS Shell Dlg 2",
	}
	lines := []string{}
	for _, name := range names {
		lines = append(lines, `"`+name+`"="Noto Sans CJK JP"`)
	}
	return strings.Join(lines, "\n")
}

func TestLineInstallerRunsOnlyAnInstallerSignedByLY(test *testing.T) {
	examples := []struct {
		name     string
		verifier fixedVerifier
		want     error
		journal  []string
	}{
		{"signed by LY Corporation", fixedVerifier{signer: lyCorporation()}, nil, []string{"fetch installer", "run /tmp/LineInst.exe", "discard /tmp/LineInst.exe"}},
		{"signed by someone else", fixedVerifier{signer: signing.NewSubject("Evil Corp", "010401039979", []string{"Evil Corp"}, []string{"JP"})}, application.ErrUntrustedInstaller, []string{"fetch installer", "discard /tmp/LineInst.exe"}},
		{"LY Corporation name from another organization", fixedVerifier{signer: signing.NewSubject("LY Corporation", "010401039979", []string{"Evil Corp"}, []string{"US"})}, application.ErrUntrustedInstaller, []string{"fetch installer", "discard /tmp/LineInst.exe"}},
		{"chain does not verify", fixedVerifier{err: errors.New("x509: certificate signed by unknown authority")}, application.ErrUntrustedInstaller, []string{"fetch installer", "discard /tmp/LineInst.exe"}},
	}
	for _, example := range examples {
		test.Run(example.name, func(test *testing.T) {
			events := newJournal()
			err := lineInstallerFor(test, events, presentFiles{}, example.verifier).Install(test.Context())
			if !errors.Is(err, example.want) || !slices.Equal(events.list(), example.journal) {
				test.Errorf("Install = %v, journal %q", err, events.list())
			}
		})
	}
}

func TestLineInstallerSkipsWhenLineIsAlreadyInstalled(test *testing.T) {
	events := newJournal()
	present := presentFiles{installationFor(test, install.Standalone).LauncherRelativePath()}
	if err := lineInstallerFor(test, events, present, fixedVerifier{}).Install(test.Context()); err != nil || len(events.list()) != 0 {
		test.Errorf("Install = %v, journal = %q", err, events.list())
	}
}

func TestFontProvisionerExplainsHowToInstallMissingFonts(test *testing.T) {
	events := newJournal()
	err := fontProvisionerFor(events, "", fixedFontLibrary{err: fonts.ErrMissingFonts}).CheckFonts(test.Context())
	if !errors.Is(err, fonts.ErrMissingFonts) || !strings.Contains(err.Error(), "sudo pacman -S noto-fonts-cjk noto-fonts") {
		test.Errorf("err = %v", err)
	}
}

func TestEnsureFontsReappliesThePatchOnlyWhenProtonDroppedIt(test *testing.T) {
	examples := map[string]struct {
		registry string
		want     []string
	}{
		"still applied":     {systemRegistryWithJapaneseSubstitutes(), []string{}},
		"dropped by Proton": {"", []string{"add fonts", "import registry"}},
	}
	for name, example := range examples {
		test.Run(name, func(test *testing.T) {
			events := newJournal()
			provisioner := fontProvisionerFor(events, example.registry, fixedFontLibrary{located: []string{"/fonts/cjk.ttc"}})
			if err := provisioner.EnsureFonts(test.Context()); err != nil || !slices.Equal(events.list(), example.want) {
				test.Errorf("EnsureFonts = %v, journal %q", err, events.list())
			}
		})
	}
}

func TestLauncherEntries(test *testing.T) {
	examples := []struct {
		name string
		kind install.Launcher
		want []string
	}{
		{"standalone skips Faugus", install.Standalone, []string{"icon /icons/line.png", "desktop install"}},
		{"faugus gets its game entry", install.Faugus, []string{"icon /icons/line.png", "faugus upsert", "desktop install"}},
	}
	for _, example := range examples {
		test.Run(example.name, func(test *testing.T) {
			events := newJournal()
			entries := application.NewLauncherEntries(recordingIconExtractor{journal: events}, "/icons/line.png", fakeFaugus{journal: events}, fakeDesktop{journal: events}, installationFor(test, example.kind), discardingLogger())
			if err := entries.Install(test.Context()); err != nil || !slices.Equal(events.list(), example.want) {
				test.Errorf("Install = %v, journal %q", err, events.list())
			}
		})
	}
}

func TestSetupSavesTheConfigurationFirstAndRunsEveryStepInOrder(test *testing.T) {
	events := newJournal()
	setup := setupWith(test, events, fixedFontLibrary{located: []string{"/fonts/cjk.ttc"}}, refresherFixture{}.build(test, events))
	if err := setup.Run(test.Context()); err != nil {
		test.Fatal(err)
	}
	want := []string{
		"save config",
		"sign /s/unsigned.dll", "publish",
		"run wineboot",
		"lock pfx", "scan", "mark", "unlock pfx",
		"add fonts", "import registry",
		"fetch installer", "run /tmp/LineInst.exe", "discard /tmp/LineInst.exe",
		"lock pfx", "scan", "mark", "unlock pfx",
		"icon /icons/line.png", "desktop install",
	}
	if got := events.list(); !slices.Equal(got, want) {
		test.Errorf("journal =\n%q\nwant\n%q", got, want)
	}
}

func TestSetupStopsAtTheFirstFailureAfterSavingTheConfiguration(test *testing.T) {
	events := newJournal()
	setup := setupWith(test, events, fixedFontLibrary{err: fonts.ErrMissingFonts}, refresherFixture{}.build(test, events))
	if err := setup.Run(test.Context()); !errors.Is(err, fonts.ErrMissingFonts) {
		test.Errorf("err = %v, want ErrMissingFonts", err)
	}
	if got := events.list(); !slices.Equal(got, []string{"save config"}) {
		test.Errorf("journal = %q", got)
	}
}

func TestSetupSurfacesAnUnreadableProtonLine(test *testing.T) {
	unreadable := errors.New("permission denied")
	events := newJournal()
	setup := setupWith(test, events, fixedFontLibrary{}, refresherFixture{lineUnreadable: unreadable}.build(test, events))
	if err := setup.Run(test.Context()); !errors.Is(err, unreadable) {
		test.Errorf("err = %v, want %v", err, unreadable)
	}
}
