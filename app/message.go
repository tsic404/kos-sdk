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
)

// Ready is the app's handshake message. It carries no other field, so an app
// hands the console no markup and no script (KOSD-C-75).
type Ready struct {
	Type MessageType `json:"type"`
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

// Token is the short-lived application token of one window instance
// (KOSD-C-75). It binds the account, the app, the window instance, the
// permissions and an expiry; the console issues it and the control plane keeps
// only its digest. The value crosses the handshake once.
type Token struct {
	Value       string                `json:"value"`
	Account     string                `json:"account"`
	AppID       string                `json:"appId"`
	Window      string                `json:"window"`
	Permissions []manifest.Permission `json:"permissions"`
	ExpiresAt   time.Time             `json:"expiresAt"`
}

// Init is the console's handshake answer (KOSD-C-75). Its field set is closed
// and carries no console session material.
type Init struct {
	Type         MessageType           `json:"type"`
	Token        Token                 `json:"token"`
	Theme        json.RawMessage       `json:"theme"`
	Capabilities []manifest.Permission `json:"capabilities"`
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
