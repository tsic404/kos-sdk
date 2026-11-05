// Package sdktc3 verifies the application interface: a valid App completes the
// KOSD-C-75 handshake, an App whose permission name is outside the fixed set is
// rejected before an artifact exists, and the SDK's exported surface has no
// entry point that reads a console session (SDK-AC-3, SDK-TC-3, P1).
package sdktc3

import (
	"encoding/json"
	"slices"
	"testing"
	"time"

	"github.com/tsic404/kos-sdk/app"
	"github.com/tsic404/kos-sdk/manifest"
	"github.com/tsic404/kos-sdk/tests/acceptance/fixtures"
	"github.com/tsic404/kos-sdk/validate"
)

const (
	appOrigin     = "https://files.apps.kos.example"
	consoleOrigin = "https://console.kos.example"
	windowSource  = app.Source("files-window")
	parentSource  = app.Source("console-window")
)

var now = time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)

func clock() time.Time { return now }

// validApp returns the fixture App after the manifest check, so the handshake
// below runs on the permission list the App actually declares.
func validApp(t *testing.T) manifest.AppManifest {
	t.Helper()
	result := manifest.ValidateAppManifest(fixtures.MinimalApp())
	if !result.OK() {
		t.Fatalf("fixture App rejected: %v", result.Issues)
	}
	return *result.Manifest
}

func tokenFor(permissions ...manifest.Permission) app.Token {
	return app.Token{
		Value:       "app-token-1",
		Account:     "account-1",
		AppID:       "files",
		Window:      "window-1",
		Permissions: permissions,
		ExpiresAt:   now.Add(5 * time.Minute),
	}
}

func theme() json.RawMessage { return json.RawMessage(`{"mode":"dark"}`) }

func newHost(t *testing.T, token app.Token, declared, granted []manifest.Permission) *app.Host {
	t.Helper()
	host, err := app.NewHost(app.HostConfig{
		AppID:    "files",
		Entry:    appOrigin + "/",
		Source:   windowSource,
		Window:   "window-1",
		Token:    token,
		Theme:    theme(),
		Declared: declared,
		Granted:  granted,
		Now:      clock,
	})
	if err != nil {
		t.Fatalf("host rejected a valid handshake: %v", err)
	}
	return host
}

func newWindow(t *testing.T) *app.Window {
	t.Helper()
	window, err := app.NewWindow(app.WindowConfig{
		AppID:         "files",
		ConsoleOrigin: consoleOrigin + "/apps/files",
		Parent:        parentSource,
		Now:           clock,
	})
	if err != nil {
		t.Fatalf("window rejected a valid handshake: %v", err)
	}
	return window
}

func jsonKeys(t *testing.T, value any) []string {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	keys := make([]string, 0, len(decoded))
	for key := range decoded {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

func TestValidAppCompletesHandshake(t *testing.T) {
	declared := validApp(t).Permissions
	granted := []manifest.Permission{manifest.PermissionExternalLink}

	host := newHost(t, tokenFor(granted...), declared, granted)
	window := newWindow(t)

	delivery, ok := host.Receive(app.Event{
		Origin: appOrigin,
		Source: windowSource,
		Data:   window.Ready(),
	})
	if !ok {
		t.Fatal("a ready from the embedded window was ignored")
	}
	if delivery.TargetOrigin != appOrigin {
		t.Fatalf("target origin = %q, want %q", delivery.TargetOrigin, appOrigin)
	}
	if delivery.Message.Type != app.TypeInit {
		t.Fatalf("handshake answer type = %q, want %q", delivery.Message.Type, app.TypeInit)
	}
	if !slices.Equal(delivery.Message.Capabilities, granted) {
		t.Fatalf("capabilities = %v, want %v", delivery.Message.Capabilities, granted)
	}
	if string(delivery.Message.Theme) != string(theme()) {
		t.Fatalf("theme = %s, want %s", delivery.Message.Theme, theme())
	}

	// The init names the token, the theme and the capabilities, and nothing
	// else: there is no field a console session could ride along in.
	if got := jsonKeys(t, delivery.Message); !slices.Equal(got, []string{"capabilities", "theme", "token", "type"}) {
		t.Fatalf("init field set = %v", got)
	}
	if got := jsonKeys(t, delivery.Message.Token); !slices.Equal(got,
		[]string{"account", "appId", "expiresAt", "permissions", "value", "window"}) {
		t.Fatalf("token field set = %v", got)
	}

	session, err := window.Accept(app.Event{
		Origin: consoleOrigin,
		Source: parentSource,
		Data:   delivery.Message,
	})
	if err != nil {
		t.Fatalf("valid init rejected: %v", err)
	}
	if session.Token.Value != "app-token-1" {
		t.Fatalf("session token = %q", session.Token.Value)
	}
	if !session.Allows(manifest.PermissionExternalLink) {
		t.Fatal("a granted permission is not available to the app")
	}
	if session.Allows(manifest.PermissionNotifications) {
		t.Fatal("an undeclared permission is available to the app")
	}

	// The token crosses the handshake once.
	if _, deliveredAgain := host.Receive(app.Event{Origin: appOrigin, Source: windowSource, Data: window.Ready()}); deliveredAgain {
		t.Fatal("a second ready was answered; the token must be delivered once")
	}
	if _, err := window.Accept(app.Event{Origin: consoleOrigin, Source: parentSource, Data: delivery.Message}); err == nil {
		t.Fatal("a second init was accepted")
	}
}

func TestUndeclaredPermissionIsNotAvailable(t *testing.T) {
	declared := validApp(t).Permissions
	granted := []manifest.Permission{manifest.PermissionClipboardRead, manifest.PermissionNotifications}

	// notifications is not declared by the App manifest, so it is not a
	// capability of this window even when it was granted.
	host := newHost(t, tokenFor(manifest.PermissionClipboardRead), declared, granted)
	delivery, ok := host.Receive(app.Event{Origin: appOrigin, Source: windowSource, Data: newWindow(t).Ready()})
	if !ok {
		t.Fatal("a ready from the embedded window was ignored")
	}
	if !slices.Equal(delivery.Message.Capabilities, []manifest.Permission{manifest.PermissionClipboardRead}) {
		t.Fatalf("capabilities = %v, want only clipboard-read", delivery.Message.Capabilities)
	}

	// A token that binds a permission the window does not have is a handshake
	// the console must not build.
	if _, err := app.NewHost(app.HostConfig{
		AppID:    "files",
		Entry:    appOrigin,
		Source:   windowSource,
		Window:   "window-1",
		Token:    tokenFor(manifest.PermissionClipboardRead, manifest.PermissionNotifications),
		Theme:    theme(),
		Declared: declared,
		Granted:  granted,
		Now:      clock,
	}); err == nil {
		t.Fatal("a token binding an undeclared permission was accepted")
	}
}

func TestPermissionOutsideFixedSetIsRejected(t *testing.T) {
	declaration := fixtures.MinimalApp()
	declaration["permissions"] = []any{"clipboard-read", "camera"}

	result := manifest.ValidateAppManifest(declaration)
	if result.OK() {
		t.Fatal("an App declaring a permission outside the fixed set was accepted")
	}
	if result.Manifest != nil {
		t.Fatal("a rejected App produced an artifact")
	}
	var found bool
	for _, issue := range result.Issues {
		if issue.Path == "$.permissions[1]" && issue.Code == validate.CodePermissionNotInSet {
			found = true
		}
	}
	if !found {
		t.Fatalf("no permission-not-in-set issue at $.permissions[1]: %v", result.Issues)
	}

	// The handshake refuses the same name, so a declaration that escaped the
	// manifest check still cannot become a capability.
	if _, err := app.NewHost(app.HostConfig{
		AppID:    "files",
		Entry:    appOrigin,
		Source:   windowSource,
		Window:   "window-1",
		Token:    tokenFor("camera"),
		Theme:    theme(),
		Declared: []manifest.Permission{"camera"},
		Granted:  []manifest.Permission{"camera"},
		Now:      clock,
	}); err == nil {
		t.Fatal("a permission outside the fixed set reached the handshake")
	}
}

func TestMessagesFromAnotherOriginOrWindowAreIgnored(t *testing.T) {
	declared := validApp(t).Permissions
	granted := []manifest.Permission{manifest.PermissionExternalLink}
	host := newHost(t, tokenFor(granted...), declared, granted)
	window := newWindow(t)

	ignored := []struct {
		name  string
		event app.Event
	}{
		{"another origin", app.Event{Origin: "https://evil.example", Source: windowSource, Data: window.Ready()}},
		{"another window", app.Event{Origin: appOrigin, Source: "other-window", Data: window.Ready()}},
		{"unknown type", app.Event{Origin: appOrigin, Source: windowSource, Data: map[string]any{"type": "render"}}},
		{"ready with markup", app.Event{Origin: appOrigin, Source: windowSource, Data: map[string]any{
			"type": "ready", "html": "<script>alert(1)</script>",
		}}},
		{"ready with script payload", app.Event{Origin: appOrigin, Source: windowSource, Data: map[string]any{
			"type": "ready", "payload": "<img src=x onerror=alert(1)>",
		}}},
		{"not an object", app.Event{Origin: appOrigin, Source: windowSource, Data: "<script>alert(1)</script>"}},
	}
	for _, testCase := range ignored {
		t.Run(testCase.name, func(t *testing.T) {
			if _, ok := host.Receive(testCase.event); ok {
				t.Fatal("the host answered a message it must ignore")
			}
		})
	}

	delivery, ok := host.Receive(app.Event{Origin: appOrigin, Source: windowSource, Data: window.Ready()})
	if !ok {
		t.Fatal("the ready from the embedded window was ignored after invalid messages")
	}

	if _, err := window.Accept(app.Event{Origin: "https://evil.example", Source: parentSource, Data: delivery.Message}); err == nil {
		t.Fatal("an init from another origin was accepted")
	}
	if _, err := window.Accept(app.Event{Origin: consoleOrigin, Source: "other-window", Data: delivery.Message}); err == nil {
		t.Fatal("an init from another window was accepted")
	}
	if _, err := window.Accept(app.Event{Origin: consoleOrigin, Source: parentSource, Data: map[string]any{
		"type": "init", "token": delivery.Message.Token, "theme": theme(),
		"capabilities": granted, "cookie": "console-session",
	}}); err == nil {
		t.Fatal("an init carrying an extra field was accepted")
	}

	expired := delivery.Message
	expired.Token.ExpiresAt = now.Add(-time.Minute)
	if _, err := window.Accept(app.Event{Origin: consoleOrigin, Source: parentSource, Data: expired}); err == nil {
		t.Fatal("an expired token was accepted")
	}

	mismatched := delivery.Message
	mismatched.Token.Permissions = []manifest.Permission{manifest.PermissionNotifications}
	if _, err := window.Accept(app.Event{Origin: consoleOrigin, Source: parentSource, Data: mismatched}); err == nil {
		t.Fatal("an init whose token binds other permissions was accepted")
	}

	if _, err := window.Accept(app.Event{Origin: consoleOrigin, Source: parentSource, Data: delivery.Message}); err != nil {
		t.Fatalf("the valid init was rejected after invalid ones: %v", err)
	}
}

func TestHostRefusesWildcardTargetOrigin(t *testing.T) {
	for _, entry := range []string{"*", "http://files.apps.kos.example", "https://user:pass@files.apps.kos.example"} {
		if _, err := app.NewHost(app.HostConfig{
			AppID:    "files",
			Entry:    entry,
			Source:   windowSource,
			Window:   "window-1",
			Token:    tokenFor(),
			Theme:    theme(),
			Declared: validApp(t).Permissions,
			Granted:  nil,
			Now:      clock,
		}); err == nil {
			t.Fatalf("entry %q was accepted; the handshake never uses a wildcard or credential-bearing origin", entry)
		}
	}
}

func TestDefaultPortIsDroppedFromOrigin(t *testing.T) {
	declared := validApp(t).Permissions
	granted := []manifest.Permission{manifest.PermissionExternalLink}

	// An entry written with the default https port still handshakes: the
	// browser reports event.origin without that port.
	host, err := app.NewHost(app.HostConfig{
		AppID:    "files",
		Entry:    appOrigin + ":443/",
		Source:   windowSource,
		Window:   "window-1",
		Token:    tokenFor(granted...),
		Theme:    theme(),
		Declared: declared,
		Granted:  granted,
		Now:      clock,
	})
	if err != nil {
		t.Fatalf("host rejected an entry carrying the default port: %v", err)
	}
	window, err := app.NewWindow(app.WindowConfig{
		AppID:         "files",
		ConsoleOrigin: consoleOrigin + ":443/",
		Parent:        parentSource,
		Now:           clock,
	})
	if err != nil {
		t.Fatalf("window rejected a console origin carrying the default port: %v", err)
	}

	delivery, ok := host.Receive(app.Event{Origin: appOrigin, Source: windowSource, Data: window.Ready()})
	if !ok {
		t.Fatal("a ready from https://host was ignored for an entry written https://host:443")
	}
	if delivery.TargetOrigin != appOrigin {
		t.Fatalf("target origin = %q, want %q", delivery.TargetOrigin, appOrigin)
	}
	if _, err := window.Accept(app.Event{Origin: consoleOrigin, Source: parentSource, Data: delivery.Message}); err != nil {
		t.Fatalf("init from https://console was rejected for a console origin written https://console:443: %v", err)
	}

	// A non-default port is part of the origin and stays.
	host, err = app.NewHost(app.HostConfig{
		AppID:    "files",
		Entry:    appOrigin + ":8443/",
		Source:   windowSource,
		Window:   "window-1",
		Token:    tokenFor(granted...),
		Theme:    theme(),
		Declared: declared,
		Granted:  granted,
		Now:      clock,
	})
	if err != nil {
		t.Fatalf("host rejected an entry carrying a non-default port: %v", err)
	}
	if _, ok := host.Receive(app.Event{Origin: appOrigin, Source: windowSource, Data: window.Ready()}); ok {
		t.Fatal("an origin without the declared port was accepted")
	}
	delivery, ok = host.Receive(app.Event{Origin: appOrigin + ":8443", Source: windowSource, Data: window.Ready()})
	if !ok {
		t.Fatal("a ready from the declared non-default port was ignored")
	}
	if delivery.TargetOrigin != appOrigin+":8443" {
		t.Fatalf("target origin = %q, want %q", delivery.TargetOrigin, appOrigin+":8443")
	}
}

func TestHandshakeWithNoPermissions(t *testing.T) {
	declaration := fixtures.MinimalApp()
	declaration["permissions"] = []any{}
	result := manifest.ValidateAppManifest(declaration)
	if !result.OK() {
		t.Fatalf("an App with an empty permission set was rejected: %v", result.Issues)
	}
	declared := result.Manifest.Permissions
	if len(declared) != 0 {
		t.Fatalf("declared permissions = %v, want none", declared)
	}

	host, err := app.NewHost(app.HostConfig{
		AppID:    "files",
		Entry:    appOrigin,
		Source:   windowSource,
		Window:   "window-1",
		Token:    tokenFor(),
		Theme:    theme(),
		Declared: declared,
		Granted:  nil,
		Now:      clock,
	})
	if err != nil {
		t.Fatalf("host rejected a zero-permission handshake: %v", err)
	}
	delivery, ok := host.Receive(app.Event{Origin: appOrigin, Source: windowSource, Data: newWindow(t).Ready()})
	if !ok {
		t.Fatal("a ready from the embedded window was ignored")
	}
	if delivery.Message.Capabilities == nil {
		t.Fatal("capabilities is null, want an empty array")
	}

	encoded, err := json.Marshal(delivery.Message)
	if err != nil {
		t.Fatalf("marshal init: %v", err)
	}
	var wire map[string]any
	if err := json.Unmarshal(encoded, &wire); err != nil {
		t.Fatalf("unmarshal init: %v", err)
	}
	capabilities, ok := wire["capabilities"].([]any)
	if !ok || len(capabilities) != 0 {
		t.Fatalf("capabilities on the wire = %v, want []", wire["capabilities"])
	}

	var roundTripped any
	if err := json.Unmarshal(encoded, &roundTripped); err != nil {
		t.Fatalf("round trip init: %v", err)
	}
	session, err := newWindow(t).Accept(app.Event{Origin: consoleOrigin, Source: parentSource, Data: roundTripped})
	if err != nil {
		t.Fatalf("zero-permission init rejected: %v", err)
	}
	if len(session.Capabilities) != 0 {
		t.Fatalf("session capabilities = %v, want none", session.Capabilities)
	}
	for _, permission := range manifest.AppPermissions {
		if session.Allows(permission) {
			t.Fatalf("permission %q is available without a declaration", permission)
		}
	}
}

func TestTokenBoundElsewhereIsRejected(t *testing.T) {
	declared := validApp(t).Permissions
	granted := []manifest.Permission{manifest.PermissionExternalLink}

	hostConfig := func(token app.Token) app.HostConfig {
		return app.HostConfig{
			AppID:    "files",
			Entry:    appOrigin,
			Source:   windowSource,
			Window:   "window-1",
			Token:    token,
			Theme:    theme(),
			Declared: declared,
			Granted:  granted,
			Now:      clock,
		}
	}

	otherApp := tokenFor(granted...)
	otherApp.AppID = "other-app"
	if _, err := app.NewHost(hostConfig(otherApp)); err == nil {
		t.Fatal("a token bound to another app was accepted")
	}

	otherWindow := tokenFor(granted...)
	otherWindow.Window = "other-window"
	if _, err := app.NewHost(hostConfig(otherWindow)); err == nil {
		t.Fatal("a token bound to another window instance was accepted")
	}

	host := newHost(t, tokenFor(granted...), declared, granted)
	delivery, ok := host.Receive(app.Event{Origin: appOrigin, Source: windowSource, Data: newWindow(t).Ready()})
	if !ok {
		t.Fatal("a ready from the embedded window was ignored")
	}

	foreignApp := delivery.Message
	foreignApp.Token.AppID = "other-app"
	if _, err := newWindow(t).Accept(app.Event{Origin: consoleOrigin, Source: parentSource, Data: foreignApp}); err == nil {
		t.Fatal("an init token bound to another app was accepted")
	}

	unbound := delivery.Message
	unbound.Token.Window = ""
	if _, err := newWindow(t).Accept(app.Event{Origin: consoleOrigin, Source: parentSource, Data: unbound}); err == nil {
		t.Fatal("an init token not bound to a window instance was accepted")
	}
}
