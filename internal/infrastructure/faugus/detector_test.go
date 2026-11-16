package faugus_test

import (
	"errors"
	"os/exec"
	"testing"

	"github.com/pan-daniel/line-proton/internal/domain/install"
	"github.com/pan-daniel/line-proton/internal/infrastructure/faugus"
)

func lookingUp(found map[string]string) func(string) (string, error) {
	return func(name string) (string, error) {
		path, present := found[name]
		if !present {
			return "", exec.ErrNotFound
		}
		return path, nil
	}
}

func TestDetectorPicksFaugusWhenItsLauncherIsOnThePath(test *testing.T) {
	detector := faugus.NewDetector(lookingUp(map[string]string{"faugus-launcher": "/usr/bin/faugus-launcher"}))
	if got := detector.Detect(); got != install.Faugus {
		test.Errorf("Detect = %v, want faugus", got)
	}
}

func TestDetectorFallsBackToStandalone(test *testing.T) {
	detector := faugus.NewDetector(func(string) (string, error) { return "", errors.New("permission denied") })
	if got := detector.Detect(); got != install.Standalone {
		test.Errorf("Detect = %v, want standalone", got)
	}
}
