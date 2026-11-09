package nfelo

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Projection is one row of nfelo's historic projections file, after team codes
// are mapped to ESPN abbreviations.
type Projection struct {
	Season, Week         int
	Home, Away           string  // ESPN abbreviations, after mapping
	HomeSpread           float64 // home perspective; negative = home favored
	HomeWinProb          float64
	HomeQB, AwayQB       string
	HomeQBAdj, AwayQBAdj float64 // Elo points
}

// Required CSV columns. The client reads them by name, never by position.
const (
	columnSeason      = "season"
	columnWeek        = "week"
	columnAwayTeam    = "away_team"
	columnHomeTeam    = "home_team"
	columnHomeSpread  = "home_line_pre_regression"
	columnHomeWinProb = "home_probability_nfelo"
	columnHomeQB      = "home_538_qb"
	columnAwayQB      = "away_538_qb"
	columnHomeQBAdj   = "home_538_qb_adj"
	columnAwayQBAdj   = "away_538_qb_adj"
)

// columnIndexes holds each field's position in the CSV header.
type columnIndexes struct {
	season, week         int
	awayTeam, homeTeam   int
	homeSpread           int
	homeWinProb          int
	homeQB, awayQB       int
	homeQBAdj, awayQBAdj int
}

// Projections fetches the historic projections CSV and returns one Projection
// per usable row. Request failures, a missing required header, and a row with
// an unparseable number are errors. A row with an empty required cell is
// dropped.
func (c *Client) Projections(ctx context.Context) ([]Projection, error) {
	resp, err := c.get(ctx, projectionsPath)
	if err != nil {
		return nil, fmt.Errorf("projections: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	projections, err := parseProjections(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("projections: %w", err)
	}
	return projections, nil
}

// parseProjections reads the CSV header by name, then parses every row.
func parseProjections(body io.Reader) ([]Projection, error) {
	reader := csv.NewReader(body)

	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}
	columns, err := parseColumns(header)
	if err != nil {
		return nil, err
	}

	projections := make([]Projection, 0)
	for line := 2; ; line++ {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read row %d: %w", line, err)
		}

		projection, ok, err := parseRow(record, columns, line)
		if err != nil {
			return nil, err
		}
		if ok {
			projections = append(projections, projection)
		}
	}
	return projections, nil
}

// parseColumns maps the required header names to their positions. A missing
// name is an error because a reordered header is fine and a renamed one is not.
func parseColumns(header []string) (columnIndexes, error) {
	index := make(map[string]int, len(header))
	for position, name := range header {
		index[strings.TrimSpace(name)] = position
	}

	missing := make([]string, 0)
	position := func(name string) int {
		at, ok := index[name]
		if !ok {
			missing = append(missing, name)
			return 0
		}
		return at
	}

	columns := columnIndexes{
		season:      position(columnSeason),
		week:        position(columnWeek),
		awayTeam:    position(columnAwayTeam),
		homeTeam:    position(columnHomeTeam),
		homeSpread:  position(columnHomeSpread),
		homeWinProb: position(columnHomeWinProb),
		homeQB:      position(columnHomeQB),
		awayQB:      position(columnAwayQB),
		homeQBAdj:   position(columnHomeQBAdj),
		awayQBAdj:   position(columnAwayQBAdj),
	}
	if len(missing) > 0 {
		return columnIndexes{}, fmt.Errorf("missing required columns: %s", strings.Join(missing, ", "))
	}
	return columns, nil
}

// parseRow converts one record. The bool is false when a required cell is
// empty, which drops the row instead of failing the whole file.
func parseRow(record []string, columns columnIndexes, line int) (Projection, bool, error) {
	values := rowValues{
		season:      strings.TrimSpace(record[columns.season]),
		week:        strings.TrimSpace(record[columns.week]),
		awayTeam:    strings.TrimSpace(record[columns.awayTeam]),
		homeTeam:    strings.TrimSpace(record[columns.homeTeam]),
		homeSpread:  strings.TrimSpace(record[columns.homeSpread]),
		homeWinProb: strings.TrimSpace(record[columns.homeWinProb]),
		homeQB:      strings.TrimSpace(record[columns.homeQB]),
		awayQB:      strings.TrimSpace(record[columns.awayQB]),
		homeQBAdj:   strings.TrimSpace(record[columns.homeQBAdj]),
		awayQBAdj:   strings.TrimSpace(record[columns.awayQBAdj]),
	}
	if values.empty() {
		return Projection{}, false, nil
	}

	season, err := parseInteger(values.season, columnSeason, line)
	if err != nil {
		return Projection{}, false, err
	}
	week, err := parseInteger(values.week, columnWeek, line)
	if err != nil {
		return Projection{}, false, err
	}
	homeSpread, err := parseNumber(values.homeSpread, columnHomeSpread, line)
	if err != nil {
		return Projection{}, false, err
	}
	homeWinProb, err := parseNumber(values.homeWinProb, columnHomeWinProb, line)
	if err != nil {
		return Projection{}, false, err
	}
	homeQBAdj, err := parseNumber(values.homeQBAdj, columnHomeQBAdj, line)
	if err != nil {
		return Projection{}, false, err
	}
	awayQBAdj, err := parseNumber(values.awayQBAdj, columnAwayQBAdj, line)
	if err != nil {
		return Projection{}, false, err
	}

	return Projection{
		Season:      season,
		Week:        week,
		Home:        mapTeam(values.homeTeam),
		Away:        mapTeam(values.awayTeam),
		HomeSpread:  homeSpread,
		HomeWinProb: homeWinProb,
		HomeQB:      values.homeQB,
		AwayQB:      values.awayQB,
		HomeQBAdj:   homeQBAdj,
		AwayQBAdj:   awayQBAdj,
	}, true, nil
}

// rowValues holds one row's required cells as trimmed strings.
type rowValues struct {
	season, week         string
	awayTeam, homeTeam   string
	homeSpread           string
	homeWinProb          string
	homeQB, awayQB       string
	homeQBAdj, awayQBAdj string
}

// empty reports whether any required cell is blank.
func (r rowValues) empty() bool {
	return r.season == "" || r.week == "" || r.awayTeam == "" || r.homeTeam == "" ||
		r.homeSpread == "" || r.homeWinProb == "" || r.homeQB == "" || r.awayQB == "" ||
		r.homeQBAdj == "" || r.awayQBAdj == ""
}

// parseInteger reads a whole-number cell and names the row and column on error.
func parseInteger(value, column string, line int) (int, error) {
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("row %d %s %q: %w", line, column, value, err)
	}
	return parsed, nil
}

// parseNumber reads a decimal cell and names the row and column on error.
func parseNumber(value, column string, line int) (float64, error) {
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, fmt.Errorf("row %d %s %q: %w", line, column, value, err)
	}
	return parsed, nil
}
