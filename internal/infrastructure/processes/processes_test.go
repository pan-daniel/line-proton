package processes_test

import (
	"testing"
	"testing/fstest"

	"github.com/pan-daniel/line-proton/internal/infrastructure/processes"
)

const runnerDirectory = "/home/u/.local/share/Steam/compatibilitytools.d/Proton-LINE"

type fakeProcess struct {
	commandLine string
	environment string
}

func processTable(running map[string]fakeProcess) fstest.MapFS {
	table := fstest.MapFS{"self/cmdline": {Data: []byte("ignored\x00")}}
	for identifier, process := range running {
		table[identifier+"/cmdline"] = &fstest.MapFile{Data: []byte(process.commandLine)}
		table[identifier+"/environ"] = &fstest.MapFile{Data: []byte(process.environment)}
	}
	return table
}

func inUse(test *testing.T, running map[string]fakeProcess, directory string) bool {
	test.Helper()
	busy, err := processes.NewRunnerActivity(processes.NewScanner(processTable(running)), directory).InUse()
	if err != nil {
		test.Fatal(err)
	}
	return busy
}

func TestRunnerActivity(test *testing.T) {
	examples := []struct {
		name    string
		running map[string]fakeProcess
		want    bool
	}{
		{"wineserver from the runner", map[string]fakeProcess{"42": {commandLine: runnerDirectory + "/files/bin/wineserver\x00"}}, true},
		{"umu-run starting the runner", map[string]fakeProcess{"43": {commandLine: "python3\x00umu-run\x00LineLauncher.exe\x00", environment: "HOME=/home/u\x00PROTONPATH=" + runnerDirectory + "\x00"}}, true},
		{"unrelated shell", map[string]fakeProcess{"44": {commandLine: "/usr/bin/bash\x00", environment: "PROTONPATH=/elsewhere/Proton-GE\x00"}}, false},
		{"staging copy with a longer name", map[string]fakeProcess{"45": {commandLine: runnerDirectory + "-spike/files/bin/wine\x00", environment: "PROTONPATH=" + runnerDirectory + "-spike\x00"}}, false},
	}
	for _, example := range examples {
		test.Run(example.name, func(test *testing.T) {
			if got := inUse(test, example.running, runnerDirectory); got != example.want {
				test.Errorf("InUse = %v, want %v", got, example.want)
			}
		})
	}
}

func TestProcessesWithoutReadableEnvironmentStillMatchByCommandLine(test *testing.T) {
	table := fstest.MapFS{"7/cmdline": {Data: []byte(runnerDirectory + "/files/bin/wine\x00")}}
	busy, err := processes.NewRunnerActivity(processes.NewScanner(table), runnerDirectory).InUse()
	if err != nil || !busy {
		test.Errorf("InUse = %v, %v", busy, err)
	}
}
