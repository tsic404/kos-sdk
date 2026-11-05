package localcheck_test

import (
	"strings"
	"testing"

	"github.com/tsic404/kos-sdk/localcheck"
	"github.com/tsic404/kos-sdk/tests/acceptance/fixtures"
)

// renderedStatuses maps each case row of a rendered report to the status it was
// rendered with. Case rows are the indented "<status> <id> ..." lines.
func renderedStatuses(rendered string) map[string]string {
	found := make(map[string]string)
	for _, line := range strings.Split(rendered, "\n") {
		if !strings.HasPrefix(line, "  ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		found[fields[1]] = fields[0]
	}
	return found
}

func TestReportRendersGroupsAndSeparatesFailedFromNotRun(t *testing.T) {
	set := fixtures.SuiteCaseSet(map[string]localcheck.Outcome{
		"CONF-TC-3": {Status: localcheck.StatusFailed, Detail: "asset digest changed after packing"},
		"CONF-TC-4": {Status: localcheck.StatusNotRun, Detail: "no installed app on this machine"},
	})
	report, err := localcheck.Run(set, set.Digest())
	if err != nil {
		t.Fatalf("run the case set: %v", err)
	}
	rendered := report.String()

	for _, line := range []string{
		"conclusion: " + string(report.Conclusion),
		"run digest: " + report.Digest.String(),
		"released:   " + report.Released.String(),
		"failed: CONF-TC-3",
		"not run: CONF-TC-4",
		"asset digest changed after packing",
		"no installed app on this machine",
	} {
		if !strings.Contains(rendered, line) {
			t.Fatalf("report does not render %q:\n%s", line, rendered)
		}
	}
	for _, group := range []string{"capability", "provider", "extension", "app"} {
		if !strings.Contains(rendered, "\n"+group+"\n") {
			t.Fatalf("report does not list group %q:\n%s", group, rendered)
		}
	}

	want := map[string]string{
		"CONF-TC-1": "passed",
		"CONF-TC-2": "passed",
		"CONF-TC-3": "failed",
		"CONF-TC-4": "not-run",
	}
	statuses := renderedStatuses(rendered)
	for id, status := range want {
		if statuses[id] != status {
			t.Fatalf("case %s rendered with status %q, want %q:\n%s", id, statuses[id], status, rendered)
		}
	}
}

func TestReportRendersACleanRunWithoutFailures(t *testing.T) {
	set := fixtures.SuiteCaseSet(nil)
	report, err := localcheck.Run(set, set.Digest())
	if err != nil {
		t.Fatalf("run the case set: %v", err)
	}
	rendered := report.String()

	if !strings.Contains(rendered, "conclusion: passed") {
		t.Fatalf("report does not state the pass:\n%s", rendered)
	}
	if !strings.Contains(rendered, "failed: none") || !strings.Contains(rendered, "not run: none") {
		t.Fatalf("report does not render the empty lists as none:\n%s", rendered)
	}
}
