package authenticode_test

import (
	"bytes"
	"debug/pe"
	"encoding/binary"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/pan-daniel/line-proton/internal/domain/signing"
	"github.com/pan-daniel/line-proton/internal/infrastructure/authenticode"
	"github.com/pan-daniel/line-proton/internal/infrastructure/certstore"
)

func windowsExecutable(test *testing.T, architecture string) string {
	test.Helper()
	return builtWindowsExecutable(test, architecture)
}

func strippedWindowsExecutable(test *testing.T, architecture string) string {
	test.Helper()
	return builtWindowsExecutable(test, architecture, "-ldflags=-s -w")
}

func builtWindowsExecutable(test *testing.T, architecture string, flags ...string) string {
	test.Helper()
	output := filepath.Join(test.TempDir(), "hello.exe")
	arguments := append(append([]string{"build"}, flags...), "-o", output, "./testdata/hello")
	command := exec.CommandContext(test.Context(), "go", arguments...)
	command.Env = append(os.Environ(), "GOOS=windows", "GOARCH="+architecture, "CGO_ENABLED=0")
	if combined, err := command.CombinedOutput(); err != nil {
		test.Fatalf("build fixture: %v\n%s", err, combined)
	}
	return output
}

func signerAt(test *testing.T, directory string) authenticode.Authenticode {
	test.Helper()
	certificate, err := certstore.New(directory, time.Now).LoadOrCreate(signing.ImpersonatedWindowsIdentity())
	if err != nil {
		test.Fatal(err)
	}
	return authenticode.New(certificate)
}

func stateOf(test *testing.T, adapter authenticode.Authenticode, path string) signing.State {
	test.Helper()
	state, err := adapter.Inspect(path)
	if err != nil {
		test.Fatal(err)
	}
	return state
}

func signed(test *testing.T, adapter authenticode.Authenticode, path string) string {
	test.Helper()
	if err := adapter.Sign(path); err != nil {
		test.Fatal(err)
	}
	return path
}

func TestFreshExecutableIsUnsigned(test *testing.T) {
	if got := stateOf(test, signerAt(test, test.TempDir()), windowsExecutable(test, "amd64")); got != signing.Unsigned {
		test.Errorf("state = %v, want unsigned", got)
	}
}

func TestSignedExecutableIsSignedByUs(test *testing.T) {
	for _, architecture := range []string{"amd64", "386"} {
		test.Run(architecture, func(test *testing.T) {
			adapter := signerAt(test, test.TempDir())
			executable := signed(test, adapter, windowsExecutable(test, architecture))
			if got := stateOf(test, adapter, executable); got != signing.SignedByUs {
				test.Errorf("state = %v, want signed by us", got)
			}
		})
	}
}

func TestExecutableSignedWithAnotherCertificateIsSignedByOther(test *testing.T) {
	executable := signed(test, signerAt(test, test.TempDir()), windowsExecutable(test, "amd64"))
	if got := stateOf(test, signerAt(test, test.TempDir()), executable); got != signing.SignedByOther {
		test.Errorf("state = %v, want signed by other", got)
	}
}

func TestTamperedExecutableIsDamaged(test *testing.T) {
	adapter := signerAt(test, test.TempDir())
	executable := signed(test, adapter, windowsExecutable(test, "amd64"))
	content, err := os.ReadFile(executable)
	if err != nil {
		test.Fatal(err)
	}
	content[len(content)/2] ^= 0xff
	if err := os.WriteFile(executable, content, 0o644); err != nil {
		test.Fatal(err)
	}
	if got := stateOf(test, adapter, executable); got != signing.Damaged {
		test.Errorf("state = %v, want damaged", got)
	}
}

func TestNonPortableExecutableIsNotSignable(test *testing.T) {
	text := filepath.Join(test.TempDir(), "notes.dll")
	if err := os.WriteFile(text, []byte("not a PE file"), 0o644); err != nil {
		test.Fatal(err)
	}
	if got := stateOf(test, signerAt(test, test.TempDir()), text); got != signing.NotSignable {
		test.Errorf("state = %v, want not signable", got)
	}
}

func TestSignRefusesSymlinks(test *testing.T) {
	link := filepath.Join(test.TempDir(), "link.exe")
	if err := os.Symlink(windowsExecutable(test, "amd64"), link); err != nil {
		test.Fatal(err)
	}
	if err := signerAt(test, test.TempDir()).Sign(link); !errors.Is(err, authenticode.ErrNotRegularFile) {
		test.Errorf("err = %v, want ErrNotRegularFile", err)
	}
}

func TestSignKeepsReadOnlyMode(test *testing.T) {
	executable := windowsExecutable(test, "amd64")
	if err := os.Chmod(executable, 0o555); err != nil {
		test.Fatal(err)
	}
	signed(test, signerAt(test, test.TempDir()), executable)
	details, err := os.Stat(executable)
	if err != nil {
		test.Fatal(err)
	}
	if details.Mode().Perm() != 0o555 {
		test.Errorf("mode = %v, want 0555", details.Mode().Perm())
	}
}

func TestSignReplacesTheFileAndLeavesHardLinksAlone(test *testing.T) {
	adapter := signerAt(test, test.TempDir())
	executable := windowsExecutable(test, "amd64")
	sibling := filepath.Join(filepath.Dir(executable), "sibling.exe")
	if err := os.Link(executable, sibling); err != nil {
		test.Fatal(err)
	}
	signed(test, adapter, executable)
	if got := stateOf(test, adapter, sibling); got != signing.Unsigned {
		test.Errorf("hard-linked sibling = %v, want unsigned", got)
	}
	leftovers, _ := filepath.Glob(filepath.Join(filepath.Dir(executable), ".*"))
	if len(leftovers) != 0 {
		test.Errorf("temporary files left behind: %v", leftovers)
	}
}

func cutShortInsideTheLastSection(test *testing.T, path string) string {
	test.Helper()
	parsed, err := pe.Open(path)
	if err != nil {
		test.Fatal(err)
	}
	defer parsed.Close()
	lastSection := parsed.Sections[len(parsed.Sections)-1]
	if err := os.Truncate(path, int64(lastSection.Offset)+8); err != nil {
		test.Fatal(err)
	}
	return path
}

func TestExecutableWhoseSectionsExtendPastTheEndIsNotSignable(test *testing.T) {
	for _, architecture := range []string{"amd64", "386"} {
		test.Run(architecture, func(test *testing.T) {
			stub := cutShortInsideTheLastSection(test, strippedWindowsExecutable(test, architecture))
			if got := stateOf(test, signerAt(test, test.TempDir()), stub); got != signing.NotSignable {
				test.Errorf("state = %v, want not signable", got)
			}
		})
	}
}

func lastSectionShrunkTo(test *testing.T, path string, rawSize uint32) string {
	test.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		test.Fatal(err)
	}
	parsed, err := pe.NewFile(bytes.NewReader(content))
	if err != nil {
		test.Fatal(err)
	}
	lastIndex := len(parsed.Sections) - 1
	lastOffset := parsed.Sections[lastIndex].Offset
	sectionTable := binary.LittleEndian.Uint32(content[0x3c:]) + 24 + uint32(parsed.SizeOfOptionalHeader)
	sizeOfRawData := sectionTable + uint32(lastIndex)*40 + 16
	binary.LittleEndian.PutUint32(content[sizeOfRawData:], rawSize)
	if err := os.WriteFile(path, content[:lastOffset+rawSize], 0o644); err != nil {
		test.Fatal(err)
	}
	return path
}

func TestExecutableWhoseLastSectionIsShorterThanTheFileAlignmentIsNotSignable(test *testing.T) {
	stub := lastSectionShrunkTo(test, strippedWindowsExecutable(test, "386"), 8)
	if got := stateOf(test, signerAt(test, test.TempDir()), stub); got != signing.NotSignable {
		test.Errorf("state = %v, want not signable", got)
	}
}

func TestStrippedExecutableIsStillSignable(test *testing.T) {
	adapter := signerAt(test, test.TempDir())
	executable := strippedWindowsExecutable(test, "386")
	if got := stateOf(test, adapter, executable); got != signing.Unsigned {
		test.Errorf("state = %v, want unsigned", got)
	}
	if got := stateOf(test, adapter, signed(test, adapter, executable)); got != signing.SignedByUs {
		test.Errorf("state = %v, want signed by us", got)
	}
}
