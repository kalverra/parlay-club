package espn

import (
	"context"
	"fmt"
	"net/url"
)

// RosterPlayer is one player on an NFL team roster.
type RosterPlayer struct {
	ID          string
	FullName    string
	Position    string
	HeadshotURL string
}

// rosterResponse is the slice of ESPN's team roster JSON this app reads.
type rosterResponse struct {
	Athletes []struct {
		Items []struct {
			ID       string `json:"id"`
			FullName string `json:"fullName"`
			Position struct {
				Abbreviation string `json:"abbreviation"`
			} `json:"position"`
			Headshot struct {
				Href string `json:"href"`
			} `json:"headshot"`
		} `json:"items"`
	} `json:"athletes"`
}

// Roster fetches a team's roster and flattens the position groups into
// athletes, skipping athletes without an ID or full name.
func (c *Client) Roster(ctx context.Context, espnTeamID string) ([]RosterPlayer, error) {
	path := fmt.Sprintf("/teams/%s/roster", url.PathEscape(espnTeamID))
	var raw rosterResponse
	if err := c.get(ctx, path, nil, &raw); err != nil {
		return nil, fmt.Errorf("roster %s: %w", espnTeamID, err)
	}

	players := make([]RosterPlayer, 0)
	for _, group := range raw.Athletes {
		for _, item := range group.Items {
			if item.ID == "" || item.FullName == "" {
				continue
			}
			players = append(players, RosterPlayer{
				ID:          item.ID,
				FullName:    item.FullName,
				Position:    item.Position.Abbreviation,
				HeadshotURL: normalizeHTTPSURL(item.Headshot.Href),
			})
		}
	}
	return players, nil
}
