package odds

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// Prop market types this client maps into odds rows. The strings match the
// app's market names.
const (
	MarketPassYds    = "PASS_YDS"
	MarketPassTds    = "PASS_TDS"
	MarketRushYds    = "RUSH_YDS"
	MarketRushAtt    = "RUSH_ATT"
	MarketRecYds     = "REC_YDS"
	MarketReceptions = "RECEPTIONS"
	MarketAnytimeTD  = "ANYTIME_TD"
	MarketKickerPts  = "KICKER_PTS"
	MarketTeamTotal  = "TEAM_TOTAL"
)

// SideYES is the only side an anytime touchdown prop carries. It has no line.
const SideYES = "YES"

// propsPath is the bulk prop endpoint under the base URL.
const propsPath = "/props"

// propsMarkets is the canonical market filter sent to /props. Parlay API may
// still return rows tagged with book-specific alias keys, which parseProps
// maps back.
const propsMarkets = "team_totals," +
	"player_pass_yds," +
	"player_pass_tds," +
	"player_rush_yds," +
	"player_rush_attempts," +
	"player_reception_yds," +
	"player_receptions," +
	"player_anytime_td," +
	"player_kicking_points"

// propMarketAliases maps every canonical and Parlay API alias market key to
// this client's market name.
var propMarketAliases = map[string]string{
	"player_pass_yds":                 MarketPassYds,
	"player_passing_yards":            MarketPassYds,
	"player_pass_yards":               MarketPassYds,
	"player_pass_tds":                 MarketPassTds,
	"player_passing_tds":              MarketPassTds,
	"player_passing_touchdowns":       MarketPassTds,
	"player_rush_yds":                 MarketRushYds,
	"player_rushing_yards":            MarketRushYds,
	"player_rush_yards":               MarketRushYds,
	"player_rush_attempts":            MarketRushAtt,
	"player_rushing_attempts":         MarketRushAtt,
	"player_reception_yds":            MarketRecYds,
	"player_receiving_yds":            MarketRecYds,
	"player_receiving_yards":          MarketRecYds,
	"player_receptions":               MarketReceptions,
	"player_anytime_td":               MarketAnytimeTD,
	"player_anytime_touchdown_scorer": MarketAnytimeTD,
	"anytime_td":                      MarketAnytimeTD,
	"anytime_touchdown_scorer":        MarketAnytimeTD,
	"player_kicking_points":           MarketKickerPts,
	"team_totals":                     MarketTeamTotal,
}

// Prop is one prop selection from one book, shaped like the odds table. Team is
// set for TEAM_TOTAL and PlayerName for player props. Line is nil for an
// anytime touchdown, matching the schema. Side is OVER, UNDER, or YES. Link and
// Sid are the book's deep link and selection ID for this side, and are empty
// when the book supplied none.
type Prop struct {
	EventID    string
	Book       string
	MarketType string
	Team       string
	PlayerName string
	Side       string
	Line       *float64
	Price      int
	Link       string
	Sid        string
}

// Props fetches the full-game prop board for one event and maps it into one row
// per priced selection. Parlay API bills one flat cost per call whatever the
// number of markets and books, so the request asks for every prop market at
// once with the largest page the endpoint allows.
func (c *Client) Props(ctx context.Context, eventID string) ([]Prop, Credits, error) {
	if strings.TrimSpace(eventID) == "" {
		return nil, Credits{}, errors.New("props: event ID is empty")
	}

	query := url.Values{}
	query.Set("eventId", eventID)
	query.Set("markets", propsMarkets)
	query.Set("oddsFormat", "american")
	query.Set("includeLinks", "true")
	query.Set("includeSids", "true")
	query.Set("limit", "10000")

	var raw []propResponse
	credits, err := c.get(ctx, propsPath, query, &raw)
	if err != nil {
		return nil, Credits{}, fmt.Errorf("props: %w", err)
	}

	return parseProps(eventID, raw), credits, nil
}

// parseProps maps a /props response into rows. Rows for another event, rows
// outside the full game, rows with an unknown market, and rows with no subject
// or book are skipped rather than failing the whole board.
func parseProps(eventID string, raw []propResponse) []Prop {
	var props []Prop
	for _, item := range raw {
		if item.EventID != "" && item.EventID != eventID {
			continue
		}
		if item.Period != "" && item.Period != "FULL" {
			continue
		}

		subject := strings.TrimSpace(item.Player)
		if subject == "" {
			subject = strings.TrimSpace(item.PlayerName)
		}
		book := strings.TrimSpace(item.Bookmaker)
		if subject == "" || book == "" {
			continue
		}

		marketType, ok := propMarketAliases[item.MarketKey]
		if !ok {
			continue
		}

		base := Prop{
			EventID:    eventID,
			Book:       book,
			MarketType: marketType,
		}

		if marketType == MarketAnytimeTD {
			if item.OverPrice == nil {
				continue
			}
			base.PlayerName = subject
			base.Side = SideYES
			base.Price = *item.OverPrice
			base.Link = item.OverLink
			base.Sid = item.OverSid
			props = append(props, base)
			continue
		}

		if item.Line == nil {
			continue
		}
		base.Line = item.Line
		if marketType == MarketTeamTotal {
			base.Team = subject
		} else {
			base.PlayerName = subject
		}

		if item.OverPrice != nil {
			over := base
			over.Side = SideOver
			over.Price = *item.OverPrice
			over.Link = item.OverLink
			over.Sid = item.OverSid
			props = append(props, over)
		}
		if item.UnderPrice != nil {
			under := base
			under.Side = SideUnder
			under.Price = *item.UnderPrice
			under.Link = item.UnderLink
			under.Sid = item.UnderSid
			props = append(props, under)
		}
	}
	return props
}

// propResponse is one flat prop quote from Parlay API's /props array.
type propResponse struct {
	EventID    string   `json:"event_id"`
	Bookmaker  string   `json:"bookmaker"`
	Player     string   `json:"player"`
	PlayerName string   `json:"player_name"`
	MarketKey  string   `json:"market_key"`
	Period     string   `json:"period"`
	Line       *float64 `json:"line"`
	OverPrice  *int     `json:"over_price"`
	UnderPrice *int     `json:"under_price"`
	OverLink   string   `json:"over_link"`
	UnderLink  string   `json:"under_link"`
	OverSid    string   `json:"over_sid"`
	UnderSid   string   `json:"under_sid"`
}
