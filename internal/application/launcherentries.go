package application

import (
	"context"
	"errors"
	"log/slog"

	"github.com/pan-daniel/line-proton/internal/domain/install"
	"github.com/pan-daniel/line-proton/internal/domain/launcher"
)

type LauncherEntries struct {
	icons        IconExtractor
	iconPath     string
	faugus       launcher.Faugus
	desktop      launcher.Desktop
	installation install.Installation
	logger       *slog.Logger
}

func NewLauncherEntries(icons IconExtractor, iconPath string, faugus launcher.Faugus, desktop launcher.Desktop, installation install.Installation, logger *slog.Logger) LauncherEntries {
	return LauncherEntries{icons: icons, iconPath: iconPath, faugus: faugus, desktop: desktop, installation: installation, logger: logger}
}

func (entries LauncherEntries) Install(_ context.Context) error {
	if err := entries.icons.ExtractPNG(entries.installation.LauncherExecutable(), entries.iconPath); err != nil {
		return err
	}
	if err := entries.registerWithFaugus(); err != nil {
		return err
	}
	return entries.desktop.Install(launcher.NewDesktopEntry(entries.installation, entries.iconPath))
}

func (entries LauncherEntries) Remove() error {
	return errors.Join(entries.removeFromFaugus(), entries.desktop.Remove())
}

func (entries LauncherEntries) registerWithFaugus() error {
	if !entries.installation.Launcher().UsesFaugus() {
		return nil
	}
	if err := entries.faugus.Upsert(launcher.NewFaugusGame(entries.installation, entries.iconPath)); err != nil {
		return err
	}
	return entries.adviseRestartIfFaugusIsOpen()
}

func (entries LauncherEntries) adviseRestartIfFaugusIsOpen() error {
	running, err := entries.faugus.Running()
	if err != nil || !running {
		return err
	}
	entries.logger.Warn("restart Faugus to see the LINE entry")
	return nil
}

func (entries LauncherEntries) removeFromFaugus() error {
	if !entries.installation.Launcher().UsesFaugus() {
		return nil
	}
	return entries.faugus.Remove()
}
