package certstore_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pan-daniel/line-proton/internal/domain/signing"
	"github.com/pan-daniel/line-proton/internal/infrastructure/certstore"
)

func TestLoadOrCreateGeneratesTheImpersonatedIdentityOnce(test *testing.T) {
	directory := test.TempDir()
	store := certstore.New(directory, time.Now)
	first, err := store.LoadOrCreate(signing.ImpersonatedWindowsIdentity())
	if err != nil {
		test.Fatal(err)
	}
	second, err := store.LoadOrCreate(signing.ImpersonatedWindowsIdentity())
	if err != nil {
		test.Fatal(err)
	}
	if !first.Leaf.Equal(second.Leaf) {
		test.Error("second load generated a new certificate")
	}
	if first.Leaf.Subject.CommonName != "Microsoft Windows" {
		test.Errorf("common name = %q", first.Leaf.Subject.CommonName)
	}
}

func TestPrivateKeyIsReadableOnlyByOwner(test *testing.T) {
	directory := test.TempDir()
	if _, err := certstore.New(directory, time.Now).LoadOrCreate(signing.ImpersonatedWindowsIdentity()); err != nil {
		test.Fatal(err)
	}
	details, err := os.Stat(filepath.Join(directory, "key.pem"))
	if err != nil {
		test.Fatal(err)
	}
	if details.Mode().Perm() != 0o600 {
		test.Errorf("key mode = %v, want 0600", details.Mode().Perm())
	}
}

func TestRemoveDeletesBothFilesAndToleratesAbsence(test *testing.T) {
	directory := test.TempDir()
	store := certstore.New(directory, time.Now)
	if _, err := store.LoadOrCreate(signing.ImpersonatedWindowsIdentity()); err != nil {
		test.Fatal(err)
	}
	if err := store.Remove(); err != nil {
		test.Fatal(err)
	}
	if store.Exists() {
		test.Error("certificate still exists")
	}
	if err := store.Remove(); err != nil {
		test.Errorf("second remove failed: %v", err)
	}
}

func TestAHalfWrittenPairCountsAsAbsentAndIsRegenerated(test *testing.T) {
	for _, leftover := range []string{"key.pem", "cert.pem"} {
		test.Run("only "+leftover, func(test *testing.T) {
			directory := test.TempDir()
			if err := os.WriteFile(filepath.Join(directory, leftover), []byte("truncated"), 0o600); err != nil {
				test.Fatal(err)
			}
			store := certstore.New(directory, time.Now)
			if store.Exists() {
				test.Fatal("a lone " + leftover + " counts as a certificate")
			}
			certificate, err := store.LoadOrCreate(signing.ImpersonatedWindowsIdentity())
			if err != nil || certificate.Leaf.Subject.CommonName != "Microsoft Windows" {
				test.Fatalf("LoadOrCreate = %v, %v", certificate, err)
			}
			if entries, _ := os.ReadDir(directory); len(entries) != 2 || !store.Exists() {
				test.Errorf("directory holds %v, want exactly cert.pem and key.pem", entries)
			}
		})
	}
}
