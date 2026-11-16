package health

import "slices"

type Check struct {
	name    string
	healthy bool
	detail  string
}

func Passed(name, detail string) Check {
	return Check{name: name, healthy: true, detail: detail}
}

func Failed(name, detail string) Check {
	return Check{name: name, detail: detail}
}

func (check Check) failed() bool {
	return !check.healthy
}

func (check Check) line() string {
	return symbol(check.healthy) + " " + check.name + " — " + check.detail
}

func symbol(healthy bool) string {
	if healthy {
		return "✓"
	}
	return "✗"
}

type Report struct {
	checks []Check
}

func NewReport(checks ...Check) Report {
	return Report{checks: slices.Clone(checks)}
}

func (report Report) Healthy() bool {
	return !slices.ContainsFunc(report.checks, Check.failed)
}

func (report Report) Lines() []string {
	lines := make([]string, 0, len(report.checks))
	for _, check := range report.checks {
		lines = append(lines, check.line())
	}
	return lines
}
