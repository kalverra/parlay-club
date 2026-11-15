package espn

import (
	"context"
	"fmt"
	"math"
	"net/url"
	"strings"
)

// BoxScore holds per-player game stats parsed from ESPN's /summary endpoint.
// When HasStats is false, ESPN did not return player box score groups (e.g. an
// empty or unpopulated feed), which callers must treat as missing data rather
// than DNP.
type BoxScore struct {
	Completed bool
	HasStats  bool
	Players   []PlayerBoxStats
}

// PlayerBoxStats holds one player's combined box score across all stat groups
// they appeared in. Missing categories default to 0.
type PlayerBoxStats struct {
	PlayerID    string
	DisplayName string
	TeamAbbrev  string
	Passing     PassingStats
	Rushing     RushingStats
	Receiving   ReceivingStats
	OtherTDs    int
	Kicking     KickingStats
}

// PassingStats holds passing box score totals for one player.
type PassingStats struct {
	PassingYards int
	PassingTDs   int
}

// RushingStats holds rushing box score totals for one player.
type RushingStats struct {
	RushingAttempts int
	RushingYards    int
	RushingTDs      int
}

// ReceivingStats holds receiving box score totals for one player.
type ReceivingStats struct {
	Receptions     int
	ReceivingYards int
	ReceivingTDs   int
}

// KickingStats holds kicking box score totals for one player.
type KickingStats struct {
	FieldGoalsMade     int
	ExtraPointsMade    int
	TotalKickingPoints int
}

type rawSummaryBoxScore struct {
	Header struct {
		Competitions []struct {
			Status struct {
				Type struct {
					Completed bool `json:"completed"`
				} `json:"type"`
			} `json:"status"`
		} `json:"competitions"`
	} `json:"header"`
	Boxscore struct {
		Players []rawBoxTeamPlayers `json:"players"`
	} `json:"boxscore"`
}

type rawBoxTeamPlayers struct {
	Team struct {
		Abbreviation string `json:"abbreviation"`
	} `json:"team"`
	Statistics []rawBoxStatGroup `json:"statistics"`
}

type rawBoxStatGroup struct {
	Name     string   `json:"name"`
	Keys     []string `json:"keys"`
	Athletes []struct {
		Athlete struct {
			ID          string `json:"id"`
			DisplayName string `json:"displayName"`
		} `json:"athlete"`
		Stats []string `json:"stats"`
	} `json:"athletes"`
}

// BoxScore fetches GET /summary?event={eventID} and parses boxscore.players[]
// by key name.
func (c *Client) BoxScore(ctx context.Context, eventID string) (BoxScore, error) {
	var raw rawSummaryBoxScore
	query := url.Values{"event": {eventID}}
	if err := c.get(ctx, "/summary", query, &raw); err != nil {
		return BoxScore{}, fmt.Errorf("boxscore %s: %w", eventID, err)
	}
	return parseBoxScore(raw), nil
}

func parseBoxScore(raw rawSummaryBoxScore) BoxScore {
	completed := false
	if len(raw.Header.Competitions) > 0 {
		completed = raw.Header.Competitions[0].Status.Type.Completed
	}

	hasStats := len(raw.Boxscore.Players) > 0
	if !hasStats {
		return BoxScore{
			Completed: completed,
			HasStats:  false,
		}
	}

	byKey := make(map[string]*PlayerBoxStats)
	order := make([]string, 0)

	getOrCreate := func(id, name, team string) *PlayerBoxStats {
		key := id
		if key == "" {
			key = team + ":" + strings.ToLower(strings.TrimSpace(name))
		}
		if existing, ok := byKey[key]; ok {
			if existing.DisplayName == "" {
				existing.DisplayName = name
			}
			if existing.TeamAbbrev == "" {
				existing.TeamAbbrev = team
			}
			return existing
		}
		p := &PlayerBoxStats{
			PlayerID:    id,
			DisplayName: name,
			TeamAbbrev:  team,
		}
		byKey[key] = p
		order = append(order, key)
		return p
	}

	for _, teamBlock := range raw.Boxscore.Players {
		teamAbbrev := strings.TrimSpace(teamBlock.Team.Abbreviation)
		for _, group := range teamBlock.Statistics {
			applyStatGroup(group, teamAbbrev, getOrCreate)
		}
	}

	players := make([]PlayerBoxStats, 0, len(order))
	for _, key := range order {
		players = append(players, *byKey[key])
	}

	return BoxScore{
		Completed: completed,
		HasStats:  true,
		Players:   players,
	}
}

func applyStatGroup(group rawBoxStatGroup, teamAbbrev string, getOrCreate func(id, name, team string) *PlayerBoxStats) {
	hasTotalKickingPts := false
	isKickingGroup := strings.EqualFold(group.Name, "kicking")
	for _, k := range group.Keys {
		if k == "totalKickingPoints" {
			hasTotalKickingPts = true
		}
		switch k {
		case "fieldGoalsMade/fieldGoalAttempts",
			"fieldGoalsMade-fieldGoalAttempts",
			"fieldGoalsMade",
			"extraPointsMade/extraPointAttempts",
			"extraPointsMade-extraPointAttempts",
			"extraPointsMade":
			isKickingGroup = true
		}
	}

	for _, row := range group.Athletes {
		id := strings.TrimSpace(row.Athlete.ID)
		name := strings.TrimSpace(row.Athlete.DisplayName)
		if id == "" && name == "" {
			continue
		}
		p := getOrCreate(id, name, teamAbbrev)
		for i, key := range group.Keys {
			if i >= len(row.Stats) {
				break
			}
			applyBoxKey(p, key, row.Stats[i])
		}
		if isKickingGroup && !hasTotalKickingPts {
			p.Kicking.TotalKickingPoints = p.Kicking.FieldGoalsMade*3 + p.Kicking.ExtraPointsMade
		}
	}
}

func applyBoxKey(p *PlayerBoxStats, key, val string) {
	switch key {
	case "passingYards":
		p.Passing.PassingYards = parseBoxInt(val)
	case "passingTouchdowns":
		p.Passing.PassingTDs = parseBoxInt(val)
	case "rushingAttempts":
		p.Rushing.RushingAttempts = parseBoxInt(val)
	case "rushingYards":
		p.Rushing.RushingYards = parseBoxInt(val)
	case "rushingTouchdowns":
		p.Rushing.RushingTDs = parseBoxInt(val)
	case "receptions":
		p.Receiving.Receptions = parseBoxInt(val)
	case "receivingYards":
		p.Receiving.ReceivingYards = parseBoxInt(val)
	case "receivingTouchdowns":
		p.Receiving.ReceivingTDs = parseBoxInt(val)
	case "kickReturnTouchdowns", "puntReturnTouchdowns", "returnTouchdowns",
		"interceptionTouchdowns", "defensiveTouchdowns":
		p.OtherTDs += parseBoxInt(val)
	case "fieldGoalsMade/fieldGoalAttempts", "fieldGoalsMade-fieldGoalAttempts", "fieldGoalsMade":
		p.Kicking.FieldGoalsMade = int(math.Round(parseMadeAttempts(val)))
	case "extraPointsMade/extraPointAttempts", "extraPointsMade-extraPointAttempts", "extraPointsMade":
		p.Kicking.ExtraPointsMade = int(math.Round(parseMadeAttempts(val)))
	case "totalKickingPoints":
		p.Kicking.TotalKickingPoints = parseBoxInt(val)
	}
}

func parseBoxInt(s string) int {
	return int(math.Round(parseStat(s)))
}
