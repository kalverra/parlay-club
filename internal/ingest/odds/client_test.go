package odds_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/ingest/odds"
)

const testAPIKey = "test-key"

// fixtureServer serves fixture and asserts the request the client puts on the
// wire. It also sets the credit headers from credits.
func fixtureServer(
	t *testing.T,
	fixture, wantPath string,
	credits odds.Credits,
	wantQuery url.Values,
) *httptest.Server {
	t.Helper()

	//nolint:gosec // the fixture name is a test constant.
	raw, err := os.ReadFile(filepath.Join("testdata", fixture))
	require.NoError(t, err)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, wantPath, r.URL.Path)
		assert.Equal(t, wantQuery, r.URL.Query())
		assert.Equal(t, "parlay-club", r.Header.Get("User-Agent"))

		w.Header().Set("X-Requests-Remaining", strconv.Itoa(credits.Remaining))
		w.Header().Set("X-Requests-Used", strconv.Itoa(credits.Used))
		w.Header().Set("X-Requests-Last", strconv.Itoa(credits.Last))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(raw)
	}))
	t.Cleanup(server.Close)
	return server
}

// apiKeyQuery is the query every Odds API request carries.
func apiKeyQuery() url.Values {
	query := url.Values{}
	query.Set("apiKey", testAPIKey)
	return query
}

// linesQuery is the game-lines query.
func linesQuery() url.Values {
	query := apiKeyQuery()
	query.Set("regions", "us")
	query.Set("markets", "h2h,spreads,totals")
	query.Set("oddsFormat", "american")
	return query
}

// onlyBook returns one event's rows for one book, in fixture order.
func onlyBook(lines []odds.Line, eventID, book string) []odds.Line {
	var rows []odds.Line
	for _, line := range lines {
		if line.EventID == eventID && line.Book == book {
			rows = append(rows, line)
		}
	}
	return rows
}
