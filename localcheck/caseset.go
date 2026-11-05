// Package localcheck runs the conformance case set on an author's machine
// (SDK-C-5).
//
// The entry reuses the suite's case set and its comparability judgement and
// adds no pass standard of its own: a case counts as passed only when its own
// check reports a pass, and a run is comparable only when its case-set digest
// equals the one the distribution recorded (CONF-AC-1, CONF-C-3). Failed and
// not-run cases are listed apart and never counted as passed.
package localcheck

import (
	"sort"
	"strings"

	"github.com/tsic404/kos-sdk/internal/framedhash"
	"github.com/tsic404/kos-sdk/validate"
)

// Group is one of the four groups a suite run reports (CONF-C-1).
type Group string

const (
	GroupCapability Group = "capability"
	GroupProvider   Group = "provider"
	GroupExtension  Group = "extension"
	GroupApp        Group = "app"
)

// groups is the order a report lists the four groups in.
var groups = [...]Group{GroupCapability, GroupProvider, GroupExtension, GroupApp}

func (g Group) isKnown() bool {
	return groupRank(g) < len(groups)
}

// groupRank returns the position of g in the report, or len(groups) when g is
// not one of the four groups.
func groupRank(g Group) int {
	for index, known := range groups {
		if g == known {
			return index
		}
	}
	return len(groups)
}

// Status is what one case observed. A case that reports neither a pass nor a
// not-run outcome is recorded as failed: it did not report a pass, so it never
// counts as one.
type Status string

const (
	StatusPassed Status = "passed"
	StatusFailed Status = "failed"
	StatusNotRun Status = "not-run"
)

// Outcome is the result one case reports about the subject it inspected.
// Detail says what the case observed — the difference on failure, the unmet
// precondition when it did not run — so the report can point at the case.
type Outcome struct {
	Status Status
	Detail string
}

// Case is one case of the suite's set: the group it belongs to, the contract
// entries it verifies, the version of its criterion, and its check (CONF-C-1).
type Case struct {
	ID        string
	Group     Group
	Contracts []string
	Criterion string
	Run       func() Outcome
}

// CaseSet is the set a run executes. It is the suite's set: the entry neither
// adds, removes nor reorders cases before running them.
type CaseSet struct {
	Cases []Case
}

// Digest is the case-set digest of CONF-C-3 in its "sha256:<hex>" wire form.
type Digest string

// String returns the digest in its wire form.
func (d Digest) String() string { return string(d) }

// Digest returns the digest CONF-C-3 defines: the case set and each case's
// criterion version, in case-id order. Group and contract attribution are
// report metadata and stay out of it, so the same set written in another order
// still yields the same digest.
func (s CaseSet) Digest() Digest {
	hasher := framedhash.New()
	for _, one := range s.byID() {
		hasher.Add(one.ID, []byte(one.Criterion))
	}
	return Digest(hasher.Sum())
}

// Validate reports the reasons a case set cannot stand in for a suite run: a
// case must sit in one of the four groups, name the contract entries it
// verifies without a blank one, carry its criterion version and its check, and
// use an id no other case uses (CONF-C-1). An empty set is rejected as well: a
// run that executes nothing concludes nothing.
func (s CaseSet) Validate() error {
	if len(s.Cases) == 0 {
		return &validate.InvalidError{
			What: "case set",
			Issues: []validate.Issue{{
				Path:    "$.cases",
				Code:    validate.CodeMissingField,
				Message: "the case set carries no case",
			}},
		}
	}

	c := &validate.Checker{}
	seen := make(map[string]bool, len(s.Cases))
	for index, one := range s.Cases {
		path := validate.Index("$.cases", index)
		switch {
		case one.ID == "":
			c.Add(validate.Join(path, "id"), validate.CodeMissingField, "case id is required")
		case seen[one.ID]:
			c.Add(validate.Join(path, "id"), validate.CodeInvalidValue, "case id %q is used twice", one.ID)
		default:
			seen[one.ID] = true
		}
		if !one.Group.isKnown() {
			c.Add(validate.Join(path, "group"), validate.CodeInvalidValue,
				"group %q is not one of the four suite groups", one.Group)
		}
		if len(one.Contracts) == 0 {
			c.Add(validate.Join(path, "contracts"), validate.CodeInvalidValue,
				"case names no contract entry it verifies")
		}
		for position, entry := range one.Contracts {
			if strings.TrimSpace(entry) == "" {
				c.Add(validate.Index(validate.Join(path, "contracts"), position), validate.CodeMissingField,
					"contract entry is blank")
			}
		}
		if one.Criterion == "" {
			c.Add(validate.Join(path, "criterion"), validate.CodeMissingField, "case carries no criterion version")
		}
		if one.Run == nil {
			c.Add(validate.Join(path, "run"), validate.CodeMissingField, "case carries no check")
		}
	}
	if len(c.Issues) > 0 {
		return &validate.InvalidError{What: "case set", Issues: c.Issues}
	}
	return nil
}

// byID returns the cases in case-id order, the order the digest uses.
func (s CaseSet) byID() []Case {
	ordered := append([]Case(nil), s.Cases...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].ID < ordered[j].ID })
	return ordered
}

// byGroup returns the cases in the order a report lists them: the four groups
// of CONF-C-1, each in case-id order.
func (s CaseSet) byGroup() []Case {
	ordered := append([]Case(nil), s.Cases...)
	sort.Slice(ordered, func(i, j int) bool {
		left, right := groupRank(ordered[i].Group), groupRank(ordered[j].Group)
		if left != right {
			return left < right
		}
		return ordered[i].ID < ordered[j].ID
	})
	return ordered
}
