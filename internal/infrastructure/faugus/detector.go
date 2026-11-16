package faugus

import "github.com/pan-daniel/line-proton/internal/domain/install"

const launcherCommand = "faugus-launcher"

type Detector struct {
	lookPath func(string) (string, error)
}

func NewDetector(lookPath func(string) (string, error)) Detector {
	return Detector{lookPath: lookPath}
}

func (detector Detector) Detect() install.Launcher {
	if _, err := detector.lookPath(launcherCommand); err != nil {
		return install.Standalone
	}
	return install.Faugus
}
