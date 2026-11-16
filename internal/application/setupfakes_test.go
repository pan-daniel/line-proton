package application_test

import (
	"context"

	"github.com/pan-daniel/line-proton/internal/domain/install"
	"github.com/pan-daniel/line-proton/internal/domain/launcher"
	"github.com/pan-daniel/line-proton/internal/domain/signing"
	"github.com/pan-daniel/line-proton/internal/domain/winreg"
)

type recordingWine struct {
	journal journal
}

func (wine recordingWine) Run(_ context.Context, executable string, _ ...string) error {
	return wine.journal.add("run " + executable)
}

type fakeFaugus struct {
	fields     map[string]string
	unreadable error
	running    bool
	journal    journal
}

func (faugus fakeFaugus) Upsert(launcher.FaugusGame) error {
	return faugus.journal.add("faugus upsert")
}

func (faugus fakeFaugus) Game() (map[string]string, bool, error) {
	return faugus.fields, faugus.fields != nil, faugus.unreadable
}

func (faugus fakeFaugus) Remove() error {
	return faugus.journal.add("faugus remove")
}

func (faugus fakeFaugus) Running() (bool, error) {
	return faugus.running, nil
}

type fakeDesktop struct {
	journal journal
}

func (desktop fakeDesktop) Install(launcher.DesktopEntry) error {
	return desktop.journal.add("desktop install")
}

func (desktop fakeDesktop) Installed() bool {
	return true
}

func (desktop fakeDesktop) Remove() error {
	return desktop.journal.add("desktop remove")
}

type recordingIconExtractor struct {
	journal journal
}

func (extractor recordingIconExtractor) ExtractPNG(_, destination string) error {
	return extractor.journal.add("icon " + destination)
}

type fakeInstaller struct {
	path    string
	journal journal
}

func (installer fakeInstaller) Fetch(context.Context) (string, error) {
	return installer.path, installer.journal.add("fetch installer")
}

func (installer fakeInstaller) Discard(path string) error {
	return installer.journal.add("discard " + path)
}

type fixedVerifier struct {
	signer signing.Subject
	err    error
}

func lyCorporation() signing.Subject {
	return signing.NewSubject("LY Corporation", "010401039979", []string{"LY Corporation"}, []string{"JP"})
}

func (verifier fixedVerifier) VerifiedSigner(string) (signing.Subject, error) {
	return verifier.signer, verifier.err
}

type fixedFontLibrary struct {
	located []string
	err     error
}

func (library fixedFontLibrary) Locate([]string) ([]string, error) {
	return library.located, library.err
}

type recordingRegistry struct {
	journal journal
}

func (registry recordingRegistry) Import(context.Context, winreg.Patch) error {
	return registry.journal.add("import registry")
}

type recordingStore struct {
	journal journal
}

func (store recordingStore) Load() (install.Installation, bool, error) {
	return install.Installation{}, false, nil
}

func (store recordingStore) Save(install.Installation) error {
	return store.journal.add("save config")
}

func (store recordingStore) Remove() error {
	return store.journal.add("remove config")
}

type announcedSteps struct {
	journal journal
}

func (recorder announcedSteps) Announce(stepName string) error {
	return recorder.journal.add(stepName)
}
