package localcheck

import (
	"fmt"
	"strings"
)

// Conclusion is what a local run states about the released suite.
type Conclusion string

const (
	// ConclusionPassed: the digest matches the release and every case passed.
	ConclusionPassed Conclusion = "passed"
	// ConclusionFailed: the digest matches the release, but at least one case
	// failed or did not run.
	ConclusionFailed Conclusion = "failed"
	// ConclusionIncomparable: the digest differs from the release's, so the run
	// is not a suite result and counts as neither a pass nor a failure
	// (CONF-C-3).
	ConclusionIncomparable Conclusion = "incomparable"
)

// CaseResult is one row of the report.
type CaseResult struct {
	ID        string
	Group     Group
	Contracts []string
	Status    Status
	Detail    string
}

// Report is what a local run states: every case with its result, the failed and
// the not-run cases listed apart, the run's digest, the digest the distribution
// recorded, and the conclusion.
type Report struct {
	Cases      []CaseResult
	Passed     []CaseResult
	Failed     []CaseResult
	NotRun     []CaseResult
	Digest     Digest
	Released   Digest
	Conclusion Conclusion
}

// Run executes the set and compares it with the digest the distribution
// recorded. One case failing does not stop the run, and every case that ran
// keeps its own result. An invalid case set yields no report.
func Run(set CaseSet, released Digest) (Report, error) {
	if err := set.Validate(); err != nil {
		return Report{}, err
	}

	report := Report{Digest: set.Digest(), Released: released}
	for _, one := range set.byGroup() {
		observed := one.Run()
		result := CaseResult{
			ID:        one.ID,
			Group:     one.Group,
			Contracts: append([]string(nil), one.Contracts...),
			Status:    classify(observed.Status),
			Detail:    observed.Detail,
		}
		report.Cases = append(report.Cases, result)
		switch result.Status {
		case StatusPassed:
			report.Passed = append(report.Passed, result)
		case StatusNotRun:
			report.NotRun = append(report.NotRun, result)
		default:
			report.Failed = append(report.Failed, result)
		}
	}

	switch {
	case !sameDigest(report.Digest, released):
		report.Conclusion = ConclusionIncomparable
	case len(report.Failed) > 0 || len(report.NotRun) > 0:
		report.Conclusion = ConclusionFailed
	default:
		report.Conclusion = ConclusionPassed
	}
	return report, nil
}

// classify maps a case's own outcome onto the three report statuses. Anything
// the case did not report as a pass or a not-run is a failure, so an unexpected
// status never becomes a pass.
func classify(status Status) Status {
	switch status {
	case StatusPassed, StatusNotRun:
		return status
	default:
		return StatusFailed
	}
}

// sameDigest compares the wire form while ignoring case and surrounding
// whitespace, so the way a recorded digest was written does not decide
// comparability.
func sameDigest(left, right Digest) bool {
	return strings.EqualFold(strings.TrimSpace(string(left)), strings.TrimSpace(string(right)))
}

// String renders the report for a terminal: the conclusion and both digests,
// every case under its group, then the failed and the not-run cases apart.
func (r Report) String() string {
	var out strings.Builder
	fmt.Fprintf(&out, "conclusion: %s\n", r.Conclusion)
	fmt.Fprintf(&out, "run digest: %s\n", r.Digest)
	fmt.Fprintf(&out, "released:   %s\n", r.Released)

	for _, group := range groups {
		rows := make([]CaseResult, 0, len(r.Cases))
		for _, result := range r.Cases {
			if result.Group == group {
				rows = append(rows, result)
			}
		}
		if len(rows) == 0 {
			continue
		}
		fmt.Fprintf(&out, "\n%s\n", group)
		for _, result := range rows {
			fmt.Fprintf(&out, "  %-8s %s  %s", result.Status, result.ID, strings.Join(result.Contracts, ","))
			if result.Detail != "" {
				fmt.Fprintf(&out, ": %s", result.Detail)
			}
			out.WriteString("\n")
		}
	}

	fmt.Fprintf(&out, "\nfailed: %s\n", joinIDs(r.Failed))
	fmt.Fprintf(&out, "not run: %s\n", joinIDs(r.NotRun))
	return out.String()
}

func joinIDs(cases []CaseResult) string {
	if len(cases) == 0 {
		return "none"
	}
	ids := make([]string, 0, len(cases))
	for _, one := range cases {
		ids = append(ids, one.ID)
	}
	return strings.Join(ids, ", ")
}
