package fontlib_test

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/pan-daniel/line-proton/internal/domain/fonts"
	"github.com/pan-daniel/line-proton/internal/infrastructure/fontlib"
)

func emptyFile(test *testing.T, path string) string {
	test.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		test.Fatal(err)
	}
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		test.Fatal(err)
	}
	return path
}

func TestLocateFindsFontsInNestedDirectoriesInRequestedOrder(test *testing.T) {
	system, user := test.TempDir(), test.TempDir()
	thai := emptyFile(test, filepath.Join(system, "noto", "NotoSansThai-Regular.ttf"))
	cjk := emptyFile(test, filepath.Join(user, "NotoSansCJK-Regular.ttc"))
	located, err := fontlib.NewLibrary(system, user, filepath.Join(system, "missing")).Locate([]string{"NotoSansCJK-Regular.ttc", "NotoSansThai-Regular.ttf"})
	if err != nil || !slices.Equal(located, []string{cjk, thai}) {
		test.Errorf("Locate = %v, %v", located, err)
	}
}

func TestLocatePrefersEarlierDirectories(test *testing.T) {
	system, user := test.TempDir(), test.TempDir()
	first := emptyFile(test, filepath.Join(system, "NotoSansCJK-Bold.ttc"))
	emptyFile(test, filepath.Join(user, "NotoSansCJK-Bold.ttc"))
	located, err := fontlib.NewLibrary(system, user).Locate([]string{"NotoSansCJK-Bold.ttc"})
	if err != nil || !slices.Equal(located, []string{first}) {
		test.Errorf("Locate = %v, %v", located, err)
	}
}

func TestLocateNamesWhatIsMissing(test *testing.T) {
	_, err := fontlib.NewLibrary(test.TempDir()).Locate([]string{"NotoSansCJK-Bold.ttc"})
	if !errors.Is(err, fonts.ErrMissingFonts) {
		test.Errorf("err = %v, want ErrMissingFonts", err)
	}
}

func TestDistributionIdentifiers(test *testing.T) {
	release := filepath.Join(test.TempDir(), "os-release")
	content := "NAME=\"CachyOS Linux\"\nID=cachyos\nID_LIKE=\"arch\"\n"
	if err := os.WriteFile(release, []byte(content), 0o644); err != nil {
		test.Fatal(err)
	}
	if got := fontlib.DistributionIdentifiers(release); !slices.Equal(got, []string{"cachyos", "arch"}) {
		test.Errorf("identifiers = %v", got)
	}
}
