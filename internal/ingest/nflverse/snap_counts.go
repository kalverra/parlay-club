package nflverse

import (
	"context"
	"fmt"
	"io"
)

// SnapCount is one player's weekly offensive snap participation from
// snap_counts_{season}.csv.
type SnapCount struct {
	PFRPlayerID  string
	Player       string
	Team         string // ESPN abbreviation
	Season       int
	SeasonType   int // 1 = pre, 2 = regular, 3 = postseason
	Week         int
	OffenseSnaps int
	OffensePct   float64
}

var snapCountColumns = []string{
	"pfr_player_id",
	"player",
	"team",
	"season",
	"game_type",
	"week",
	"offense_snaps",
	"offense_pct",
}

// SnapCounts fetches snap_counts_{season}.csv and returns one SnapCount per
// usable row.
func (c *Client) SnapCounts(ctx context.Context, season int) ([]SnapCount, error) {
	if season <= 0 {
		return nil, fmt.Errorf("snap counts: invalid season %d", season)
	}
	path := fmt.Sprintf("/snap_counts/snap_counts_%d.csv", season)
	resp, err := c.get(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("snap counts: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	rows, err := parseSnapCounts(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("snap counts: %w", err)
	}
	return rows, nil
}

func parseSnapCounts(body io.Reader) ([]SnapCount, error) {
	return parseCSV(body, snapCountColumns, parseSnapCountRow)
}

func parseSnapCountRow(record []string, cols map[string]int, line int) (SnapCount, bool, error) {
	pfrID := cell(record, cols, "pfr_player_id")
	teamRaw := cell(record, cols, "team")
	seasonRaw := cell(record, cols, "season")
	gameTypeRaw := cell(record, cols, "game_type")
	weekRaw := cell(record, cols, "week")
	if pfrID == "" || teamRaw == "" || seasonRaw == "" || gameTypeRaw == "" || weekRaw == "" {
		return SnapCount{}, false, nil
	}

	season, err := parseInt(seasonRaw, "season", line)
	if err != nil {
		return SnapCount{}, false, err
	}
	week, err := parseInt(weekRaw, "week", line)
	if err != nil {
		return SnapCount{}, false, err
	}
	seasonType, err := parseSeasonType(gameTypeRaw, "game_type", line)
	if err != nil {
		return SnapCount{}, false, err
	}
	snaps, err := parseOptionalInt(cell(record, cols, "offense_snaps"), "offense_snaps", line)
	if err != nil {
		return SnapCount{}, false, err
	}
	pct, err := parseOptionalFloat(cell(record, cols, "offense_pct"), "offense_pct", line)
	if err != nil {
		return SnapCount{}, false, err
	}

	return SnapCount{
		PFRPlayerID:  pfrID,
		Player:       cell(record, cols, "player"),
		Team:         mapTeam(teamRaw),
		Season:       season,
		SeasonType:   seasonType,
		Week:         week,
		OffenseSnaps: snaps,
		OffensePct:   pct,
	}, true, nil
}
