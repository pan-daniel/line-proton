package hooks

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

var (
	ErrUnknownMoment  = errors.New("unknown hook moment, use pre or post")
	ErrUnknownStep    = errors.New("unknown hook step")
	ErrUnknownLocking = errors.New("unknown locking mode")
)

const PollInterval = 2 * time.Second

type Moment int

const (
	BeforeLaunch Moment = iota
	AfterLaunch
	StandaloneLaunch
	ManualSign
)

func ParseMoment(name string) (Moment, error) {
	switch name {
	case "pre":
		return BeforeLaunch, nil
	case "post":
		return AfterLaunch, nil
	}
	return BeforeLaunch, fmt.Errorf("%w: %q", ErrUnknownMoment, name)
}

func (moment Moment) String() string {
	return [...]string{"pre", "post", "launch", "sign"}[moment]
}

type Step int

const (
	RefreshRunner Step = iota
	SignPrefix
	RunLINE
	WaitUntilRunnerIdle
	EnsureFonts
)

func (step Step) String() string {
	return [...]string{"refreshing Proton-LINE", "signing prefix", "running LINE", "waiting for Proton-LINE to go idle", "ensuring fonts"}[step]
}

type Locking int

const (
	SkipWhenBusy Locking = iota
	WaitWhenBusy
	Unlocked
)

func (locking Locking) String() string {
	return [...]string{"skip when busy", "wait when busy", "unlocked"}[locking]
}

type Phase struct {
	steps   []Step
	locking Locking
}

func phaseOf(locking Locking, steps ...Step) Phase {
	return Phase{steps: steps, locking: locking}
}

func (phase Phase) Steps() []Step {
	return slices.Clone(phase.steps)
}

func (phase Phase) Locking() Locking {
	return phase.locking
}

func (phase Phase) String() string {
	names := make([]string, 0, len(phase.steps))
	for _, step := range phase.steps {
		names = append(names, step.String())
	}
	return phase.locking.String() + ": " + strings.Join(names, ", ")
}

func Phases(moment Moment) ([]Phase, error) {
	switch moment {
	case BeforeLaunch:
		return []Phase{phaseOf(SkipWhenBusy, SignPrefix)}, nil
	case AfterLaunch:
		return []Phase{phaseOf(WaitWhenBusy, WaitUntilRunnerIdle, SignPrefix, RefreshRunner, EnsureFonts)}, nil
	case ManualSign:
		return []Phase{phaseOf(WaitWhenBusy, SignPrefix)}, nil
	case StandaloneLaunch:
		return []Phase{
			phaseOf(WaitWhenBusy, RefreshRunner, SignPrefix),
			phaseOf(Unlocked, RunLINE, WaitUntilRunnerIdle),
			phaseOf(SkipWhenBusy, SignPrefix),
		}, nil
	}
	return nil, fmt.Errorf("%w: %d", ErrUnknownMoment, int(moment))
}

func IdleLimit(moment Moment) (time.Duration, bool) {
	if moment == StandaloneLaunch {
		return 0, false
	}
	return 2 * time.Minute, true
}
