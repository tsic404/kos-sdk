// Package client is the author-side exit of the control plane API. One call
// builds one request and decodes one answer: the client keeps no state, never
// retries, and never pages on its own. A failure reaches the author as the
// envelope the control plane sent, field for field.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// Client calls one control plane. It holds no state between calls and is safe
// for concurrent use.
type Client struct {
	baseURL string
	http    *http.Client
	headers http.Header
}

// Option configures a Client.
type Option func(*Client)

// WithHTTPClient replaces the HTTP client a call uses; nil keeps the default.
func WithHTTPClient(httpClient *http.Client) Option {
	return func(client *Client) {
		if httpClient != nil {
			client.http = httpClient
		}
	}
}

// WithHeader adds one header to every request, for a session or a trace id.
func WithHeader(name, value string) Option {
	return func(client *Client) { client.headers.Add(name, value) }
}

// New builds a client for the control plane at baseURL.
func New(baseURL string, options ...Option) (*Client, error) {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return nil, fmt.Errorf("client: base URL %q: %w", baseURL, err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("client: base URL %q needs a scheme and a host", baseURL)
	}
	client := &Client{
		baseURL: strings.TrimRight(parsed.String(), "/"),
		http:    http.DefaultClient,
		headers: http.Header{},
	}
	for _, option := range options {
		option(client)
	}
	return client, nil
}

// List returns one page of kind. The page and its filter are the author's:
// Limit and Continue are sent as given, and the answer's cursor is returned
// without being followed.
func (c *Client) List(ctx context.Context, kind string, options ListOptions) (ListResult, error) {
	query := url.Values{}
	setWhenNotEmpty(query, "namespace", options.Namespace)
	setWhenNotEmpty(query, "apiVersion", options.APIVersion)
	setWhenNotEmpty(query, "labelSelector", options.LabelSelector)
	setWhenNotEmpty(query, "targetApp", options.TargetApp)
	setWhenNotEmpty(query, "continue", options.Continue)
	if options.Limit > 0 {
		query.Set("limit", strconv.Itoa(options.Limit))
	}
	var page ListResult
	if err := c.do(ctx, http.MethodGet, resourcePath(kind, ""), query, nil, nil, &page); err != nil {
		return ListResult{}, err
	}
	return page, nil
}

// Get returns one object as the control plane observed it.
func (c *Client) Get(ctx context.Context, kind, namespace, name string) (Projection, error) {
	query := url.Values{}
	setWhenNotEmpty(query, "namespace", namespace)
	var projection Projection
	if err := c.do(ctx, http.MethodGet, resourcePath(kind, name), query, nil, nil, &projection); err != nil {
		return Projection{}, err
	}
	return projection, nil
}

// Create submits one object. A repeated submission is a second request: the
// client neither caches its result nor decides that the write already ran.
func (c *Client) Create(ctx context.Context, kind, namespace string, object any) (WriteResult, error) {
	query := url.Values{}
	setWhenNotEmpty(query, "namespace", namespace)
	var result WriteResult
	if err := c.do(ctx, http.MethodPost, resourcePath(kind, ""), query, object, nil, &result); err != nil {
		return WriteResult{}, err
	}
	return result, nil
}

// Update rewrites one object behind the resource version the author read.
func (c *Client) Update(ctx context.Context, kind, namespace, name string, object any, resourceVersion string) (WriteResult, error) {
	query := url.Values{}
	setWhenNotEmpty(query, "namespace", namespace)
	headers := precondition(resourceVersion)
	var result WriteResult
	if err := c.do(ctx, http.MethodPut, resourcePath(kind, name), query, object, headers, &result); err != nil {
		return WriteResult{}, err
	}
	return result, nil
}

// Delete removes one object behind the resource version the author read.
func (c *Client) Delete(ctx context.Context, kind, namespace, name, resourceVersion string) (WriteResult, error) {
	query := url.Values{}
	setWhenNotEmpty(query, "namespace", namespace)
	headers := precondition(resourceVersion)
	var result WriteResult
	if err := c.do(ctx, http.MethodDelete, resourcePath(kind, name), query, nil, headers, &result); err != nil {
		return WriteResult{}, err
	}
	return result, nil
}

// do sends exactly one request and decodes its answer; a failure is decoded
// rather than retried.
func (c *Client) do(ctx context.Context, method, path string, query url.Values, body any, headers http.Header, into any) error {
	target := c.baseURL + path
	if encoded := query.Encode(); encoded != "" {
		target += "?" + encoded
	}
	var payload io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return err
		}
		payload = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, target, payload)
	if err != nil {
		return err
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	for name, values := range c.headers {
		request.Header[name] = values
	}
	for name, values := range headers {
		request.Header[name] = values
	}
	response, err := c.http.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	answer, err := io.ReadAll(response.Body)
	if err != nil {
		return err
	}
	if response.StatusCode < http.StatusOK || response.StatusCode > 299 {
		return newAPIError(response.StatusCode, answer)
	}
	if into == nil || len(bytes.TrimSpace(answer)) == 0 {
		return nil
	}
	return json.Unmarshal(answer, into)
}

// resourcePath addresses one object class or one object of it.
func resourcePath(kind, name string) string {
	path := "/api/v1/resources/" + url.PathEscape(kind)
	if name != "" {
		path += "/" + url.PathEscape(name)
	}
	return path
}

// setWhenNotEmpty writes one query parameter only when the author gave it.
func setWhenNotEmpty(values url.Values, name, value string) {
	if value != "" {
		values.Set(name, value)
	}
}

// precondition builds the If-Match header a write carries. A resource version
// is an entity-tag, which RFC 9110 §13.1.1 requires to be quoted; a bare value
// never matches a server that compares entity-tags. An empty resource version
// sends no precondition.
func precondition(resourceVersion string) http.Header {
	headers := http.Header{}
	if resourceVersion != "" {
		headers.Set("If-Match", `"`+resourceVersion+`"`)
	}
	return headers
}
