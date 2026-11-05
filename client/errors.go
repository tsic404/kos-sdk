package client

import (
	"encoding/json"
	"fmt"
)

// APIError is one failure the control plane reported. It carries the envelope
// as sent: no code is mapped to another, no stage is renamed, and the retryable
// flag is left for the author to act on.
type APIError struct {
	// StatusCode is the HTTP status the response carried.
	StatusCode int
	// Code is the stable failure code.
	Code string
	// Message is the user-facing explanation.
	Message string
	// Stage names the step that failed.
	Stage string
	// Object names the object the failure is about, when there is one.
	Object *ObjectRef
	// Retryable tells whether repeating the same request can succeed.
	Retryable bool
	// RequiresUserAction tells whether the failure waits on a user decision.
	RequiresUserAction bool
	// SideEffects tells whether the failed request left objects behind.
	SideEffects string
	// Applied lists the objects written before a failed write stopped.
	Applied []ObjectRef
	// Current is the object as the cluster holds it now.
	Current map[string]any
	// RetryAfterSeconds is how long to wait before repeating a locked request.
	RetryAfterSeconds int64
	// OwnedObjects lists the objects that keep a delete from proceeding.
	OwnedObjects []ObjectRef
	// Fields is the whole error object as decoded, so a field this type does not
	// model still reaches the author.
	Fields map[string]json.RawMessage
	// Raw is the response body exactly as it arrived.
	Raw json.RawMessage
}

// Error renders the stable code and the message; callers branch on Code.
func (e *APIError) Error() string {
	if e.Code == "" {
		return fmt.Sprintf("client: control plane answered %d", e.StatusCode)
	}
	return e.Code + ": " + e.Message
}

// errorResponse is the body of a non-2xx answer: the failure under "error".
type errorResponse struct {
	Failure json.RawMessage `json:"error"`
}

// newAPIError decodes one non-2xx answer. The field set is decoded on its own
// and assigned first, so a modeled field whose type drifted cannot cost the
// author the rest of the envelope; the typed view is then filled field by
// field. A body that is not the envelope still becomes an APIError carrying the
// status and the bytes, so no failure is invented on the way to the author.
func newAPIError(status int, body []byte) *APIError {
	failure := &APIError{StatusCode: status, Raw: append(json.RawMessage(nil), body...)}
	var response errorResponse
	if err := json.Unmarshal(body, &response); err != nil || len(response.Failure) == 0 {
		return failure
	}
	fields := map[string]json.RawMessage{}
	if err := json.Unmarshal(response.Failure, &fields); err != nil {
		return failure
	}
	failure.Fields = fields
	decodeField(fields, "code", &failure.Code)
	decodeField(fields, "message", &failure.Message)
	decodeField(fields, "stage", &failure.Stage)
	decodeField(fields, "object", &failure.Object)
	decodeField(fields, "retryable", &failure.Retryable)
	decodeField(fields, "requiresUserAction", &failure.RequiresUserAction)
	decodeField(fields, "sideEffects", &failure.SideEffects)
	decodeField(fields, "applied", &failure.Applied)
	decodeField(fields, "current", &failure.Current)
	decodeField(fields, "retryAfterSeconds", &failure.RetryAfterSeconds)
	decodeField(fields, "ownedObjects", &failure.OwnedObjects)
	return failure
}

// decodeField fills one modeled field from its raw bytes; a value whose type
// drifted leaves that field at zero while its bytes stay in Fields.
func decodeField(fields map[string]json.RawMessage, name string, target any) {
	if raw, found := fields[name]; found {
		_ = json.Unmarshal(raw, target)
	}
}
