// Package odds fetches NFL game lines from Parlay API, an Odds API-compatible
// feed. The client takes a base URL, an API key, and an HTTP client, returns
// plain structs, and never touches the database.
package odds

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	// DefaultBaseURL is Parlay API's NFL root.
	DefaultBaseURL = "https://parlay-api.com/v1/sports/americanfootball_nfl"
	// TheOddsAPIBaseURL is The Odds API's NFL root. The client parses the same
	// event and game-line shapes from both providers.
	TheOddsAPIBaseURL = "https://api.the-odds-api.com/v4/sports/americanfootball_nfl"
	// eventsPath is the free event-list endpoint under the base URL.
	eventsPath = "/events"
	// oddsPath is the game-lines endpoint under the base URL.
	oddsPath = "/odds"
	// defaultTimeout bounds a request when the caller does not supply a client.
	defaultTimeout = 10 * time.Second
	// userAgent identifies this app to Parlay API.
	userAgent = "parlay-club"
)

// Credits is what one response's credit headers say. The API sends them on
// every authenticated call, including the free event list.
type Credits struct {
	Remaining int
	Used      int
	Last      int
}

// ErrNoAPIKey reports a client built without an API key.
var ErrNoAPIKey = errors.New("odds API key is empty")

// Client talks to Parlay API. Create one with NewClient and share it.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewClient builds a client for baseURL and apiKey. An empty baseURL means
// DefaultBaseURL, and a nil httpClient means one with a 10 second timeout.
func NewClient(baseURL, apiKey string, httpClient *http.Client) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultTimeout}
	}
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		apiKey:     apiKey,
		httpClient: httpClient,
	}
}

// get fetches path under the base URL, adds the API key and query, decodes the
// JSON body into out, and returns the response's credit headers.
func (c *Client) get(ctx context.Context, path string, query url.Values, out any) (Credits, error) {
	if c.apiKey == "" {
		return Credits{}, ErrNoAPIKey
	}
	if query == nil {
		query = url.Values{}
	}
	query.Set("apiKey", c.apiKey)

	endpoint := c.baseURL + path
	if encoded := query.Encode(); encoded != "" {
		endpoint += "?" + encoded
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return Credits{}, fmt.Errorf("build %s request: %w", path, err)
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return Credits{}, fmt.Errorf("get %s: %w", path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return Credits{}, fmt.Errorf("get %s: unexpected status %s", path, resp.Status)
	}
	credits, err := parseCredits(resp.Header)
	if err != nil {
		return Credits{}, fmt.Errorf("get %s: %w", path, err)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return Credits{}, fmt.Errorf("decode %s: %w", path, err)
	}
	return credits, nil
}

// parseCredits reads the credit headers. A missing or unreadable remaining
// count is an error, because the caller cannot enforce a credit floor without
// it. The used and last counts are informational and default to zero.
func parseCredits(header http.Header) (Credits, error) {
	remaining, err := requiredHeaderInt(header, "X-Requests-Remaining")
	if err != nil {
		return Credits{}, err
	}
	return Credits{
		Remaining: remaining,
		Used:      headerInt(header, "X-Requests-Used"),
		Last:      headerInt(header, "X-Requests-Last"),
	}, nil
}

// requiredHeaderInt reads an integer header or reports that it is missing.
func requiredHeaderInt(header http.Header, key string) (int, error) {
	raw := header.Get(key)
	if raw == "" {
		return 0, fmt.Errorf("missing %s header", key)
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s header %q: %w", key, raw, err)
	}
	return value, nil
}

// headerInt reads an informational integer header, returning zero when it is
// missing or not a number.
func headerInt(header http.Header, key string) int {
	value, err := strconv.Atoi(header.Get(key))
	if err != nil {
		return 0
	}
	return value
}
