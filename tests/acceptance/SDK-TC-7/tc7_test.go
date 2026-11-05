// Package sdktc7 verifies the local consistency entry: it runs the suite's case
// set, lists the failed and not-run cases, and marks a run whose case-set digest
// differs from the released one as incomparable (SDK-AC-6, SDK-TC-7, P2).
package sdktc7

import (
	"slices"
	"testing"

	"github.com/tsic404/kos-sdk/localcheck"
	"github.com/tsic404/kos-sdk/tests/acceptance/fixtures"
)

func run(t *testing.T, set localcheck.CaseSet, released localcheck.Digest) localcheck.Report {
	t.Helper()
	report, err := localcheck.Run(set, released)
	if err != nil {
		t.Fatalf("run the case set: %v", err)
	}
	return report
}

func ids(cases []localcheck.CaseResult) []string {
	found := make([]string, 0, len(cases))
	for _, one := range cases {
		found = append(found, one.ID)
	}
	return found
}

func find(t *testing.T, cases []localcheck.CaseResult, id string) localcheck.CaseResult {
	t.Helper()
	for _, one := range cases {
		if one.ID == id {
			return one
		}
	}
	t.Fatalf("case %s is not in the report", id)
	return localcheck.CaseResult{}
}

// withCriterion returns a copy of set with one case's criterion version changed,
// the way a suite edit moves a case away from the released version.
func withCriterion(set localcheck.CaseSet, id, criterion string) localcheck.CaseSet {
	altered := localcheck.CaseSet{Cases: append([]localcheck.Case(nil), set.Cases...)}
	for index := range altered.Cases {
		if altered.Cases[index].ID == id {
			altered.Cases[index].Criterion = criterion
		}
	}
	return altered
}

func TestFailedCasesAreListed(t *testing.T) {
	set := fixtures.SuiteCaseSet(map[string]localcheck.Outcome{
		"CONF-TC-3": {Status: localcheck.StatusFailed, Detail: "asset digest changed after packing"},
	})
	report := run(t, set, set.Digest())

	if got := ids(report.Failed); !slices.Equal(got, []string{"CONF-TC-3"}) {
		t.Fatalf("failed cases = %v, want [CONF-TC-3]", got)
	}
	if detail := find(t, report.Failed, "CONF-TC-3").Detail; detail != "asset digest changed after packing" {
		t.Fatalf("failed case detail = %q, want the difference the case observed", detail)
	}
	if slices.Contains(ids(report.Passed), "CONF-TC-3") {
		t.Fatal("the failed case is counted as passed")
	}
	if len(report.Cases) != len(set.Cases) {
		t.Fatalf("report carries %d cases, want %d: one failure stopped the run", len(report.Cases), len(set.Cases))
	}
	if report.Conclusion != localcheck.ConclusionFailed {
		t.Fatalf("conclusion = %q, want %q", report.Conclusion, localcheck.ConclusionFailed)
	}
}

func TestDigestMismatchIsIncomparable(t *testing.T) {
	released := fixtures.SuiteCaseSet(nil).Digest()
	altered := withCriterion(fixtures.SuiteCaseSet(nil), "CONF-TC-3", "2")

	report := run(t, altered, released)

	if report.Conclusion != localcheck.ConclusionIncomparable {
		t.Fatalf("conclusion = %q, want %q: every case passed but the digest differs from the release",
			report.Conclusion, localcheck.ConclusionIncomparable)
	}
	if len(report.Failed) != 0 || len(report.NotRun) != 0 {
		t.Fatalf("report carries %d failed and %d not-run cases, want none",
			len(report.Failed), len(report.NotRun))
	}
	if report.Digest == report.Released {
		t.Fatalf("both digests are %s, want the run digest and the released digest apart", report.Digest)
	}
	if report.Released != released {
		t.Fatalf("released digest = %s, want %s", report.Released, released)
	}
}

func TestNotRunCasesAreNotCountedAsPassed(t *testing.T) {
	set := fixtures.SuiteCaseSet(map[string]localcheck.Outcome{
		"CONF-TC-4": {Status: localcheck.StatusNotRun, Detail: "no installed app on this machine"},
	})
	report := run(t, set, set.Digest())

	if got := ids(report.NotRun); !slices.Equal(got, []string{"CONF-TC-4"}) {
		t.Fatalf("not-run cases = %v, want [CONF-TC-4]", got)
	}
	if slices.Contains(ids(report.Passed), "CONF-TC-4") {
		t.Fatal("the case that did not run is counted as passed")
	}
	if len(report.Failed) != 0 {
		t.Fatalf("failed cases = %v, want none: a case that did not run is not a failure", ids(report.Failed))
	}
	if report.Conclusion != localcheck.ConclusionFailed {
		t.Fatalf("conclusion = %q, want %q", report.Conclusion, localcheck.ConclusionFailed)
	}
}

func TestIncomparableOutranksFailure(t *testing.T) {
	set := fixtures.SuiteCaseSet(map[string]localcheck.Outcome{
		"CONF-TC-3": {Status: localcheck.StatusFailed, Detail: "asset digest changed after packing"},
	})
	altered := withCriterion(set, "CONF-TC-2", "2")

	report := run(t, altered, set.Digest())

	if report.Conclusion != localcheck.ConclusionIncomparable {
		t.Fatalf("conclusion = %q, want %q: a run off the released case set is neither a pass nor a failure",
			report.Conclusion, localcheck.ConclusionIncomparable)
	}
	if got := ids(report.Failed); !slices.Equal(got, []string{"CONF-TC-3"}) {
		t.Fatalf("failed cases = %v, want [CONF-TC-3]: the case results stay readable", got)
	}
}

func TestReportKeepsGroupsAndAttribution(t *testing.T) {
	set := fixtures.SuiteCaseSet(nil)
	report := run(t, set, set.Digest())

	want := []localcheck.Group{
		localcheck.GroupCapability,
		localcheck.GroupProvider,
		localcheck.GroupExtension,
		localcheck.GroupApp,
	}
	got := make([]localcheck.Group, 0, len(report.Cases))
	for _, one := range report.Cases {
		got = append(got, one.Group)
		if len(one.Contracts) == 0 {
			t.Fatalf("case %s carries no contract attribution", one.ID)
		}
	}
	if !slices.Equal(got, want) {
		t.Fatalf("report groups = %v, want %v", got, want)
	}
}

func TestRepeatedRunOfTheSameSetYieldsTheSameDigest(t *testing.T) {
	first := fixtures.SuiteCaseSet(nil)
	second := fixtures.SuiteCaseSet(nil)

	if first.Digest() != second.Digest() {
		t.Fatalf("digests differ: %s and %s", first.Digest(), second.Digest())
	}
	report := run(t, second, first.Digest())
	if report.Conclusion != localcheck.ConclusionPassed {
		t.Fatalf("conclusion = %q, want %q: every case passed against the released digest",
			report.Conclusion, localcheck.ConclusionPassed)
	}
}
