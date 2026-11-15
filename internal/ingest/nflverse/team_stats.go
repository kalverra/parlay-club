package nflverse

import (
	"context"
	"fmt"
	"io"
)

// TeamWeek is one team's weekly offense and opponent-allowed totals from
// stats_team_week_{season}.csv, after team codes are mapped to ESPN.
type TeamWeek struct {
	Season          int
	SeasonType      int // 1 = pre, 2 = regular, 3 = postseason
	Week            int
	Team            string // ESPN abbreviation
	Opponent        string // ESPN abbreviation
	PassingEPA      float64
	RushingEPA      float64
	Attempts        int
	Carries         int
	PassingYards    int
	RushingYards    int
	OppPassingEPA   float64
	OppRushingEPA   float64
	OppAttempts     int
	OppCarries      int
	OppPassingYards int
	OppRushingYards int
}

var teamStatsColumns = []string{
	"season",
	"week",
	"season_type",
	"team",
	"opponent_team",
	"passing_epa",
	"rushing_epa",
	"attempts",
	"carries",
	"passing_yards",
	"rushing_yards",
}

type teamWeekKey struct {
	season     int
	seasonType int
	week       int
	team       string
}

// TeamStats fetches stats_team_week_{season}.csv and returns one TeamWeek per
// usable row, with each row's opponent-allowed totals joined from the
// opponent's matching game row.
func (c *Client) TeamStats(ctx context.Context, season int) ([]TeamWeek, error) {
	if season <= 0 {
		return nil, fmt.Errorf("team stats: invalid season %d", season)
	}
	path := fmt.Sprintf("/stats_team/stats_team_week_%d.csv", season)
	resp, err := c.get(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("team stats: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	rows, err := parseTeamStats(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("team stats: %w", err)
	}
	return rows, nil
}

func parseTeamStats(body io.Reader) ([]TeamWeek, error) {
	rows, err := parseCSV(body, teamStatsColumns, parseTeamStatsRow)
	if err != nil {
		return nil, err
	}

	byKey := make(map[teamWeekKey]TeamWeek, len(rows))
	for _, r := range rows {
		byKey[teamWeekKey{
			season:     r.Season,
			seasonType: r.SeasonType,
			week:       r.Week,
			team:       r.Team,
		}] = r
	}

	for i := range rows {
		opp, ok := byKey[teamWeekKey{
			season:     rows[i].Season,
			seasonType: rows[i].SeasonType,
			week:       rows[i].Week,
			team:       rows[i].Opponent,
		}]
		if !ok {
			continue
		}
		rows[i].OppPassingEPA = opp.PassingEPA
		rows[i].OppRushingEPA = opp.RushingEPA
		rows[i].OppAttempts = opp.Attempts
		rows[i].OppCarries = opp.Carries
		rows[i].OppPassingYards = opp.PassingYards
		rows[i].OppRushingYards = opp.RushingYards
	}
	return rows, nil
}

func parseTeamStatsRow(record []string, cols map[string]int, line int) (TeamWeek, bool, error) {
	seasonRaw := cell(record, cols, "season")
	weekRaw := cell(record, cols, "week")
	seasonTypeRaw := cell(record, cols, "season_type")
	teamRaw := cell(record, cols, "team")
	oppRaw := cell(record, cols, "opponent_team")
	if seasonRaw == "" || weekRaw == "" || seasonTypeRaw == "" || teamRaw == "" || oppRaw == "" {
		return TeamWeek{}, false, nil
	}

	season, err := parseInt(seasonRaw, "season", line)
	if err != nil {
		return TeamWeek{}, false, err
	}
	week, err := parseInt(weekRaw, "week", line)
	if err != nil {
		return TeamWeek{}, false, err
	}
	seasonType, err := parseSeasonType(seasonTypeRaw, "season_type", line)
	if err != nil {
		return TeamWeek{}, false, err
	}
	passEPA, err := parseOptionalFloat(cell(record, cols, "passing_epa"), "passing_epa", line)
	if err != nil {
		return TeamWeek{}, false, err
	}
	rushEPA, err := parseOptionalFloat(cell(record, cols, "rushing_epa"), "rushing_epa", line)
	if err != nil {
		return TeamWeek{}, false, err
	}
	attempts, err := parseOptionalInt(cell(record, cols, "attempts"), "attempts", line)
	if err != nil {
		return TeamWeek{}, false, err
	}
	carries, err := parseOptionalInt(cell(record, cols, "carries"), "carries", line)
	if err != nil {
		return TeamWeek{}, false, err
	}
	passYds, err := parseOptionalInt(cell(record, cols, "passing_yards"), "passing_yards", line)
	if err != nil {
		return TeamWeek{}, false, err
	}
	rushYds, err := parseOptionalInt(cell(record, cols, "rushing_yards"), "rushing_yards", line)
	if err != nil {
		return TeamWeek{}, false, err
	}

	return TeamWeek{
		Season:       season,
		SeasonType:   seasonType,
		Week:         week,
		Team:         mapTeam(teamRaw),
		Opponent:     mapTeam(oppRaw),
		PassingEPA:   passEPA,
		RushingEPA:   rushEPA,
		Attempts:     attempts,
		Carries:      carries,
		PassingYards: passYds,
		RushingYards: rushYds,
	}, true, nil
}
