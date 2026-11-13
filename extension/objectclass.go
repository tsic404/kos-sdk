// Package extension is the author-side declaration surface for extension
// object classes and controllers (KOSD-C-19, KOSD-C-20).
package extension

import (
	"strings"

	"github.com/tsic404/kos-sdk/validate"
)

// CoreObjectKinds are the kernel kinds closed by KOSD-C-01.
var CoreObjectKinds = []string{
	"Application",
	"ApplicationDefinition",
	"Volume",
	"Endpoint",
	"Provider",
	"SystemComponent",
	"BackupPolicy",
	"RestoreJob",
	"NodeDevice",
	"VolumeMigration",
}

// GroupKindFields is the class reference's closed field set.
var GroupKindFields = []string{"group", "kind"}

// GroupKind names one object class. An empty group is the run kernel's core
// group, so a controller may name a core-group object without inventing one.
type GroupKind struct {
	Group string `json:"group,omitempty"`
	Kind  string `json:"kind"`
}

// ObjectClassFields is the declaration's closed field set.
var ObjectClassFields = []string{"group", "kind", "resource", "versions", "namespaced", "references"}

// ObjectClassDeclaration declares one extension object class. The class belongs
// to the extension's own API group and is served only while its component is
// installed (KOSD-C-19).
type ObjectClassDeclaration struct {
	Group      string   `json:"group"`
	Kind       string   `json:"kind"`
	Resource   string   `json:"resource"`
	Versions   []string `json:"versions"`
	Namespaced bool     `json:"namespaced,omitempty"`
	References []string `json:"references,omitempty"`
}

// CheckGroupKind validates one class reference at path.
func CheckGroupKind(c *validate.Checker, value any, path string) GroupKind {
	record, ok := c.Record(value, path)
	if !ok {
		return GroupKind{}
	}
	c.RejectUnknown(record, GroupKindFields, path)

	reference := GroupKind{}
	reference.Kind, _ = c.RequireString(record, "kind", path)
	reference.Group = checkOptionalGroup(c, record, path)
	return reference
}

// checkOptionalGroup reads a group that may be empty: the empty group is the
// core group the kernel declares for Node, Event and the other baseline kinds.
func checkOptionalGroup(c *validate.Checker, record map[string]any, path string) string {
	raw, present := c.Optional(record, "group")
	if !present {
		return ""
	}
	group, ok := raw.(string)
	if !ok || strings.TrimSpace(group) != group {
		c.Add(validate.Join(path, "group"), validate.CodeInvalidType,
			"expected a string with no surrounding whitespace; the empty string is the core group")
		return ""
	}
	return group
}

// CheckObjectClass validates a declaration at path.
func CheckObjectClass(c *validate.Checker, value any, path string) ObjectClassDeclaration {
	record, ok := c.Record(value, path)
	if !ok {
		return ObjectClassDeclaration{}
	}
	c.RejectUnknown(record, ObjectClassFields, path)

	declaration := ObjectClassDeclaration{}
	declaration.Group, _ = c.RequireString(record, "group", path)
	declaration.Kind = checkKind(c, record, path)
	declaration.Resource, _ = c.RequireString(record, "resource", path)
	declaration.Versions, _ = c.RequireStringArray(record, "versions", path)
	if raw, present := c.Optional(record, "namespaced"); present {
		declaration.Namespaced, _ = c.Boolean(raw, validate.Join(path, "namespaced"))
	}
	declaration.References, _ = c.OptionalStringArray(record, "references", path)
	return declaration
}

// DeclareObjectClass builds a declaration, rejecting one the author must not hold.
func DeclareObjectClass(input any) (ObjectClassDeclaration, error) {
	c := &validate.Checker{}
	declaration := CheckObjectClass(c, input, "$")
	if len(c.Issues) > 0 {
		return ObjectClassDeclaration{}, &validate.InvalidError{
			What:   "object class declaration",
			Issues: c.Issues,
		}
	}
	return declaration, nil
}

func checkKind(c *validate.Checker, record map[string]any, path string) string {
	kind, ok := c.RequireString(record, "kind", path)
	if !ok {
		return ""
	}
	if isCoreKind(kind) {
		c.Add(validate.Join(path, "kind"), validate.CodeInvalidValue,
			"%q is a kernel kind closed by KOSD-C-01; an extension declares its own kind", kind)
		return ""
	}
	return kind
}

func isCoreKind(kind string) bool {
	for _, core := range CoreObjectKinds {
		if core == kind {
			return true
		}
	}
	return false
}
