package app

import (
	"encoding/json"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/tsic404/kos-sdk/manifest"
)

// WindowConfig configures the app side of one window instance's handshake.
type WindowConfig struct {
	// AppID is the App's manifest identifier; the init token must match it.
	AppID string
	// ConsoleOrigin is the origin of the console that embeds the app; only its
	// origin is accepted as the origin of an inbound event.
	ConsoleOrigin string
	// Parent is the embedding window (event.source of an inbound event).
	Parent Source
	// Now supplies the current time; nil uses time.Now.
	Now func() time.Time
}

// Window is the app side of the handshake: it emits the ready message and
// accepts exactly one init from the console it is embedded in. It is safe for
// concurrent use.
type Window struct {
	mu       sync.Mutex
	appID    string
	origin   string
	parent   Source
	now      func() time.Time
	accepted bool
}

// NewWindow builds the app side, rejecting a configuration that could not
// verify the console it expects.
func NewWindow(config WindowConfig) (*Window, error) {
	if config.AppID == "" {
		return nil, fmt.Errorf("app: window needs an app id")
	}
	origin, err := originOf(config.ConsoleOrigin)
	if err != nil {
		return nil, err
	}
	if config.Parent == "" {
		return nil, fmt.Errorf("app: window needs the embedding window's source")
	}
	now := config.Now
	if now == nil {
		now = time.Now
	}
	return &Window{appID: config.AppID, origin: origin, parent: config.Parent, now: now}, nil
}

// Ready is the message the app posts to the console once it has loaded.
func (w *Window) Ready() Ready {
	return Ready{Type: TypeReady}
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
	if err := checkInit(init, w.appID, w.now()); err != nil {
		return Session{}, err
	}
	w.accepted = true
	return Session{
		Token:        init.Token,
		Theme:        slices.Clone(init.Theme),
		Capabilities: slices.Clone(init.Capabilities),
	}, nil
}

// Session is the app's own session after the handshake: the window's app token,
// the console theme and the capabilities the app may use. It holds no console
// session material.
type Session struct {
	Token        Token
	Theme        json.RawMessage
	Capabilities []manifest.Permission
}

// Allows reports whether this window may use permission. A permission the App
// manifest does not declare is never available (KOSD-C-75).
func (s Session) Allows(permission manifest.Permission) bool {
	return slices.Contains(s.Capabilities, permission)
}

// checkInit verifies an init against this window: the token must belong to the
// app, be bound to a window instance, bind exactly the capabilities delivered,
// and still be current (KOSD-C-75).
func checkInit(init Init, appID string, now time.Time) error {
	token := init.Token
	if token.Value == "" {
		return fmt.Errorf("app: init token value is empty")
	}
	if token.Account == "" {
		return fmt.Errorf("app: init token is not bound to an account")
	}
	if token.AppID != appID {
		return fmt.Errorf("app: init token belongs to app %q, not %q", token.AppID, appID)
	}
	if token.Window == "" {
		return fmt.Errorf("app: init token is not bound to a window instance")
	}
	if init.Capabilities == nil {
		return fmt.Errorf("app: init carries no capability list")
	}
	if err := checkPermissionSet(init.Capabilities); err != nil {
		return err
	}
	if !samePermissions(token.Permissions, init.Capabilities) {
		return fmt.Errorf("app: init token permissions %v do not match the capabilities %v",
			token.Permissions, init.Capabilities)
	}
	if len(init.Theme) == 0 || string(init.Theme) == "null" {
		return fmt.Errorf("app: init carries no theme")
	}
	if !token.ExpiresAt.After(now) {
		return fmt.Errorf("app: init token expired at %s", token.ExpiresAt.Format(time.RFC3339))
	}
	return nil
}
