package application_test

import (
	"slices"
	"time"

	"github.com/pan-daniel/line-proton/internal/domain/prefix"
	"github.com/pan-daniel/line-proton/internal/domain/signing"
)

type journal struct {
	entries *[]string
}

func newJournal() journal {
	return journal{entries: &[]string{}}
}

func (record journal) add(entry string) error {
	*record.entries = append(*record.entries, entry)
	return nil
}

func (record journal) list() []string {
	return slices.Clone(*record.entries)
}

type recordingSigner struct {
	journal  journal
	failures map[string]error
}

func (signer recordingSigner) Sign(path string) error {
	_ = signer.journal.add("sign " + path)
	return signer.failures[path]
}

type fixedInspector struct {
	states map[string]signing.State
}

func (inspector fixedInspector) Inspect(path string) (signing.State, error) {
	return inspector.states[path], nil
}

type fakeInventory struct {
	files       []prefix.SystemFile
	scanError   error
	marker      prefix.Marker
	markerError error
	journal     journal
}

func newFakeInventory(events journal, files ...prefix.SystemFile) fakeInventory {
	return fakeInventory{files: files, marker: prefix.Unmarked(), journal: events}
}

func (inventory fakeInventory) Scan() ([]prefix.SystemFile, error) {
	_ = inventory.journal.add("scan")
	return inventory.files, inventory.scanError
}

func (inventory fakeInventory) Marker() (prefix.Marker, error) {
	return inventory.marker, inventory.markerError
}

func (inventory fakeInventory) Mark(time.Time) error {
	return inventory.journal.add("mark")
}

func (inventory fakeInventory) DetachLink(path string) error {
	return inventory.journal.add("detach " + path)
}

type presentFiles []string

func (present presentFiles) Exists(relativePath string) bool {
	return slices.Contains(present, relativePath)
}

type fakePrefixDirectory struct {
	present presentFiles
	journal journal
}

func (directory fakePrefixDirectory) Exists(relativePath string) bool {
	return directory.present.Exists(relativePath)
}

func (directory fakePrefixDirectory) Remove() error {
	return directory.journal.add("remove prefix")
}

type fakeFontTarget struct {
	systemRegistry string
	journal        journal
}

func (target fakeFontTarget) AddFonts([]string) error {
	return target.journal.add("add fonts")
}

func (target fakeFontTarget) ReadSystemRegistry() (string, error) {
	return target.systemRegistry, nil
}

type fakeProtonLock struct {
	journal journal
}

func (lock fakeProtonLock) Acquire() (func() error, error) {
	return func() error { return lock.journal.add("unlock pfx") }, lock.journal.add("lock pfx")
}
