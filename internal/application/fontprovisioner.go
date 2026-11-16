package application

import (
	"context"
	"fmt"

	"github.com/pan-daniel/line-proton/internal/domain/fonts"
	"github.com/pan-daniel/line-proton/internal/domain/prefix"
)

type FontProvisioner struct {
	library  fonts.Library
	target   prefix.FontTarget
	registry RegistryImporter
	plan     fonts.Plan
	hint     string
}

func NewFontProvisioner(library fonts.Library, target prefix.FontTarget, registry RegistryImporter, plan fonts.Plan, hint string) FontProvisioner {
	return FontProvisioner{library: library, target: target, registry: registry, plan: plan, hint: hint}
}

func (provisioner FontProvisioner) CheckFonts(_ context.Context) error {
	_, err := provisioner.locateFonts()
	return err
}

func (provisioner FontProvisioner) InstallFonts(ctx context.Context) error {
	located, err := provisioner.locateFonts()
	if err != nil {
		return err
	}
	if err := provisioner.target.AddFonts(located); err != nil {
		return err
	}
	return provisioner.registry.Import(ctx, provisioner.plan.RegistryPatch())
}

func (provisioner FontProvisioner) EnsureFonts(ctx context.Context) error {
	systemRegistry, err := provisioner.target.ReadSystemRegistry()
	if err != nil {
		return err
	}
	if provisioner.plan.AppliedIn(systemRegistry) {
		return nil
	}
	return provisioner.InstallFonts(ctx)
}

func (provisioner FontProvisioner) locateFonts() ([]string, error) {
	located, err := provisioner.library.Locate(provisioner.plan.Files())
	if err != nil {
		return nil, fmt.Errorf("%w; install them with: %s", err, provisioner.hint)
	}
	return located, nil
}
