// Package sdktc2 verifies that the author client hands the control plane's
// failure envelope through unchanged, returns exactly the page the author asked
// for, and repeats a write only when the author does (SDK-AC-2, SDK-TC-2).
package sdktc2

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tsic404/kos-sdk/client"
)

const (
	// kernelGroup is the API group the kernel object classes belong to.
	kernelGroup = "kos.tsic.top"
	// kind is the object class these cases read and write.
	kind = "Application"
	// scope is the namespace the cases work in.
	scope = "default"
	// resourcePath is where one object class is served.
	resourcePath = "/api/v1/resources/" + kind
	// defaultPageSize is the page the control plane returns without a limit.
	defaultPageSize = 50
	// seedVersion is the resource version an object stored without a request
	// carries.
	seedVersion = "42"
)

// observedAt is the observation time every projection carries.
var observedAt = time.Date(2026, 11, 5, 4, 0, 0, 0, time.UTC)

// objectRef is the reference shape the control plane sends.
type objectRef struct {
	Group     string `json:"group"`
	Kind      string `json:"kind"`
	Namespace string `json:"namespace,omitempty"`
	Name      string `json:"name"`
}

// failure is the failure envelope the control plane sends (KOSD-C-12).
type failure struct {
	Code               string      `json:"code"`
	Message            string      `json:"message"`
	Stage              string      `json:"stage"`
	Object             *objectRef  `json:"object,omitempty"`
	Retryable          bool        `json:"retryable"`
	RequiresUserAction bool        `json:"requiresUserAction"`
	SideEffects        string      `json:"sideEffects"`
	Applied            []objectRef `json:"applied,omitempty"`
	// Detail is a field the client's type does not model; a major version may
	// add one (KOSD-C-12), so the author must still receive it.
	Detail string `json:"detail,omitempty"`
}

// errorAnswer is the body of a non-2xx answer.
type errorAnswer struct {
	Error failure `json:"error"`
}

// writeAnswer is the body of an accepted write.
type writeAnswer struct {
	Accepted   bool       `json:"accepted"`
	Ref        *objectRef `json:"ref,omitempty"`
	Idempotent bool       `json:"idempotent,omitempty"`
}

// listAnswer is the body of a list answer.
type listAnswer struct {
	Items    []map[string]any `json:"items"`
	Continue string           `json:"continue,omitempty"`
}

// recordedRequest is one call the control plane received.
type recordedRequest struct {
	// Method is the HTTP method the call carried.
	Method string
	// Path is the path the call carried.
	Path string
	// Query is the parameters the call carried.
	Query url.Values
	// Header is the request headers the call carried.
	Header http.Header
	// Body is the object a write submitted.
	Body map[string]any
}

// recordedAnswer is the response written for one call.
type recordedAnswer struct {
	// Status is the HTTP status written.
	Status int
	// Body is the bytes written.
	Body []byte
}

// controlPlane is the API the client talks to. It records every request and
// every answer, so a case can judge both what reached the control plane and
// what the author was handed.
type controlPlane struct {
	mu          sync.Mutex
	requests    []recordedRequest
	answers     []recordedAnswer
	objects     map[string]map[string]any
	versions    map[string]string
	refusals    map[string]failure
	rawRefusals map[string]recordedAnswer
	version     int
	url         string
}

// newControlPlane starts one control plane for the case to talk to.
func newControlPlane(t *testing.T) *controlPlane {
	t.Helper()
	plane := &controlPlane{
		objects:     map[string]map[string]any{},
		versions:    map[string]string{},
		refusals:    map[string]failure{},
		rawRefusals: map[string]recordedAnswer{},
	}
	server := httptest.NewServer(http.HandlerFunc(plane.serve))
	t.Cleanup(server.Close)
	plane.url = server.URL
	return plane
}

// serve records one call, answers it, and records the answer.
func (p *controlPlane) serve(w http.ResponseWriter, r *http.Request) {
	body := []byte{}
	if r.Body != nil {
		read, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		body = read
	}
	request := recordedRequest{Method: r.Method, Path: r.URL.Path, Query: r.URL.Query(), Header: r.Header.Clone()}
	if len(bytes.TrimSpace(body)) > 0 {
		if err := json.Unmarshal(body, &request.Body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	p.mu.Lock()
	status, answer := p.answer(request)
	p.requests = append(p.requests, request)
	p.answers = append(p.answers, recordedAnswer{Status: status, Body: answer})
	p.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(answer)
}

// answer routes one call; the caller holds the lock.
func (p *controlPlane) answer(request recordedRequest) (int, []byte) {
	if request.Path == resourcePath {
		switch request.Method {
		case http.MethodGet:
			return p.list(request)
		case http.MethodPost:
			return p.create(request)
		}
	}
	if name, found := strings.CutPrefix(request.Path, resourcePath+"/"); found {
		switch request.Method {
		case http.MethodGet:
			return p.read(request, name)
		case http.MethodPut:
			return p.update(request, name)
		case http.MethodDelete:
			return p.remove(request, name)
		}
	}
	return http.StatusNotFound, encode(errorAnswer{Error: unknownRoute(request.Path)})
}

// list answers one list call with the page the request asked for; the caller
// holds the lock.
func (p *controlPlane) list(request recordedRequest) (int, []byte) {
	namespace := request.Query.Get("namespace")
	limit := defaultPageSize
	if raw := request.Query.Get("limit"); raw != "" {
		size, err := strconv.Atoi(raw)
		if err != nil || size < 1 {
			return http.StatusBadRequest, encode(errorAnswer{Error: invalidField("limit 必须是正整数")})
		}
		limit = size
	}
	keys := make([]string, 0, len(p.objects))
	for key, object := range p.objects {
		if !strings.HasPrefix(key, namespace+"/") || !matchesSelector(object, request.Query.Get("labelSelector")) {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	start := 0
	if cursor := request.Query.Get("continue"); cursor != "" {
		last, err := decodeCursor(cursor)
		if err != nil {
			return http.StatusBadRequest, encode(errorAnswer{Error: invalidField("continue 不是合法的分页游标")})
		}
		start = sort.Search(len(keys), func(index int) bool { return keys[index] > last })
	}
	end := min(start+limit, len(keys))
	answer := listAnswer{Items: make([]map[string]any, 0, end-start)}
	for _, key := range keys[start:end] {
		answer.Items = append(answer.Items, p.projection(key))
	}
	if end < len(keys) && len(answer.Items) > 0 {
		answer.Continue = encodeCursor(keys[end-1])
	}
	return http.StatusOK, encode(answer)
}

// create answers one write. A write the case refused fails once, and a repeat of
// the same write is the same result rather than a second object.
func (p *controlPlane) create(request recordedRequest) (int, []byte) {
	namespace := request.Query.Get("namespace")
	name := nestedName(request.Body)
	key := namespace + "/" + name
	if answer, refused := p.rawRefusals[key]; refused {
		return answer.Status, answer.Body
	}
	if refusal, refused := p.refusals[key]; refused {
		return statusOf(refusal.Code), encode(errorAnswer{Error: refusal})
	}
	ref := objectRef{Group: kernelGroup, Kind: kind, Namespace: namespace, Name: name}
	if _, exists := p.objects[key]; exists {
		return http.StatusAccepted, encode(writeAnswer{Accepted: true, Ref: &ref, Idempotent: true})
	}
	p.objects[key] = request.Body
	p.bumpVersion(key)
	return http.StatusAccepted, encode(writeAnswer{Accepted: true, Ref: &ref})
}

// update replaces one stored object behind the author's precondition.
func (p *controlPlane) update(request recordedRequest, name string) (int, []byte) {
	namespace := request.Query.Get("namespace")
	key := namespace + "/" + name
	if answer, refused := p.preconditionFailure(request, key); refused {
		return answer.Status, answer.Body
	}
	ref := objectRef{Group: kernelGroup, Kind: kind, Namespace: namespace, Name: name}
	p.objects[key] = request.Body
	p.bumpVersion(key)
	return http.StatusAccepted, encode(writeAnswer{Accepted: true, Ref: &ref})
}

// remove deletes one stored object behind the author's precondition.
func (p *controlPlane) remove(request recordedRequest, name string) (int, []byte) {
	namespace := request.Query.Get("namespace")
	key := namespace + "/" + name
	if answer, refused := p.preconditionFailure(request, key); refused {
		return answer.Status, answer.Body
	}
	ref := objectRef{Group: kernelGroup, Kind: kind, Namespace: namespace, Name: name}
	delete(p.objects, key)
	delete(p.versions, key)
	return http.StatusAccepted, encode(writeAnswer{Accepted: true, Ref: &ref})
}

// preconditionFailure judges one write the way a standard HTTP server judges
// If-Match: a value the author sent must equal the object's current entity-tag.
func (p *controlPlane) preconditionFailure(request recordedRequest, key string) (recordedAnswer, bool) {
	namespace, name, _ := strings.Cut(key, "/")
	if _, exists := p.objects[key]; !exists {
		return recordedAnswer{Status: http.StatusNotFound, Body: encode(errorAnswer{Error: notFound(namespace, name)})}, true
	}
	if match := request.Header.Get("If-Match"); match != "" && match != entityTag(p.versions[key]) {
		return recordedAnswer{Status: http.StatusConflict, Body: encode(errorAnswer{Error: conflict(namespace, name)})}, true
	}
	return recordedAnswer{}, false
}

// bumpVersion records that one write changed the object's resource version.
func (p *controlPlane) bumpVersion(key string) {
	p.version++
	p.versions[key] = strconv.Itoa(p.version)
}

// entityTag renders one resource version as the quoted entity-tag RFC 9110
// §13.1.1 compares.
func entityTag(version string) string {
	return `"` + version + `"`
}

// read answers one single-object read.
func (p *controlPlane) read(request recordedRequest, name string) (int, []byte) {
	namespace := request.Query.Get("namespace")
	key := namespace + "/" + name
	if _, exists := p.objects[key]; !exists {
		return http.StatusNotFound, encode(errorAnswer{Error: notFound(namespace, name)})
	}
	return http.StatusOK, encode(p.projection(key))
}

// projection renders one stored object as the control plane projects it.
func (p *controlPlane) projection(key string) map[string]any {
	namespace, name, _ := strings.Cut(key, "/")
	return map[string]any{
		"ref":        objectRef{Group: kernelGroup, Kind: kind, Namespace: namespace, Name: name},
		"observedAt": observedAt,
		"stale":      false,
		"object":     p.objects[key],
	}
}

// seed stores one object without a request, as an earlier write would have.
func (p *controlPlane) seed(namespace, name string, labels map[string]string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	object := application(name)
	if labels != nil {
		object["metadata"].(map[string]any)["labels"] = labels
	}
	p.objects[namespace+"/"+name] = object
	p.versions[namespace+"/"+name] = seedVersion
}

// refuse makes one write fail with the given envelope.
func (p *controlPlane) refuse(namespace, name string, refusal failure) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.refusals[namespace+"/"+name] = refusal
}

// refuseRaw makes one write fail with an answer body written byte for byte, for
// an envelope whose field types the case does not model.
func (p *controlPlane) refuseRaw(namespace, name string, status int, body []byte) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.rawRefusals[namespace+"/"+name] = recordedAnswer{Status: status, Body: body}
}

// requestAt returns the call at one position.
func (p *controlPlane) requestAt(t *testing.T, index int) recordedRequest {
	t.Helper()
	p.mu.Lock()
	defer p.mu.Unlock()
	if index >= len(p.requests) {
		t.Fatalf("control plane received %d calls, wanted one at %d", len(p.requests), index)
	}
	return p.requests[index]
}

// answerAt returns the answer written at one position.
func (p *controlPlane) answerAt(t *testing.T, index int) recordedAnswer {
	t.Helper()
	p.mu.Lock()
	defer p.mu.Unlock()
	if index >= len(p.answers) {
		t.Fatalf("control plane wrote %d answers, wanted one at %d", len(p.answers), index)
	}
	return p.answers[index]
}

// callCount returns how many calls the control plane received.
func (p *controlPlane) callCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.requests)
}

// count returns how many calls one method and path reached the control plane.
func (p *controlPlane) count(method, path string) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	total := 0
	for _, request := range p.requests {
		if request.Method == method && request.Path == path {
			total++
		}
	}
	return total
}

// objectCount returns how many objects the control plane holds.
func (p *controlPlane) objectCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.objects)
}

// TestFailureEnvelopeReachesTheAuthorUnchanged checks both failure classes the
// design names: a read of an absent object and a write the caller may not make.
func TestFailureEnvelopeReachesTheAuthorUnchanged(t *testing.T) {
	plane := newControlPlane(t)
	plane.refuse(scope, "web", permissionDenied(scope, "web"))
	author := newAuthor(t, plane)

	_, missing := author.Get(context.Background(), kind, scope, "missing")
	assertEnvelope(t, plane, 0, missing, "not_found")
	assertUnmodeledField(t, plane, 0, missing, "detail")

	_, denied := author.Create(context.Background(), kind, scope, application("web"))
	assertEnvelope(t, plane, 1, denied, "permission_denied")
	assertUnmodeledField(t, plane, 1, denied, "detail")

	if got := plane.count(http.MethodPost, resourcePath); got != 1 {
		t.Fatalf("a refused write reached the control plane %d times, want 1", got)
	}
}

// TestRetryableFailureIsNotRetried checks that the flag survives decoding and
// that the client still sends the write once.
func TestRetryableFailureIsNotRetried(t *testing.T) {
	plane := newControlPlane(t)
	plane.refuse(scope, "web", writeFailed(scope, "web"))
	author := newAuthor(t, plane)

	_, err := author.Create(context.Background(), kind, scope, application("web"))
	var apiErr *client.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("author got %v, want the control plane's failure envelope", err)
	}
	if apiErr.Code != "write_failed" || !apiErr.Retryable {
		t.Fatalf("author got code %q retryable %v, want write_failed retryable", apiErr.Code, apiErr.Retryable)
	}
	if apiErr.SideEffects != "applied" || len(apiErr.Applied) != 1 {
		t.Fatalf("side effects %q with %d applied objects, want applied with 1", apiErr.SideEffects, len(apiErr.Applied))
	}
	if got := plane.count(http.MethodPost, resourcePath); got != 1 {
		t.Fatalf("a retryable failure produced %d requests, want 1", got)
	}
}

// TestListReturnsThePageTheAuthorAskedFor checks that the page parameters travel
// as given and that the answer is that page, not a merged listing.
func TestListReturnsThePageTheAuthorAskedFor(t *testing.T) {
	plane := newControlPlane(t)
	for _, name := range []string{"app-1", "app-2", "app-3", "app-4", "app-5"} {
		plane.seed(scope, name, nil)
	}
	author := newAuthor(t, plane)

	first, err := author.List(context.Background(), kind, client.ListOptions{Namespace: scope, Limit: 2})
	if err != nil {
		t.Fatalf("list first page: %v", err)
	}
	assertNames(t, first.Items, "app-1", "app-2")
	if first.Continue == "" {
		t.Fatalf("first page carries no cursor, so no second page exists")
	}
	if got := plane.callCount(); got != 1 {
		t.Fatalf("one list call reached the control plane %d times, want 1", got)
	}
	assertQuery(t, plane, 0, map[string]string{"namespace": scope, "limit": "2"})
	assertAnswerPage(t, plane, 0, first)

	second, err := author.List(context.Background(), kind, client.ListOptions{Namespace: scope, Limit: 2, Continue: first.Continue})
	if err != nil {
		t.Fatalf("list second page: %v", err)
	}
	assertNames(t, second.Items, "app-3", "app-4")
	assertQuery(t, plane, 1, map[string]string{"namespace": scope, "limit": "2", "continue": first.Continue})
	assertAnswerPage(t, plane, 1, second)
	if got := plane.callCount(); got != 2 {
		t.Fatalf("two list calls reached the control plane %d times, want 2: the client followed the cursor on its own", got)
	}
	if second.Continue == first.Continue {
		t.Fatalf("the second page returned the first page's cursor %q", second.Continue)
	}
}

// TestListSendsTheAuthorsFilter checks that the filter is sent verbatim and that
// the client invents no page limit of its own.
func TestListSendsTheAuthorsFilter(t *testing.T) {
	plane := newControlPlane(t)
	plane.seed(scope, "web", map[string]string{"tier": "web"})
	plane.seed(scope, "db", map[string]string{"tier": "db"})
	author := newAuthor(t, plane)

	page, err := author.List(context.Background(), kind, client.ListOptions{Namespace: scope, LabelSelector: "tier=web"})
	if err != nil {
		t.Fatalf("list with a filter: %v", err)
	}
	assertNames(t, page.Items, "web")
	assertQuery(t, plane, 0, map[string]string{"namespace": scope, "labelSelector": "tier=web"})
	if raw := plane.requestAt(t, 0).Query.Get("limit"); raw != "" {
		t.Fatalf("the client invented a page limit %q, want none", raw)
	}
}

// TestRepeatedWriteReachesTheControlPlaneTwice checks that the same write sent
// twice is two requests and one object, as the control plane judges it.
func TestRepeatedWriteReachesTheControlPlaneTwice(t *testing.T) {
	plane := newControlPlane(t)
	author := newAuthor(t, plane)
	object := application("web")

	first, err := author.Create(context.Background(), kind, scope, object)
	if err != nil {
		t.Fatalf("first write: %v", err)
	}
	second, err := author.Create(context.Background(), kind, scope, object)
	if err != nil {
		t.Fatalf("repeat write: %v", err)
	}

	if !first.Accepted || !second.Accepted {
		t.Fatalf("accepted = %v then %v, want true both times", first.Accepted, second.Accepted)
	}
	if first.Ref == nil || second.Ref == nil || *first.Ref != *second.Ref {
		t.Fatalf("write refs %v and %v differ, want the same object", first.Ref, second.Ref)
	}
	if first.Idempotent {
		t.Errorf("the first write reported the desired state was already in place")
	}
	if !second.Idempotent {
		t.Errorf("the repeat was not reported idempotent, so the control plane judged it a second write")
	}
	if got := plane.count(http.MethodPost, resourcePath); got != 2 {
		t.Fatalf("the same write reached the control plane %d times, want 2", got)
	}
	if got := plane.objectCount(); got != 1 {
		t.Fatalf("the control plane holds %d objects after the same write twice, want 1", got)
	}
}

// TestFailureEnvelopeWithADriftedFieldStillReachesTheAuthor checks that one
// modeled field sent with another type does not cost the author the code, the
// stage, the retryable flag, or the field set.
func TestFailureEnvelopeWithADriftedFieldStillReachesTheAuthor(t *testing.T) {
	plane := newControlPlane(t)
	body := []byte(`{"error":{"code":"conflict","message":"版本冲突","stage":"apply","retryable":true,"requiresUserAction":false,"sideEffects":"none","retryAfterSeconds":"30"}}`)
	plane.refuseRaw(scope, "web", http.StatusConflict, body)
	author := newAuthor(t, plane)

	_, err := author.Create(context.Background(), kind, scope, application("web"))
	assertEnvelope(t, plane, 0, err, "conflict")
	var apiErr *client.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("author got %v, want the control plane's failure envelope", err)
	}
	raw, found := apiErr.Fields["retryAfterSeconds"]
	if !found {
		t.Fatalf("author's field set drops retryAfterSeconds")
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil || value != "30" {
		t.Errorf("retryAfterSeconds = %s, want the string \"30\" as sent", raw)
	}
}

// TestUpdateCarriesTheAuthorsPrecondition checks that an update names the
// object it read and sends the version as the entity-tag RFC 9110 requires.
func TestUpdateCarriesTheAuthorsPrecondition(t *testing.T) {
	plane := newControlPlane(t)
	plane.seed(scope, "web", nil)
	author := newAuthor(t, plane)
	object := application("web")
	object["spec"] = map[string]any{"marker": "updated"}

	result, err := author.Update(context.Background(), kind, scope, "web", object, seedVersion)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	want := client.ObjectRef{Group: kernelGroup, Kind: kind, Namespace: scope, Name: "web"}
	if !result.Accepted || result.Ref == nil || *result.Ref != want {
		t.Fatalf("update answer = %+v, want accepted for %v", result, want)
	}
	request := plane.requestAt(t, 0)
	if request.Method != http.MethodPut || request.Path != resourcePath+"/web" {
		t.Errorf("update reached the control plane as %s %s, want PUT %s/web", request.Method, request.Path, resourcePath)
	}
	if got := request.Header.Get("If-Match"); got != `"42"` {
		t.Errorf("If-Match = %q, want the entity-tag %q", got, `"42"`)
	}
	assertQuery(t, plane, 0, map[string]string{"namespace": scope})
	if got := plane.count(http.MethodPut, resourcePath+"/web"); got != 1 {
		t.Errorf("update reached the control plane %d times, want 1", got)
	}
}

// TestDeleteCarriesTheAuthorsPrecondition checks that a removal sends the
// author's version as an entity-tag and decodes the answer.
func TestDeleteCarriesTheAuthorsPrecondition(t *testing.T) {
	plane := newControlPlane(t)
	plane.seed(scope, "web", nil)
	author := newAuthor(t, plane)

	result, err := author.Delete(context.Background(), kind, scope, "web", seedVersion)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	want := client.ObjectRef{Group: kernelGroup, Kind: kind, Namespace: scope, Name: "web"}
	if !result.Accepted || result.Ref == nil || *result.Ref != want {
		t.Fatalf("delete answer = %+v, want accepted for %v", result, want)
	}
	request := plane.requestAt(t, 0)
	if request.Method != http.MethodDelete || request.Path != resourcePath+"/web" {
		t.Errorf("delete reached the control plane as %s %s, want DELETE %s/web", request.Method, request.Path, resourcePath)
	}
	if got := request.Header.Get("If-Match"); got != `"42"` {
		t.Errorf("If-Match = %q, want the entity-tag %q", got, `"42"`)
	}
	if got := plane.objectCount(); got != 0 {
		t.Errorf("the control plane holds %d objects after the delete, want 0", got)
	}
}

// TestWriteWithoutAPreconditionSendsNone checks that an author who read no
// version writes unconditionally and the client invents no If-Match.
func TestWriteWithoutAPreconditionSendsNone(t *testing.T) {
	plane := newControlPlane(t)
	plane.seed(scope, "web", nil)
	author := newAuthor(t, plane)

	if _, err := author.Delete(context.Background(), kind, scope, "web", ""); err != nil {
		t.Fatalf("delete without a resource version: %v", err)
	}
	if values, found := plane.requestAt(t, 0).Header["If-Match"]; found {
		t.Errorf("delete without a resource version sent If-Match %q, want none", values)
	}
	if got := plane.objectCount(); got != 0 {
		t.Errorf("the control plane holds %d objects after the delete, want 0", got)
	}
}

// TestStalePreconditionReachesTheAuthor checks that a stale version fails with
// the control plane's conflict envelope, unchanged and sent once.
func TestStalePreconditionReachesTheAuthor(t *testing.T) {
	plane := newControlPlane(t)
	plane.seed(scope, "web", nil)
	author := newAuthor(t, plane)

	_, err := author.Update(context.Background(), kind, scope, "web", application("web"), "41")
	assertEnvelope(t, plane, 0, err, "conflict")
	if got := plane.count(http.MethodPut, resourcePath+"/web"); got != 1 {
		t.Errorf("a failed update reached the control plane %d times, want 1", got)
	}
	if got := plane.objectCount(); got != 1 {
		t.Errorf("the control plane holds %d objects after the refused update, want the original 1", got)
	}
}

// assertEnvelope compares the author's failure with the bytes the control plane
// wrote for that call.
func assertEnvelope(t *testing.T, plane *controlPlane, index int, err error, wantCode string) {
	t.Helper()
	var apiErr *client.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("author got %v, want an APIError carrying the control plane's envelope", err)
	}
	answer := plane.answerAt(t, index)
	if apiErr.StatusCode != answer.Status {
		t.Errorf("status = %d, want the control plane's %d", apiErr.StatusCode, answer.Status)
	}
	if !bytes.Equal(apiErr.Raw, answer.Body) {
		t.Errorf("raw body = %s, want the bytes the control plane wrote %s", apiErr.Raw, answer.Body)
	}
	sent, sentFields := decodeEnvelope(t, answer.Body)
	if sent.Code != wantCode {
		t.Fatalf("control plane sent code %q, the case expects %q", sent.Code, wantCode)
	}
	if apiErr.Code != wantCode {
		t.Errorf("code = %q, want %q unchanged", apiErr.Code, wantCode)
	}
	assertFields(t, sentFields, apiErr.Fields)
	if apiErr.Message != sent.Message || apiErr.Stage != sent.Stage || apiErr.SideEffects != sent.SideEffects {
		t.Errorf("author got message %q stage %q sideEffects %q, want %q %q %q",
			apiErr.Message, apiErr.Stage, apiErr.SideEffects, sent.Message, sent.Stage, sent.SideEffects)
	}
	if apiErr.Retryable != sent.Retryable || apiErr.RequiresUserAction != sent.RequiresUserAction {
		t.Errorf("author got retryable %v requiresUserAction %v, want %v %v",
			apiErr.Retryable, apiErr.RequiresUserAction, sent.Retryable, sent.RequiresUserAction)
	}
	if !reflect.DeepEqual(apiErr.Object, wireRef(sent.Object)) {
		t.Errorf("author got object %v, want %v", apiErr.Object, sent.Object)
	}
}

// assertFields compares the author's field set with the control plane's.
func assertFields(t *testing.T, sent, received map[string]json.RawMessage) {
	t.Helper()
	if len(received) != len(sent) {
		t.Errorf("author's error carries %d fields, the control plane sent %d", len(received), len(sent))
	}
	for name, raw := range sent {
		other, found := received[name]
		if !found {
			t.Errorf("author's error drops %q", name)
			continue
		}
		var want, got any
		if err := json.Unmarshal(raw, &want); err != nil {
			t.Fatalf("decode %q: %v", name, err)
		}
		if err := json.Unmarshal(other, &got); err != nil {
			t.Fatalf("decode %q: %v", name, err)
		}
		if !reflect.DeepEqual(want, got) {
			t.Errorf("field %q = %v, want %v", name, got, want)
		}
	}
	for name := range received {
		if _, found := sent[name]; !found {
			t.Errorf("author's error carries %q, which the control plane never sent", name)
		}
	}
}

// assertUnmodeledField checks that a field the client's type does not model
// reaches the author with the bytes the control plane wrote for it. assertFields
// already compares the two field sets both ways; this case names the field whose
// loss a subset would hide.
func assertUnmodeledField(t *testing.T, plane *controlPlane, index int, err error, name string) {
	t.Helper()
	var apiErr *client.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("author got %v, want an APIError carrying the control plane's envelope", err)
	}
	_, sent := decodeEnvelope(t, plane.answerAt(t, index).Body)
	want, carried := sent[name]
	if !carried {
		t.Errorf("the control plane sent no %q, so this case cannot judge it", name)
		return
	}
	got, found := apiErr.Fields[name]
	if !found {
		t.Errorf("author's error drops %q, a field the client does not model; its fields = %v", name, fieldNames(apiErr.Fields))
		return
	}
	if !bytes.Equal(got, want) {
		t.Errorf("field %q = %s, want the bytes the control plane wrote %s", name, got, want)
	}
}

// fieldNames lists one envelope's field names in sorted order.
func fieldNames(fields map[string]json.RawMessage) []string {
	names := make([]string, 0, len(fields))
	for name := range fields {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// decodeEnvelope splits one failure answer into its typed envelope and its
// field set.
func decodeEnvelope(t *testing.T, body []byte) (failure, map[string]json.RawMessage) {
	t.Helper()
	var answer struct {
		Error json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal(body, &answer); err != nil {
		t.Fatalf("decode failure answer %s: %v", body, err)
	}
	var sent failure
	if err := json.Unmarshal(answer.Error, &sent); err != nil {
		t.Fatalf("decode failure envelope %s: %v", answer.Error, err)
	}
	fields := map[string]json.RawMessage{}
	if err := json.Unmarshal(answer.Error, &fields); err != nil {
		t.Fatalf("decode failure fields %s: %v", answer.Error, err)
	}
	return sent, fields
}

// assertAnswerPage checks that the author's page is the page the control plane
// wrote for that call.
func assertAnswerPage(t *testing.T, plane *controlPlane, index int, page client.ListResult) {
	t.Helper()
	var answer struct {
		Items []struct {
			Ref objectRef `json:"ref"`
		} `json:"items"`
	}
	if err := json.Unmarshal(plane.answerAt(t, index).Body, &answer); err != nil {
		t.Fatalf("decode list answer: %v", err)
	}
	if len(answer.Items) != len(page.Items) {
		t.Fatalf("author got %d items, the control plane answered %d", len(page.Items), len(answer.Items))
	}
	for position, item := range answer.Items {
		got := page.Items[position].Ref
		if got != (client.ObjectRef(item.Ref)) {
			t.Errorf("item %d = %v, want %v", position, got, item.Ref)
		}
	}
}

// assertNames checks the projected object names in order.
func assertNames(t *testing.T, items []client.Projection, want ...string) {
	t.Helper()
	got := make([]string, 0, len(items))
	for _, item := range items {
		got = append(got, item.Ref.Name)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("page names = %v, want %v", got, want)
	}
}

// assertQuery checks the parameters one call carried.
func assertQuery(t *testing.T, plane *controlPlane, index int, want map[string]string) {
	t.Helper()
	query := plane.requestAt(t, index).Query
	for name, value := range want {
		if got := query.Get(name); got != value {
			t.Errorf("call %d sent %s=%q, want %q", index, name, got, value)
		}
	}
	for name := range query {
		if _, expected := want[name]; !expected {
			t.Errorf("call %d sent unexpected parameter %s=%q", index, name, query.Get(name))
		}
	}
}

// newAuthor builds the client under test.
func newAuthor(t *testing.T, plane *controlPlane) *client.Client {
	t.Helper()
	author, err := client.New(plane.url)
	if err != nil {
		t.Fatalf("build client: %v", err)
	}
	return author
}

// application is the object a case submits.
func application(name string) map[string]any {
	return map[string]any{
		"apiVersion": kernelGroup + "/v1beta1",
		"kind":       kind,
		"metadata":   map[string]any{"name": name, "namespace": scope},
		"spec":       map[string]any{},
	}
}

// nestedName reads metadata.name out of a submitted object.
func nestedName(object map[string]any) string {
	metadata, _ := object["metadata"].(map[string]any)
	name, _ := metadata["name"].(string)
	return name
}

// matchesSelector answers whether one object carries every selector term.
func matchesSelector(object map[string]any, selector string) bool {
	if selector == "" {
		return true
	}
	metadata, _ := object["metadata"].(map[string]any)
	labels, _ := metadata["labels"].(map[string]string)
	for _, term := range strings.Split(selector, ",") {
		key, value, found := strings.Cut(term, "=")
		if !found || labels[key] != value {
			return false
		}
	}
	return true
}

// wireRef converts an envelope reference into the author-facing type.
func wireRef(ref *objectRef) *client.ObjectRef {
	if ref == nil {
		return nil
	}
	return &client.ObjectRef{Group: ref.Group, Kind: ref.Kind, Namespace: ref.Namespace, Name: ref.Name}
}

// statusOf maps a stable code to the status the control plane answers with.
func statusOf(code string) int {
	switch code {
	case "not_found":
		return http.StatusNotFound
	case "invalid_field":
		return http.StatusBadRequest
	case "permission_denied":
		return http.StatusForbidden
	case "write_failed":
		return http.StatusBadGateway
	case "conflict":
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}

// notFound is the failure of a read that names an absent object.
func notFound(namespace, name string) failure {
	return failure{
		Code:               "not_found",
		Message:            "对象 " + kernelGroup + "/" + kind + "/" + namespace + "/" + name + " 不存在",
		Stage:              "validate",
		Object:             &objectRef{Group: kernelGroup, Kind: kind, Namespace: namespace, Name: name},
		RequiresUserAction: true,
		SideEffects:        "none",
		Detail:             "quota 3/3",
	}
}

// permissionDenied is the failure of a write the caller may not make.
func permissionDenied(namespace, name string) failure {
	return failure{
		Code:               "permission_denied",
		Message:            "当前身份不能执行写操作",
		Stage:              "authorize",
		Object:             &objectRef{Group: kernelGroup, Kind: kind, Namespace: namespace, Name: name},
		RequiresUserAction: true,
		SideEffects:        "none",
		Detail:             "quota 3/3",
	}
}

// writeFailed is a retryable failure that left objects behind.
func writeFailed(namespace, name string) failure {
	ref := objectRef{Group: kernelGroup, Kind: kind, Namespace: namespace, Name: name}
	return failure{
		Code:        "write_failed",
		Message:     "写入失败",
		Stage:       "apply",
		Object:      &ref,
		Retryable:   true,
		SideEffects: "applied",
		Applied:     []objectRef{ref},
	}
}

// conflict is the failure of a write whose precondition no longer holds.
func conflict(namespace, name string) failure {
	return failure{
		Code:               "conflict",
		Message:            "资源版本已变化",
		Stage:              "apply",
		Object:             &objectRef{Group: kernelGroup, Kind: kind, Namespace: namespace, Name: name},
		RequiresUserAction: true,
		SideEffects:        "none",
	}
}

// invalidField is the failure of a request parameter the model rejects.
func invalidField(message string) failure {
	return failure{
		Code:               "invalid_field",
		Message:            message,
		Stage:              "validate",
		RequiresUserAction: true,
		SideEffects:        "none",
	}
}

// unknownRoute is the failure of a path the control plane does not serve.
func unknownRoute(path string) failure {
	return failure{
		Code:               "not_found",
		Message:            "没有这个接口：" + path,
		Stage:              "validate",
		RequiresUserAction: true,
		SideEffects:        "none",
	}
}

// encodeCursor encodes the position a page ended at.
func encodeCursor(key string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(key))
}

// decodeCursor reads the position a cursor names.
func decodeCursor(cursor string) (string, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(cursor)
	return string(decoded), err
}

// encode writes one answer body.
func encode(value any) []byte {
	encoded, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return encoded
}
