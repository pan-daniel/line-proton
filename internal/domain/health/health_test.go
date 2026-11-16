package health_test

import (
	"slices"
	"testing"

	"github.com/pan-daniel/line-proton/internal/domain/health"
)

func TestReportIsHealthyOnlyWhenEveryCheckPassed(test *testing.T) {
	healthy := health.NewReport(health.Passed("umu-run", "found"))
	sick := health.NewReport(health.Passed("umu-run", "found"), health.Failed("fonts", "missing"))
	if !healthy.Healthy() || sick.Healthy() {
		test.Errorf("healthy = %v, sick = %v", healthy.Healthy(), sick.Healthy())
	}
}

func TestReportLines(test *testing.T) {
	report := health.NewReport(health.Passed("umu-run", "/usr/bin/umu-run"), health.Failed("fonts", "missing NotoSansThai-Regular.ttf"))
	want := []string{"✓ umu-run — /usr/bin/umu-run", "✗ fonts — missing NotoSansThai-Regular.ttf"}
	if got := report.Lines(); !slices.Equal(got, want) {
		test.Errorf("Lines = %q", got)
	}
}
