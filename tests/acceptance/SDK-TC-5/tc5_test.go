// Package sdktc5 verifies that a produced manifest carries exactly the fields
// the framework defines — no SDK field and no filled-in default (SDK-AC-4,
// SDK-TC-5, P1).
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
	if !slices.Equal(keys(produced), sortedCopy(manifest.ExtensionManifestFields)) {
		t.Fatalf("extension field set = %v, want %v", keys(produced), manifest.ExtensionManifestFields)
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
	for _, key := range []string{"objectClasses", "controllers", "apps"} {
		if _, ok := contributions[key]; !ok {
			t.Fatalf("declared contribution class %q is missing from the produced manifest", key)
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
