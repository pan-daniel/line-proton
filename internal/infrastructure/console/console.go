package console

import (
	"context"
	"io"
	"log/slog"
	"slices"
	"strings"
)

type Handler struct {
	output     io.Writer
	minimum    slog.Level
	attributes []slog.Attr
}

func NewHandler(output io.Writer, minimum slog.Level) Handler {
	return Handler{output: output, minimum: minimum}
}

func (handler Handler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= handler.minimum
}

func (handler Handler) Handle(_ context.Context, record slog.Record) error {
	_, err := io.WriteString(handler.output, handler.format(record))
	return err
}

func (handler Handler) WithAttrs(attributes []slog.Attr) slog.Handler {
	return Handler{output: handler.output, minimum: handler.minimum, attributes: slices.Concat(handler.attributes, attributes)}
}

func (handler Handler) WithGroup(string) slog.Handler {
	return handler
}

func (handler Handler) format(record slog.Record) string {
	parts := []string{marker(record.Level) + " " + record.Message}
	for _, attribute := range handler.attributes {
		parts = append(parts, formatted(attribute))
	}
	record.Attrs(func(attribute slog.Attr) bool {
		parts = append(parts, formatted(attribute))
		return true
	})
	return strings.Join(parts, " ") + "\n"
}

func formatted(attribute slog.Attr) string {
	value := attribute.Value.String()
	if strings.ContainsAny(value, " \t\"") {
		return attribute.Key + "=" + quote(value)
	}
	return attribute.Key + "=" + value
}

func quote(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `\"`) + `"`
}

func marker(level slog.Level) string {
	switch {
	case level >= slog.LevelError:
		return "✗"
	case level >= slog.LevelWarn:
		return "!"
	case level >= slog.LevelInfo:
		return "→"
	}
	return "·"
}

type Announcer struct {
	logger *slog.Logger
}

func NewAnnouncer(logger *slog.Logger) Announcer {
	return Announcer{logger: logger}
}

func (announcer Announcer) Announce(stepName string) error {
	announcer.logger.Info(stepName)
	return nil
}

type LogWriter struct {
	logger *slog.Logger
}

func NewLogWriter(logger *slog.Logger) LogWriter {
	return LogWriter{logger: logger}
}

func (writer LogWriter) Write(content []byte) (int, error) {
	writer.logger.Debug(strings.TrimRight(string(content), "\n"))
	return len(content), nil
}
