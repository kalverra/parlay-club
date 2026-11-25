// Package nflverse fetches pre-aggregated NFL team, player, snap count,
// expected-opportunity, and player-ID crosswalk CSVs from nflverse-data and
// ffverse/ffopportunity GitHub releases. The client takes a base URL and an
// HTTP client, returns plain structs, and never touches the database.
package nflverse

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	// DefaultBaseURL is the GitHub releases download prefix for nflverse-data.
	DefaultBaseURL = "https://github.com/nflverse/nflverse-data/releases/download"
	// DefaultFFOpportunityBaseURL is the GitHub releases download prefix for
	// ffverse's weekly expected-opportunity CSVs.
	DefaultFFOpportunityBaseURL = "https://github.com/ffverse/ffopportunity/releases/download/latest-data"
	// defaultTimeout bounds a request when the caller does not supply a client.
	defaultTimeout = 30 * time.Second
	// userAgent identifies this app to GitHub.
	userAgent = "parlay-club"

	// SeasonTypePreseason matches ESPN's preseason season_type.
	SeasonTypePreseason = 1
	// SeasonTypeRegular matches ESPN's regular-season season_type.
	SeasonTypeRegular = 2
	// SeasonTypePostseason matches ESPN's postseason season_type.
	SeasonTypePostseason = 3
)

// Client talks to nflverse-data GitHub releases. Create one with NewClient and
// share it.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient builds a client for baseURL. An empty baseURL means DefaultBaseURL,
// and a nil httpClient means one with a 30 second timeout.
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
	return c.getURL(ctx, c.baseURL+path, path)
}

// getURL fetches targetURL. label identifies the resource in error messages.
func (c *Client) getURL(ctx context.Context, targetURL, label string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build %s request: %w", label, err)
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get %s: %w", label, err)
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("get %s: unexpected status %s", label, resp.Status)
	}
	return resp, nil
}

// parseCSV reads a CSV stream by header name, resolving required columns once
// and calling parseRow for every data row (1-indexed line >= 2).
func parseCSV[T any](
	body io.Reader,
	required []string,
	parseRow func(record []string, cols map[string]int, line int) (T, bool, error),
) ([]T, error) {
	return parseCSVWithOptional(body, required, nil, parseRow)
}

// parseCSVWithOptional reads a CSV stream by header name, resolving required
// columns (failing if any are missing) plus any present optional columns.
func parseCSVWithOptional[T any](
	body io.Reader,
	required []string,
	optional []string,
	parseRow func(record []string, cols map[string]int, line int) (T, bool, error),
) ([]T, error) {
	reader := csv.NewReader(body)
	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}

	cols, err := resolveColumnsWithOptional(header, required, optional)
	if err != nil {
		return nil, err
	}

	out := make([]T, 0)
	for line := 2; ; line++ {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read row %d: %w", line, err)
		}
		row, ok, err := parseRow(record, cols, line)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, row)
		}
	}
	return out, nil
}

// resolveColumnsWithOptional maps every required column name (and any present
// optional column name) to its 0-based index in header. Any missing required
// column fails the file immediately.
func resolveColumnsWithOptional(header, required, optional []string) (map[string]int, error) {
	index := make(map[string]int, len(header))
	for pos, name := range header {
		index[strings.TrimSpace(name)] = pos
	}

	cols := make(map[string]int, len(required)+len(optional))
	var missing []string
	for _, name := range required {
		pos, ok := index[name]
		if !ok {
			missing = append(missing, name)
			continue
		}
		cols[name] = pos
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required columns: %s", strings.Join(missing, ", "))
	}
	for _, name := range optional {
		if pos, ok := index[name]; ok {
			cols[name] = pos
		}
	}
	return cols, nil
}

// cell returns the trimmed value of column name in record.
func cell(record []string, cols map[string]int, name string) string {
	return strings.TrimSpace(record[cols[name]])
}

// optionalCell returns the trimmed value of an optional column, or "" when absent.
func optionalCell(record []string, cols map[string]int, name string) string {
	pos, ok := cols[name]
	if !ok {
		return ""
	}
	return strings.TrimSpace(record[pos])
}

// parseSeasonType converts nflverse season_type/game_type codes into ESPN's
// integer season_type (1 = pre, 2 = regular, 3 = postseason).
func parseSeasonType(value, column string, line int) (int, error) {
	switch strings.ToUpper(value) {
	case "PRE":
		return SeasonTypePreseason, nil
	case "REG":
		return SeasonTypeRegular, nil
	case "POST", "WC", "DIV", "CON", "SB":
		return SeasonTypePostseason, nil
	default:
		return 0, fmt.Errorf("row %d %s %q: unknown season type", line, column, value)
	}
}

// parseInt parses a required integer cell and names the row and column on error.
func parseInt(value, column string, line int) (int, error) {
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("row %d %s %q: %w", line, column, value, err)
	}
	return parsed, nil
}

// parseOptionalInt parses an integer cell, returning 0 when blank or "NA".
func parseOptionalInt(value, column string, line int) (int, error) {
	if value == "" || strings.EqualFold(value, "NA") {
		return 0, nil
	}
	return parseInt(value, column, line)
}

// parseOptionalFloat parses a float cell, returning 0 when blank, "NA", or "NaN".
func parseOptionalFloat(value, column string, line int) (float64, error) {
	if value == "" || strings.EqualFold(value, "NA") || strings.EqualFold(value, "NaN") {
		return 0, nil
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, fmt.Errorf("row %d %s %q: %w", line, column, value, err)
	}
	return parsed, nil
}
