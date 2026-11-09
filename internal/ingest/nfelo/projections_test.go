package nfelo_test

import (
	"bytes"
	"encoding/csv"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/ingest/nfelo"
)

const projectionsPath = "/historic_projected_spreads.csv"

// requiredHeader is the fixture's required columns, in the same order the
// client reads them by name.
var requiredHeader = []string{
	"season",
	"week",
	"away_team",
	"home_team",
	"home_line_pre_regression",
	"home_probability_nfelo",
	"home_538_qb",
	"away_538_qb",
	"home_538_qb_adj",
	"away_538_qb_adj",
}

// firstRow is one complete row that pairs with requiredHeader.
func firstRow() []string {
	return []string{
		"2026", "9", "OAK", "SF", "-6.5", "0.25",
		"Brock Purdy", "Kirk Cousins", "5.5", "-2.5",
	}
}

// csvBody renders a header and rows as CSV.
func csvBody(t *testing.T, header []string, rows ...[]string) string {
	t.Helper()

	var body bytes.Buffer
	writer := csv.NewWriter(&body)
	require.NoError(t, writer.Write(header))
	for _, row := range rows {
		require.NoError(t, writer.Write(row))
	}
	writer.Flush()
	require.NoError(t, writer.Error())
	return body.String()
}

// csvServer serves body at the projections path and checks the request.
func csvServer(t *testing.T, body string) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, projectionsPath, r.URL.Path)
		assert.Equal(t, "parlay-club", r.Header.Get("User-Agent"))
		w.Header().Set("Content-Type", "text/csv")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return server
}

// parseCSV serves body and parses it through the client.
func parseCSV(t *testing.T, body string) ([]nfelo.Projection, error) {
	t.Helper()

	server := csvServer(t, body)
	return nfelo.NewClient(server.URL, server.Client()).Projections(t.Context())
}

// weekNineProjection finds a 2026 week 9 fixture row for a team pair.
func weekNineProjection(t *testing.T, projections []nfelo.Projection, home, away string) nfelo.Projection {
	t.Helper()

	for _, projection := range projections {
		if projection.Season == 2026 && projection.Week == 9 &&
			projection.Home == home && projection.Away == away {
			return projection
		}
	}
	require.FailNow(t, "projection not found", "2026 week 9 %s vs %s", away, home)
	return nfelo.Projection{}
}

// savedFixture returns the real CSV saved under testdata.
func savedFixture(t *testing.T) []byte {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join("testdata", "historic_projected_spreads.csv"))
	require.NoError(t, err)
	require.NotEmpty(t, raw)
	return raw
}

func TestProjectionsParsesTheSavedFixture(t *testing.T) {
	t.Parallel()

	raw := savedFixture(t)
	server := csvServer(t, string(raw))

	projections, err := nfelo.NewClient(server.URL, server.Client()).Projections(t.Context())
	require.NoError(t, err)
	require.NotEmpty(t, projections)

	// Rows with a blank required cell are dropped; the fixture has 1,551 data
	// rows and 1,449 that carry every required value.
	assert.Len(t, projections, 1449)

	got := weekNineProjection(t, projections, "MIN", "BUF")
	assert.Equal(t, 2026, got.Season)
	assert.Equal(t, 9, got.Week)
	assert.InDelta(t, 3.5, got.HomeSpread, 1e-9)
	assert.InDelta(t, 0.3689304504324768, got.HomeWinProb, 1e-9)
	assert.Equal(t, "Carson Wentz", got.HomeQB)
	assert.Equal(t, "Josh Allen", got.AwayQB)
	assert.InDelta(t, -33.88252258300781, got.HomeQBAdj, 1e-9)
	assert.InDelta(t, 12.901379585266113, got.AwayQBAdj, 1e-9)
}

func TestProjectionsReadsColumnsByHeaderName(t *testing.T) {
	t.Parallel()

	shuffledHeader := []string{
		"home_538_qb_adj", "week", "away_team", "", "home_probability_nfelo",
		"season", "away_538_qb", "home_team", "home_538_qb",
		"home_line_pre_regression", "away_538_qb_adj",
	}
	shuffledRow := []string{
		"5.5", "9", "OAK", "0", "0.25",
		"2026", "Kirk Cousins", "SF", "Brock Purdy",
		"-6.5", "-2.5",
	}

	orderedHeader := append([]string{""}, requiredHeader...)
	orderedRow := append([]string{"0"}, firstRow()...)

	different, err := parseCSV(t, csvBody(t, orderedHeader, orderedRow))
	require.NoError(t, err)
	reordered, err := parseCSV(t, csvBody(t, shuffledHeader, shuffledRow))
	require.NoError(t, err)

	want := nfelo.Projection{
		Season:      2026,
		Week:        9,
		Home:        "SF",
		Away:        "LV",
		HomeSpread:  -6.5,
		HomeWinProb: 0.25,
		HomeQB:      "Brock Purdy",
		AwayQB:      "Kirk Cousins",
		HomeQBAdj:   5.5,
		AwayQBAdj:   -2.5,
	}
	assert.Equal(t, []nfelo.Projection{want}, different)
	assert.Equal(t, different, reordered, "a reordered header parses the same")
}

func TestProjectionsReportsMissingRequiredHeaders(t *testing.T) {
	t.Parallel()

	for _, missing := range requiredHeader {
		t.Run(missing, func(t *testing.T) {
			t.Parallel()

			header := make([]string, 0, len(requiredHeader))
			row := make([]string, 0, len(requiredHeader))
			for position, name := range requiredHeader {
				if name == missing {
					continue
				}
				header = append(header, name)
				row = append(row, firstRow()[position])
			}

			_, err := parseCSV(t, csvBody(t, header, row))
			require.Error(t, err)
			assert.Contains(t, err.Error(), "missing required columns")
			assert.Contains(t, err.Error(), missing)
		})
	}
}

func TestProjectionsReportsAnUnparseableNumber(t *testing.T) {
	t.Parallel()

	for _, position := range []int{0, 1, 4, 5, 8, 9} {
		name := requiredHeader[position]
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			row := firstRow()
			row[position] = "not-a-number"

			_, err := parseCSV(t, csvBody(t, append([]string{""}, requiredHeader...), append([]string{"0"}, row...)))
			require.Error(t, err)
			assert.Contains(t, err.Error(), name)
			assert.Contains(t, err.Error(), "not-a-number")
		})
	}
}

func TestProjectionsDropsRowsWithAnEmptyRequiredCell(t *testing.T) {
	t.Parallel()

	header := append([]string{""}, requiredHeader...)
	for position, name := range requiredHeader {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			row := append([]string{"0"}, firstRow()...)
			row[position+1] = ""

			projections, err := parseCSV(t, csvBody(t, header, row))
			require.NoError(t, err)
			assert.Empty(t, projections)
		})
	}
}

func TestProjectionsReturnsAnEmptyListForAHeaderOnlyFile(t *testing.T) {
	t.Parallel()

	projections, err := parseCSV(t, csvBody(t, append([]string{""}, requiredHeader...)))
	require.NoError(t, err)
	assert.Empty(t, projections)
	assert.NotNil(t, projections)
}

func TestProjectionsReportsAnEmptyResponse(t *testing.T) {
	t.Parallel()

	_, err := parseCSV(t, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read header")
}

func TestProjectionsReportsAMalformedCSV(t *testing.T) {
	t.Parallel()

	body := csvBody(t, append([]string{""}, requiredHeader...), []string{"0", "2026"})
	_, err := parseCSV(t, body)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read row 2")
}

func TestProjectionsReportsAnHTTPFailure(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	t.Cleanup(server.Close)

	_, err := nfelo.NewClient(server.URL, server.Client()).Projections(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "404 Not Found")
}
