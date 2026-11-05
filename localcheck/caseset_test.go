package localcheck_test

import (
	"errors"
	"testing"

	"github.com/tsic404/kos-sdk/localcheck"
	"github.com/tsic404/kos-sdk/validate"
)

func passing() localcheck.Outcome {
	return localcheck.Outcome{Status: localcheck.StatusPassed}
}

func testCase(id string, group localcheck.Group, contracts []string, criterion string,
	run func() localcheck.Outcome) localcheck.Case {
	return localcheck.Case{
		ID:        id,
		Group:     group,
		Contracts: contracts,
		Criterion: criterion,
		Run:       run,
	}
}

func validCase(id string) localcheck.Case {
	return testCase(id, localcheck.GroupCapability, []string{"KOSD-C-09"}, "1", passing)
}

func hasIssue(issues []validate.Issue, path string) bool {
	for _, issue := range issues {
		if issue.Path == path {
			return true
		}
	}
	return false
}

func TestValidateRejectsCaseSetsThatCannotStandInForTheSuite(t *testing.T) {
	tests := []struct {
		name string
		set  localcheck.CaseSet
		path string
	}{
		{
			name: "empty set",
			set:  localcheck.CaseSet{},
			path: "$.cases",
		},
		{
			name: "group outside the four",
			set: localcheck.CaseSet{Cases: []localcheck.Case{
				testCase("CONF-TC-1", "widget", []string{"KOSD-C-09"}, "1", passing),
			}},
			path: "$.cases[0].group",
		},
		{
			name: "no contract attribution",
			set: localcheck.CaseSet{Cases: []localcheck.Case{
				testCase("CONF-TC-1", localcheck.GroupCapability, nil, "1", passing),
			}},
			path: "$.cases[0].contracts",
		},
		{
			name: "blank contract attribution",
			set: localcheck.CaseSet{Cases: []localcheck.Case{
				testCase("CONF-TC-1", localcheck.GroupCapability, []string{" ", ""}, "1", passing),
			}},
			path: "$.cases[0].contracts[0]",
		},
		{
			name: "no criterion version",
			set: localcheck.CaseSet{Cases: []localcheck.Case{
				testCase("CONF-TC-1", localcheck.GroupCapability, []string{"KOSD-C-09"}, "", passing),
			}},
			path: "$.cases[0].criterion",
		},
		{
			name: "no check",
			set: localcheck.CaseSet{Cases: []localcheck.Case{
				testCase("CONF-TC-1", localcheck.GroupCapability, []string{"KOSD-C-09"}, "1", nil),
			}},
			path: "$.cases[0].run",
		},
		{
			name: "repeated id",
			set: localcheck.CaseSet{Cases: []localcheck.Case{
				validCase("CONF-TC-1"),
				validCase("CONF-TC-1"),
			}},
			path: "$.cases[1].id",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.set.Validate()
			var invalid *validate.InvalidError
			if !errors.As(err, &invalid) {
				t.Fatalf("validate error = %v, want a rejection", err)
			}
			if !hasIssue(invalid.Issues, test.path) {
				t.Fatalf("issues = %v, want one at %s", invalid.Issues, test.path)
			}
		})
	}
}

func TestDigestDoesNotDependOnCaseOrder(t *testing.T) {
	first := localcheck.CaseSet{Cases: []localcheck.Case{validCase("CONF-TC-1"), validCase("CONF-TC-2")}}
	second := localcheck.CaseSet{Cases: []localcheck.Case{validCase("CONF-TC-2"), validCase("CONF-TC-1")}}

	if first.Digest() != second.Digest() {
		t.Fatalf("digests differ: %s and %s", first.Digest(), second.Digest())
	}
}

func TestDigestChangesWhenTheCaseSetChanges(t *testing.T) {
	base := localcheck.CaseSet{Cases: []localcheck.Case{validCase("CONF-TC-1"), validCase("CONF-TC-2")}}
	added := localcheck.CaseSet{Cases: []localcheck.Case{
		validCase("CONF-TC-1"),
		validCase("CONF-TC-2"),
		validCase("CONF-TC-3"),
	}}
	removed := localcheck.CaseSet{Cases: []localcheck.Case{validCase("CONF-TC-1")}}

	if base.Digest() == added.Digest() {
		t.Fatalf("adding a case left the digest at %s", base.Digest())
	}
	if base.Digest() == removed.Digest() {
		t.Fatalf("removing a case left the digest at %s", base.Digest())
	}
	if added.Digest() == removed.Digest() {
		t.Fatalf("two different case sets share the digest %s", added.Digest())
	}
}

func TestUnexpectedStatusIsRecordedAsFailed(t *testing.T) {
	set := localcheck.CaseSet{Cases: []localcheck.Case{
		testCase("CONF-TC-1", localcheck.GroupCapability, []string{"KOSD-C-09"}, "1", func() localcheck.Outcome {
			return localcheck.Outcome{Status: "flaky"}
		}),
	}}

	report, err := localcheck.Run(set, set.Digest())
	if err != nil {
		t.Fatalf("run the case set: %v", err)
	}
	if len(report.Passed) != 0 {
		t.Fatalf("passed cases = %v, want none", report.Passed)
	}
	if len(report.Failed) != 1 || report.Failed[0].Status != localcheck.StatusFailed {
		t.Fatalf("failed cases = %v, want the case recorded as failed", report.Failed)
	}
	if report.Conclusion != localcheck.ConclusionFailed {
		t.Fatalf("conclusion = %q, want %q", report.Conclusion, localcheck.ConclusionFailed)
	}
}
