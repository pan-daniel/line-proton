package launcher

import (
	"maps"
	"strings"

	"github.com/pan-daniel/line-proton/internal/domain/install"
	"github.com/pan-daniel/line-proton/internal/domain/runner"
)

const (
	GameID          = "line"
	Title           = "LINE"
	launchArguments = "PROTON_ENABLE_WAYLAND=0 PROTON_FSR4_UPGRADE=0"
)

type FaugusGame struct {
	fields map[string]string
}

func NewFaugusGame(installation install.Installation, icon string) FaugusGame {
	return FaugusGame{fields: map[string]string{
		"gameid":           GameID,
		"title":            Title,
		"path":             installation.LauncherExecutable(),
		"prefix":           installation.Prefix(),
		"launch_arguments": launchArguments,
		"runner":           runner.LineRunnerName,
		"icon":             icon,
		"pre_launch":       HookCommand(installation.Executable(), "pre"),
		"post_launch":      HookCommand(installation.Executable(), "post"),
	}}
}

func (game FaugusGame) Fields() map[string]string {
	return maps.Clone(game.fields)
}

func HookCommand(executable, moment string) string {
	return ShellQuote(executable) + " hook " + moment
}

func HooksPointTo(fields map[string]string, executable string) bool {
	return fields["pre_launch"] == HookCommand(executable, "pre") && fields["post_launch"] == HookCommand(executable, "post")
}

func ShellQuote(text string) string {
	return "'" + strings.ReplaceAll(text, "'", `'\''`) + "'"
}

type DesktopEntry struct {
	exec string
	icon string
}

func NewDesktopEntry(installation install.Installation, icon string) DesktopEntry {
	return DesktopEntry{exec: execFor(installation), icon: icon}
}

func execFor(installation install.Installation) string {
	if installation.Launcher().UsesFaugus() {
		return "faugus-launcher --game " + GameID
	}
	return desktopExecArgument(installation.Executable()) + " launch"
}

func desktopExecArgument(text string) string {
	quoted := `"` + strings.NewReplacer(`"`, `\"`, "`", "\\`", "$", `\$`, `\`, `\\`).Replace(text) + `"`
	return strings.NewReplacer(`\`, `\\`, "%", "%%").Replace(quoted)
}

func (entry DesktopEntry) Render() string {
	return strings.Join([]string{
		"[Desktop Entry]",
		"Type=Application",
		"Name=" + Title,
		"Exec=" + entry.exec,
		"Icon=" + entry.icon,
		"Categories=Network;InstantMessaging;",
		"StartupWMClass=line.exe",
		"",
	}, "\n")
}

type Faugus interface {
	Upsert(game FaugusGame) error
	Game() (map[string]string, bool, error)
	Remove() error
	Running() (bool, error)
}

type Desktop interface {
	Install(entry DesktopEntry) error
	Installed() bool
	Remove() error
}
