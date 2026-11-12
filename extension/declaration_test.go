package extension_test

import (
	"errors"
	"testing"

	"github.com/tsic404/kos-sdk/extension"
	"github.com/tsic404/kos-sdk/validate"
)

func hasIssue(issues []validate.Issue, path string, code validate.Code) bool {
	for _, issue := range issues {
		if issue.Path == path && issue.Code == code {
			return true
		}
	}
	return false
}

func TestDeclareObjectClass(t *testing.T) {
	declared, err := extension.DeclareObjectClass(map[string]any{
		"group":      "files.kos.example",
		"kind":       "FileShare",
		"resource":   "fileshares",
		"versions":   []any{"v1alpha1"},
		"namespaced": true,
	})
	if err != nil {
		t.Fatalf("valid object class rejected: %v", err)
	}
	if declared.Kind != "FileShare" || !declared.Namespaced || len(declared.Versions) != 1 {
		t.Fatalf("unexpected declaration %+v", declared)
	}
}

func TestObjectClassRejectsKernelKind(t *testing.T) {
	_, err := extension.DeclareObjectClass(map[string]any{
		"group":    "example.kos",
		"kind":     "Application",
		"resource": "applications",
		"versions": []any{"v1"},
	})

	var invalid *validate.InvalidError
	if !errors.As(err, &invalid) {
		t.Fatalf("expected an InvalidError, got %v", err)
	}
	if !hasIssue(invalid.Issues, "$.kind", validate.CodeInvalidValue) {
		t.Fatalf("expected an invalid-value issue for $.kind, got %v", invalid.Issues)
	}
}

func TestObjectClassRequiresResourceAndVersions(t *testing.T) {
	_, err := extension.DeclareObjectClass(map[string]any{
		"group": "example.kos",
		"kind":  "Widget",
	})

	var invalid *validate.InvalidError
	if !errors.As(err, &invalid) {
		t.Fatalf("expected an InvalidError, got %v", err)
	}
	if !hasIssue(invalid.Issues, "$.resource", validate.CodeMissingField) {
		t.Fatalf("expected a missing-field issue for $.resource, got %v", invalid.Issues)
	}
	if !hasIssue(invalid.Issues, "$.versions", validate.CodeMissingField) {
		t.Fatalf("expected a missing-field issue for $.versions, got %v", invalid.Issues)
	}
}

func TestGroupKindAllowsTheCoreGroup(t *testing.T) {
	c := &validate.Checker{}
	reference := extension.CheckGroupKind(c, map[string]any{"kind": "Event"}, "$")
	if len(c.Issues) > 0 {
		t.Fatalf("a core-group kind was rejected: %v", c.Issues)
	}
	if reference.Group != "" || reference.Kind != "Event" {
		t.Fatalf("unexpected class reference %+v", reference)
	}
}

func TestControllerRejectsKernelKind(t *testing.T) {
	_, err := extension.DeclareController(map[string]any{
		"id": "widget-controller",
		"scope": map[string]any{
			"kinds": []any{map[string]any{"kind": "Volume"}},
		},
	})

	var invalid *validate.InvalidError
	if !errors.As(err, &invalid) {
		t.Fatalf("expected an InvalidError, got %v", err)
	}
	if !hasIssue(invalid.Issues, "$.scope.kinds[0]", validate.CodeInvalidValue) {
		t.Fatalf("expected an invalid-value issue for the kernel kind, got %v", invalid.Issues)
	}
}

func TestControllerAcceptsDeclaredClassesAndDownstreamScope(t *testing.T) {
	declared, err := extension.DeclareController(map[string]any{
		"id":      "widget-controller",
		"manages": []any{map[string]any{"group": "example.kos", "kind": "Widget"}},
		"scope": map[string]any{
			"groups":     []any{"example.kos", "apps"},
			"kinds":      []any{map[string]any{"group": "apps", "kind": "Deployment"}},
			"namespaces": []any{"widgets"},
		},
	})
	if err != nil {
		t.Fatalf("valid controller rejected: %v", err)
	}
	if len(declared.Manages) != 1 || len(declared.Scope.Kinds) != 1 || len(declared.Scope.Namespaces) != 1 {
		t.Fatalf("unexpected declaration %+v", declared)
	}
}
