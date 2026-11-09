// Package espn fetches NFL data from ESPN's public site API. Every client takes
// a base URL and an HTTP client, returns plain structs, and never touches the
// database.
package espn

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	// DefaultBaseURL is ESPN's NFL site API root.
	DefaultBaseURL = "https://site.api.espn.com/apis/site/v2/sports/football/nfl"
	// DefaultWebBaseURL is ESPN's athlete web API root.
	DefaultWebBaseURL = "https://site.web.api.espn.com/apis/common/v3/sports/football/nfl"
	// defaultTimeout bounds a request when the caller does not supply a client.
	defaultTimeout = 10 * time.Second
	// userAgent identifies this app to ESPN.
	userAgent = "parlay-club"
)

// Client talks to ESPN. Create one with NewClient and share it.
type Client struct {
	baseURL    string
	webBaseURL string
	httpClient *http.Client
}

// NewClient builds a client for baseURL. An empty baseURL means DefaultBaseURL,
// and a nil httpClient means one with a 10 second timeout. A custom baseURL is
// also used for webBaseURL so httptest.NewServer works with one URL.
func NewClient(baseURL string, httpClient *http.Client) *Client {
	webBaseURL := baseURL
	if baseURL == "" {
		baseURL = DefaultBaseURL
		webBaseURL = DefaultWebBaseURL
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultTimeout}
	}
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		webBaseURL: strings.TrimRight(webBaseURL, "/"),
		httpClient: httpClient,
	}
}

// get fetches path under the base URL, adds query, and decodes the JSON body
// into out.
func (c *Client) get(ctx context.Context, path string, query url.Values, out any) error {
	endpoint := c.baseURL + path
	if encoded := query.Encode(); encoded != "" {
		endpoint += "?" + encoded
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("build %s request: %w", path, err)
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("get %s: %w", path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("get %s: unexpected status %s", path, resp.Status)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return nil
}

// getWeb fetches path under webBaseURL, adds query, and decodes the JSON body
// into out.
func (c *Client) getWeb(ctx context.Context, path string, query url.Values, out any) error {
	endpoint := c.webBaseURL + path
	if encoded := query.Encode(); encoded != "" {
		endpoint += "?" + encoded
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("build %s request: %w", path, err)
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("get %s: %w", path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("get %s: unexpected status %s", path, resp.Status)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return nil
}
