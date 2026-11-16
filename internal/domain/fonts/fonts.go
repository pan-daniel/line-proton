package fonts

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/pan-daniel/line-proton/internal/domain/winreg"
)

var ErrUnknownVariant = errors.New("unknown font variant")

var ErrMissingFonts = errors.New("required fonts are not installed")

type Variant int

const (
	Japanese Variant = iota
	SimplifiedChinese
	TraditionalChinese
	Korean
)

var (
	variantCodes    = []string{"jp", "sc", "tc", "kr"}
	variantFamilies = []string{"Noto Sans CJK JP", "Noto Sans CJK SC", "Noto Sans CJK TC", "Noto Sans CJK KR"}
	traditional     = []string{"TW", "HK", "MO"}
	requiredFiles   = []string{"NotoSansCJK-Regular.ttc", "NotoSansCJK-Bold.ttc", "NotoSansThai-Regular.ttf"}
	substituted     = []string{
		"Segoe UI", "Segoe UI Semibold", "Segoe UI Light", "Yu Gothic", "Yu Gothic UI", "Meiryo", "Meiryo UI",
		"MS UI Gothic", "Tahoma", "Helvetica", "Arial Unicode MS", "MS Shell Dlg", "MS Shell Dlg 2",
	}
)

func ParseVariant(code string) (Variant, error) {
	index := slices.Index(variantCodes, strings.ToLower(code))
	if index < 0 {
		return Japanese, fmt.Errorf("%w: %q (use jp, sc, tc or kr)", ErrUnknownVariant, code)
	}
	return Variant(index), nil
}

func VariantForLocale(locale string) Variant {
	language, territory := splitLocale(locale)
	switch {
	case language == "ko":
		return Korean
	case language == "zh" && slices.Contains(traditional, territory):
		return TraditionalChinese
	case language == "zh":
		return SimplifiedChinese
	}
	return Japanese
}

type LocaleSettings struct {
	all           string
	characterType string
	language      string
}

func NewLocaleSettings(all, characterType, language string) LocaleSettings {
	return LocaleSettings{all: all, characterType: characterType, language: language}
}

func (settings LocaleSettings) Locale() string {
	return cmp.Or(settings.all, settings.characterType, settings.language)
}

func (settings LocaleSettings) Variant() Variant {
	return VariantForLocale(settings.Locale())
}

func splitLocale(locale string) (string, string) {
	withoutEncoding, _, _ := strings.Cut(locale, ".")
	withoutModifier, _, _ := strings.Cut(withoutEncoding, "@")
	language, territory, _ := strings.Cut(withoutModifier, "_")
	return strings.ToLower(language), strings.ToUpper(territory)
}

func (variant Variant) Code() string {
	return variantCodes[variant]
}

func (variant Variant) Family() string {
	return variantFamilies[variant]
}

func (variant Variant) String() string {
	return variant.Family()
}

type Plan struct {
	variant Variant
}

func PlanFor(variant Variant) Plan {
	return Plan{variant: variant}
}

func (plan Plan) Variant() Variant {
	return plan.variant
}

func (plan Plan) Files() []string {
	return slices.Clone(requiredFiles)
}

func (plan Plan) RegistryPatch() winreg.Patch {
	family := plan.variant.Family()
	return winreg.NewPatch(
		winreg.NewKey(`HKEY_LOCAL_MACHINE\Software\Microsoft\Windows NT\CurrentVersion\FontSubstitutes`, substitutesFor(family)...),
		winreg.NewKey(`HKEY_LOCAL_MACHINE\Software\Microsoft\Windows NT\CurrentVersion\FontLink\SystemLink`,
			winreg.MultiStringValue(family, "NotoSansThai-Regular.ttf,Noto Sans Thai")),
		winreg.NewKey(`HKEY_CURRENT_USER\Control Panel\Desktop`,
			winreg.StringValue("FontSmoothing", "2"),
			winreg.DWordValue("FontSmoothingType", 2),
			winreg.DWordValue("FontSmoothingGamma", 0x578),
			winreg.DWordValue("FontSmoothingOrientation", 1)),
	)
}

func (plan Plan) AppliedIn(systemRegistry string) bool {
	family := plan.variant.Family()
	return !slices.ContainsFunc(substituted, func(name string) bool {
		return !strings.Contains(systemRegistry, `"`+name+`"="`+family+`"`)
	})
}

func substitutesFor(family string) []winreg.Value {
	values := make([]winreg.Value, 0, len(substituted))
	for _, name := range substituted {
		values = append(values, winreg.StringValue(name, family))
	}
	return values
}

type hint struct {
	families []string
	command  string
}

var hints = []hint{
	{families: []string{"arch"}, command: "sudo pacman -S noto-fonts-cjk noto-fonts"},
	{families: []string{"debian", "ubuntu"}, command: "sudo apt install fonts-noto-cjk fonts-noto-core"},
	{families: []string{"fedora", "rhel"}, command: "sudo dnf install google-noto-sans-cjk-fonts google-noto-sans-thai-fonts"},
	{families: []string{"opensuse", "suse"}, command: "sudo zypper install noto-sans-cjk-fonts noto-sans-thai-fonts"},
}

func InstallHint(identifiers []string) string {
	index := slices.IndexFunc(hints, func(candidate hint) bool { return candidate.matches(identifiers) })
	if index < 0 {
		return "install Noto Sans CJK and Noto Sans Thai with your package manager"
	}
	return hints[index].command
}

func (candidate hint) matches(identifiers []string) bool {
	return slices.ContainsFunc(identifiers, func(identifier string) bool { return slices.Contains(candidate.families, identifier) })
}

type Library interface {
	Locate(names []string) ([]string, error)
}
