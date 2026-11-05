package manifest_test

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

func TestPresentNullIsNotAMissingField(t *testing.T) {
	app := fixtures.MinimalApp()
	app["window"] = nil

	result := manifest.ValidateAppManifest(app)
	if result.OK() {
		t.Fatal("an App with a null window was accepted")
	}
	if !hasIssue(result.Issues, "$.window", validate.CodeInvalidType) {
		t.Fatalf("expected an invalid-type issue for $.window, got %v", result.Issues)
	}
	if hasIssue(result.Issues, "$.window", validate.CodeMissingField) {
		t.Fatalf("a present null must not be reported as missing: %v", result.Issues)
	}
}

func TestOptionalFieldPresentNullIsInvalid(t *testing.T) {
	app := fixtures.MinimalApp()
	app["deepLinks"] = nil

	result := manifest.ValidateAppManifest(app)
	if result.OK() {
		t.Fatalf("a null deepLinks was accepted: %v", result.Issues)
	}
	if !hasIssue(result.Issues, "$.deepLinks", validate.CodeInvalidType) {
		t.Fatalf("expected an invalid-type issue for $.deepLinks, got %v", result.Issues)
	}
}

func TestEntryWithCredentialsIsRejected(t *testing.T) {
	app := fixtures.MinimalApp()
	app["entry"] = "https://user:password@app.kos.example/"

	result := manifest.ValidateAppManifest(app)
	if result.OK() {
		t.Fatal("an entry carrying credentials was accepted")
	}
	if !hasIssue(result.Issues, "$.entry", validate.CodeInvalidValue) {
		t.Fatalf("expected an invalid-value issue for $.entry, got %v", result.Issues)
	}
}

func TestNumericFieldsAcceptGoNumericTypes(t *testing.T) {
	forms := map[string]any{
		"int":         1024,
		"int64":       int64(1024),
		"float32":     float32(1024),
		"float64":     float64(1024),
		"json.Number": json.Number("1024"),
	}
	for name, width := range forms {
		t.Run(name, func(t *testing.T) {
			app := fixtures.MinimalApp()
			app["window"].(map[string]any)["width"] = width

			result := manifest.ValidateAppManifest(app)
			if !result.OK() {
				t.Fatalf("width as %s was rejected: %v", name, result.Issues)
			}
			if result.Manifest.Window.Width != 1024 {
				t.Fatalf("width = %v, want 1024", result.Manifest.Window.Width)
			}
		})
	}
}

func TestNonNumericWidthReportsItsType(t *testing.T) {
	app := fixtures.MinimalApp()
	app["window"].(map[string]any)["width"] = "1024"

	result := manifest.ValidateAppManifest(app)
	if result.OK() {
		t.Fatal("a string width was accepted")
	}
	for _, issue := range result.Issues {
		if issue.Path == "$.window.width" {
			if issue.Code != validate.CodeInvalidType || !strings.Contains(issue.Message, "string") {
				t.Fatalf("width issue does not name the offending type: %v", issue)
			}
			return
		}
	}
	t.Fatalf("expected an issue for $.window.width, got %v", result.Issues)
}
