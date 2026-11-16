package faugus_test

import (
	"encoding/json/v2"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/pan-daniel/line-proton/internal/domain/fonts"
	"github.com/pan-daniel/line-proton/internal/domain/install"
	"github.com/pan-daniel/line-proton/internal/domain/launcher"
	"github.com/pan-daniel/line-proton/internal/infrastructure/faugus"
	"github.com/pan-daniel/line-proton/internal/infrastructure/processes"
)

func game(test *testing.T) launcher.FaugusGame {
	test.Helper()
	installation, err := install.New("/home/u/Faugus/line", "Proton-CachyOS Latest", fonts.Japanese, "/usr/bin/line-proton", install.Faugus)
	if err != nil {
		test.Fatal(err)
	}
	return launcher.NewFaugusGame(installation, faugus.IconFile("/home/u"))
}

func library(test *testing.T, existing string) (faugus.Library, string) {
	test.Helper()
	file := filepath.Join(test.TempDir(), "games.json")
	if err := os.WriteFile(file, []byte(existing), 0o600); err != nil {
		test.Fatal(err)
	}
	return faugus.NewLibrary(file, processes.NewScanner(fstest.MapFS{})), file
}

func readGames(test *testing.T, file string) []map[string]any {
	test.Helper()
	raw, err := os.ReadFile(file)
	if err != nil {
		test.Fatal(err)
	}
	var games []map[string]any
	if err := json.Unmarshal(raw, &games); err != nil {
		test.Fatal(err)
	}
	return games
}

func TestUpsertAddsLineAndKeepsOtherGamesAndUnknownFields(test *testing.T) {
	games, file := library(test, `[{"gameid":"gw2","title":"Guild Wars 2","playtime":13773,"future_field":"kept"}]`)
	if err := games.Upsert(game(test)); err != nil {
		test.Fatal(err)
	}
	if err := games.Upsert(game(test)); err != nil {
		test.Fatal(err)
	}
	stored := readGames(test, file)
	if len(stored) != 2 || stored[0]["future_field"] != "kept" || stored[0]["playtime"] != float64(13773) {
		test.Fatalf("stored = %v", stored)
	}
	if stored[1]["gameid"] != "line" || stored[1]["runner"] != "Proton-LINE" || stored[1]["lossless_flow"] != float64(100) {
		test.Errorf("line entry = %v", stored[1])
	}
	details, _ := os.Stat(file)
	if details.Mode().Perm() != 0o600 {
		test.Errorf("mode = %v, want preserved 0600", details.Mode().Perm())
	}
}

func TestGameAndRemove(test *testing.T) {
	games, _ := library(test, `[]`)
	if err := games.Upsert(game(test)); err != nil {
		test.Fatal(err)
	}
	fields, found, err := games.Game()
	if err != nil || !found || fields["pre_launch"] != "'/usr/bin/line-proton' hook pre" {
		test.Errorf("Game = %v, %v, %v", fields, found, err)
	}
	if err := games.Remove(); err != nil {
		test.Fatal(err)
	}
	if _, found, _ := games.Game(); found {
		test.Error("game still present after Remove")
	}
}

func TestRunningDetectsTheFaugusWindowButNotAGameRun(test *testing.T) {
	window := fstest.MapFS{"10/cmdline": {Data: []byte("/usr/bin/python3\x00-m\x00faugus.launcher\x00")}}
	gameRun := fstest.MapFS{"11/cmdline": {Data: []byte("/usr/bin/python3\x00-m\x00faugus.launcher\x00--game\x00line\x00")}}
	if running, _ := faugus.NewLibrary("unused", processes.NewScanner(window)).Running(); !running {
		test.Error("Faugus window not detected")
	}
	if running, _ := faugus.NewLibrary("unused", processes.NewScanner(gameRun)).Running(); running {
		test.Error("a game run was mistaken for the Faugus window")
	}
}

func TestPathsFollowTheNativeFaugusLayout(test *testing.T) {
	if got := faugus.GamesFile("/home/u"); got != "/home/u/.local/share/faugus-launcher/games.json" {
		test.Errorf("GamesFile = %q", got)
	}
	if got := faugus.IconFile("/home/u"); got != "/home/u/.local/share/faugus-launcher/icons/line.png" {
		test.Errorf("IconFile = %q", got)
	}
}

func TestGameReturnsOnlyTheTextFields(test *testing.T) {
	games, _ := library(test, `[{"gameid":"line","title":"LINE","playtime":42,"hidden":false}]`)
	fields, found, err := games.Game()
	if err != nil || !found || len(fields) != 2 || fields["gameid"] != "line" || fields["title"] != "LINE" {
		test.Errorf("Game = %v, %v, %v", fields, found, err)
	}
}

func TestUpsertLeavesAMalformedGamesFileUntouched(test *testing.T) {
	malformed := `[{"gameid":"gw2","title":"Guild Wars 2",`
	games, file := library(test, malformed)
	if err := games.Upsert(game(test)); err == nil {
		test.Error("Upsert into a malformed games.json succeeded")
	}
	if raw, err := os.ReadFile(file); err != nil || string(raw) != malformed {
		test.Errorf("games.json = %q, %v; want it byte-identical", raw, err)
	}
	if entries, _ := os.ReadDir(filepath.Dir(file)); len(entries) != 1 {
		test.Errorf("leftovers next to games.json: %v", entries)
	}
}
