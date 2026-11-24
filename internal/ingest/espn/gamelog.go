package espn

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// GameLogEntry holds per-game opponent and result metadata aligned with GameLog.Markets.
type GameLogEntry struct {
	EventID  string `json:"eventId"`
	Week     int    `json:"week"`
	Opponent string `json:"opponent"`
	AtVs     string `json:"atVs"`
	Result   string `json:"result"`
	Score    string `json:"score"`
}

// GameLog holds the parsed game log for an athlete, most recent games first.
type GameLog struct {
	AthleteID   string
	GamesPlayed int
	Markets     map[string][]float64
	Entries     []GameLogEntry
}

// rawGamelogResponse is the slice of ESPN's athlete gamelog JSON this app reads.
type rawGamelogResponse struct {
	Names       []string                   `json:"names"`
	Labels      []string                   `json:"labels"`
	Categories  []rawCategory              `json:"categories"`
	SeasonTypes []rawSeasonType            `json:"seasonTypes"`
	Events      map[string]rawGamelogEvent `json:"events"`
}

type rawGamelogEvent struct {
	Week       int    `json:"week"`
	AtVs       string `json:"atVs"`
	GameResult string `json:"gameResult"`
	Score      string `json:"score"`
	Opponent   struct {
		Abbreviation string `json:"abbreviation"`
	} `json:"opponent"`
}

type rawCategory struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Count       int    `json:"count"`
}

type rawSeasonType struct {
	DisplayName string `json:"displayName"`
	Categories  []struct {
		DisplayName string `json:"displayName"`
		SplitType   string `json:"splitType"`
		Events      []struct {
			EventID string   `json:"eventId"`
			Stats   []string `json:"stats"`
		} `json:"events"`
	} `json:"categories"`
}

// GameLog fetches an athlete's game log from ESPN's web API.
func (c *Client) GameLog(ctx context.Context, athleteID string) (GameLog, error) {
	path := fmt.Sprintf("/athletes/%s/gamelog", url.PathEscape(athleteID))
	var raw rawGamelogResponse
	if err := c.getWeb(ctx, path, nil, &raw); err != nil {
		return GameLog{}, fmt.Errorf("gamelog %s: %w", athleteID, err)
	}

	return parseGameLog(athleteID, raw), nil
}

// ParseGameLogJSON decodes raw ESPN athlete gamelog JSON into per-market stat
// slices, most recent games first.
func ParseGameLogJSON(raw []byte) (map[string][]float64, error) {
	var parsed rawGamelogResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("parse gamelog json: %w", err)
	}
	return parseGameLog("", parsed).Markets, nil
}

type colIndices struct {
	passYds    int
	passTDs    int
	rushYds    int
	rushAtt    int
	recYds     int
	rec        int
	rushTDs    int
	recTDs     int
	retTDs     int
	fgMade     int
	xpMade     int
	kickingPts int
}

func newColIndices() colIndices {
	return colIndices{
		passYds:    -1,
		passTDs:    -1,
		rushYds:    -1,
		rushAtt:    -1,
		recYds:     -1,
		rec:        -1,
		rushTDs:    -1,
		recTDs:     -1,
		retTDs:     -1,
		fgMade:     -1,
		xpMade:     -1,
		kickingPts: -1,
	}
}

func initMarkets(cols colIndices) map[string][]float64 {
	m := make(map[string][]float64)
	if cols.passYds != -1 {
		m["PASS_YDS"] = make([]float64, 0)
	}
	if cols.passTDs != -1 {
		m["PASS_TDS"] = make([]float64, 0)
	}
	if cols.rushYds != -1 {
		m["RUSH_YDS"] = make([]float64, 0)
	}
	if cols.rushAtt != -1 {
		m["RUSH_ATT"] = make([]float64, 0)
	}
	if cols.recYds != -1 {
		m["REC_YDS"] = make([]float64, 0)
	}
	if cols.rec != -1 {
		m["RECEPTIONS"] = make([]float64, 0)
	}
	if cols.rushTDs != -1 || cols.recTDs != -1 || cols.retTDs != -1 {
		m["ANYTIME_TD"] = make([]float64, 0)
	}
	if (cols.fgMade != -1 && cols.xpMade != -1) || cols.kickingPts != -1 {
		m["KICKER_PTS"] = make([]float64, 0)
	}
	return m
}

func isPreseason(stName, catName, splitType string) bool {
	if splitType == "1" {
		return true
	}
	return strings.Contains(stName, "preseason") || strings.Contains(catName, "preseason")
}

func parseGameLog(athleteID string, raw rawGamelogResponse) GameLog {
	cols := resolveColumnIndices(raw)
	markets := initMarkets(cols)
	entries := make([]GameLogEntry, 0)
	gamesPlayed := 0

	for _, st := range raw.SeasonTypes {
		stName := strings.ToLower(st.DisplayName)
		for _, cat := range st.Categories {
			catName := strings.ToLower(cat.DisplayName)
			if isPreseason(stName, catName, cat.SplitType) {
				continue
			}

			for _, ev := range cat.Events {
				gamesPlayed++
				recordEventStats(ev.Stats, cols, markets)
				entries = append(entries, buildGameLogEntry(ev.EventID, raw.Events))
			}
		}
	}

	return GameLog{
		AthleteID:   athleteID,
		GamesPlayed: gamesPlayed,
		Markets:     markets,
		Entries:     entries,
	}
}

func buildGameLogEntry(eventID string, events map[string]rawGamelogEvent) GameLogEntry {
	entry := GameLogEntry{EventID: eventID}
	if meta, ok := events[eventID]; ok {
		entry.Week = meta.Week
		entry.Opponent = meta.Opponent.Abbreviation
		entry.AtVs = meta.AtVs
		entry.Result = meta.GameResult
		entry.Score = meta.Score
	}
	return entry
}

func recordEventStats(stats []string, cols colIndices, markets map[string][]float64) {
	if cols.passYds != -1 {
		markets["PASS_YDS"] = append(markets["PASS_YDS"], parseStat(getStat(stats, cols.passYds)))
	}
	if cols.passTDs != -1 {
		markets["PASS_TDS"] = append(markets["PASS_TDS"], parseStat(getStat(stats, cols.passTDs)))
	}
	if cols.rushYds != -1 {
		markets["RUSH_YDS"] = append(markets["RUSH_YDS"], parseStat(getStat(stats, cols.rushYds)))
	}
	if cols.rushAtt != -1 {
		markets["RUSH_ATT"] = append(markets["RUSH_ATT"], parseStat(getStat(stats, cols.rushAtt)))
	}
	if cols.recYds != -1 {
		markets["REC_YDS"] = append(markets["REC_YDS"], parseStat(getStat(stats, cols.recYds)))
	}
	if cols.rec != -1 {
		markets["RECEPTIONS"] = append(markets["RECEPTIONS"], parseStat(getStat(stats, cols.rec)))
	}
	if _, ok := markets["ANYTIME_TD"]; ok {
		rush := statOrZero(stats, cols.rushTDs)
		rec := statOrZero(stats, cols.recTDs)
		ret := statOrZero(stats, cols.retTDs)
		markets["ANYTIME_TD"] = append(markets["ANYTIME_TD"], rush+rec+ret)
	}
	if _, ok := markets["KICKER_PTS"]; ok {
		markets["KICKER_PTS"] = append(markets["KICKER_PTS"], kickerPoints(stats, cols))
	}
}

func statOrZero(stats []string, col int) float64 {
	if col == -1 {
		return 0
	}
	return parseStat(getStat(stats, col))
}

func kickerPoints(stats []string, cols colIndices) float64 {
	if cols.fgMade != -1 && cols.xpMade != -1 {
		fg := parseMadeAttempts(getStat(stats, cols.fgMade))
		xp := parseMadeAttempts(getStat(stats, cols.xpMade))
		return fg*3.0 + xp
	}
	if cols.kickingPts != -1 {
		return parseStat(getStat(stats, cols.kickingPts))
	}
	return 0
}

func resolveColumnIndices(raw rawGamelogResponse) colIndices {
	cols := newColIndices()
	maxCols := max(len(raw.Names), len(raw.Labels))

	catForCol := make([]string, maxCols)
	idx := 0
	for _, cat := range raw.Categories {
		for c := 0; c < cat.Count && idx < len(catForCol); c++ {
			catForCol[idx] = strings.ToLower(cat.Name)
			idx++
		}
	}

	for i := range maxCols {
		name := ""
		if i < len(raw.Names) {
			name = raw.Names[i]
		}
		label := ""
		if i < len(raw.Labels) {
			label = strings.TrimSpace(raw.Labels[i])
		}
		cat := catForCol[i]

		matchColumnByName(name, i, &cols)
		matchColumnByCatLabel(cat, label, i, &cols)
	}

	return cols
}

func matchColumnByName(name string, i int, cols *colIndices) {
	switch name {
	case "passingYards":
		cols.passYds = i
	case "passingTouchdowns":
		cols.passTDs = i
	case "rushingYards":
		cols.rushYds = i
	case "rushingAttempts":
		cols.rushAtt = i
	case "receivingYards":
		cols.recYds = i
	case "receptions":
		cols.rec = i
	case "rushingTouchdowns":
		cols.rushTDs = i
	case "receivingTouchdowns":
		cols.recTDs = i
	case "returnTouchdowns", "kickReturnTouchdowns", "puntReturnTouchdowns":
		cols.retTDs = i
	case "fieldGoalsMade-fieldGoalAttempts", "fieldGoalsMade":
		cols.fgMade = i
	case "extraPointsMade-extraPointAttempts", "extraPointsMade":
		cols.xpMade = i
	case "totalKickingPoints":
		cols.kickingPts = i
	}
}

func matchColumnByCatLabel(cat, label string, i int, cols *colIndices) {
	switch cat {
	case "passing":
		switch label {
		case "YDS":
			if cols.passYds == -1 {
				cols.passYds = i
			}
		case "TD":
			if cols.passTDs == -1 {
				cols.passTDs = i
			}
		}
	case "rushing":
		switch label {
		case "YDS":
			if cols.rushYds == -1 {
				cols.rushYds = i
			}
		case "CAR", "ATT":
			if cols.rushAtt == -1 {
				cols.rushAtt = i
			}
		case "TD":
			if cols.rushTDs == -1 {
				cols.rushTDs = i
			}
		}
	case "receiving":
		switch label {
		case "YDS":
			if cols.recYds == -1 {
				cols.recYds = i
			}
		case "REC":
			if cols.rec == -1 {
				cols.rec = i
			}
		case "TD":
			if cols.recTDs == -1 {
				cols.recTDs = i
			}
		}
	case "kicking":
		switch label {
		case "FG":
			if cols.fgMade == -1 {
				cols.fgMade = i
			}
		case "XP":
			if cols.xpMade == -1 {
				cols.xpMade = i
			}
		case "PTS":
			if cols.kickingPts == -1 {
				cols.kickingPts = i
			}
		}
	}
}

func getStat(stats []string, idx int) string {
	if idx < 0 || idx >= len(stats) {
		return ""
	}
	return stats[idx]
}

func parseStat(s string) float64 {
	s = strings.TrimSpace(strings.ReplaceAll(s, ",", ""))
	if s == "" || s == "-" {
		return 0
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return v
}

func parseMadeAttempts(s string) float64 {
	s = strings.TrimSpace(strings.ReplaceAll(s, ",", ""))
	if s == "" || s == "-" {
		return 0
	}
	if idx := strings.IndexAny(s, "/-"); idx >= 0 {
		return parseStat(s[:idx])
	}
	return parseStat(s)
}
