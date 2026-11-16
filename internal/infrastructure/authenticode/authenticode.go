package authenticode

import (
	"context"
	"crypto"
	"debug/pe"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"

	"github.com/pan-daniel/line-proton/internal/domain/signing"
	"github.com/sassoftware/relic/v8/lib/authenticode"
	"github.com/sassoftware/relic/v8/lib/binpatch"
	"github.com/sassoftware/relic/v8/lib/certloader"
	"github.com/sassoftware/relic/v8/signers/sigerrors"
)

var ErrNotRegularFile = errors.New("refusing to sign something that is not a regular file")

type Authenticode struct {
	certificate *certloader.Certificate
}

func New(certificate *certloader.Certificate) Authenticode {
	return Authenticode{certificate: certificate}
}

func (adapter Authenticode) Sign(path string) error {
	details, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !details.Mode().IsRegular() {
		return fmt.Errorf("%w: %s", ErrNotRegularFile, path)
	}
	original, err := os.Open(path)
	if err != nil {
		return err
	}
	defer original.Close()
	patch, err := adapter.signaturePatch(original)
	if err != nil {
		return fmt.Errorf("sign %s: %w", path, err)
	}
	return replaceWithPatchedCopy(original, patch, path, details.Mode().Perm())
}

func (adapter Authenticode) Inspect(path string) (signing.State, error) {
	file, err := os.Open(path)
	if err != nil {
		return signing.NotSignable, err
	}
	defer file.Close()
	if sectionsExtendPastTheEnd(file) {
		return signing.NotSignable, nil
	}
	signatures, verifyErr := authenticode.VerifyPE(file, false)
	return adapter.stateOf(file, signatures, verifyErr), nil
}

func (adapter Authenticode) signaturePatch(original *os.File) (*binpatch.PatchSet, error) {
	digest, err := authenticode.DigestPE(original, crypto.SHA256, false)
	if err != nil {
		return nil, err
	}
	patch, _, err := digest.Sign(context.Background(), adapter.certificate, nil)
	return patch, err
}

func replaceWithPatchedCopy(original *os.File, patch *binpatch.PatchSet, path string, mode fs.FileMode) error {
	reserved, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".line-proton-*")
	if err != nil {
		return err
	}
	temporary := reserved.Name()
	if err := errors.Join(reserved.Close(), writePatchedCopy(original, patch, temporary, mode)); err != nil {
		return errors.Join(err, os.Remove(temporary))
	}
	return os.Rename(temporary, path)
}

func writePatchedCopy(original *os.File, patch *binpatch.PatchSet, temporary string, mode fs.FileMode) error {
	if _, err := original.Seek(0, io.SeekStart); err != nil {
		return err
	}
	if err := patch.Apply(original, temporary); err != nil {
		return err
	}
	return os.Chmod(temporary, mode)
}

func (adapter Authenticode) stateOf(file io.ReaderAt, signatures []authenticode.PESignature, verifyErr error) signing.State {
	if errors.As(verifyErr, new(sigerrors.NotSignedError)) {
		return signing.Unsigned
	}
	if verifyErr != nil {
		return stateOfUnverifiable(file)
	}
	if len(signatures) == 0 {
		return signing.Unsigned
	}
	if signatures[0].Certificate.Equal(adapter.certificate.Leaf) {
		return signing.SignedByUs
	}
	return signing.SignedByOther
}

func stateOfUnverifiable(file io.ReaderAt) signing.State {
	parsed, err := pe.NewFile(file)
	if err != nil {
		return signing.NotSignable
	}
	parsed.Close()
	return signing.Damaged
}

func sectionsExtendPastTheEnd(file *os.File) bool {
	details, err := file.Stat()
	if err != nil {
		return false
	}
	parsed, err := pe.NewFile(file)
	if err != nil {
		return false
	}
	defer parsed.Close()
	return anySectionEndsBeyond(parsed, details.Size())
}

func anySectionEndsBeyond(parsed *pe.File, fileSize int64) bool {
	alignment := fileAlignmentOf(parsed.OptionalHeader)
	return slices.ContainsFunc(parsed.Sections, func(section *pe.Section) bool {
		return section.Size > 0 && alignedRawDataEnd(section, alignment) > fileSize
	})
}

func fileAlignmentOf(optionalHeader any) int64 {
	switch header := optionalHeader.(type) {
	case *pe.OptionalHeader32:
		return max(int64(header.FileAlignment), 1)
	case *pe.OptionalHeader64:
		return max(int64(header.FileAlignment), 1)
	}
	return 1
}

func alignedRawDataEnd(section *pe.Section, alignment int64) int64 {
	return int64(section.Offset) + roundedUpTo(int64(section.Size), alignment)
}

func roundedUpTo(size int64, alignment int64) int64 {
	return (size + alignment - 1) / alignment * alignment
}
