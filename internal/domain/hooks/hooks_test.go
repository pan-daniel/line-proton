package hooks_test

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/pan-daniel/line-proton/internal/domain/hooks"
)

func described(test *testing.T, moment hooks.Moment) []string {
	test.Helper()
	phases, err := hooks.Phases(moment)
	if err != nil {
		test.Fatal(err)
	}
	descriptions := []string{}
	for _, phase := range phases {
		descriptions = append(descriptions, phase.String())
	}
	return descriptions
}

func TestPhasesPerMoment(test *testing.T) {
	examples := map[hooks.Moment][]string{
		hooks.BeforeLaunch: {
			"skip when busy: signing prefix",
		},
		hooks.AfterLaunch: {
			"wait when busy: waiting for Proton-LINE to go idle, signing prefix, refreshing Proton-LINE, ensuring fonts",
		},
		hooks.ManualSign: {
			"wait when busy: signing prefix",
		},
		hooks.StandaloneLaunch: {
			"wait when busy: refreshing Proton-LINE, signing prefix",
			"unlocked: running LINE, waiting for Proton-LINE to go idle",
			"skip when busy: signing prefix",
		},
	}
	for moment, want := range examples {
		test.Run(moment.String(), func(test *testing.T) {
			if got := described(test, moment); !slices.Equal(got, want) {
				test.Errorf("Phases(%v) = %q, want %q", moment, got, want)
			}
		})
	}
}

func TestPhaseStepsAreACopy(test *testing.T) {
	phases, err := hooks.Phases(hooks.BeforeLaunch)
	if err != nil {
		test.Fatal(err)
	}
	phase := phases[0]
	steps := phase.Steps()
	steps[0] = hooks.RunLINE
	if phase.Steps()[0] != hooks.SignPrefix {
		test.Error("phase was mutated through Steps()")
	}
}

func TestIdleLimit(test *testing.T) {
	limit, bounded := hooks.IdleLimit(hooks.AfterLaunch)
	if !bounded || limit != 2*time.Minute {
		test.Errorf("post hook limit = %v, %v", limit, bounded)
	}
	if _, bounded := hooks.IdleLimit(hooks.StandaloneLaunch); bounded {
		test.Error("standalone launch waits for LINE without a limit")
	}
}

func TestParseMoment(test *testing.T) {
	if moment, err := hooks.ParseMoment("post"); err != nil || moment != hooks.AfterLaunch {
		test.Errorf("ParseMoment(post) = %v, %v", moment, err)
	}
	if _, err := hooks.ParseMoment("during"); !errors.Is(err, hooks.ErrUnknownMoment) {
		test.Errorf("err = %v", err)
	}
}

func TestPhasesRefuseAnUnknownMoment(test *testing.T) {
	if phases, err := hooks.Phases(hooks.Moment(99)); !errors.Is(err, hooks.ErrUnknownMoment) || len(phases) != 0 {
		test.Errorf("Phases = %v, %v; want ErrUnknownMoment", phases, err)
	}
}
