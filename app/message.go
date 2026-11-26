// Package app is the application interface of the author SDK: the KOSD-C-75
// handshake, its postMessage structure and the fixed permission set of
// KOSD-C-73. It carries no console session material: no message, token or
// interface of this package can hold a console cookie or a kubeconfig.
package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/tsic404/kos-sdk/manifest"
)

// MessageType is one message type of the handshake. The set is closed: a
// message of another type is ignored (KOSD-C-75).
type MessageType string

const (
	// TypeReady is the app's handshake message; the app sends it once it has
	// loaded.
	TypeReady MessageType = "ready"
	// TypeInit is the console's answer: the app token, the theme and the
	// capabilities the app may use.
	TypeInit MessageType = "init"
	// TypeAlive is the app's liveness report (KOSD-C-81).
	TypeAlive MessageType = "alive"
	// TypeUnsaved is the app's unsaved-changes report (KOSD-C-81).
	TypeUnsaved MessageType = "unsaved"
)

// AliveInterval is how often an app reports alive. It stays well inside the
// console's decision window (DASH-C-74), so a running window never looks stopped.
const AliveInterval = 10 * time.Second

// Ready is the app's handshake message. Window optionally names the window
// instance; when empty the console attributes the message to the iframe it came
// from (KOSD-C-81). It carries no other field, so an app hands the console no
// markup and no script (KOSD-C-75).
type Ready struct {
	Type   MessageType `json:"type"`
	Window string      `json:"window,omitempty"`
}

// Alive is the app's liveness report (KOSD-C-81). The console marks a window
// stopped when it stays silent longer than its decision window, so the app
// reports at AliveInterval for as long as the window runs.
type Alive struct {
	Type   MessageType `json:"type"`
	Window string      `json:"window,omitempty"`
}

// Unsaved is the app's unsaved-changes report (KOSD-C-81). True keeps the
// console from closing the window on the first close request.
type Unsaved struct {
	Type    MessageType `json:"type"`
	Unsaved bool        `json:"unsaved"`
	Window  string      `json:"window,omitempty"`
}

// Source identifies the window a message came from (event.source, KOSD-C-75).
// The host assigns one value per embedded window and compares it with the
// event.
type Source string

// Event is one inbound postMessage event: the sender's origin, the sending
// window and the decoded data.
type Event struct {
	Origin string
	Source Source
	Data   any
}

// Token is the console-side record of one window instance's short-lived
// application token (KOSD-C-75). It binds the account, the app, the window
// instance, the permissions and an expiry; the control plane keeps only its
// digest. Only Value crosses the handshake, so the app cannot read these
// bindings and the console enforces them when the token is used.
type Token struct {
	Value       string                `json:"value"`
	Account     string                `json:"account"`
	AppID       string                `json:"appId"`
	Window      string                `json:"window"`
	Permissions []manifest.Permission `json:"permissions"`
	ExpiresAt   time.Time             `json:"expiresAt"`
}

// Init is the console's handshake answer in the console's wire shape
// (KOSD-C-75): the opaque token, the theme, the capabilities the app may use and
// the token's expiry. Its field set is closed and carries no console session
// material.
type Init struct {
	Type         MessageType           `json:"type"`
	Token        string                `json:"token"`
	Theme        string                `json:"theme"`
	Capabilities []manifest.Permission `json:"capabilities"`
	ExpiresAt    time.Time             `json:"expiresAt"`
}

// Delivery is the init message and the exact origin it is posted to. The SDK
// never yields a wildcard target origin.
type Delivery struct {
	TargetOrigin string
	Message      Init
}

// decodeClosed decodes value into target and rejects any field target does not
// define, so a message cannot carry a payload the handshake does not name.
func decodeClosed(value any, target any) bool {
	encoded, err := json.Marshal(value)
	if err != nil {
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	return decoder.Decode(target) == nil
}

// decodeReady decodes a ready message. The type is required and must be the
// string "ready": a missing type or a JSON null would otherwise pass as the zero
// value, and a null window would land on the empty string and pass as an absent
// field, so the host would answer a message the console ignores and spend the
// one-shot handshake on it (KOSD-C-75, KOSD-C-81).
func decodeReady(value any) (Ready, bool) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return Ready{}, false
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &fields); err != nil {
		return Ready{}, false
	}
	rawType, named := fields["type"]
	if !named || len(rawType) == 0 || rawType[0] != '"' {
		return Ready{}, false
	}
	var ready Ready
	if err := json.Unmarshal(rawType, &ready.Type); err != nil || ready.Type != TypeReady {
		return Ready{}, false
	}
	for name, raw := range fields {
		switch name {
		case "type":
		case "window":
			if len(raw) == 0 || raw[0] != '"' {
				return Ready{}, false
			}
			if err := json.Unmarshal(raw, &ready.Window); err != nil {
				return Ready{}, false
			}
		default:
			return Ready{}, false
		}
	}
	return ready, true
}

// originOf reduces an absolute https address to the origin a postMessage event
// carries: scheme, host and port, without path, query or credentials. The
// browser drops the default https port from event.origin, so this does too;
// other ports stay.
func originOf(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", fmt.Errorf("app: origin %q: %w", raw, err)
	}
	if !strings.EqualFold(parsed.Scheme, "https") || parsed.Host == "" {
		return "", fmt.Errorf("app: origin %q must be an absolute https address", raw)
	}
	if parsed.User != nil {
		return "", fmt.Errorf("app: origin %q must not carry credentials", raw)
	}
	host := strings.ToLower(parsed.Hostname())
	port := parsed.Port()
	if port == "" || port == "443" {
		if strings.Contains(host, ":") {
			host = "[" + host + "]"
		}
		return "https://" + host, nil
	}
	return "https://" + net.JoinHostPort(host, port), nil
}

// capabilities returns the permissions an app may use: those both declared by
// its manifest and granted to this window, in contract order. A permission the
// manifest does not declare is never available (KOSD-C-75).
func capabilities(declared, granted []manifest.Permission) ([]manifest.Permission, error) {
	for _, list := range [][]manifest.Permission{declared, granted} {
		if err := checkPermissionSet(list); err != nil {
			return nil, err
		}
	}
	declaredSet := permissionSet(declared)
	grantedSet := permissionSet(granted)
	available := make([]manifest.Permission, 0, len(declared))
	for _, permission := range manifest.AppPermissions {
		_, declared := declaredSet[permission]
		_, granted := grantedSet[permission]
		if declared && granted {
			available = append(available, permission)
		}
	}
	return available, nil
}

// checkPermissionSet rejects a name outside the fixed set of KOSD-C-75.
func checkPermissionSet(permissions []manifest.Permission) error {
	for _, permission := range permissions {
		if !inFixedSet(permission) {
			return fmt.Errorf("app: permission %q is not in the fixed set %v", permission, manifest.AppPermissions)
		}
	}
	return nil
}

func inFixedSet(permission manifest.Permission) bool {
	for _, known := range manifest.AppPermissions {
		if known == permission {
			return true
		}
	}
	return false
}

func permissionSet(permissions []manifest.Permission) map[manifest.Permission]struct{} {
	set := make(map[manifest.Permission]struct{}, len(permissions))
	for _, permission := range permissions {
		set[permission] = struct{}{}
	}
	return set
}

// samePermissions reports set equality, independent of order and repetition.
func samePermissions(left, right []manifest.Permission) bool {
	leftSet := permissionSet(left)
	rightSet := permissionSet(right)
	if len(leftSet) != len(rightSet) {
		return false
	}
	for permission := range leftSet {
		if _, ok := rightSet[permission]; !ok {
			return false
		}
	}
	return true
}
