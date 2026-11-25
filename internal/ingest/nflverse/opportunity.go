package nflverse

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

// PlayerOpportunity holds one player's weekly expected-opportunity metrics from
// ep_weekly_{season}.csv.
type PlayerOpportunity struct {
	PlayerID      string // GSIS ID
	PlayerName    string
	Position      string
	Team          string // ESPN abbreviation
	Season        int
	SeasonType    int // 2 = regular, 3 = postseason
	Week          int
	ExpPassYds    float64
	ExpRushYds    float64
	ExpRecYds     float64
	ExpReceptions float64
	ExpPassTDs    float64
	ExpRushTDs    float64
	ExpRecTDs     float64
}

var opportunityColumns = []string{
	"season",
	"posteam",
	"week",
	"player_id",
	"full_name",
	"position",
	"receptions_exp",
	"pass_yards_gained_exp",
	"rec_yards_gained_exp",
	"rush_yards_gained_exp",
	"pass_touchdown_exp",
	"rec_touchdown_exp",
	"rush_touchdown_exp",
}

var optionalOpportunityColumns = []string{
	"season_type",
}

// FFOpportunity fetches ep_weekly_{season}.csv and returns one PlayerOpportunity
// per usable row. When the client uses DefaultBaseURL, it reads from the
// upstream ffverse/ffopportunity release; custom base URLs request
// /ff_opportunity/ep_weekly_{season}.csv.
func (c *Client) FFOpportunity(ctx context.Context, season int) ([]PlayerOpportunity, error) {
	if season <= 0 {
		return nil, fmt.Errorf("ff opportunity: invalid season %d", season)
	}
	var (
		resp *http.Response
		err  error
	)
	if c.baseURL == DefaultBaseURL {
		target := fmt.Sprintf("%s/ep_weekly_%d.csv", DefaultFFOpportunityBaseURL, season)
		resp, err = c.getURL(ctx, target, fmt.Sprintf("/ff_opportunity/ep_weekly_%d.csv", season))
	} else {
		resp, err = c.get(ctx, fmt.Sprintf("/ff_opportunity/ep_weekly_%d.csv", season))
	}
	if err != nil {
		return nil, fmt.Errorf("ff opportunity: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	rows, err := parseFFOpportunity(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ff opportunity: %w", err)
	}
	return rows, nil
}

func parseFFOpportunity(body io.Reader) ([]PlayerOpportunity, error) {
	return parseCSVWithOptional(body, opportunityColumns, optionalOpportunityColumns, parseFFOpportunityRow)
}

func parseFFOpportunityRow(record []string, cols map[string]int, line int) (PlayerOpportunity, bool, error) {
	playerID := cell(record, cols, "player_id")
	teamRaw := cell(record, cols, "posteam")
	seasonRaw := cell(record, cols, "season")
	weekRaw := cell(record, cols, "week")
	if playerID == "" || teamRaw == "" || seasonRaw == "" || weekRaw == "" {
		return PlayerOpportunity{}, false, nil
	}

	season, err := parseInt(seasonRaw, "season", line)
	if err != nil {
		return PlayerOpportunity{}, false, err
	}
	week, err := parseInt(weekRaw, "week", line)
	if err != nil {
		return PlayerOpportunity{}, false, err
	}

	seasonType := seasonTypeFromWeek(season, week)
	if stRaw := optionalCell(record, cols, "season_type"); stRaw != "" {
		seasonType, err = parseSeasonType(stRaw, "season_type", line)
		if err != nil {
			return PlayerOpportunity{}, false, err
		}
	}

	opp := PlayerOpportunity{
		PlayerID:   playerID,
		PlayerName: cell(record, cols, "full_name"),
		Position:   normalizePosition(cell(record, cols, "position")),
		Team:       mapTeam(teamRaw),
		Season:     season,
		SeasonType: seasonType,
		Week:       week,
	}

	floats := []struct {
		col string
		dst *float64
	}{
		{"pass_yards_gained_exp", &opp.ExpPassYds},
		{"rush_yards_gained_exp", &opp.ExpRushYds},
		{"rec_yards_gained_exp", &opp.ExpRecYds},
		{"receptions_exp", &opp.ExpReceptions},
		{"pass_touchdown_exp", &opp.ExpPassTDs},
		{"rush_touchdown_exp", &opp.ExpRushTDs},
		{"rec_touchdown_exp", &opp.ExpRecTDs},
	}
	for _, f := range floats {
		*f.dst, err = parseOptionalFloat(cell(record, cols, f.col), f.col, line)
		if err != nil {
			return PlayerOpportunity{}, false, err
		}
	}

	return opp, true, nil
}

// seasonTypeFromWeek maps an nflverse sequential week number (1..18 regular,
// 19+ postseason since 2021; 1..17 regular, 18+ postseason prior to 2021) into
// ESPN's integer season_type.
func seasonTypeFromWeek(season, week int) int {
	regularWeeks := 18
	if season <= 2020 {
		regularWeeks = 17
	}
	if week > regularWeeks {
		return SeasonTypePostseason
	}
	return SeasonTypeRegular
}
