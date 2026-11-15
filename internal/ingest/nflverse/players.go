package nflverse

import (
	"context"
	"fmt"
	"io"
)

const playersPath = "/players/players.csv"

// Player is one crosswalk row from nflverse's players.csv, mapping GSIS and
// PFR IDs to ESPN's athlete ID.
type Player struct {
	GSISID      string
	PFRID       string
	ESPNID      string
	DisplayName string
	LatestTeam  string // ESPN abbreviation, after mapping
	Position    string
}

var playerColumns = []string{
	"gsis_id",
	"pfr_id",
	"espn_id",
	"display_name",
	"latest_team",
	"position",
}

// Players fetches players.csv and returns one Player per row that carries both
// a GSIS ID and an ESPN ID.
func (c *Client) Players(ctx context.Context) ([]Player, error) {
	resp, err := c.get(ctx, playersPath)
	if err != nil {
		return nil, fmt.Errorf("players: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	players, err := parsePlayers(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("players: %w", err)
	}
	return players, nil
}

func parsePlayers(body io.Reader) ([]Player, error) {
	return parseCSV(body, playerColumns, func(record []string, cols map[string]int, _ int) (Player, bool, error) {
		gsisID := cell(record, cols, "gsis_id")
		espnID := cell(record, cols, "espn_id")
		if gsisID == "" || espnID == "" {
			return Player{}, false, nil
		}
		return Player{
			GSISID:      gsisID,
			PFRID:       cell(record, cols, "pfr_id"),
			ESPNID:      espnID,
			DisplayName: cell(record, cols, "display_name"),
			LatestTeam:  mapTeam(cell(record, cols, "latest_team")),
			Position:    cell(record, cols, "position"),
		}, true, nil
	})
}
