package sdktc3

import (
	"encoding/json"
	"slices"
	"testing"
	"time"

	"github.com/tsic404/kos-sdk/app"
	"github.com/tsic404/kos-sdk/manifest"
	"github.com/tsic404/kos-sdk/tests/acceptance/fixtures"
)

// consoleInitJSON is the init message the delivered console posts. Its shape is
// the one in kos-dashboard pkg/apphost/host.go: an opaque token string, a string
// theme, the declared capabilities and the expiry at the top level. The browser
// side of the console (pkg/bff/assets/shell/apphost.js) only accepts a string
// token. The file permission is the console's own name for it.
const consoleInitJSON = `{"type":"init","token":"opaque-app-token","theme":"system","capabilities":["file-picker","external-link"],"expiresAt":"2026-11-05T12:05:00Z"}`

// consoleIsInit mirrors the console's isInit check: the message names itself and
// carries a non-empty token string.
func consoleIsInit(wire map[string]any) bool {
	if wire["type"] != "init" {
		return false
	}
	token, ok := wire["token"].(string)
	return ok && token != ""
}

func TestConsoleShapedInitIsAccepted(t *testing.T) {
	var data any
	if err := json.Unmarshal([]byte(consoleInitJSON), &data); err != nil {
		t.Fatalf("read the console's init: %v", err)
	}
	session, err := newWindow(t).Accept(app.Event{
		Origin: consoleOrigin,
		Source: parentSource,
		Data:   data,
	})
	if err != nil {
		t.Fatalf("the console's init was rejected: %v", err)
	}
	if session.Token != "opaque-app-token" {
		t.Fatalf("session token = %q, want the console's opaque value", session.Token)
	}
	if session.Theme != "system" {
		t.Fatalf("session theme = %q, want system", session.Theme)
	}
	if !session.ExpiresAt.Equal(time.Date(2026, 11, 5, 12, 5, 0, 0, time.UTC)) {
		t.Fatalf("session expiry = %s, want the console's expiresAt", session.ExpiresAt)
	}
	if !slices.Equal(session.Capabilities, []manifest.Permission{manifest.PermissionFilePicker, manifest.PermissionExternalLink}) {
		t.Fatalf("session capabilities = %v, want [file-picker external-link]", session.Capabilities)
	}
	if !session.Allows(manifest.PermissionFilePicker) {
		t.Fatal("the console's file permission is not available to the app")
	}
}

// TestManifestUsesTheConsoleFilePermission proves the input side uses the same
// wire name as the handshake output: an App declaring the console's
// `file-picker` is valid, and the old `file-selection` name is not a permission
// the console could ever grant.
func TestManifestUsesTheConsoleFilePermission(t *testing.T) {
	declaration := fixtures.MinimalApp()
	declaration["permissions"] = []any{"file-picker"}
	accepted := manifest.ValidateAppManifest(declaration)
	if !accepted.OK() {
		t.Fatalf("an App declaring file-picker was rejected: %v", accepted.Issues)
	}
	if !slices.Equal(accepted.Manifest.Permissions, []manifest.Permission{manifest.PermissionFilePicker}) {
		t.Fatalf("packaged permissions = %v, want file-picker", accepted.Manifest.Permissions)
	}

	declaration["permissions"] = []any{"file-selection"}
	if manifest.ValidateAppManifest(declaration).OK() {
		t.Fatal("an App declaring file-selection was accepted; the console never grants that name")
	}
}

func TestHandshakeAnswerUsesConsoleWire(t *testing.T) {
	declared := validApp(t).Permissions
	granted := []manifest.Permission{manifest.PermissionExternalLink}
	host := newHost(t, tokenFor(granted...), declared, granted)

	delivery, ok := host.Receive(app.Event{
		Origin: appOrigin,
		Source: windowSource,
		Data:   newWindow(t).Ready(),
	})
	if !ok {
		t.Fatal("a ready from the embedded window was ignored")
	}

	encoded, err := json.Marshal(delivery.Message)
	if err != nil {
		t.Fatalf("marshal the handshake answer: %v", err)
	}
	var wire map[string]any
	if err := json.Unmarshal(encoded, &wire); err != nil {
		t.Fatalf("read the handshake answer: %v", err)
	}
	if !consoleIsInit(wire) {
		t.Fatalf("the console's isInit refuses the handshake answer: %s", encoded)
	}
	if _, ok := wire["token"].(string); !ok {
		t.Fatalf("token on the wire is %T, want the console's opaque string", wire["token"])
	}
	if _, ok := wire["theme"].(string); !ok {
		t.Fatalf("theme on the wire is %T, want the console's string", wire["theme"])
	}
	if _, ok := wire["expiresAt"].(string); !ok {
		t.Fatalf("expiresAt on the wire is %T, want the console's top-level timestamp", wire["expiresAt"])
	}
	keys := make([]string, 0, len(wire))
	for key := range wire {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	want := []string{"capabilities", "expiresAt", "theme", "token", "type"}
	if !slices.Equal(keys, want) {
		t.Fatalf("init field set = %v, want the console's %v", keys, want)
	}
}
