package espn

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// TeamInjuries holds key injuries for one team in a game.
type TeamInjuries struct {
	Team         string   `json:"team"`
	TeamID       string   `json:"team_id"`
	Abbreviation string   `json:"abbreviation"`
	Injuries     []Injury `json:"injuries"`
}

// Injury holds one player's injury report.
type Injury struct {
	AthleteID string `json:"athlete_id"`
	FullName  string `json:"full_name"`
	Position  string `json:"position"`
	Status    string `json:"status"`
	Detail    string `json:"detail"`
}

// SummaryInjuries fetches GET /summary?event={eventID} and returns the raw
// injuries JSON array (or "[]" when absent).
func (c *Client) SummaryInjuries(ctx context.Context, eventID string) (string, error) {
	var raw struct {
		Injuries json.RawMessage `json:"injuries"`
	}
	query := url.Values{"event": {eventID}}
	if err := c.get(ctx, "/summary", query, &raw); err != nil {
		return "", fmt.Errorf("summary injuries %s: %w", eventID, err)
	}
	if len(raw.Injuries) == 0 || string(raw.Injuries) == "null" {
		return "[]", nil
	}
	return string(raw.Injuries), nil
}

// keyPositions defines the positions kept when parsing injury reports:
// QB, RB, WR, TE, LT, C, CB, S. OT is accepted as tackle in ESPN's taxonomy.
var keyPositions = map[string]bool{
	"QB": true,
	"RB": true,
	"WR": true,
	"TE": true,
	"LT": true,
	"OT": true,
	"C":  true,
	"CB": true,
	"S":  true,
}

// rawInjuryTeam represents one team's injury list in ESPN's summary JSON.
type rawInjuryTeam struct {
	Team struct {
		ID           string `json:"id"`
		DisplayName  string `json:"displayName"`
		Abbreviation string `json:"abbreviation"`
	} `json:"team"`
	Injuries []struct {
		Status  string `json:"status"`
		Athlete struct {
			ID       string `json:"id"`
			FullName string `json:"fullName"`
			Position struct {
				Abbreviation string `json:"abbreviation"`
			} `json:"position"`
		} `json:"athlete"`
		Details struct {
			Type   string `json:"type"`
			Detail string `json:"detail"`
		} `json:"details"`
	} `json:"injuries"`
}

// ParseInjuries extracts per-team injuries filtered to key positions
// (QB, RB, WR, TE, LT, C, CB, S).
func ParseInjuries(rawJSON string) ([]TeamInjuries, error) {
	trimmed := strings.TrimSpace(rawJSON)
	if trimmed == "" || trimmed == "[]" || trimmed == "null" {
		return []TeamInjuries{}, nil
	}

	var rawTeams []rawInjuryTeam
	if err := json.Unmarshal([]byte(rawJSON), &rawTeams); err != nil {
		return nil, fmt.Errorf("parse injuries: %w", err)
	}

	result := make([]TeamInjuries, 0, len(rawTeams))
	for _, rawTeam := range rawTeams {
		teamName := rawTeam.Team.DisplayName
		if teamName == "" {
			teamName = rawTeam.Team.Abbreviation
		}

		injuries := make([]Injury, 0)
		for _, item := range rawTeam.Injuries {
			pos := strings.ToUpper(strings.TrimSpace(item.Athlete.Position.Abbreviation))
			if !keyPositions[pos] {
				continue
			}
			if item.Athlete.ID == "" || item.Athlete.FullName == "" {
				continue
			}

			detail := item.Details.Type
			if item.Details.Detail != "" && item.Details.Detail != "Not Specified" {
				if detail != "" {
					detail = detail + " (" + item.Details.Detail + ")"
				} else {
					detail = item.Details.Detail
				}
			}

			injuries = append(injuries, Injury{
				AthleteID: item.Athlete.ID,
				FullName:  item.Athlete.FullName,
				Position:  item.Athlete.Position.Abbreviation,
				Status:    item.Status,
				Detail:    detail,
			})
		}

		result = append(result, TeamInjuries{
			Team:         teamName,
			TeamID:       rawTeam.Team.ID,
			Abbreviation: rawTeam.Team.Abbreviation,
			Injuries:     injuries,
		})
	}

	return result, nil
}
