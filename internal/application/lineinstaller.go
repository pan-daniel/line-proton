package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/pan-daniel/line-proton/internal/domain/install"
	"github.com/pan-daniel/line-proton/internal/domain/prefix"
	"github.com/pan-daniel/line-proton/internal/domain/signing"
)

type LineInstaller struct {
	wine         Wine
	contents     prefix.Contents
	installer    Installer
	publishers   signing.PublisherVerifier
	installation install.Installation
}

func NewLineInstaller(wine Wine, contents prefix.Contents, installer Installer, publishers signing.PublisherVerifier, installation install.Installation) LineInstaller {
	return LineInstaller{wine: wine, contents: contents, installer: installer, publishers: publishers, installation: installation}
}

func (lineInstaller LineInstaller) Install(ctx context.Context) error {
	if lineInstaller.contents.Exists(lineInstaller.installation.LauncherRelativePath()) {
		return nil
	}
	downloaded, err := lineInstaller.installer.Fetch(ctx)
	if err != nil {
		return err
	}
	return errors.Join(lineInstaller.runVerified(ctx, downloaded), lineInstaller.installer.Discard(downloaded))
}

func (lineInstaller LineInstaller) runVerified(ctx context.Context, downloaded string) error {
	if err := lineInstaller.verify(downloaded); err != nil {
		return err
	}
	return lineInstaller.wine.Run(ctx, downloaded)
}

func (lineInstaller LineInstaller) verify(downloaded string) error {
	signer, err := lineInstaller.publishers.VerifiedSigner(downloaded)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrUntrustedInstaller, err)
	}
	if !signing.LYCorporation().Signed(signer) {
		return fmt.Errorf("%w: signed by %q", ErrUntrustedInstaller, signer.String())
	}
	return nil
}
