// Package extension is the author-side declaration surface for extension
// object classes and controllers (KOSD-C-19, KOSD-C-20).
package extension

import (
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

// ObjectClassFields is the declaration's closed field set.
var ObjectClassFields = []string{"apiGroup", "version", "kind", "scope"}

// ObjectClassScopes are the scopes an extension object class may declare.
var ObjectClassScopes = []string{"Namespaced", "Cluster"}

// ObjectClassScope is the scope of an extension object class.
type ObjectClassScope string

const (
	ScopeNamespaced ObjectClassScope = "Namespaced"
	ScopeCluster    ObjectClassScope = "Cluster"
)

// ObjectClassDeclaration declares one extension object class. The class belongs
// to the extension's own API group and is served only while its component is
// installed.
type ObjectClassDeclaration struct {
	APIGroup string           `json:"apiGroup"`
	Version  string           `json:"version"`
	Kind     string           `json:"kind"`
	Scope    ObjectClassScope `json:"scope"`
}

// CheckObjectClass validates a declaration at path.
func CheckObjectClass(c *validate.Checker, value any, path string) ObjectClassDeclaration {
	record, ok := c.Record(value, path)
	if !ok {
		return ObjectClassDeclaration{}
	}
	c.RejectUnknown(record, ObjectClassFields, path)

	declaration := ObjectClassDeclaration{}
	declaration.APIGroup, _ = c.RequireString(record, "apiGroup", path)
	declaration.Version, _ = c.RequireString(record, "version", path)
	declaration.Kind = checkKind(c, record, path)
	declaration.Scope = checkScope(c, record, path)
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

func checkScope(c *validate.Checker, record map[string]any, path string) ObjectClassScope {
	scope, ok := c.RequireString(record, "scope", path)
	if !ok {
		return ""
	}
	switch ObjectClassScope(scope) {
	case ScopeNamespaced:
		return ScopeNamespaced
	case ScopeCluster:
		return ScopeCluster
	default:
		c.Add(validate.Join(path, "scope"), validate.CodeInvalidValue,
			"scope must be one of Namespaced, Cluster")
		return ""
	}
}

func isCoreKind(kind string) bool {
	for _, core := range CoreObjectKinds {
		if core == kind {
			return true
		}
	}
	return false
}
