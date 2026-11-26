package app

import (
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/tsic404/kos-sdk/manifest"
)

// WindowConfig configures the app side of one window instance's handshake.
type WindowConfig struct {
	// ConsoleOrigin is the origin of the console that embeds the app; only its
	// origin is accepted as the origin of an inbound event.
	ConsoleOrigin string
	// Parent is the embedding window (event.source of an inbound event).
	Parent Source
	// Window is the window instance identifier the console embedded; every
	// report names it so the console can attribute it (KOSD-C-81).
	Window string
	// Now supplies the current time; nil uses time.Now.
	Now func() time.Time
}

// Window is the app side of the handshake: it emits the ready message and
// accepts exactly one init from the console it is embedded in. It is safe for
// concurrent use.
type Window struct {
	mu       sync.Mutex
	origin   string
	parent   Source
	window   string
	now      func() time.Time
	accepted bool
}

// NewWindow builds the app side, rejecting a configuration that could not
// verify the console it expects.
func NewWindow(config WindowConfig) (*Window, error) {
	origin, err := originOf(config.ConsoleOrigin)
	if err != nil {
		return nil, err
	}
	if config.Parent == "" {
		return nil, fmt.Errorf("app: window needs the embedding window's source")
	}
	if config.Window == "" {
		return nil, fmt.Errorf("app: window needs a window instance id")
	}
	now := config.Now
	if now == nil {
		now = time.Now
	}
	return &Window{origin: origin, parent: config.Parent, window: config.Window, now: now}, nil
}

// Ready is the message the app posts to the console once it has loaded. It
// names this window instance (KOSD-C-81).
func (w *Window) Ready() Ready {
	return Ready{Type: TypeReady, Window: w.window}
}

// Alive is the liveness report the app posts while the window runs
// (KOSD-C-81). Post it after Accept has succeeded; the console ignores reports
// before the handshake.
func (w *Window) Alive() Alive {
	return Alive{Type: TypeAlive, Window: w.window}
}

// Unsaved is the unsaved-changes report the app posts when its state changes
// (KOSD-C-81). Post it after Accept has succeeded; the console ignores reports
// before the handshake.
func (w *Window) Unsaved(unsaved bool) Unsaved {
	return Unsaved{Type: TypeUnsaved, Unsaved: unsaved, Window: w.window}
}

// Accept checks one inbound event as the console's init. An event from another
// origin or another window, a message whose structure is not an init, and an
// init whose token is bound elsewhere or has expired are rejected and leave
// the window uninitialised (KOSD-C-75).
func (w *Window) Accept(event Event) (Session, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.accepted {
		return Session{}, fmt.Errorf("app: window already accepted an init")
	}
	if event.Origin != w.origin {
		return Session{}, fmt.Errorf("app: init from origin %q, want %q", event.Origin, w.origin)
	}
	if event.Source != w.parent {
		return Session{}, fmt.Errorf("app: init from window %q, want the embedding window %q", event.Source, w.parent)
	}
	var init Init
	if !decodeClosed(event.Data, &init) || init.Type != TypeInit {
		return Session{}, fmt.Errorf("app: event data is not an init message")
	}
	if err := checkInit(init, w.now()); err != nil {
		return Session{}, err
	}
	w.accepted = true
	return Session{
		Token:        init.Token,
		Theme:        init.Theme,
		Capabilities: slices.Clone(init.Capabilities),
		ExpiresAt:    init.ExpiresAt,
	}, nil
}

// Session is the app's own session after the handshake: the window's opaque app
// token, the console theme, the capabilities the app may use and the token's
// expiry. It holds no console session material.
type Session struct {
	Token        string
	Theme        string
	Capabilities []manifest.Permission
	ExpiresAt    time.Time
}

// Allows reports whether this window may use permission. A permission the App
// manifest does not declare is never available (KOSD-C-75).
func (s Session) Allows(permission manifest.Permission) bool {
	return slices.Contains(s.Capabilities, permission)
}

// checkInit verifies an init against this window: it must carry a token, a
// theme and a capability list inside the fixed set, and the token must still be
// current (KOSD-C-75). The token is opaque, so its bindings are enforced by the
// console when the app uses it.
func checkInit(init Init, now time.Time) error {
	if init.Token == "" {
		return fmt.Errorf("app: init token is empty")
	}
	if init.Theme == "" {
		return fmt.Errorf("app: init carries no theme")
	}
	if init.Capabilities == nil {
		return fmt.Errorf("app: init carries no capability list")
	}
	if err := checkPermissionSet(init.Capabilities); err != nil {
		return err
	}
	if !init.ExpiresAt.After(now) {
		return fmt.Errorf("app: init token expired at %s", init.ExpiresAt.Format(time.RFC3339))
	}
	return nil
}
