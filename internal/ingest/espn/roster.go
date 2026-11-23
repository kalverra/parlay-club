package espn

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

// RosterPlayer is one player on an NFL team roster.
type RosterPlayer struct {
	ID           string
	FullName     string
	Position     string
	HeadshotURL  string
	InjuryStatus string
	InjuryDate   string
}

type rosterInjury struct {
	Status string `json:"status"`
	Date   string `json:"date"`
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
			Injuries []rosterInjury `json:"injuries"`
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
			injuryStatus, injuryDate := selectRosterInjury(item.Injuries)
			players = append(players, RosterPlayer{
				ID:           item.ID,
				FullName:     item.FullName,
				Position:     item.Position.Abbreviation,
				HeadshotURL:  normalizeHTTPSURL(item.Headshot.Href),
				InjuryStatus: injuryStatus,
				InjuryDate:   injuryDate,
			})
		}
	}
	return players, nil
}

func selectRosterInjury(injuries []rosterInjury) (string, string) {
	var bestStatus, bestDate string
	for _, inj := range injuries {
		status := strings.TrimSpace(inj.Status)
		if status == "" {
			continue
		}
		if bestStatus == "" || injuryStatusPriority(status) > injuryStatusPriority(bestStatus) {
			bestStatus = status
			bestDate = strings.TrimSpace(inj.Date)
		}
	}
	return bestStatus, bestDate
}

func injuryStatusPriority(status string) int {
	switch strings.ToLower(status) {
	case "out":
		return 4
	case "doubtful":
		return 3
	case "questionable":
		return 2
	case "injured reserve":
		return 1
	default:
		return 0
	}
}
