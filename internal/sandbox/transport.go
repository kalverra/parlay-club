package sandbox

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

// ErrNoResponse reports that the registry has no response for a request at the
// transport's current stage. The replay transport has no network fallback, so
// callers can treat this as a hard scenario failure.
var ErrNoResponse = errors.New("sandbox replay has no response")

// FallbackHeader marks a response the registry synthesized as an empty body
// because the scenario captures no data for that optional request. A captured
// response never carries it.
const FallbackHeader = "X-Sandbox-Replay"

// FallbackEmpty is the FallbackHeader value for an empty fallback body.
const FallbackEmpty = "empty"

// credentialKey names the query parameters that identify a caller rather than
// a resource. They are ignored when matching so a synthetic key still replays
// a captured request.
var credentialKey = strings.NewReplacer("-", "", "_", "")

// Transport answers ingest requests from a preloaded Registry. It never opens a
// connection, so an unregistered request fails instead of reaching a provider.
type Transport struct {
	registry *Registry

	mu    sync.RWMutex
	stage Stage
}

// NewTransport builds a replay transport for one registry and stage. A nil
// registry is a wiring mistake and panics at construction.
func NewTransport(registry *Registry, stage Stage) *Transport {
	if registry == nil {
		panic("sandbox: nil Registry")
	}
	return &Transport{registry: registry, stage: stage}
}

// Stage returns the stage the transport is currently replaying.
func (t *Transport) Stage() Stage {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.stage
}

// SetStage changes the stage used by later requests. Callers that share an
// HTTP client can switch snapshots without rebuilding the clients.
func (t *Transport) SetStage(stage Stage) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.stage = stage
}

// RoundTrip looks up the request in the registry and returns the saved
// response. An unknown request is an error naming the method, URL, and stage.
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req == nil || req.URL == nil {
		return nil, errors.New("sandbox replay: request has no URL")
	}

	stage := t.Stage()
	key, err := requestKeyFor(stage, req.Method, req.URL)
	if err != nil {
		return nil, fmt.Errorf("sandbox replay: %w", err)
	}
	saved, ok := t.registry.responses[key]
	if !ok {
		saved, ok = t.registry.fallback(key)
	}
	if !ok {
		return nil, fmt.Errorf(
			"%w: %s %s at stage %q",
			ErrNoResponse,
			strings.ToUpper(req.Method),
			displayURL(req.URL),
			stage,
		)
	}

	header := saved.header.Clone()
	if saved.empty {
		header.Set(FallbackHeader, FallbackEmpty)
	}

	return &http.Response{
		Status:        fmt.Sprintf("%d %s", saved.status, http.StatusText(saved.status)),
		StatusCode:    saved.status,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        header,
		Body:          io.NopCloser(bytes.NewReader(saved.body)),
		ContentLength: int64(len(saved.body)),
		Request:       req,
	}, nil
}

// requestKey is the identity of one replayed request. It keeps the stage, so
// the same URL can answer with a different snapshot in another stage.
type requestKey struct {
	stage  Stage
	method string
	host   string
	path   string
	query  string
}

// requestKeyFor normalizes a request for lookup. The host and method are
// case-insensitive; query parameters are sorted, and credentials are dropped.
func requestKeyFor(stage Stage, method string, location *url.URL) (requestKey, error) {
	if location == nil {
		return requestKey{}, errors.New("request has no URL")
	}
	return requestKey{
		stage:  stage,
		method: strings.ToUpper(method),
		host:   strings.ToLower(location.Host),
		path:   location.EscapedPath(),
		query:  normalizedQuery(location.Query()),
	}, nil
}

// normalizedQuery sorts the query and removes credential parameters. The empty
// string means the request has no meaningful query.
func normalizedQuery(values url.Values) string {
	cleaned := make(url.Values, len(values))
	for key, list := range values {
		if isCredential(key) {
			continue
		}
		for _, value := range list {
			cleaned.Add(key, value)
		}
	}
	return cleaned.Encode()
}

// isCredential reports whether a query parameter names a caller credential.
func isCredential(key string) bool {
	normalized := credentialKey.Replace(strings.ToLower(key))
	switch normalized {
	case "apikey", "token", "accesstoken":
		return true
	default:
		return false
	}
}

// displayURL is the request URL with credentials removed, for error messages.
func displayURL(location *url.URL) string {
	withoutCredentials := *location
	withoutCredentials.User = nil
	withoutCredentials.RawQuery = normalizedQuery(location.Query())
	return withoutCredentials.String()
}
