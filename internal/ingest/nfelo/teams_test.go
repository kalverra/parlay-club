package nfelo_test

import (
	"encoding/csv"
	"io"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/ingest/nfelo"
)

// espnTeamCodes is ESPN's 32 NFL abbreviations. A projection that maps to a
// code outside this set can never match a game.
var espnTeamCodes = map[string]struct{}{
	"ARI": {}, "ATL": {}, "BAL": {}, "BUF": {}, "CAR": {}, "CHI": {},
	"CIN": {}, "CLE": {}, "DAL": {}, "DEN": {}, "DET": {}, "GB": {},
	"HOU": {}, "IND": {}, "JAX": {}, "KC": {}, "LAC": {}, "LAR": {},
	"LV": {}, "MIA": {}, "MIN": {}, "NE": {}, "NO": {}, "NYG": {},
	"NYJ": {}, "PHI": {}, "PIT": {}, "SEA": {}, "SF": {}, "TB": {},
	"TEN": {}, "WSH": {},
}

func TestProjectionsMapsNfeloTeamCodesToESPN(t *testing.T) {
	t.Parallel()

	raw := savedFixture(t)
	server := csvServer(t, string(raw))
	projections, err := nfelo.NewClient(server.URL, server.Client()).Projections(t.Context())
	require.NoError(t, err)

	raiders := weekNineProjection(t, projections, "SF", "LV")
	assert.Equal(t, "LV", raiders.Away, "OAK maps to ESPN's LV")

	commanders := weekNineProjection(t, projections, "WSH", "LAR")
	assert.Equal(t, "WSH", commanders.Home, "WAS maps to ESPN's WSH")
	assert.Equal(t, "LAR", commanders.Away, "codes that agree pass through")

	codes := make(map[string]struct{})
	for _, projection := range projections {
		codes[projection.Home] = struct{}{}
		codes[projection.Away] = struct{}{}
	}
	assert.Len(t, codes, len(espnTeamCodes), "the fixture covers every NFL team")
	for code := range codes {
		_, ok := espnTeamCodes[code]
		assert.True(t, ok, "nfelo code %q is not an ESPN abbreviation", code)
	}
}

// TestFixtureConfirmsThePreRegressionSpreadColumn guards the design's choice of
// home_line_pre_regression. The saved file proves that column is not the market
// line, while home_closing_line_rounded_nfelo usually echoes it.
func TestFixtureConfirmsThePreRegressionSpreadColumn(t *testing.T) {
	t.Parallel()

	fixture := readFixtureColumns(t)

	assert.Greater(t, fixture.closingMatchesMarket, len(fixture.current)/2,
		"the closing column echoes the market on most current rows")
	assert.Less(t, fixture.preRegressionMatchesMarket, len(fixture.current)/5,
		"the pre-regression column is not just the market line")

	server := csvServer(t, string(savedFixture(t)))
	projections, err := nfelo.NewClient(server.URL, server.Client()).Projections(t.Context())
	require.NoError(t, err)

	raiders := weekNineProjection(t, projections, "SF", "LV")
	assert.InDelta(t, -6.5, raiders.HomeSpread, 1e-9,
		"the model spread comes from home_line_pre_regression, not the market or closing column")
}

// fixtureColumns counts how often the two candidate spread columns equal the
// market line on the current season.
type fixtureColumns struct {
	current                    []string
	preRegressionMatchesMarket int
	closingMatchesMarket       int
}

// readFixtureColumns reads the saved CSV directly, before the client drops rows
// with a blank required cell.
func readFixtureColumns(t *testing.T) fixtureColumns {
	t.Helper()

	raw := savedFixture(t)
	reader := csv.NewReader(strings.NewReader(string(raw)))

	header, err := reader.Read()
	require.NoError(t, err)
	index := make(map[string]int, len(header))
	for position, name := range header {
		index[name] = position
	}
	marketAt := index["home_line_close"]
	preRegressionAt := index["home_line_pre_regression"]
	closingAt := index["home_closing_line_rounded_nfelo"]
	seasonAt := index["season"]

	var result fixtureColumns
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		if record[seasonAt] != "2026" {
			continue
		}
		result.current = append(result.current, record[seasonAt])

		market := parseFixtureNumber(t, record[marketAt])
		if parseFixtureNumber(t, record[preRegressionAt]) == market {
			result.preRegressionMatchesMarket++
		}
		if parseFixtureNumber(t, record[closingAt]) == market {
			result.closingMatchesMarket++
		}
	}
	return result
}

// parseFixtureNumber reads one numeric fixture cell.
func parseFixtureNumber(t *testing.T, value string) float64 {
	t.Helper()

	parsed, err := strconv.ParseFloat(value, 64)
	require.NoError(t, err)
	return parsed
}
