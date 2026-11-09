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
// when the book supplied none. EventSid is the book's own ID for the event,
// used by books whose combined link needs an event page.
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
	EventSid   string
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

// EventProps fetches one event's prop board from The Odds API's per-event odds
// endpoint and maps it into one row per priced selection. The endpoint bills one
// credit per market returned, so the request asks only for the prop markets.
func (c *Client) EventProps(ctx context.Context, eventID string) ([]Prop, Credits, error) {
	eventID = strings.TrimSpace(eventID)
	if eventID == "" {
		return nil, Credits{}, errors.New("event props: event ID is empty")
	}

	query := url.Values{}
	query.Set("regions", "us")
	query.Set("markets", propsMarkets)
	query.Set("oddsFormat", "american")
	query.Set("includeLinks", "true")
	query.Set("includeSids", "true")

	var raw eventResponse
	path := eventsPath + "/" + url.PathEscape(eventID) + oddsPath
	credits, err := c.get(ctx, path, query, &raw)
	if err != nil {
		return nil, Credits{}, fmt.Errorf("event props: %w", err)
	}

	return parseEventProps(eventID, raw), credits, nil
}

// parseEventProps maps a per-event odds object's prop markets into rows. Books
// and markets this client does not read are skipped, as are outcomes with no
// subject, no price, or no point on a market that needs one.
func parseEventProps(eventID string, event eventResponse) []Prop {
	var props []Prop
	for _, book := range event.Bookmakers {
		bookKey := strings.TrimSpace(book.Key)
		if bookKey == "" {
			continue
		}
		for _, market := range book.Markets {
			marketType, ok := propMarketAliases[market.Key]
			if !ok {
				continue
			}
			for _, outcome := range market.Outcomes {
				prop, ok := mapPropOutcome(eventID, bookKey, book.Sid, marketType, outcome)
				if !ok {
					continue
				}
				props = append(props, prop)
			}
		}
	}
	return props
}

// mapPropOutcome turns one per-event outcome into a prop row, or reports false
// when it should be skipped. Anytime touchdown outcomes name their side in Name
// and carry no point; every other prop prices Over and Under.
func mapPropOutcome(
	eventID, book, eventSid, marketType string,
	outcome outcomeResponse,
) (Prop, bool) {
	subject := strings.TrimSpace(outcome.Description)
	if subject == "" || outcome.Price == nil {
		return Prop{}, false
	}

	prop := Prop{
		EventID:    eventID,
		Book:       book,
		MarketType: marketType,
		Price:      *outcome.Price,
		Link:       outcome.Link,
		Sid:        outcome.Sid,
		EventSid:   eventSid,
	}

	if marketType == MarketAnytimeTD {
		if outcome.Name != "Yes" {
			return Prop{}, false
		}
		prop.PlayerName = subject
		prop.Side = SideYES
		return prop, true
	}

	if outcome.Point == nil {
		return Prop{}, false
	}
	side, err := totalSide(outcome.Name)
	if err != nil {
		return Prop{}, false
	}
	prop.Line = outcome.Point
	prop.Side = side
	if marketType == MarketTeamTotal {
		prop.Team = subject
	} else {
		prop.PlayerName = subject
	}
	return prop, true
}

// MergeProps combines a primary prop board with a secondary one. Rows are keyed
// by book, market, subject, and side, so one book's row gains another's deep
// link, selection ID, and event ID without duplicating the selection. A row that
// gains a link also adopts the linked line and price when they differ, keeping
// the stored line consistent with the deep link. Selections present only in
// secondary are appended.
func MergeProps(primary, secondary []Prop) []Prop {
	if len(primary) == 0 {
		return append([]Prop(nil), secondary...)
	}
	if len(secondary) == 0 {
		return append([]Prop(nil), primary...)
	}

	eventSids := make(map[string]string)
	for _, sec := range secondary {
		if sec.EventSid != "" {
			eventSids[sec.Book] = sec.EventSid
		}
	}

	merged := make([]Prop, len(primary), len(primary)+len(secondary))
	copy(merged, primary)
	index := make(map[propKey]int, len(merged))
	for i, prim := range merged {
		if prim.EventSid == "" {
			prim.EventSid = eventSids[prim.Book]
		}
		merged[i] = prim
		index[propKeyOf(prim)] = i
	}

	for _, sec := range secondary {
		key := propKeyOf(sec)
		i, ok := index[key]
		if !ok {
			index[key] = len(merged)
			merged = append(merged, sec)
			continue
		}

		prim := merged[i]
		switch {
		case prim.Link == "" && sec.Link != "":
			prim.Link = sec.Link
			prim.Sid = sec.Sid
			if sec.EventSid != "" {
				prim.EventSid = sec.EventSid
			}
			if !samePropLine(prim.Line, sec.Line) {
				prim.Line = sec.Line
				prim.Price = sec.Price
			}
		case prim.Link != "":
			if prim.Sid == "" && samePropLine(prim.Line, sec.Line) {
				prim.Sid = sec.Sid
			}
			if prim.EventSid == "" {
				prim.EventSid = sec.EventSid
			}
		}
		merged[i] = prim
	}
	return merged
}

// propKey identifies one selection across providers. The line is deliberately
// excluded so a book's Over at one number can adopt another provider's linked
// number instead of leaving an unlinked duplicate.
type propKey struct {
	book       string
	marketType string
	team       string
	playerName string
	side       string
}

// propKeyOf builds the merge key for a row.
func propKeyOf(prop Prop) propKey {
	return propKey{
		book:       prop.Book,
		marketType: prop.MarketType,
		team:       prop.Team,
		playerName: prop.PlayerName,
		side:       prop.Side,
	}
}

// samePropLine reports whether two optional lines match exactly. Two nil lines
// match, which is what an anytime touchdown carries.
func samePropLine(left, right *float64) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
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
