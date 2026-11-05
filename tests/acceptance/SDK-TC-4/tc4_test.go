// Package sdktc4 verifies that every required manifest field and every window
// field is rejected when absent, that a permission name outside the fixed set
// is rejected, and that valid manifests pass (SDK-AC-4, SDK-TC-4, P0).
package sdktc4

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/tsic404/kos-sdk/manifest"
	"github.com/tsic404/kos-sdk/tests/acceptance/fixtures"
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

type manifestTarget int

const (
	targetExtension manifestTarget = iota
	targetApp
)

type missingFieldCase struct {
	name   string
	target manifestTarget
	path   string
	remove func() any
}

func withoutKey(key string) any {
	extension := fixtures.MinimalExtension()
	delete(extension, key)
	return extension
}

func withoutAppKey(key string) any {
	app := fixtures.MinimalApp()
	delete(app, key)
	return app
}

func withoutWindowKey(key string) any {
	app := fixtures.MinimalApp()
	delete(app["window"].(map[string]any), key)
	return app
}

// missingFieldCases covers all 23 required sites: 9 extension fields, 7 App
// fields and 7 window fields (KOSD-C-18, KOSD-C-73).
func missingFieldCases() []missingFieldCase {
	return []missingFieldCase{
		{"extension id", targetExtension, "$.id", func() any { return withoutKey("id") }},
		{"extension version", targetExtension, "$.version", func() any { return withoutKey("version") }},
		{"extension frameworkVersion", targetExtension, "$.frameworkVersion", func() any { return withoutKey("frameworkVersion") }},
		{"extension contributions", targetExtension, "$.contributions", func() any { return withoutKey("contributions") }},
		{"extension dependencies", targetExtension, "$.dependencies", func() any { return withoutKey("dependencies") }},
		{"extension permissions", targetExtension, "$.permissions", func() any { return withoutKey("permissions") }},
		{"extension config", targetExtension, "$.config", func() any { return withoutKey("config") }},
		{"extension status", targetExtension, "$.status", func() any { return withoutKey("status") }},
		{"extension uninstallRetention", targetExtension, "$.uninstallRetention", func() any { return withoutKey("uninstallRetention") }},
		{"app id", targetApp, "$.id", func() any { return withoutAppKey("id") }},
		{"app name", targetApp, "$.name", func() any { return withoutAppKey("name") }},
		{"app version", targetApp, "$.version", func() any { return withoutAppKey("version") }},
		{"app icon", targetApp, "$.icon", func() any { return withoutAppKey("icon") }},
		{"app entry", targetApp, "$.entry", func() any { return withoutAppKey("entry") }},
		{"app window", targetApp, "$.window", func() any { return withoutAppKey("window") }},
		{"app permissions", targetApp, "$.permissions", func() any { return withoutAppKey("permissions") }},
		{"app window width", targetApp, "$.window.width", func() any { return withoutWindowKey("width") }},
		{"app window height", targetApp, "$.window.height", func() any { return withoutWindowKey("height") }},
		{"app window minWidth", targetApp, "$.window.minWidth", func() any { return withoutWindowKey("minWidth") }},
		{"app window minHeight", targetApp, "$.window.minHeight", func() any { return withoutWindowKey("minHeight") }},
		{"app window resizable", targetApp, "$.window.resizable", func() any { return withoutWindowKey("resizable") }},
		{"app window mode", targetApp, "$.window.mode", func() any { return withoutWindowKey("mode") }},
		{"app window defaultState", targetApp, "$.window.defaultState", func() any { return withoutWindowKey("defaultState") }},
	}
}

func TestMissingRequiredFieldIsRejected(t *testing.T) {
	for _, testCase := range missingFieldCases() {
		t.Run(testCase.name, func(t *testing.T) {
			input := testCase.remove()

			var issues []validate.Issue
			var produced bool
			switch testCase.target {
			case targetExtension:
				result := manifest.ValidateExtensionManifest(input)
				issues, produced = result.Issues, result.Manifest != nil
			case targetApp:
				result := manifest.ValidateAppManifest(input)
				issues, produced = result.Issues, result.Manifest != nil
			}

			if produced {
				t.Fatal("a rejected manifest must not yield an artifact")
			}
			if !hasIssue(issues, testCase.path, validate.CodeMissingField) {
				t.Fatalf("expected a missing-field issue at %s, got %v", testCase.path, issues)
			}
		})
	}
}

func TestAppPermissionOutsideFixedSetIsRejected(t *testing.T) {
	app := fixtures.MinimalApp()
	app["permissions"] = []any{"clipboard-read", "camera"}

	result := manifest.ValidateAppManifest(app)

	if result.OK() {
		t.Fatal("an App requesting an undeclared permission was accepted")
	}
	if !hasIssue(result.Issues, "$.permissions[1]", validate.CodePermissionNotInSet) {
		t.Fatalf("expected a permission-not-in-set issue at $.permissions[1], got %v", result.Issues)
	}
	for _, issue := range result.Issues {
		if issue.Code == validate.CodePermissionNotInSet && !strings.Contains(issue.Message, "camera") {
			t.Fatalf("issue does not name the offending permission: %v", issue)
		}
	}
}

func TestEmbeddedAppWithUndeclaredPermissionIsRejected(t *testing.T) {
	extension := fixtures.MinimalExtension()
	app := fixtures.MinimalApp()
	app["permissions"] = []any{"camera"}
	extension["contributions"].(map[string]any)["apps"] = []any{app}

	result := manifest.ValidateExtensionManifest(extension)

	if result.OK() {
		t.Fatal("an extension carrying an undeclared permission was accepted")
	}
	if !hasIssue(result.Issues, "$.contributions.apps[0].permissions[0]", validate.CodePermissionNotInSet) {
		t.Fatalf("expected the embedded App to be rejected at its own path, got %v", result.Issues)
	}
}

func TestValidManifestsPass(t *testing.T) {
	extension := manifest.ValidateExtensionManifest(fixtures.MinimalExtension())
	if !extension.OK() {
		t.Fatalf("valid extension rejected: %v", extension.Issues)
	}
	if extension.Manifest.ID != "kos.example.files" {
		t.Fatalf("unexpected extension id %q", extension.Manifest.ID)
	}
	if len(extension.Manifest.Contributions.Apps) != 1 {
		t.Fatalf("expected one embedded App, got %d", len(extension.Manifest.Contributions.Apps))
	}

	app := manifest.ValidateAppManifest(fixtures.MinimalApp())
	if !app.OK() {
		t.Fatalf("valid App rejected: %v", app.Issues)
	}
	if app.Manifest.Entry != "https://files.apps.kos.example/" {
		t.Fatalf("unexpected entry %q", app.Manifest.Entry)
	}
}

func TestNullPermissionsIsTheEmptySet(t *testing.T) {
	app := fixtures.MinimalApp()
	app["permissions"] = nil

	result := manifest.ValidateAppManifest(app)
	if !result.OK() {
		t.Fatalf("an App declaring null permissions was rejected: %v", result.Issues)
	}
	if result.Manifest.Permissions == nil {
		t.Fatal("null permissions must produce an empty set, not a nil slice")
	}

	encoded, err := json.Marshal(result.Manifest)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	var produced map[string]any
	if err := json.Unmarshal(encoded, &produced); err != nil {
		t.Fatalf("unmarshal manifest: %v", err)
	}
	permissions, ok := produced["permissions"].([]any)
	if !ok || len(permissions) != 0 {
		t.Fatalf("produced permissions = %v, want []", produced["permissions"])
	}
}
