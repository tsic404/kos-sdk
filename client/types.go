package client

import "time"

// ObjectRef identifies one domain object across reads and writes.
type ObjectRef struct {
	// Group is the object's API group.
	Group string `json:"group"`
	// Kind is the object's kind.
	Kind string `json:"kind"`
	// Namespace is empty for cluster-scoped kinds.
	Namespace string `json:"namespace,omitempty"`
	// Name is the object's name inside its scope.
	Name string `json:"name"`
}

// Projection is one object as the control plane observed it, with the time the
// observation was taken.
type Projection struct {
	// Ref identifies the projected object.
	Ref ObjectRef `json:"ref"`
	// ObservedAt is when the source observed this object.
	ObservedAt time.Time `json:"observedAt"`
	// Stale tells whether the observation was older than the freshness bound.
	Stale bool `json:"stale"`
	// Sources lists the discovery adapters that reported this object.
	Sources []string `json:"sources,omitempty"`
	// Object is the projected object itself.
	Object map[string]any `json:"object"`
}

// ListOptions is the paging and filtering one list request carries. Every field
// is sent as the author wrote it: the client adds no page limit of its own and
// follows no cursor on its own.
type ListOptions struct {
	// Namespace restricts the read to one namespace.
	Namespace string
	// APIVersion names the served version; empty leaves the choice to the API.
	APIVersion string
	// LabelSelector is the label selector the request carries.
	LabelSelector string
	// TargetApp is the reverse lookup of the endpoints exposing one application.
	TargetApp string
	// Limit is the page size; zero sends no limit.
	Limit int
	// Continue is the cursor a previous answer returned; empty starts at the top.
	Continue string
}

// ListResult is one page of projections plus the cursor of the next page. The
// cursor is returned as sent; nothing follows it.
type ListResult struct {
	Items    []Projection `json:"items"`
	Continue string       `json:"continue,omitempty"`
}

// WriteResult is the answer to a write: acceptance plus the object the write
// targets. Progress is read from the object, not from this answer.
type WriteResult struct {
	// Accepted is always true on a successful answer.
	Accepted bool `json:"accepted"`
	// Ref identifies the object the write targets.
	Ref *ObjectRef `json:"ref,omitempty"`
	// Idempotent tells that the desired state was already in place.
	Idempotent bool `json:"idempotent,omitempty"`
}
