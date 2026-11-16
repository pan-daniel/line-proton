package main

import (
	"cmp"
	"os"
	"path/filepath"
)

type environment struct {
	home                  string
	configFile            string
	dataDirectory         string
	stateDirectory        string
	runnersDirectory      string
	applicationsDirectory string
}

func detectEnvironment() (environment, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return environment{}, err
	}
	return environmentFor(home), nil
}

func environmentFor(home string) environment {
	data := xdgDirectory("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	return environment{
		home:                  home,
		configFile:            filepath.Join(xdgDirectory("XDG_CONFIG_HOME", filepath.Join(home, ".config")), "line-proton", "config.json"),
		dataDirectory:         filepath.Join(data, "line-proton"),
		stateDirectory:        filepath.Join(xdgDirectory("XDG_STATE_HOME", filepath.Join(home, ".local", "state")), "line-proton"),
		runnersDirectory:      filepath.Join(home, ".local", "share", "Steam", "compatibilitytools.d"),
		applicationsDirectory: filepath.Join(data, "applications"),
	}
}

func (environment environment) stagingDirectory() string {
	return filepath.Join(filepath.Dir(environment.runnersDirectory), ".line-proton-staging")
}

func xdgDirectory(variable, fallback string) string {
	return cmp.Or(os.Getenv(variable), fallback)
}
