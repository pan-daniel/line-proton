package runner_test

import (
	"errors"
	"testing"

	"github.com/pan-daniel/line-proton/internal/domain/runner"
)

func mustRunner(test *testing.T, name, version string) runner.Runner {
	test.Helper()
	parsed, err := runner.ParseVersion(version)
	if err != nil {
		test.Fatal(err)
	}
	built, err := runner.New(name, parsed)
	if err != nil {
		test.Fatal(err)
	}
	return built
}

func TestSelectSourcePrefersCachyOSThenGEThenNewest(test *testing.T) {
	cachy := mustRunner(test, "Proton-CachyOS Latest", "1792530497 cachyos-11.0")
	geProton := mustRunner(test, "Proton-GE Latest", "1799999999 GE-Proton10-29")
	older := mustRunner(test, "GE-Proton10-28", "1700000000 GE-Proton10-28")
	newer := mustRunner(test, "proton-custom", "1800000000 custom")
	line := mustRunner(test, runner.LineRunnerName, "1900000000 cachyos")
	examples := []struct {
		name      string
		installed []runner.Runner
		want      string
	}{
		{"cachyos wins", []runner.Runner{older, geProton, cachy, line}, "Proton-CachyOS Latest"},
		{"ge when no cachyos", []runner.Runner{older, geProton, newer}, "Proton-GE Latest"},
		{"newest otherwise", []runner.Runner{older, newer, line}, "proton-custom"},
	}
	for _, example := range examples {
		test.Run(example.name, func(test *testing.T) {
			got, err := runner.SelectSource(example.installed, "")
			if err != nil || got.Name() != example.want {
				test.Errorf("SelectSource = %q, %v; want %q", got.Name(), err, example.want)
			}
		})
	}
}

func TestSelectSourceHonoursTheRequestedName(test *testing.T) {
	geProton := mustRunner(test, "Proton-GE Latest", "1 ge")
	cachy := mustRunner(test, "Proton-CachyOS Latest", "2 cachy")
	got, err := runner.SelectSource([]runner.Runner{cachy, geProton}, "Proton-GE Latest")
	if err != nil || got.Name() != "Proton-GE Latest" {
		test.Errorf("SelectSource = %q, %v", got.Name(), err)
	}
	if _, err := runner.SelectSource([]runner.Runner{cachy}, "missing"); !errors.Is(err, runner.ErrRunnerNotFound) {
		test.Errorf("err = %v, want ErrRunnerNotFound", err)
	}
}

func TestSelectSourceNeverPicksProtonLINEOrItsWorkAreas(test *testing.T) {
	line := mustRunner(test, runner.LineRunnerName, "1 x")
	retired := mustRunner(test, runner.LineRunnerName+".retired", "9 x")
	if _, err := runner.SelectSource([]runner.Runner{line, retired}, ""); !errors.Is(err, runner.ErrNoRunners) {
		test.Errorf("err = %v, want ErrNoRunners", err)
	}
}

func TestNewRejectsPathLikeNames(test *testing.T) {
	version, _ := runner.ParseVersion("1 x")
	for _, name := range []string{"", ".", "..", "a/b"} {
		test.Run(name, func(test *testing.T) {
			if _, err := runner.New(name, version); !errors.Is(err, runner.ErrInvalidName) {
				test.Errorf("New(%q) err = %v", name, err)
			}
		})
	}
}

func TestAssess(test *testing.T) {
	source := runner.Present(mustRunner(test, "Proton-CachyOS Latest", "2 new"))
	examples := []struct {
		name   string
		line   runner.Installed
		source runner.Installed
		want   runner.Freshness
	}{
		{"missing line runner", runner.Absent(), source, runner.NeedsBuild},
		{"older line runner", runner.Present(mustRunner(test, runner.LineRunnerName, "1 old")), source, runner.Outdated},
		{"same version", runner.Present(mustRunner(test, runner.LineRunnerName, "2 new")), source, runner.Current},
		{"source gone", runner.Present(mustRunner(test, runner.LineRunnerName, "1 old")), runner.Absent(), runner.SourceMissing},
	}
	for _, example := range examples {
		test.Run(example.name, func(test *testing.T) {
			if got := runner.Assess(example.line, example.source); got != example.want {
				test.Errorf("Assess = %v, want %v", got, example.want)
			}
		})
	}
}

func TestIsSignedBinary(test *testing.T) {
	examples := map[string]bool{
		"files/lib/wine/x86_64-windows/gdiplus.dll":                    true,
		"files/lib/wine/dxvk/i386-windows/dxgi.dll":                    true,
		"files/lib/wine/x86_64-windows/winealsa.DRV":                   true,
		"files/lib/wine/x86_64-windows/msacm32.acm":                    true,
		"files/lib/wine/x86_64-windows/quartz.ax":                      true,
		"files/lib/wine/x86_64-windows/hhctrl.ocx":                     true,
		"files/lib/wine/x86_64-windows/joy.cpl":                        true,
		"files/lib/wine/x86_64-windows/stdole2.tlb":                    true,
		"files/lib/wine/x86_64-windows/sane.ds":                        true,
		"files/lib/wine/x86_64-windows/mountmgr.sys":                   true,
		"files/share/default_pfx/drive_c/windows/system32/notepad.exe": true,
		"files/lib/wine/i386-windows/krnl386.exe16":                    false,
		"files/lib/wine/x86_64-unix/ntdll.so":                          false,
		"files/bin/wine":                                               false,
		"protonfixes/gamefixes-steam/123.dll":                          false,
	}
	for path, want := range examples {
		test.Run(path, func(test *testing.T) {
			if got := runner.IsSignedBinary(path); got != want {
				test.Errorf("IsSignedBinary(%q) = %v, want %v", path, got, want)
			}
		})
	}
}

func TestToolManifestDeclaresProtonLINE(test *testing.T) {
	want := "\"compatibilitytools\"\n" +
		"{\n" +
		"  \"compat_tools\"\n" +
		"  {\n" +
		"    \"Proton-LINE\"\n" +
		"    {\n" +
		"      \"install_path\" \".\"\n" +
		"      \"display_name\" \"Proton-LINE\"\n" +
		"      \"from_oslist\"  \"windows\"\n" +
		"      \"to_oslist\"    \"linux\"\n" +
		"    }\n" +
		"  }\n" +
		"}\n"
	if got := runner.ToolManifest(); got != want {
		test.Errorf("ToolManifest =\n%s\nwant\n%s", got, want)
	}
}
