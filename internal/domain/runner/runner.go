package runner

import (
	"cmp"
	"errors"
	"fmt"
	"path"
	"slices"
	"strconv"
	"strings"
)

const (
	LineRunnerName   = "Proton-LINE"
	ToolManifestName = "compatibilitytool.vdf"
)

var (
	ErrEmptyVersion   = errors.New("empty runner version")
	ErrInvalidName    = errors.New("invalid runner name")
	ErrNoRunners      = errors.New("no Proton runner installed")
	ErrRunnerNotFound = errors.New("runner not found")
)

var (
	sourcePreference  = []string{"Proton-GE Latest", "Proton-CachyOS Latest"}
	signedDirectories = []string{"files/lib/wine/", "files/share/default_pfx/drive_c/windows/"}
	signedExtensions  = []string{".dll", ".drv", ".exe", ".acm", ".ax", ".ocx", ".cpl", ".tlb", ".ds", ".sys"}
)

type Version struct {
	raw string
}

func ParseVersion(raw string) (Version, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return Version{}, ErrEmptyVersion
	}
	return Version{raw: trimmed}, nil
}

func (version Version) String() string {
	return version.raw
}

func (version Version) Equal(other Version) bool {
	return version.raw == other.raw
}

func (version Version) build() int64 {
	first, _, _ := strings.Cut(version.raw, " ")
	number, err := strconv.ParseInt(first, 10, 64)
	if err != nil {
		return 0
	}
	return number
}

type Runner struct {
	name    string
	version Version
}

func New(name string, version Version) (Runner, error) {
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\`) {
		return Runner{}, fmt.Errorf("%w: %q", ErrInvalidName, name)
	}
	return Runner{name: name, version: version}, nil
}

func (runner Runner) Name() string {
	return runner.name
}

func (runner Runner) Version() Version {
	return runner.version
}

func (runner Runner) IsLineRunner() bool {
	return runner.name == LineRunnerName || strings.HasPrefix(runner.name, LineRunnerName+".")
}

func SelectSource(installed []Runner, requested string) (Runner, error) {
	candidates := slices.DeleteFunc(slices.Clone(installed), Runner.IsLineRunner)
	if requested != "" {
		return named(candidates, requested)
	}
	if len(candidates) == 0 {
		return Runner{}, ErrNoRunners
	}
	return slices.MaxFunc(candidates, bySourcePreference), nil
}

func named(candidates []Runner, name string) (Runner, error) {
	index := slices.IndexFunc(candidates, func(candidate Runner) bool { return candidate.name == name })
	if index < 0 {
		return Runner{}, fmt.Errorf("%w: %s", ErrRunnerNotFound, name)
	}
	return candidates[index], nil
}

func bySourcePreference(first, second Runner) int {
	return cmp.Or(
		cmp.Compare(slices.Index(sourcePreference, first.name), slices.Index(sourcePreference, second.name)),
		cmp.Compare(first.version.build(), second.version.build()),
		cmp.Compare(first.name, second.name),
	)
}

type Installed struct {
	runner  Runner
	present bool
}

func Present(found Runner) Installed {
	return Installed{runner: found, present: true}
}

func Absent() Installed {
	return Installed{}
}

func (installed Installed) Runner() (Runner, bool) {
	return installed.runner, installed.present
}

type Freshness int

const (
	NeedsBuild Freshness = iota
	Outdated
	Current
	SourceMissing
)

func Assess(line, source Installed) Freshness {
	if !source.present {
		return SourceMissing
	}
	if !line.present {
		return NeedsBuild
	}
	if !line.runner.version.Equal(source.runner.version) {
		return Outdated
	}
	return Current
}

func (freshness Freshness) RequiresBuild() bool {
	return freshness == NeedsBuild || freshness == Outdated
}

func (freshness Freshness) String() string {
	return [...]string{"needs build", "outdated", "current", "source missing"}[freshness]
}

func IsSignedBinary(relativePath string) bool {
	slashed := path.Clean(strings.ReplaceAll(relativePath, `\`, "/"))
	inSignedDirectory := slices.ContainsFunc(signedDirectories, func(directory string) bool {
		return strings.HasPrefix(slashed, directory)
	})
	return inSignedDirectory && slices.Contains(signedExtensions, strings.ToLower(path.Ext(slashed)))
}

func ToolManifest() string {
	return strings.Join([]string{
		`"compatibilitytools"`,
		`{`,
		`  "compat_tools"`,
		`  {`,
		`    "` + LineRunnerName + `"`,
		`    {`,
		`      "install_path" "."`,
		`      "display_name" "` + LineRunnerName + `"`,
		`      "from_oslist"  "windows"`,
		`      "to_oslist"    "linux"`,
		`    }`,
		`  }`,
		`}`,
		``,
	}, "\n")
}

type Catalog interface {
	Installed() ([]Runner, error)
	Find(name string) (Installed, error)
	Remove(name string) error
}

type Workshop interface {
	Stage(source Runner) (Staging, error)
}

type Staging interface {
	Binaries() ([]string, error)
	Publish() error
	Discard() error
}
