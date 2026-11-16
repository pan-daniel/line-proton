package registry

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf16"

	"github.com/pan-daniel/line-proton/internal/domain/winreg"
)

type Wine interface {
	Run(ctx context.Context, executable string, arguments ...string) error
}

func Render(patch winreg.Patch) string {
	var builder strings.Builder
	builder.WriteString("Windows Registry Editor Version 5.00\r\n")
	for _, key := range patch.Keys() {
		builder.WriteString(renderKey(key))
	}
	return builder.String()
}

func renderKey(key winreg.Key) string {
	lines := []string{"", "[" + key.Path() + "]"}
	for _, value := range key.Values() {
		lines = append(lines, renderValue(value))
	}
	return strings.Join(lines, "\r\n") + "\r\n"
}

func renderValue(value winreg.Value) string {
	name := quote(value.Name())
	switch value.Kind() {
	case winreg.DWord:
		return fmt.Sprintf("%s=dword:%08x", name, value.Number())
	case winreg.MultiString:
		return name + "=hex(7):" + hexUTF16(value.Items())
	}
	return name + "=" + quote(value.Text())
}

func quote(text string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(text) + `"`
}

func hexUTF16(items []string) string {
	units := utf16.Encode([]rune(strings.Join(items, "\x00") + "\x00\x00"))
	pairs := make([]string, 0, len(units)*2)
	for _, unit := range units {
		pairs = append(pairs, fmt.Sprintf("%02x", byte(unit)), fmt.Sprintf("%02x", byte(unit>>8)))
	}
	return strings.Join(pairs, ",")
}

type Importer struct {
	prefix string
	wine   Wine
}

func NewImporter(prefix string, wine Wine) Importer {
	return Importer{prefix: prefix, wine: wine}
}

func (importer Importer) Import(ctx context.Context, patch winreg.Patch) error {
	file := filepath.Join(importer.prefix, "drive_c", "line-proton.reg")
	if err := os.WriteFile(file, []byte(Render(patch)), 0o644); err != nil {
		return err
	}
	return errors.Join(importer.wine.Run(ctx, "regedit", "/S", `C:\line-proton.reg`), os.Remove(file))
}
