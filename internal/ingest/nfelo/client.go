// Package nfelo fetches nfelo's projected spreads CSV from GitHub. The client
// takes a base URL and an HTTP client, returns plain structs, and never touches
// the database.
package nfelo

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const (
	// DefaultBaseURL is the raw GitHub directory that holds nfelo's output data.
	DefaultBaseURL = "https://raw.githubusercontent.com/greerreNFL/nfelo/main/output_data"
	// projectionsPath is the historic projections CSV under the base URL.
	projectionsPath = "/historic_projected_spreads.csv"
	// defaultTimeout bounds a request when the caller does not supply a client.
	defaultTimeout = 10 * time.Second
	// userAgent identifies this app to GitHub.
	userAgent = "parlay-club"
)

// Client talks to nfelo's GitHub output data. Create one with NewClient and
// share it.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient builds a client for baseURL. An empty baseURL means DefaultBaseURL,
// and a nil httpClient means one with a 10 second timeout.
func NewClient(baseURL string, httpClient *http.Client) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultTimeout}
	}
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: httpClient,
	}
}

// get fetches path under the base URL. The response body is left open so the
// caller can stream it into a parser.
func (c *Client) get(ctx context.Context, path string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("build %s request: %w", path, err)
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get %s: %w", path, err)
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("get %s: unexpected status %s", path, resp.Status)
	}
	return resp, nil
}
