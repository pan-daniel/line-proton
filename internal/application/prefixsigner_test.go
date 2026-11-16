package application_test

import (
	"bytes"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/pan-daniel/line-proton/internal/application"
	"github.com/pan-daniel/line-proton/internal/domain/prefix"
	"github.com/pan-daniel/line-proton/internal/domain/signing"
)

var noon = time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)

func clockAt(moment time.Time) application.Clock {
	return func() time.Time { return moment }
}

func discardingLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

func bufferedLogger(output *bytes.Buffer) *slog.Logger {
	return slog.New(slog.NewTextHandler(output, nil))
}

func prefixSignerFor(inventory fakeInventory, states map[string]signing.State, events journal, logger *slog.Logger) application.PrefixSigner {
	return application.NewPrefixSigner(inventory, fakeProtonLock{journal: events}, fixedInspector{states: states}, recordingSigner{journal: events}, clockAt(noon), logger)
}

func TestPrefixSignerHoldsProtonsPrefixLockWhileSigningDetachingAndMarking(test *testing.T) {
	events := newJournal()
	inventory := newFakeInventory(events,
		prefix.NewSystemFile("/p/unsigned.dll", prefix.RegularFile, noon),
		prefix.NewSystemFile("/p/foreign.dll", prefix.LinkElsewhere, noon),
		prefix.NewSystemFile("/p/runner.dll", prefix.LinkIntoLineRunner, noon),
		prefix.NewSystemFile("/p/signed.dll", prefix.RegularFile, noon),
	)
	states := map[string]signing.State{
		"/p/unsigned.dll": signing.Unsigned,
		"/p/foreign.dll":  signing.Unsigned,
		"/p/signed.dll":   signing.SignedByUs,
	}
	if err := prefixSignerFor(inventory, states, events, discardingLogger()).Sign(test.Context()); err != nil {
		test.Fatal(err)
	}
	want := []string{"lock pfx", "scan", "sign /p/unsigned.dll", "detach /p/foreign.dll", "sign /p/foreign.dll", "mark", "unlock pfx"}
	if got := events.list(); !slices.Equal(got, want) {
		test.Errorf("journal = %q, want %q", got, want)
	}
}

func TestPrefixSignerDoesNothingWhenTheMarkerCoversEverything(test *testing.T) {
	events := newJournal()
	inventory := newFakeInventory(events, prefix.NewSystemFile("/p/old.dll", prefix.RegularFile, noon.Add(-time.Hour)))
	inventory.marker = prefix.MarkedAt(noon)
	if err := prefixSignerFor(inventory, nil, events, discardingLogger()).Sign(test.Context()); err != nil {
		test.Fatal(err)
	}
	if got := events.list(); !slices.Equal(got, []string{"lock pfx", "scan", "unlock pfx"}) {
		test.Errorf("journal = %q", got)
	}
}

func TestPrefixSignerWarnsAboutDamagedSignaturesAndLeavesThemAlone(test *testing.T) {
	events := newJournal()
	var logs bytes.Buffer
	inventory := newFakeInventory(events, prefix.NewSystemFile("/p/damaged.dll", prefix.RegularFile, noon))
	states := map[string]signing.State{"/p/damaged.dll": signing.Damaged}
	if err := prefixSignerFor(inventory, states, events, bufferedLogger(&logs)).Sign(test.Context()); err != nil {
		test.Fatal(err)
	}
	if slices.Contains(events.list(), "sign /p/damaged.dll") {
		test.Error("a damaged file was signed again")
	}
	if !strings.Contains(logs.String(), "level=WARN") || !strings.Contains(logs.String(), "/p/damaged.dll") {
		test.Errorf("logs = %q", logs.String())
	}
}

func TestPrefixSignerSkipsBrokenLinks(test *testing.T) {
	events := newJournal()
	inventory := newFakeInventory(events, prefix.NewSystemFile("/p/broken.dll", prefix.BrokenLink, noon))
	if err := prefixSignerFor(inventory, nil, events, discardingLogger()).Sign(test.Context()); err != nil {
		test.Fatal(err)
	}
	if got := events.list(); !slices.Equal(got, []string{"lock pfx", "scan", "mark", "unlock pfx"}) {
		test.Errorf("journal = %q", got)
	}
}

func TestPrefixSignerLeavesStubsThatCannotBeSignedAloneAndCarriesOn(test *testing.T) {
	events := newJournal()
	inventory := newFakeInventory(events,
		prefix.NewSystemFile("/p/ddhelp.exe", prefix.RegularFile, noon),
		prefix.NewSystemFile("/p/dosx.exe", prefix.LinkElsewhere, noon),
		prefix.NewSystemFile("/p/unsigned.dll", prefix.RegularFile, noon),
	)
	states := map[string]signing.State{
		"/p/ddhelp.exe":   signing.NotSignable,
		"/p/dosx.exe":     signing.NotSignable,
		"/p/unsigned.dll": signing.Unsigned,
	}
	if err := prefixSignerFor(inventory, states, events, discardingLogger()).Sign(test.Context()); err != nil {
		test.Fatal(err)
	}
	want := []string{"lock pfx", "scan", "sign /p/unsigned.dll", "mark", "unlock pfx"}
	if got := events.list(); !slices.Equal(got, want) {
		test.Errorf("journal = %q, want %q", got, want)
	}
}
