package extension

import (
	"github.com/tsic404/kos-sdk/validate"
)

// ControllerFields is the controller declaration's closed field set.
var ControllerFields = []string{"id", "manages", "scope"}

// ControllerScopeFields is one write scope's closed field set.
var ControllerScopeFields = []string{"groups", "kinds", "namespaces"}

// ControllerScope is the declared reach of one controller: API groups,
// downstream kinds and namespaces (KOSD-C-04, KOSD-C-18, KOSD-C-20).
type ControllerScope struct {
	Groups     []string    `json:"groups,omitempty"`
	Kinds      []GroupKind `json:"kinds,omitempty"`
	Namespaces []string    `json:"namespaces,omitempty"`
}

// ControllerDeclaration declares an extension controller, the object classes it
// serves and the write scope it was granted. It never writes kernel objects
// (KOSD-C-03, KOSD-C-20).
type ControllerDeclaration struct {
	ID      string          `json:"id"`
	Manages []GroupKind     `json:"manages,omitempty"`
	Scope   ControllerScope `json:"scope,omitempty"`
}

// CheckController validates a controller declaration at path.
func CheckController(c *validate.Checker, value any, path string) ControllerDeclaration {
	record, ok := c.Record(value, path)
	if !ok {
		return ControllerDeclaration{}
	}
	c.RejectUnknown(record, ControllerFields, path)

	declaration := ControllerDeclaration{}
	declaration.ID, _ = c.RequireString(record, "id", path)
	declaration.Manages = checkManages(c, record, path)
	declaration.Scope = checkControllerScope(c, record, path)
	return declaration
}

// DeclareController builds a declaration, rejecting one the author must not hold.
func DeclareController(input any) (ControllerDeclaration, error) {
	c := &validate.Checker{}
	declaration := CheckController(c, input, "$")
	if len(c.Issues) > 0 {
		return ControllerDeclaration{}, &validate.InvalidError{
			What:   "controller declaration",
			Issues: c.Issues,
		}
	}
	return declaration, nil
}

func checkManages(c *validate.Checker, record map[string]any, path string) []GroupKind {
	items, ok := c.OptionalArray(record, "manages", path)
	if !ok {
		return nil
	}
	managesPath := validate.Join(path, "manages")
	managed := make([]GroupKind, 0, len(items))
	for index, item := range items {
		reference := CheckGroupKind(c, item, validate.Index(managesPath, index))
		if isCoreKind(reference.Kind) {
			c.Add(validate.Index(managesPath, index), validate.CodeInvalidValue,
				"%q is a kernel kind closed by KOSD-C-01; a controller serves declared classes only", reference.Kind)
		}
		managed = append(managed, reference)
	}
	return managed
}

func checkControllerScope(c *validate.Checker, record map[string]any, path string) ControllerScope {
	value, ok := c.Optional(record, "scope")
	if !ok {
		return ControllerScope{}
	}
	scopePath := validate.Join(path, "scope")
	declared, ok := c.Record(value, scopePath)
	if !ok {
		return ControllerScope{}
	}
	c.RejectUnknown(declared, ControllerScopeFields, scopePath)

	scope := ControllerScope{}
	scope.Groups = checkGroupList(c, declared, "groups", scopePath)
	scope.Namespaces, _ = c.OptionalStringArray(declared, "namespaces", scopePath)
	if items, ok := c.OptionalArray(declared, "kinds", scopePath); ok {
		kindsPath := validate.Join(scopePath, "kinds")
		scope.Kinds = make([]GroupKind, 0, len(items))
		for index, item := range items {
			reference := CheckGroupKind(c, item, validate.Index(kindsPath, index))
			if isCoreKind(reference.Kind) {
				c.Add(validate.Index(kindsPath, index), validate.CodeInvalidValue,
					"a controller may not write the kernel kind %q (KOSD-C-03, KOSD-C-20)", reference.Kind)
			}
			scope.Kinds = append(scope.Kinds, reference)
		}
	}
	return scope
}

// checkGroupList reads API groups that may name the core group as the empty
// string, the way the kernel does.
func checkGroupList(c *validate.Checker, record map[string]any, key, path string) []string {
	items, ok := c.OptionalArray(record, key, path)
	if !ok {
		return nil
	}
	listPath := validate.Join(path, key)
	groups := make([]string, 0, len(items))
	for index, item := range items {
		group, ok := item.(string)
		if !ok {
			c.Add(validate.Index(listPath, index), validate.CodeInvalidType, "expected a string")
			continue
		}
		groups = append(groups, group)
	}
	return groups
}
