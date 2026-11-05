package app

import (
	"encoding/json"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/tsic404/kos-sdk/manifest"
)

// HostConfig configures the console side of one window instance's handshake.
type HostConfig struct {
	// AppID is the App's manifest identifier; the token must be bound to it.
	AppID string
	// Entry is the App manifest entry (KOSD-C-73); only its origin is accepted
	// as the origin of an inbound event.
	Entry string
	// Source is the window the console embedded (event.source).
	Source Source
	// Window is the window instance identifier the token is bound to.
	Window string
	// Token is the short-lived token the console issued for this window.
	Token Token
	// Theme is the console's current appearance, passed through unchanged.
	Theme json.RawMessage
	// Declared is the permission list of the App manifest.
	Declared []manifest.Permission
	// Granted is the subset the user confirmed for this window.
	Granted []manifest.Permission
	// Now supplies the current time; nil uses time.Now.
	Now func() time.Time
}

// Host is the console side of the handshake. It answers one valid ready per
// window instance; a later ready is ignored, because the token is delivered
// once (KOSD-C-75). It is safe for concurrent use.
type Host struct {
	mu        sync.Mutex
	origin    string
	source    Source
	delivered bool
	init      Init
}

// NewHost builds the console side, rejecting a handshake that would hand the
// app a capability the manifest does not declare or a token bound to another
// app, another window or another permission set.
func NewHost(config HostConfig) (*Host, error) {
	if config.AppID == "" {
		return nil, fmt.Errorf("app: host needs an app id")
	}
	origin, err := originOf(config.Entry)
	if err != nil {
		return nil, err
	}
	if config.Source == "" {
		return nil, fmt.Errorf("app: host needs the embedded window's source")
	}
	if config.Window == "" {
		return nil, fmt.Errorf("app: host needs a window instance id")
	}
	if len(config.Theme) == 0 || string(config.Theme) == "null" {
		return nil, fmt.Errorf("app: host needs a theme for the init message")
	}
	available, err := capabilities(config.Declared, config.Granted)
	if err != nil {
		return nil, err
	}
	if err := checkToken(config.Token, config.AppID, config.Window, available); err != nil {
		return nil, err
	}
	now := config.Now
	if now == nil {
		now = time.Now
	}
	if !config.Token.ExpiresAt.After(now()) {
		return nil, fmt.Errorf("app: token for app %q expired at %s",
			config.AppID, config.Token.ExpiresAt.Format(time.RFC3339))
	}

	token := config.Token
	token.Permissions = slices.Clone(available)
	return &Host{
		origin: origin,
		source: config.Source,
		init: Init{
			Type:         TypeInit,
			Token:        token,
			Theme:        slices.Clone(config.Theme),
			Capabilities: slices.Clone(available),
		},
	}, nil
}

// Receive handles one inbound event. It returns the init delivery when the
// event is the app's ready from the embedded window and no delivery has been
// made yet. Every other event — another origin, another source, an unknown
// type, a message carrying a field the handshake does not name — is ignored.
func (h *Host) Receive(event Event) (Delivery, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.delivered {
		return Delivery{}, false
	}
	if event.Origin != h.origin || event.Source != h.source {
		return Delivery{}, false
	}
	var ready Ready
	if !decodeClosed(event.Data, &ready) || ready.Type != TypeReady {
		return Delivery{}, false
	}
	h.delivered = true
	return Delivery{TargetOrigin: h.origin, Message: h.init}, true
}

// checkToken verifies the token is bound to this app, this window and exactly
// the capabilities the handshake hands over (KOSD-C-75).
func checkToken(token Token, appID, window string, available []manifest.Permission) error {
	if token.Value == "" {
		return fmt.Errorf("app: token value is empty")
	}
	if token.Account == "" {
		return fmt.Errorf("app: token is not bound to an account")
	}
	if token.AppID != appID {
		return fmt.Errorf("app: token belongs to app %q, not %q", token.AppID, appID)
	}
	if token.Window != window {
		return fmt.Errorf("app: token belongs to window %q, not %q", token.Window, window)
	}
	if !samePermissions(token.Permissions, available) {
		return fmt.Errorf("app: token permissions %v do not match the capabilities %v",
			token.Permissions, available)
	}
	return nil
}
