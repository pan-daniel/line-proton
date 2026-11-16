package application

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"time"

	"github.com/pan-daniel/line-proton/internal/domain/prefix"
	"github.com/pan-daniel/line-proton/internal/domain/signing"
	"github.com/pan-daniel/line-proton/internal/shared/sequence"
)

type PrefixSigner struct {
	prefix     prefix.Signable
	protonLock prefix.ProtonLock
	inspector  signing.Inspector
	signer     signing.Signer
	clock      Clock
	logger     *slog.Logger
}

func NewPrefixSigner(signable prefix.Signable, protonLock prefix.ProtonLock, inspector signing.Inspector, signer signing.Signer, clock Clock, logger *slog.Logger) PrefixSigner {
	return PrefixSigner{prefix: signable, protonLock: protonLock, inspector: inspector, signer: signer, clock: clock, logger: logger}
}

func (prefixSigner PrefixSigner) Sign(_ context.Context) error {
	release, err := prefixSigner.protonLock.Acquire()
	if err != nil {
		return err
	}
	return errors.Join(prefixSigner.signWhileProtonWaits(), release())
}

func (prefixSigner PrefixSigner) signWhileProtonWaits() error {
	files, err := prefixSigner.prefix.Scan()
	if err != nil {
		return err
	}
	marker, err := prefixSigner.prefix.Marker()
	if err != nil {
		return err
	}
	if marker.CoversAll(files) {
		return nil
	}
	return prefixSigner.secureAll(files, prefixSigner.clock())
}

func (prefixSigner PrefixSigner) secureAll(files []prefix.SystemFile, startedAt time.Time) error {
	if err := sequence.ForEach(slices.Values(files), prefixSigner.secure); err != nil {
		return err
	}
	return prefixSigner.prefix.Mark(startedAt)
}

func (prefixSigner PrefixSigner) secure(file prefix.SystemFile) error {
	if !file.NeedsInspection() {
		return nil
	}
	state, err := prefixSigner.inspector.Inspect(file.Path())
	if err != nil {
		return err
	}
	if state == signing.Damaged {
		return prefixSigner.leaveDamaged(file.Path())
	}
	return prefixSigner.carryOut(prefix.Decide(file, state), file.Path())
}

func (prefixSigner PrefixSigner) leaveDamaged(path string) error {
	prefixSigner.logger.Warn("signature does not verify, leaving the file untouched", "file", path)
	return nil
}

func (prefixSigner PrefixSigner) carryOut(action prefix.Action, path string) error {
	switch action {
	case prefix.SignInPlace:
		return prefixSigner.signer.Sign(path)
	case prefix.ReplaceLinkWithSignedCopy:
		return prefixSigner.detachAndSign(path)
	}
	return nil
}

func (prefixSigner PrefixSigner) detachAndSign(path string) error {
	if err := prefixSigner.prefix.DetachLink(path); err != nil {
		return err
	}
	return prefixSigner.signer.Sign(path)
}
