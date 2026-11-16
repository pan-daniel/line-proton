package faugus

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"io/fs"
	"iter"
	"maps"
	"os"
	"path/filepath"
	"slices"

	"github.com/pan-daniel/line-proton/internal/domain/launcher"
	"github.com/pan-daniel/line-proton/internal/infrastructure/processes"
)

var gameDefaults = map[string]any{
	"gameid": "", "title": "", "path": "", "prefix": "", "launch_arguments": "", "game_arguments": "",
	"mangohud": "", "gamemode": "", "sdl_enabled": "", "runner": "", "addapp_enabled": "", "addapp": "",
	"addapp_bat": "", "addapp_delay": "", "addapp_first": "", "cover": "", "lossless_enabled": false,
	"lossless_multiplier": 1, "lossless_flow": 100, "lossless_performance": false, "lossless_hdr": false,
	"lossless_present": "", "playtime": 0, "hidden": false, "no_sleep": false, "category": false, "icon": "",
	"steamgriddb_id": "", "pre_launch": "", "post_launch": "", "steam_user": "", "disable_umu": "",
	"runtime": "", "last_played": "", "discord_rpc": "",
}

func GamesFile(home string) string {
	return filepath.Join(dataDirectory(home), "games.json")
}

func IconFile(home string) string {
	return filepath.Join(dataDirectory(home), "icons", launcher.GameID+".png")
}

func dataDirectory(home string) string {
	return filepath.Join(home, ".local", "share", "faugus-launcher")
}

type Library struct {
	gamesFile string
	processes processes.Scanner
}

func NewLibrary(gamesFile string, scanner processes.Scanner) Library {
	return Library{gamesFile: gamesFile, processes: scanner}
}

func (library Library) Upsert(game launcher.FaugusGame) error {
	games, mode, err := library.load()
	if err != nil {
		return err
	}
	return library.save(upserted(games, game), mode)
}

func (library Library) Game() (map[string]string, bool, error) {
	games, _, err := library.load()
	index := slices.IndexFunc(games, isLineGame)
	if err != nil || index < 0 {
		return nil, false, err
	}
	return textFields(games[index]), true, nil
}

func (library Library) Remove() error {
	games, mode, err := library.load()
	if err != nil {
		return err
	}
	return library.save(slices.DeleteFunc(slices.Clone(games), isLineGame), mode)
}

func (library Library) Running() (bool, error) {
	return library.processes.AnyMatching(isFaugusWindow)
}

func (library Library) load() ([]map[string]any, fs.FileMode, error) {
	raw, err := os.ReadFile(library.gamesFile)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, 0o644, nil
	}
	if err != nil {
		return nil, 0, err
	}
	details, err := os.Stat(library.gamesFile)
	if err != nil {
		return nil, 0, err
	}
	var games []map[string]any
	return games, details.Mode().Perm(), json.Unmarshal(raw, &games)
}

func (library Library) save(games []map[string]any, mode fs.FileMode) error {
	raw, err := json.Marshal(games, jsontext.WithIndent("    "), json.Deterministic(true))
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(library.gamesFile), 0o755); err != nil {
		return err
	}
	temporary := library.gamesFile + ".line-proton"
	if err := os.WriteFile(temporary, raw, mode); err != nil {
		return err
	}
	return os.Rename(temporary, library.gamesFile)
}

func upserted(games []map[string]any, game launcher.FaugusGame) []map[string]any {
	index := slices.IndexFunc(games, isLineGame)
	updated := slices.Clone(games)
	if index < 0 {
		return append(updated, merged(gameDefaults, game.Fields()))
	}
	updated[index] = merged(games[index], game.Fields())
	return updated
}

func merged(base map[string]any, fields map[string]string) map[string]any {
	result := maps.Clone(base)
	for key, value := range fields {
		result[key] = value
	}
	return result
}

func isLineGame(game map[string]any) bool {
	return game["gameid"] == launcher.GameID
}

func textFields(game map[string]any) map[string]string {
	return maps.Collect(onlyTexts(maps.All(game)))
}

func onlyTexts(fields iter.Seq2[string, any]) iter.Seq2[string, string] {
	return func(yield func(string, string) bool) {
		fields(func(key string, value any) bool {
			return yieldIfText(yield, key, value)
		})
	}
}

func yieldIfText(yield func(string, string) bool, key string, value any) bool {
	text, isText := value.(string)
	return !isText || yield(key, text)
}

func isFaugusWindow(process processes.Process) bool {
	arguments := process.Arguments()
	return slices.Contains(arguments, "faugus.launcher") && !slices.Contains(arguments, "--game")
}
