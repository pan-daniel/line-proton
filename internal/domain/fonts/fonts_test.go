package fonts_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/pan-daniel/line-proton/internal/domain/fonts"
	"github.com/pan-daniel/line-proton/internal/domain/winreg"
)

func TestVariantForLocale(test *testing.T) {
	examples := map[string]fonts.Variant{
		"ja_JP.UTF-8":   fonts.Japanese,
		"ko_KR.UTF-8":   fonts.Korean,
		"zh_TW.UTF-8":   fonts.TraditionalChinese,
		"zh_HK.UTF-8@x": fonts.TraditionalChinese,
		"zh_CN.UTF-8":   fonts.SimplifiedChinese,
		"pl_PL.UTF-8":   fonts.Japanese,
		"th_TH.UTF-8":   fonts.Japanese,
		"":              fonts.Japanese,
	}
	for locale, want := range examples {
		test.Run(locale, func(test *testing.T) {
			if got := fonts.VariantForLocale(locale); got != want {
				test.Errorf("VariantForLocale(%q) = %v, want %v", locale, got, want)
			}
		})
	}
}

func TestParseVariant(test *testing.T) {
	got, err := fonts.ParseVariant("TC")
	if err != nil || got != fonts.TraditionalChinese {
		test.Errorf("ParseVariant(TC) = %v, %v", got, err)
	}
	if _, err := fonts.ParseVariant("xx"); !errors.Is(err, fonts.ErrUnknownVariant) {
		test.Errorf("err = %v, want ErrUnknownVariant", err)
	}
}

func TestPlanSubstitutesWindowsFontsWithTheVariantFamily(test *testing.T) {
	patch := fonts.PlanFor(fonts.Korean).RegistryPatch()
	substitutes := patch.Keys()[0]
	segoe := substitutes.Values()[0]
	if segoe.Name() != "Segoe UI" || segoe.Text() != "Noto Sans CJK KR" {
		test.Errorf("first substitute = %q -> %q", segoe.Name(), segoe.Text())
	}
}

func TestPlanLinksThaiAndEnablesSmoothing(test *testing.T) {
	keys := fonts.PlanFor(fonts.Japanese).RegistryPatch().Keys()
	link := keys[1].Values()[0]
	if link.Kind() != winreg.MultiString || link.Name() != "Noto Sans CJK JP" || !slices.Equal(link.Items(), []string{"NotoSansThai-Regular.ttf,Noto Sans Thai"}) {
		test.Errorf("system link = %v %q %q", link.Kind(), link.Name(), link.Items())
	}
	smoothing := keys[2].Values()
	if smoothing[0].Name() != "FontSmoothing" || smoothing[0].Text() != "2" || smoothing[2].Number() != 0x578 {
		test.Errorf("smoothing = %+v", smoothing)
	}
}

func systemRegistryWithSubstitutes(family string) string {
	names := []string{
		"Segoe UI", "Segoe UI Semibold", "Segoe UI Light", "Yu Gothic", "Yu Gothic UI", "Meiryo", "Meiryo UI",
		"MS UI Gothic", "Tahoma", "Helvetica", "Arial Unicode MS", "MS Shell Dlg", "MS Shell Dlg 2",
	}
	lines := []string{`[Software\\Microsoft\\Windows NT\\CurrentVersion\\FontSubstitutes] 1794776272`, `"Arial CE,0"="Arial,238"`}
	for _, name := range names {
		lines = append(lines, `"`+name+`"="`+family+`"`)
	}
	return strings.Join(lines, "\n") + "\n"
}

func TestPlanAppliedInRecognisesItsSubstitutes(test *testing.T) {
	examples := []struct {
		name     string
		registry string
		want     bool
	}{
		{"every substitute present", systemRegistryWithSubstitutes("Noto Sans CJK JP"), true},
		{"other variant applied", systemRegistryWithSubstitutes("Noto Sans CJK KR"), false},
		{"Proton reset the fonts", `"Arial CE,0"="Arial,238"` + "\n", false},
		{"empty registry", "", false},
	}
	for _, example := range examples {
		test.Run(example.name, func(test *testing.T) {
			if got := fonts.PlanFor(fonts.Japanese).AppliedIn(example.registry); got != example.want {
				test.Errorf("AppliedIn = %v, want %v", got, example.want)
			}
		})
	}
}

func TestInstallHint(test *testing.T) {
	examples := map[string][]string{
		"sudo pacman -S noto-fonts-cjk noto-fonts":                                {"cachyos", "arch"},
		"sudo apt install fonts-noto-cjk fonts-noto-core":                         {"linuxmint", "ubuntu", "debian"},
		"sudo dnf install google-noto-sans-cjk-fonts google-noto-sans-thai-fonts": {"fedora"},
	}
	for want, identifiers := range examples {
		test.Run(want, func(test *testing.T) {
			if got := fonts.InstallHint(identifiers); got != want {
				test.Errorf("InstallHint(%v) = %q", identifiers, got)
			}
		})
	}
	if got := fonts.InstallHint([]string{"gentoo"}); got == "" {
		test.Error("unknown distribution got an empty hint")
	}
}

func TestLocaleSettingsFollowThePosixPrecedence(test *testing.T) {
	examples := []struct {
		name     string
		settings fonts.LocaleSettings
		want     fonts.Variant
	}{
		{"LC_ALL wins over everything", fonts.NewLocaleSettings("ko_KR.UTF-8", "zh_CN.UTF-8", "zh_TW.UTF-8"), fonts.Korean},
		{"LC_CTYPE wins over LANG", fonts.NewLocaleSettings("", "zh_CN.UTF-8", "zh_TW.UTF-8"), fonts.SimplifiedChinese},
		{"LANG is the last resort", fonts.NewLocaleSettings("", "", "zh_TW.UTF-8"), fonts.TraditionalChinese},
		{"no locale at all means Japanese", fonts.NewLocaleSettings("", "", ""), fonts.Japanese},
	}
	for _, example := range examples {
		test.Run(example.name, func(test *testing.T) {
			if got := example.settings.Variant(); got != example.want {
				test.Errorf("Variant = %v, want %v", got, example.want)
			}
		})
	}
}
