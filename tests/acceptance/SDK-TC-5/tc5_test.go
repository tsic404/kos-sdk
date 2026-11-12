// Package sdktc5 verifies that a produced manifest carries exactly the fields
// the framework declaration defines — no SDK field and no filled-in default
// (SDK-AC-4, SDK-TC-5, P1).
package sdktc5

import (
	"encoding/json"
	"slices"
	"sort"
	"testing"

	"github.com/tsic404/kos-sdk/manifest"
	"github.com/tsic404/kos-sdk/tests/acceptance/fixtures"
	"github.com/tsic404/kos-sdk/validate"
)

// frameworkDeclarationFields is the declaration's field set as the framework
// fixes it (KOSD-C-18); the SDK may neither add nor drop an entry.
var frameworkDeclarationFields = []string{
	"identifier",
	"version",
	"frameworkVersion",
	"apiGroup",
	"contributions",
	"dependencies",
	"permissions",
	"configuration",
	"retention",
}

func jsonObject(t *testing.T, value any) map[string]any {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal manifest: %v", err)
	}
	return decoded
}

func keys(record map[string]any) []string {
	found := make([]string, 0, len(record))
	for key := range record {
		found = append(found, key)
	}
	sort.Strings(found)
	return found
}

func sortedCopy(values []string) []string {
	copied := append([]string(nil), values...)
	sort.Strings(copied)
	return copied
}

func hasIssue(issues []validate.Issue, path string, code validate.Code) bool {
	for _, issue := range issues {
		if issue.Path == path && issue.Code == code {
			return true
		}
	}
	return false
}

func TestExtensionManifestFieldSet(t *testing.T) {
	result := manifest.ValidateExtensionManifest(fixtures.MinimalExtension())
	if !result.OK() {
		t.Fatalf("valid extension rejected: %v", result.Issues)
	}

	produced := jsonObject(t, result.Manifest)
	for _, key := range keys(produced) {
		if !slices.Contains(frameworkDeclarationFields, key) {
			t.Fatalf("the manifest carries the field %q outside the framework's set", key)
		}
	}
	for _, key := range []string{"identifier", "version", "frameworkVersion", "contributions", "dependencies", "permissions", "configuration", "retention"} {
		if _, ok := produced[key]; !ok {
			t.Fatalf("required field %q is missing from the produced manifest", key)
		}
	}

	contributions, ok := produced["contributions"].(map[string]any)
	if !ok {
		t.Fatalf("contributions is not an object: %T", produced["contributions"])
	}
	for _, key := range keys(contributions) {
		if !slices.Contains(manifest.ContributionClasses, key) {
			t.Fatalf("contributions carries an undefined class %q", key)
		}
	}
	for _, key := range []string{"objectClasses", "controllers", "capabilities", "pages"} {
		if _, ok := contributions[key]; !ok {
			t.Fatalf("declared contribution class %q is missing from the produced manifest", key)
		}
	}
}

// TestManifestMatchesTheFrameworkDeclarationShape checks every nested block the
// SDK writes against the framework's field set, so a re-added SDK field fails
// here rather than at the kernel reader.
func TestManifestMatchesTheFrameworkDeclarationShape(t *testing.T) {
	result := manifest.ValidateExtensionManifest(fixtures.MinimalExtension())
	if !result.OK() {
		t.Fatalf("valid extension rejected: %v", result.Issues)
	}
	produced := jsonObject(t, result.Manifest)

	contributions := produced["contributions"].(map[string]any)
	assertFieldsWithin(t, "object class", firstEntry(t, contributions["objectClasses"]),
		[]string{"group", "kind", "resource", "versions", "namespaced", "references"})
	assertFieldsWithin(t, "controller", firstEntry(t, contributions["controllers"]),
		[]string{"id", "manages", "scope"})

	permissions := produced["permissions"].([]any)
	permission := permissions[0].(map[string]any)
	assertFieldsWithin(t, "permission", permission,
		[]string{"contribution", "readScope", "requiredRole", "objectWriters"})
	readScope := permission["readScope"].(map[string]any)
	assertFieldsWithin(t, "read scope", readScope, []string{"groups", "kinds"})
	for _, key := range keys(readScope) {
		if key == "apiGroup" || key == "resources" {
			t.Fatalf("read scope carries the SDK shape %q, not the framework's groups and kinds", key)
		}
	}
	kind := readScope["kinds"].([]any)[0].(map[string]any)
	assertFieldsWithin(t, "read scope kind", kind, []string{"group", "kind"})

	assertFieldsWithin(t, "configuration", produced["configuration"].(map[string]any),
		[]string{"schema", "statusWriters", "conditions"})
	assertFieldsWithin(t, "dependencies", produced["dependencies"].(map[string]any),
		[]string{"capabilities", "components"})
	retention := produced["retention"].([]any)
	assertFieldsWithin(t, "retention", retention[0].(map[string]any),
		[]string{"resource", "keep", "externalReference"})

	for _, key := range []string{"id", "config", "status", "uninstallRetention"} {
		if _, ok := produced[key]; ok {
			t.Fatalf("the manifest carries the SDK field %q the framework does not define", key)
		}
	}
}

func firstEntry(t *testing.T, value any) map[string]any {
	t.Helper()
	entries, ok := value.([]any)
	if !ok || len(entries) == 0 {
		t.Fatalf("expected at least one entry, got %T", value)
	}
	entry, ok := entries[0].(map[string]any)
	if !ok {
		t.Fatalf("entry is not an object: %T", entries[0])
	}
	return entry
}

func assertFieldsWithin(t *testing.T, what string, record map[string]any, allowed []string) {
	t.Helper()
	for _, key := range keys(record) {
		if !slices.Contains(allowed, key) {
			t.Fatalf("%s carries the field %q outside the framework's set %v", what, key, allowed)
		}
	}
}

func TestAppManifestFieldSet(t *testing.T) {
	result := manifest.ValidateAppManifest(fixtures.MinimalApp())
	if !result.OK() {
		t.Fatalf("valid App rejected: %v", result.Issues)
	}

	produced := jsonObject(t, result.Manifest)
	if !slices.Equal(keys(produced), sortedCopy(manifest.AppManifestFields)) {
		t.Fatalf("App field set = %v, want %v", keys(produced), manifest.AppManifestFields)
	}

	window, ok := produced["window"].(map[string]any)
	if !ok {
		t.Fatalf("window is not an object: %T", produced["window"])
	}
	if !slices.Equal(keys(window), sortedCopy(manifest.AppWindowFields)) {
		t.Fatalf("window field set = %v, want %v", keys(window), manifest.AppWindowFields)
	}
}

func TestAbsentOptionalFieldsAreNotFilledIn(t *testing.T) {
	app := fixtures.MinimalApp()
	delete(app, "backend")
	delete(app, "deepLinks")

	result := manifest.ValidateAppManifest(app)
	if !result.OK() {
		t.Fatalf("valid UI-only App rejected: %v", result.Issues)
	}

	produced := jsonObject(t, result.Manifest)
	required := []string{"id", "name", "version", "icon", "entry", "window", "permissions"}
	if !slices.Equal(keys(produced), sortedCopy(required)) {
		t.Fatalf("App field set = %v, want %v", keys(produced), required)
	}
}

func TestFieldsOutsideTheContractAreRejected(t *testing.T) {
	app := fixtures.MinimalApp()
	app["sdkVersion"] = "0.1.0"
	appResult := manifest.ValidateAppManifest(app)
	if appResult.OK() {
		t.Fatal("an App carrying an SDK-specific field was accepted")
	}
	if !hasIssue(appResult.Issues, "$.sdkVersion", validate.CodeUnknownField) {
		t.Fatalf("expected an unknown-field issue for $.sdkVersion, got %v", appResult.Issues)
	}

	extension := fixtures.MinimalExtension()
	extension["sdkVersion"] = "0.1.0"
	extensionResult := manifest.ValidateExtensionManifest(extension)
	if extensionResult.OK() {
		t.Fatal("an extension carrying an SDK-specific field was accepted")
	}
	if !hasIssue(extensionResult.Issues, "$.sdkVersion", validate.CodeUnknownField) {
		t.Fatalf("expected an unknown-field issue for $.sdkVersion, got %v", extensionResult.Issues)
	}

	extension = fixtures.MinimalExtension()
	extension["contributions"].(map[string]any)["widgets"] = []any{}
	classResult := manifest.ValidateExtensionManifest(extension)
	if classResult.OK() {
		t.Fatal("an undefined contribution class was accepted")
	}
	if !hasIssue(classResult.Issues, "$.contributions.widgets", validate.CodeUnknownField) {
		t.Fatalf("expected an unknown-field issue for $.contributions.widgets, got %v", classResult.Issues)
	}
}

func TestSDKFieldsAreRejected(t *testing.T) {
	extension := fixtures.MinimalExtension()
	extension["config"] = map[string]any{}
	extension["status"] = map[string]any{}
	extension["uninstallRetention"] = map[string]any{}

	result := manifest.ValidateExtensionManifest(extension)
	if result.OK() {
		t.Fatal("a declaration carrying the SDK's own fields was accepted")
	}
	for _, path := range []string{"$.config", "$.status", "$.uninstallRetention"} {
		if !hasIssue(result.Issues, path, validate.CodeUnknownField) {
			t.Fatalf("expected an unknown-field issue at %s, got %v", path, result.Issues)
		}
	}
}
