package sdktc3

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/tsic404/kos-sdk/app"
	"github.com/tsic404/kos-sdk/manifest"
)

// consoleReadMessage mirrors the console's readMessage (kos-dashboard
// pkg/bff/assets/shell/apphost.js): a message names its type, may name its
// window, and `unsaved` must carry a boolean.
func consoleReadMessage(data any) map[string]any {
	object, ok := data.(map[string]any)
	if !ok {
		return nil
	}
	if window, present := object["window"]; present {
		if _, ok := window.(string); !ok {
			return nil
		}
	}
	switch object["type"] {
	case "ready", "alive":
		return object
	case "unsaved":
		if _, ok := object["unsaved"].(bool); !ok {
			return nil
		}
		return object
	default:
		return nil
	}
}

// consoleDecodeReady mirrors the console's DecodeReady: it refuses unknown
// fields, so a `ready` carrying anything else never reaches the handshake.
func consoleDecodeReady(data any) bool {
	object, ok := data.(map[string]any)
	if !ok {
		return false
	}
	for key := range object {
		if key != "type" && key != "window" {
			return false
		}
	}
	if object["type"] != "ready" {
		return false
	}
	if window, present := object["window"]; present {
		if _, ok := window.(string); !ok {
			return false
		}
	}
	return true
}

func onConsoleWire(t *testing.T, message any) any {
	t.Helper()
	encoded, err := json.Marshal(message)
	if err != nil {
		t.Fatalf("marshal %T: %v", message, err)
	}
	var wire any
	if err := json.Unmarshal(encoded, &wire); err != nil {
		t.Fatalf("read %T: %v", message, err)
	}
	return wire
}

// onWireMap renders one message as the JSON object a browser would post and
// hands it back as a plain map, so a test can add or drop a single field
// without touching the rest of the frame.
func onWireMap(t *testing.T, message any) map[string]any {
	t.Helper()
	object, ok := onConsoleWire(t, message).(map[string]any)
	if !ok {
		t.Fatalf("%T is not a JSON object on the wire", message)
	}
	return object
}

func TestReportsAreAcceptedByTheConsole(t *testing.T) {
	window := newWindow(t)

	ready := onConsoleWire(t, window.Ready())
	if !consoleDecodeReady(ready) {
		t.Fatalf("the console refuses the ready: %v", ready)
	}
	alive := onConsoleWire(t, window.Alive())
	if message := consoleReadMessage(alive); message == nil || message["type"] != "alive" {
		t.Fatalf("the console ignores the alive report: %v", alive)
	}
	unsaved := onConsoleWire(t, window.Unsaved(true))
	message := consoleReadMessage(unsaved)
	if message == nil {
		t.Fatalf("the console ignores the unsaved report: %v", unsaved)
	}
	if unsavedValue, ok := message["unsaved"].(bool); !ok || !unsavedValue {
		t.Fatalf("unsaved on the wire = %v, want true", message["unsaved"])
	}
	// A regression that always sends true would still parse, but the console
	// would never clear the mark and would veto every close (KOSD-C-81), so the
	// value itself has to be asserted.
	cleared := onConsoleWire(t, window.Unsaved(false))
	clearedMessage := consoleReadMessage(cleared)
	if clearedMessage == nil {
		t.Fatalf("the console ignores an unsaved report of false: %v", cleared)
	}
	if clearedMessage["type"] != "unsaved" {
		t.Fatalf("cleared report type = %v, want unsaved", clearedMessage["type"])
	}
	if clearedValue, ok := clearedMessage["unsaved"].(bool); !ok || clearedValue {
		t.Fatalf("unsaved on the wire = %v, want false", clearedMessage["unsaved"])
	}
	if clearedMessage["window"] != "window-1" {
		t.Fatalf("cleared report names window %v, want window-1", clearedMessage["window"])
	}

	// Every report names this window instance, so the console attributes it to
	// the instance the token belongs to (KOSD-C-81).
	for _, report := range []any{ready, alive, unsaved, cleared} {
		object := report.(map[string]any)
		if object["window"] != "window-1" {
			t.Fatalf("report names window %v, want window-1", object["window"])
		}
	}
}

func TestAliveIntervalStaysInsideTheConsoleWindow(t *testing.T) {
	// The console marks a window stopped when it stays silent longer than its
	// decision window (DASH-C-74); the SDK reports well inside it.
	if app.AliveInterval <= 0 {
		t.Fatalf("AliveInterval = %s, want a positive period", app.AliveInterval)
	}
	if app.AliveInterval >= 35*time.Second {
		t.Fatalf("AliveInterval = %s, not inside the console's decision window", app.AliveInterval)
	}
}

func TestHostOnlyAnswersItsOwnWindow(t *testing.T) {
	declared := validApp(t).Permissions
	granted := []manifest.Permission{manifest.PermissionExternalLink}
	host := newHost(t, tokenFor(granted...), declared, granted)

	if _, ok := host.Receive(app.Event{
		Origin: appOrigin,
		Source: windowSource,
		Data:   app.Ready{Type: app.TypeReady, Window: "other-window"},
	}); ok {
		t.Fatal("a ready naming another window instance was answered")
	}
	if _, ok := host.Receive(app.Event{
		Origin: appOrigin,
		Source: windowSource,
		Data:   newWindow(t).Ready(),
	}); !ok {
		t.Fatal("a ready naming its own window instance was ignored")
	}
}

// TestExplicitNullWindowIsRefused keeps the host from spending the one-shot
// handshake on a message the console's own readMessage drops: it accepts the
// field when absent or a string, never when it is another JSON type
// (KOSD-C-81).
func TestExplicitNullWindowIsRefused(t *testing.T) {
	for _, badWindow := range []string{"null", "42", "true", `{"id":"window-1"}`} {
		t.Run(badWindow, func(t *testing.T) {
			declared := validApp(t).Permissions
			granted := []manifest.Permission{manifest.PermissionExternalLink}
			host := newHost(t, tokenFor(granted...), declared, granted)

			var data any
			frame := `{"type":"ready","window":` + badWindow + `}`
			if err := json.Unmarshal([]byte(frame), &data); err != nil {
				t.Fatalf("read %s: %v", frame, err)
			}
			if _, ok := host.Receive(app.Event{Origin: appOrigin, Source: windowSource, Data: data}); ok {
				t.Fatalf("a ready carrying window %s was answered", badWindow)
			}

			// The handshake is still available to the well-formed ready.
			if _, ok := host.Receive(app.Event{
				Origin: appOrigin,
				Source: windowSource,
				Data:   newWindow(t).Ready(),
			}); !ok {
				t.Fatal("a valid ready was ignored after an invalid one")
			}
		})
	}
}

// TestReadyWithoutWindowIsAttributedToTheSource keeps the field optional: the
// console attributes a ready with no window to the iframe it came from
// (KOSD-C-81).
func TestReadyWithoutWindowIsAttributedToTheSource(t *testing.T) {
	declared := validApp(t).Permissions
	granted := []manifest.Permission{manifest.PermissionExternalLink}
	host := newHost(t, tokenFor(granted...), declared, granted)

	var data any
	if err := json.Unmarshal([]byte(`{"type":"ready"}`), &data); err != nil {
		t.Fatalf("read the ready without a window: %v", err)
	}
	if _, ok := host.Receive(app.Event{Origin: appOrigin, Source: windowSource, Data: data}); !ok {
		t.Fatal("a ready without a window field was ignored")
	}
}

// TestReadyWithoutAStringTypeIsRefused keeps the host from answering a message
// the console's readMessage drops: a ready has to name itself with the string
// "ready", so a missing type or a JSON null is not a ready and must not spend
// the one-shot handshake (KOSD-C-75).
func TestReadyWithoutAStringTypeIsRefused(t *testing.T) {
	for _, frame := range []string{
		`{"window":"window-1"}`,
		`{"type":null}`,
		`{"type":null,"window":"window-1"}`,
		`{"type":42}`,
	} {
		t.Run(frame, func(t *testing.T) {
			declared := validApp(t).Permissions
			granted := []manifest.Permission{manifest.PermissionExternalLink}
			host := newHost(t, tokenFor(granted...), declared, granted)

			var data any
			if err := json.Unmarshal([]byte(frame), &data); err != nil {
				t.Fatalf("read %s: %v", frame, err)
			}
			if _, ok := host.Receive(app.Event{Origin: appOrigin, Source: windowSource, Data: data}); ok {
				t.Fatalf("%s was answered as a ready", frame)
			}

			// The handshake is still available to a well-formed ready.
			if _, ok := host.Receive(app.Event{
				Origin: appOrigin,
				Source: windowSource,
				Data:   newWindow(t).Ready(),
			}); !ok {
				t.Fatalf("%s consumed the handshake", frame)
			}
		})
	}
}
