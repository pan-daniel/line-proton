package application

import (
	"errors"

	"github.com/pan-daniel/line-proton/internal/domain/fonts"
	"github.com/pan-daniel/line-proton/internal/domain/health"
	"github.com/pan-daniel/line-proton/internal/domain/install"
	"github.com/pan-daniel/line-proton/internal/domain/launcher"
	"github.com/pan-daniel/line-proton/internal/domain/prefix"
	"github.com/pan-daniel/line-proton/internal/domain/runner"
)

type DoctorDependencies struct {
	Installation      install.Installation
	Umu               UmuLocator
	Runners           runner.Catalog
	Prefix            prefix.Survey
	Fonts             fonts.Library
	Certificate       CertificateStore
	Faugus            launcher.Faugus
	Desktop           launcher.Desktop
	RunningExecutable RunningExecutable
	ExecutableExists  PathProbe
}

type Doctor struct {
	installation      install.Installation
	fontPlan          fonts.Plan
	umu               UmuLocator
	runners           runner.Catalog
	prefix            prefix.Survey
	fonts             fonts.Library
	certificate       CertificateStore
	faugus            launcher.Faugus
	desktop           launcher.Desktop
	runningExecutable RunningExecutable
	executableExists  PathProbe
}

func NewDoctor(dependencies DoctorDependencies) Doctor {
	return Doctor{
		installation:      dependencies.Installation,
		fontPlan:          fonts.PlanFor(dependencies.Installation.FontVariant()),
		umu:               dependencies.Umu,
		runners:           dependencies.Runners,
		prefix:            dependencies.Prefix,
		fonts:             dependencies.Fonts,
		certificate:       dependencies.Certificate,
		faugus:            dependencies.Faugus,
		desktop:           dependencies.Desktop,
		runningExecutable: dependencies.RunningExecutable,
		executableExists:  dependencies.ExecutableExists,
	}
}

func (doctor Doctor) Examine() health.Report {
	return health.NewReport(
		doctor.umuCheck(),
		doctor.runnerCheck(),
		doctor.certificateCheck(),
		doctor.prefixCheck(),
		doctor.fontCheck(),
		doctor.faugusCheck(),
		doctor.desktopCheck(),
	)
}

func (doctor Doctor) umuCheck() health.Check {
	description, err := doctor.umu.Describe()
	if err != nil {
		return health.Failed("umu-run", err.Error())
	}
	return health.Passed("umu-run", description)
}

func (doctor Doctor) runnerCheck() health.Check {
	line, lineProblem := doctor.runners.Find(runner.LineRunnerName)
	source, sourceProblem := doctor.runners.Find(doctor.installation.SourceRunner())
	if err := errors.Join(lineProblem, sourceProblem); err != nil {
		return health.Failed("Proton-LINE", "could not read the runner versions: "+err.Error())
	}
	freshness := runner.Assess(line, source)
	if freshness != runner.Current {
		return health.Failed("Proton-LINE", freshness.String()+": "+doctor.versionComparison(line, " vs ", source)+"; run `line-proton runner build`")
	}
	return health.Passed("Proton-LINE", doctor.versionComparison(line, " matches ", source))
}

func (doctor Doctor) versionComparison(line runner.Installed, relation string, source runner.Installed) string {
	return runner.LineRunnerName + " " + versionOf(line) + relation + doctor.installation.SourceRunner() + " " + versionOf(source)
}

func versionOf(installed runner.Installed) string {
	found, present := installed.Runner()
	if !present {
		return "(not installed)"
	}
	return found.Version().String()
}

func (doctor Doctor) certificateCheck() health.Check {
	if !doctor.certificate.Exists() {
		return health.Failed("certificate", "missing; run `line-proton setup`")
	}
	return health.Passed("certificate", "present")
}

func (doctor Doctor) prefixCheck() health.Check {
	files, scanProblem := doctor.prefix.Scan()
	marker, markerProblem := doctor.prefix.Marker()
	if err := errors.Join(scanProblem, markerProblem); err != nil {
		return health.Failed("prefix", "could not check the system DLLs: "+err.Error())
	}
	if !marker.CoversAll(files) {
		return health.Failed("prefix", "unsigned system DLLs; run `line-proton sign`")
	}
	return health.Passed("prefix", "system DLLs signed")
}

func (doctor Doctor) fontCheck() health.Check {
	if _, err := doctor.fonts.Locate(doctor.fontPlan.Files()); err != nil {
		return health.Failed("fonts", err.Error())
	}
	return health.Passed("fonts", doctor.fontPlan.Variant().Family())
}

func (doctor Doctor) faugusCheck() health.Check {
	if !doctor.installation.Launcher().UsesFaugus() {
		return health.Passed("faugus", "not used")
	}
	fields, found, err := doctor.faugus.Game()
	if err != nil {
		return health.Failed("faugus", "could not read the Faugus games: "+err.Error())
	}
	if !found {
		return health.Failed("faugus", "no LINE entry; run `line-proton setup`")
	}
	return doctor.hookCheck(fields)
}

func (doctor Doctor) hookCheck(fields map[string]string) health.Check {
	if !doctor.executableExists(doctor.installation.Executable()) {
		return health.Failed("faugus", "the hook binary "+doctor.installation.Executable()+" is gone; run `line-proton setup`")
	}
	running, err := doctor.runningExecutable()
	if err != nil {
		return health.Failed("faugus", "could not find the running binary: "+err.Error())
	}
	if !launcher.HooksPointTo(fields, running) {
		return health.Failed("faugus", "hooks run another binary than "+running+"; run `line-proton setup`")
	}
	return health.Passed("faugus", "LINE entry with hooks into "+running)
}

func (doctor Doctor) desktopCheck() health.Check {
	if !doctor.desktop.Installed() {
		return health.Failed("menu entry", "missing; run `line-proton setup`")
	}
	return health.Passed("menu entry", "installed")
}
