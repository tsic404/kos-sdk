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
		"apiGroup": "files.kos.example",
		"version":  "v1alpha1",
		"kind":     "FileShare",
		"scope":    "Namespaced",
	})
	if err != nil {
		t.Fatalf("valid object class rejected: %v", err)
	}
	if declared.Kind != "FileShare" || declared.Scope != extension.ScopeNamespaced {
		t.Fatalf("unexpected declaration %+v", declared)
	}
}

func TestObjectClassRejectsKernelKind(t *testing.T) {
	_, err := extension.DeclareObjectClass(map[string]any{
		"apiGroup": "example.kos",
		"version":  "v1",
		"kind":     "Application",
		"scope":    "Namespaced",
	})

	var invalid *validate.InvalidError
	if !errors.As(err, &invalid) {
		t.Fatalf("expected an InvalidError, got %v", err)
	}
	if !hasIssue(invalid.Issues, "$.kind", validate.CodeInvalidValue) {
		t.Fatalf("expected an invalid-value issue for $.kind, got %v", invalid.Issues)
	}
}

func TestObjectClassRejectsUnknownScope(t *testing.T) {
	if _, err := extension.DeclareObjectClass(map[string]any{
		"apiGroup": "example.kos",
		"version":  "v1",
		"kind":     "Widget",
		"scope":    "Global",
	}); err == nil {
		t.Fatal("an unknown scope was accepted")
	}
}

func TestControllerRejectsKernelKind(t *testing.T) {
	_, err := extension.DeclareController(map[string]any{
		"name": "widget-controller",
		"writeScopes": []any{
			map[string]any{"apiGroup": "example.kos", "kind": "Volume", "verbs": []any{"update"}},
		},
	})

	var invalid *validate.InvalidError
	if !errors.As(err, &invalid) {
		t.Fatalf("expected an InvalidError, got %v", err)
	}
	if !hasIssue(invalid.Issues, "$.writeScopes[0].kind", validate.CodeInvalidValue) {
		t.Fatalf("expected an invalid-value issue for the kernel kind, got %v", invalid.Issues)
	}
}

func TestControllerRejectsUnknownVerb(t *testing.T) {
	if _, err := extension.DeclareController(map[string]any{
		"name": "widget-controller",
		"writeScopes": []any{
			map[string]any{"apiGroup": "example.kos", "kind": "Widget", "verbs": []any{"upsert"}},
		},
	}); err == nil {
		t.Fatal("an unknown verb was accepted")
	}
}

func TestControllerAcceptsExtensionAndDownstreamScopes(t *testing.T) {
	declared, err := extension.DeclareController(map[string]any{
		"name": "widget-controller",
		"writeScopes": []any{
			map[string]any{"apiGroup": "example.kos", "kind": "Widget", "verbs": []any{"get", "update"}},
			map[string]any{"apiGroup": "apps", "kind": "Deployment", "verbs": []any{"create", "patch"}},
		},
	})
	if err != nil {
		t.Fatalf("valid controller rejected: %v", err)
	}
	if len(declared.WriteScopes) != 2 {
		t.Fatalf("expected two write scopes, got %d", len(declared.WriteScopes))
	}
}
