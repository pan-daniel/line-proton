package config_test

import (
	"path/filepath"
	"testing"

	"github.com/pan-daniel/line-proton/internal/domain/fonts"
	"github.com/pan-daniel/line-proton/internal/domain/install"
	"github.com/pan-daniel/line-proton/internal/infrastructure/config"
)

func TestSaveLoadRoundTrip(test *testing.T) {
	store := config.New(filepath.Join(test.TempDir(), "line-proton", "config.json"))
	if _, found, err := store.Load(); err != nil || found {
		test.Fatalf("empty Load = %v, %v", found, err)
	}
	saved, err := install.New("/home/u/Faugus/line", "Proton-GE Latest", fonts.TraditionalChinese, "/usr/bin/line-proton", install.Faugus)
	if err != nil {
		test.Fatal(err)
	}
	if err := store.Save(saved); err != nil {
		test.Fatal(err)
	}
	loaded, found, err := store.Load()
	if err != nil || !found || loaded != saved {
		test.Errorf("Load = %+v, %v, %v; want %+v", loaded, found, err, saved)
	}
	if err := store.Remove(); err != nil {
		test.Fatal(err)
	}
	if _, found, _ := store.Load(); found {
		test.Error("config still present after Remove")
	}
}
