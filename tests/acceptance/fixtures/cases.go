package fixtures

import "github.com/tsic404/kos-sdk/localcheck"

// SuiteCaseSet returns the four-group case set of a released suite: one case per
// group, each naming the contract entries it verifies and a criterion version
// (CONF-C-1). A case whose id appears in outcomes reports that outcome; every
// other case passes. Each call builds a fresh set so a case can change its copy.
func SuiteCaseSet(outcomes map[string]localcheck.Outcome) localcheck.CaseSet {
	cases := []localcheck.Case{
		{
			ID:        "CONF-TC-1",
			Group:     localcheck.GroupCapability,
			Contracts: []string{"KOSD-C-09", "KOSD-C-10", "KOSD-C-11"},
			Criterion: "1",
		},
		{
			ID:        "CONF-TC-2",
			Group:     localcheck.GroupProvider,
			Contracts: []string{"KOSD-C-47", "KOSD-C-48"},
			Criterion: "1",
		},
		{
			ID:        "CONF-TC-3",
			Group:     localcheck.GroupExtension,
			Contracts: []string{"KOSD-C-17", "KOSD-C-18", "KOSD-C-19", "KOSD-C-20"},
			Criterion: "1",
		},
		{
			ID:        "CONF-TC-4",
			Group:     localcheck.GroupApp,
			Contracts: []string{"KOSD-C-73", "KOSD-C-75"},
			Criterion: "1",
		},
	}
	for index := range cases {
		outcome, given := outcomes[cases[index].ID]
		cases[index].Run = func() localcheck.Outcome {
			if given {
				return outcome
			}
			return localcheck.Outcome{Status: localcheck.StatusPassed}
		}
	}
	return localcheck.CaseSet{Cases: cases}
}
