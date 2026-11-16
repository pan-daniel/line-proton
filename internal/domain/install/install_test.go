package install_test

import (
	"errors"
	"testing"

	"github.com/pan-daniel/line-proton/internal/domain/fonts"
	"github.com/pan-daniel/line-proton/internal/domain/install"
)

func mustInstallation(test *testing.T, prefix string, launcher install.Launcher) install.Installation {
	test.Helper()
	installation, err := install.New(prefix, "Proton-CachyOS Latest", fonts.Japanese, "/usr/bin/line-proton", launcher)
	if err != nil {
		test.Fatal(err)
	}
	return installation
}

func TestNewValidatesPaths(test *testing.T) {
	_, err := install.New("relative/prefix", "Proton-CachyOS Latest", fonts.Japanese, "/usr/bin/line-proton", install.Standalone)
	if !errors.Is(err, install.ErrInvalidInstallation) {
		test.Errorf("err = %v, want ErrInvalidInstallation", err)
	}
}

func TestLauncherExecutableLivesInTheProtonUserProfile(test *testing.T) {
	installation := mustInstallation(test, "/home/u/Faugus/line", install.Faugus)
	want := "/home/u/Faugus/line/drive_c/users/steamuser/AppData/Local/LINE/bin/LineLauncher.exe"
	if got := installation.LauncherExecutable(); got != want {
		test.Errorf("LauncherExecutable = %q", got)
	}
}

func TestWithExecutableReturnsACopy(test *testing.T) {
	original, _ := install.New("/p", "Proton-GE Latest", fonts.Korean, "/old", install.Standalone)
	moved, err := original.WithExecutable("/new")
	if err != nil || original.Executable() != "/old" || moved.Executable() != "/new" {
		test.Errorf("original = %q, moved = %q, %v", original.Executable(), moved.Executable(), err)
	}
}

func TestOverridesChangeOnlyTheirOwnField(test *testing.T) {
	original := mustInstallation(test, "/p", install.Faugus)
	moved, err := original.WithPrefix("/elsewhere")
	if err != nil || moved.Prefix() != "/elsewhere" || original.Prefix() != "/p" {
		test.Errorf("WithPrefix = %q, %v; original %q", moved.Prefix(), err, original.Prefix())
	}
	switched, err := moved.WithSourceRunner("Proton-GE Latest")
	if err != nil || switched.SourceRunner() != "Proton-GE Latest" || switched.Prefix() != "/elsewhere" {
		test.Errorf("WithSourceRunner = %+v, %v", switched, err)
	}
	korean, err := switched.WithFontVariant(fonts.Korean)
	if err != nil || korean.FontVariant() != fonts.Korean || korean.SourceRunner() != "Proton-GE Latest" || korean.Launcher() != install.Faugus {
		test.Errorf("WithFontVariant = %+v, %v", korean, err)
	}
}

func TestOverridesAreValidated(test *testing.T) {
	original := mustInstallation(test, "/p", install.Standalone)
	if _, err := original.WithPrefix("relative"); !errors.Is(err, install.ErrInvalidInstallation) {
		test.Errorf("WithPrefix(relative) err = %v", err)
	}
	if _, err := original.WithSourceRunner(""); !errors.Is(err, install.ErrInvalidInstallation) {
		test.Errorf("WithSourceRunner(\"\") err = %v", err)
	}
	if _, err := original.WithExecutable("line-proton"); !errors.Is(err, install.ErrInvalidInstallation) {
		test.Errorf("WithExecutable(relative) err = %v", err)
	}
	if _, err := (install.Installation{}).WithFontVariant(fonts.Korean); !errors.Is(err, install.ErrInvalidInstallation) {
		test.Errorf("WithFontVariant on an empty installation err = %v", err)
	}
}

func TestDefaultPrefix(test *testing.T) {
	if got := install.DefaultPrefix("/home/u", install.Faugus); got != "/home/u/Faugus/line" {
		test.Errorf("faugus prefix = %q", got)
	}
	if got := install.DefaultPrefix("/home/u", install.Standalone); got != "/home/u/.local/share/line-proton/prefix" {
		test.Errorf("standalone prefix = %q", got)
	}
}

func TestLauncherNamesRoundTrip(test *testing.T) {
	for _, launcher := range []install.Launcher{install.Faugus, install.Standalone} {
		test.Run(launcher.String(), func(test *testing.T) {
			parsed, err := install.ParseLauncher(launcher.String())
			if err != nil || parsed != launcher {
				test.Errorf("round trip of %v = %v, %v", launcher, parsed, err)
			}
		})
	}
	if _, err := install.ParseLauncher("faugus-flatpak"); !errors.Is(err, install.ErrUnknownLauncher) {
		test.Errorf("flatpak err = %v, want ErrUnknownLauncher", err)
	}
}

func TestSupportedArchitecture(test *testing.T) {
	if !install.SupportedArchitecture("amd64") || install.SupportedArchitecture("arm64") {
		test.Error("only amd64 is supported")
	}
}

func TestPrefixRemovable(test *testing.T) {
	examples := []struct {
		name   string
		prefix string
		owned  bool
		want   error
	}{
		{"prefix carrying LINE or our marker", "/home/u/Faugus/line", true, nil},
		{"directory without our traces", "/home/u/Games/other", false, install.ErrPrefixNotOurs},
		{"home directory", "/home/u", true, install.ErrPrefixNotOurs},
		{"home directory spelled with a slash", "/home/u/", true, install.ErrPrefixNotOurs},
		{"filesystem root", "/", true, install.ErrPrefixNotOurs},
	}
	for _, example := range examples {
		test.Run(example.name, func(test *testing.T) {
			installation := mustInstallation(test, example.prefix, install.Standalone)
			if err := installation.PrefixRemovable("/home/u", example.owned); !errors.Is(err, example.want) {
				test.Errorf("PrefixRemovable = %v, want %v", err, example.want)
			}
		})
	}
}

func TestSetupRequestOverridesOnlyWhatWasAskedFor(test *testing.T) {
	saved := mustInstallation(test, "/home/u/Faugus/line", install.Faugus)
	examples := []struct {
		name    string
		request install.SetupRequest
		want    install.Installation
	}{
		{"nothing asked keeps the saved choice", install.NewSetupRequest("", "", ""), saved},
		{"prefix", install.NewSetupRequest("/elsewhere", "", ""), mustInstallation(test, "/elsewhere", install.Faugus)},
		{"source runner", install.NewSetupRequest("", "Proton-GE Latest", ""), must(test)(saved.WithSourceRunner("Proton-GE Latest"))},
		{"font", install.NewSetupRequest("", "", "KR"), must(test)(saved.WithFontVariant(fonts.Korean))},
	}
	for _, example := range examples {
		test.Run(example.name, func(test *testing.T) {
			got, err := example.request.AppliedTo(saved)
			if err != nil || got != example.want {
				test.Errorf("AppliedTo = %+v, %v; want %+v", got, err, example.want)
			}
		})
	}
}

func must(test *testing.T) func(install.Installation, error) install.Installation {
	return func(installation install.Installation, err error) install.Installation {
		test.Helper()
		if err != nil {
			test.Fatal(err)
		}
		return installation
	}
}

func TestSetupRequestRejectsUnknownFontsAndRelativePrefixes(test *testing.T) {
	saved := mustInstallation(test, "/p", install.Standalone)
	if _, err := install.NewSetupRequest("", "", "xx").AppliedTo(saved); !errors.Is(err, fonts.ErrUnknownVariant) {
		test.Errorf("unknown font err = %v, want ErrUnknownVariant", err)
	}
	if _, err := install.NewSetupRequest("relative", "", "").AppliedTo(saved); !errors.Is(err, install.ErrInvalidInstallation) {
		test.Errorf("relative prefix err = %v, want ErrInvalidInstallation", err)
	}
}

func TestSetupRequestTellsWhetherItChoosesASourceRunner(test *testing.T) {
	if install.NewSetupRequest("/p", "", "jp").ChoosesSourceRunner() {
		test.Error("a request without --proton chooses a source runner")
	}
	chosen := install.NewSetupRequest("", "Proton-GE Latest", "")
	if !chosen.ChoosesSourceRunner() || chosen.SourceRunner() != "Proton-GE Latest" {
		test.Errorf("request = %+v", chosen)
	}
}

func TestFirstSetupPlacesThePrefixWhereTheLauncherExpectsIt(test *testing.T) {
	examples := map[install.Launcher]string{
		install.Faugus:     "/home/u/Faugus/line",
		install.Standalone: "/home/u/.local/share/line-proton/prefix",
	}
	for launcher, wantPrefix := range examples {
		test.Run(launcher.String(), func(test *testing.T) {
			got, err := install.FirstSetup("/home/u", "Proton-CachyOS Latest", fonts.Korean, "/usr/bin/line-proton", launcher)
			want, _ := install.New(wantPrefix, "Proton-CachyOS Latest", fonts.Korean, "/usr/bin/line-proton", launcher)
			if err != nil || got != want {
				test.Errorf("FirstSetup = %+v, %v; want %+v", got, err, want)
			}
		})
	}
}
