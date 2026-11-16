package console_test

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/pan-daniel/line-proton/internal/infrastructure/console"
)

func TestHandlerPrintsMarkerMessageAndAttributes(test *testing.T) {
	var output bytes.Buffer
	logger := slog.New(console.NewHandler(&output, slog.LevelInfo)).With("source", "Proton-CachyOS Latest")
	logger.Info("building Proton-LINE")
	logger.Warn("source Proton is gone")
	logger.Debug("hidden")
	want := "→ building Proton-LINE source=\"Proton-CachyOS Latest\"\n! source Proton is gone source=\"Proton-CachyOS Latest\"\n"
	if output.String() != want {
		test.Errorf("output =\n%q\nwant\n%q", output.String(), want)
	}
}

func TestAnnouncerLogsTheStepName(test *testing.T) {
	var output bytes.Buffer
	announcer := console.NewAnnouncer(slog.New(console.NewHandler(&output, slog.LevelInfo)))
	if err := announcer.Announce("signing the prefix"); err != nil || output.String() != "→ signing the prefix\n" {
		test.Errorf("Announce = %v, output = %q", err, output.String())
	}
}

func TestLogWriterTurnsLinesIntoDebugRecords(test *testing.T) {
	var output bytes.Buffer
	writer := console.NewLogWriter(slog.New(console.NewHandler(&output, slog.LevelDebug)))
	written, err := writer.Write([]byte("wine: starting\n"))
	if err != nil || written != len("wine: starting\n") || output.String() != "· wine: starting\n" {
		test.Errorf("Write = %d, %v, output = %q", written, err, output.String())
	}
}
