package application

import (
	"context"

	"github.com/pan-daniel/line-proton/internal/domain/install"
	"github.com/pan-daniel/line-proton/internal/domain/runner"
	"github.com/pan-daniel/line-proton/internal/shared/pipeline"
)

type SetupDependencies struct {
	Installation  install.Installation
	Store         install.Store
	Refresher     RunnerRefresher
	Creator       PrefixCreator
	Fonts         FontProvisioner
	PrefixSigner  PrefixSigner
	LineInstaller LineInstaller
	Entries       LauncherEntries
	Announcer     pipeline.Announcer
}

type Setup struct {
	installation  install.Installation
	store         install.Store
	refresher     RunnerRefresher
	creator       PrefixCreator
	fonts         FontProvisioner
	prefixSigner  PrefixSigner
	lineInstaller LineInstaller
	entries       LauncherEntries
	announcer     pipeline.Announcer
}

func NewSetup(dependencies SetupDependencies) Setup {
	return Setup{
		installation:  dependencies.Installation,
		store:         dependencies.Store,
		refresher:     dependencies.Refresher,
		creator:       dependencies.Creator,
		fonts:         dependencies.Fonts,
		prefixSigner:  dependencies.PrefixSigner,
		lineInstaller: dependencies.LineInstaller,
		entries:       dependencies.Entries,
		announcer:     dependencies.Announcer,
	}
}

func (setup Setup) Run(ctx context.Context) error {
	return pipeline.Run(ctx, setup.announcer,
		pipeline.NewStep("saving the configuration", setup.saveConfiguration),
		pipeline.NewStep("checking fonts", setup.fonts.CheckFonts),
		pipeline.NewStep("building Proton-LINE", setup.ensureRunner),
		pipeline.NewStep("creating the Wine prefix", setup.creator.Create),
		pipeline.NewStep("signing the prefix", setup.prefixSigner.Sign),
		pipeline.NewStep("installing fonts", setup.fonts.InstallFonts),
		pipeline.NewStep("installing LINE (click through the installer window)", setup.lineInstaller.Install),
		pipeline.NewStep("signing the prefix again", setup.prefixSigner.Sign),
		pipeline.NewStep("adding launcher entries", setup.entries.Install),
	)
}

func (setup Setup) saveConfiguration(_ context.Context) error {
	return setup.store.Save(setup.installation)
}

func (setup Setup) ensureRunner(ctx context.Context) error {
	freshness, err := setup.refresher.Refresh(ctx)
	if err != nil {
		return err
	}
	if freshness == runner.SourceMissing {
		return ErrSourceMissing
	}
	return nil
}
