package nflverse

import (
	"context"
	"fmt"
	"io"
)

// PlayerWeek is one player's weekly usage and production from
// stats_player_week_{season}.csv.
type PlayerWeek struct {
	PlayerID       string // GSIS ID
	Team           string // ESPN abbreviation
	Season         int
	SeasonType     int // 1 = pre, 2 = regular, 3 = postseason
	Week           int
	TargetShare    float64
	AirYardsShare  float64
	WOPR           float64
	PassingEPA     float64
	RushingEPA     float64
	ReceivingEPA   float64
	Targets        int
	Carries        int
	Receptions     int
	ReceivingYards int
	RushingYards   int
	PassingYards   int
	PassingTDs     int
	RushingTDs     int
	ReceivingTDs   int
}

var playerStatsColumns = []string{
	"player_id",
	"team",
	"season",
	"week",
	"season_type",
	"target_share",
	"air_yards_share",
	"wopr",
	"passing_epa",
	"rushing_epa",
	"receiving_epa",
	"targets",
	"carries",
	"receptions",
	"receiving_yards",
	"rushing_yards",
	"passing_yards",
	"passing_tds",
	"rushing_tds",
	"receiving_tds",
}

// PlayerStats fetches stats_player_week_{season}.csv and returns one PlayerWeek
// per usable row.
func (c *Client) PlayerStats(ctx context.Context, season int) ([]PlayerWeek, error) {
	if season <= 0 {
		return nil, fmt.Errorf("player stats: invalid season %d", season)
	}
	path := fmt.Sprintf("/stats_player/stats_player_week_%d.csv", season)
	resp, err := c.get(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("player stats: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	rows, err := parsePlayerStats(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("player stats: %w", err)
	}
	return rows, nil
}

func parsePlayerStats(body io.Reader) ([]PlayerWeek, error) {
	return parseCSV(body, playerStatsColumns, parsePlayerStatsRow)
}

func parsePlayerStatsRow(record []string, cols map[string]int, line int) (PlayerWeek, bool, error) {
	playerID := cell(record, cols, "player_id")
	teamRaw := cell(record, cols, "team")
	seasonRaw := cell(record, cols, "season")
	weekRaw := cell(record, cols, "week")
	seasonTypeRaw := cell(record, cols, "season_type")
	if playerID == "" || teamRaw == "" || seasonRaw == "" || weekRaw == "" || seasonTypeRaw == "" {
		return PlayerWeek{}, false, nil
	}

	season, err := parseInt(seasonRaw, "season", line)
	if err != nil {
		return PlayerWeek{}, false, err
	}
	week, err := parseInt(weekRaw, "week", line)
	if err != nil {
		return PlayerWeek{}, false, err
	}
	seasonType, err := parseSeasonType(seasonTypeRaw, "season_type", line)
	if err != nil {
		return PlayerWeek{}, false, err
	}

	pw := PlayerWeek{
		PlayerID:   playerID,
		Team:       mapTeam(teamRaw),
		Season:     season,
		SeasonType: seasonType,
		Week:       week,
	}

	floats := []struct {
		col string
		dst *float64
	}{
		{"target_share", &pw.TargetShare},
		{"air_yards_share", &pw.AirYardsShare},
		{"wopr", &pw.WOPR},
		{"passing_epa", &pw.PassingEPA},
		{"rushing_epa", &pw.RushingEPA},
		{"receiving_epa", &pw.ReceivingEPA},
	}
	for _, f := range floats {
		*f.dst, err = parseOptionalFloat(cell(record, cols, f.col), f.col, line)
		if err != nil {
			return PlayerWeek{}, false, err
		}
	}

	ints := []struct {
		col string
		dst *int
	}{
		{"targets", &pw.Targets},
		{"carries", &pw.Carries},
		{"receptions", &pw.Receptions},
		{"receiving_yards", &pw.ReceivingYards},
		{"rushing_yards", &pw.RushingYards},
		{"passing_yards", &pw.PassingYards},
		{"passing_tds", &pw.PassingTDs},
		{"rushing_tds", &pw.RushingTDs},
		{"receiving_tds", &pw.ReceivingTDs},
	}
	for _, n := range ints {
		*n.dst, err = parseOptionalInt(cell(record, cols, n.col), n.col, line)
		if err != nil {
			return PlayerWeek{}, false, err
		}
	}

	return pw, true, nil
}
