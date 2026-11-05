package extension

import (
	"github.com/tsic404/kos-sdk/validate"
)

// WriteVerbs are the verbs a controller declaration may request.
var WriteVerbs = []string{"get", "list", "watch", "create", "update", "patch", "delete"}

// ControllerFields is the controller declaration's closed field set.
var ControllerFields = []string{"name", "writeScopes"}

// ControllerScopeFields is one write scope's closed field set.
var ControllerScopeFields = []string{"apiGroup", "kind", "verbs"}

// ControllerWriteScope is one declared write scope: extension objects of a kind
// or granted downstream objects, limited to the listed verbs.
type ControllerWriteScope struct {
	APIGroup string   `json:"apiGroup"`
	Kind     string   `json:"kind"`
	Verbs    []string `json:"verbs"`
}

// ControllerDeclaration declares an extension controller and its write scope.
// The controller never writes kernel objects (KOSD-C-03, KOSD-C-20).
type ControllerDeclaration struct {
	Name        string                 `json:"name"`
	WriteScopes []ControllerWriteScope `json:"writeScopes"`
}

// CheckController validates a controller declaration at path.
func CheckController(c *validate.Checker, value any, path string) ControllerDeclaration {
	record, ok := c.Record(value, path)
	if !ok {
		return ControllerDeclaration{}
	}
	c.RejectUnknown(record, ControllerFields, path)

	declaration := ControllerDeclaration{}
	declaration.Name, _ = c.RequireString(record, "name", path)
	declaration.WriteScopes = checkWriteScopes(c, record, path)
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

func checkWriteScopes(c *validate.Checker, record map[string]any, path string) []ControllerWriteScope {
	items, ok := c.RequireArray(record, "writeScopes", path)
	if !ok {
		return nil
	}
	scopesPath := validate.Join(path, "writeScopes")
	scopes := make([]ControllerWriteScope, 0, len(items))
	for index, item := range items {
		scope, valid := checkWriteScope(c, item, validate.Index(scopesPath, index))
		if !valid {
			continue
		}
		scopes = append(scopes, scope)
	}
	return scopes
}

func checkWriteScope(c *validate.Checker, value any, path string) (ControllerWriteScope, bool) {
	record, ok := c.Record(value, path)
	if !ok {
		return ControllerWriteScope{}, false
	}
	c.RejectUnknown(record, ControllerScopeFields, path)

	scope := ControllerWriteScope{}
	scope.APIGroup, _ = c.RequireString(record, "apiGroup", path)
	scope.Kind = checkScopeKind(c, record, path)
	scope.Verbs = checkVerbs(c, record, path)
	return scope, true
}

func checkScopeKind(c *validate.Checker, record map[string]any, path string) string {
	kind, ok := c.RequireString(record, "kind", path)
	if !ok {
		return ""
	}
	if isCoreKind(kind) {
		c.Add(validate.Join(path, "kind"), validate.CodeInvalidValue,
			"a controller may not write the kernel kind %q (KOSD-C-03, KOSD-C-20)", kind)
		return ""
	}
	return kind
}

func checkVerbs(c *validate.Checker, record map[string]any, path string) []string {
	items, ok := c.RequireArray(record, "verbs", path)
	if !ok {
		return nil
	}
	verbsPath := validate.Join(path, "verbs")
	verbs := make([]string, 0, len(items))
	for index, item := range items {
		verb, ok := c.String(item, validate.Index(verbsPath, index))
		if !ok {
			continue
		}
		if !isWriteVerb(verb) {
			c.Add(validate.Index(verbsPath, index), validate.CodeInvalidValue,
				"unknown verb %q; expected one of %v", verb, WriteVerbs)
			continue
		}
		verbs = append(verbs, verb)
	}
	return verbs
}

func isWriteVerb(verb string) bool {
	for _, allowed := range WriteVerbs {
		if allowed == verb {
			return true
		}
	}
	return false
}
