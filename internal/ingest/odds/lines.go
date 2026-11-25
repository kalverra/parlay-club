package odds

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

// Market types this client maps into odds rows. The strings match the app's
// market names.
const (
	MarketML     = "ML"
	MarketSpread = "SPREAD"
	MarketTotal  = "TOTAL"
)

// Sides a game-line row can carry. TEAM means the outcome names a team; OVER
// and UNDER belong to a total.
const (
	SideTeam  = "TEAM"
	SideOver  = "OVER"
	SideUnder = "UNDER"
)

// Parlay API market keys this client reads.
const (
	marketH2H     = "h2h"
	marketSpreads = "spreads"
	marketTotals  = "totals"
)

// Line is one game-line row from one book, shaped like the odds table. Team is
// the provider's display name; the caller maps it to an ESPN abbreviation. Line
// is nil for a moneyline and set for a spread or total. Price is American odds.
// Link and Sid are the book's deep link and selection ID for this outcome, and
// are empty when the book supplied none. EventSid is the book's own ID for the
// event, used by books whose combined link needs an event page.
type Line struct {
	EventID    string
	Book       string
	MarketType string
	Team       string
	Side       string
	Line       *float64
	Price      int
	Link       string
	Sid        string
	EventSid   string
}

// Lines fetches every book's moneyline, spread, and total for the events Parlay
// API returns, and maps them into one row per priced selection. Unknown markets
// are ignored so the same client can serve later prop requests; an outcome that
// is missing its price, or its point on a spread or total, is skipped rather
// than failing the whole slate.
func (c *Client) Lines(ctx context.Context) ([]Line, Credits, error) {
	query := url.Values{}
	query.Set("regions", "us")
	query.Set("markets", "h2h,spreads,totals")
	query.Set("oddsFormat", "american")
	query.Set("includeLinks", "true")
	query.Set("includeSids", "true")

	var raw []eventResponse
	credits, err := c.get(ctx, oddsPath, query, &raw)
	if err != nil {
		return nil, Credits{}, fmt.Errorf("lines: %w", err)
	}

	lines, err := parseLines(raw)
	if err != nil {
		return nil, Credits{}, fmt.Errorf("lines: %w", err)
	}
	return lines, credits, nil
}

// ParseLinesJSON decodes raw Parlay API or Odds API game-lines JSON into
// one row per priced selection.
func ParseLinesJSON(raw []byte) ([]Line, error) {
	var parsed []eventResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("parse lines json: %w", err)
	}
	return parseLines(parsed)
}

// ParseLinesSnapshotJSON decodes raw Parlay API or Odds API game-lines JSON
// into both its event metadata and its priced selection rows.
func ParseLinesSnapshotJSON(raw []byte) ([]Event, []Line, error) {
	var parsed []eventResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, nil, fmt.Errorf("parse lines snapshot json: %w", err)
	}
	events, err := parseEvents(parsed)
	if err != nil {
		return nil, nil, err
	}
	lines, err := parseLines(parsed)
	if err != nil {
		return nil, nil, err
	}
	return events, lines, nil
}

// parseLines maps the game-line markets of every event and book into rows.
func parseLines(raw []eventResponse) ([]Line, error) {
	var lines []Line
	for _, event := range raw {
		for _, book := range event.Bookmakers {
			for _, market := range book.Markets {
				mapped, err := mapMarket(event, book, market)
				if err != nil {
					return nil, fmt.Errorf("event %s book %s market %s: %w",
						event.ID, book.Key, market.Key, err)
				}
				lines = append(lines, mapped...)
			}
		}
	}
	return lines, nil
}

// mapMarket turns one market's outcomes into rows, or nil for a market this
// client does not read.
func mapMarket(event eventResponse, book bookmakerResponse, market marketResponse) ([]Line, error) {
	switch market.Key {
	case marketH2H:
		return mapTeamMarket(event, book, market, MarketML, false)
	case marketSpreads:
		return mapTeamMarket(event, book, market, MarketSpread, true)
	case marketTotals:
		return mapTotalMarket(event, book, market)
	default:
		return nil, nil
	}
}

// mapTeamMarket maps a moneyline or spread. Both outcomes name teams; a spread
// must carry a point. Unpriced outcomes are skipped, and so is a moneyline
// outcome that names neither team, such as a three-way feed's draw.
func mapTeamMarket(
	event eventResponse,
	book bookmakerResponse,
	market marketResponse,
	marketType string,
	needsPoint bool,
) ([]Line, error) {
	lines := make([]Line, 0, len(market.Outcomes))
	for _, outcome := range market.Outcomes {
		if outcome.Price == nil {
			continue
		}
		if outcome.Name == "" {
			return nil, fmt.Errorf("outcome %q has no name", outcome.Name)
		}
		if marketType == MarketML && outcome.Name != event.HomeTeam && outcome.Name != event.AwayTeam {
			continue
		}
		if needsPoint && outcome.Point == nil {
			continue
		}

		lines = append(lines, Line{
			EventID:    event.ID,
			Book:       book.Key,
			MarketType: marketType,
			Team:       outcome.Name,
			Side:       SideTeam,
			Line:       outcome.Point,
			Price:      *outcome.Price,
			Link:       outcome.Link,
			Sid:        outcome.Sid,
			EventSid:   book.Sid,
		})
	}
	return lines, nil
}

// mapTotalMarket maps a total's Over and Under. Each outcome carries the point
// it prices, and the team column stays empty. Unpriced outcomes are skipped.
func mapTotalMarket(event eventResponse, book bookmakerResponse, market marketResponse) ([]Line, error) {
	lines := make([]Line, 0, len(market.Outcomes))
	for _, outcome := range market.Outcomes {
		if outcome.Price == nil || outcome.Point == nil {
			continue
		}
		side, err := totalSide(outcome.Name)
		if err != nil {
			return nil, err
		}

		lines = append(lines, Line{
			EventID:    event.ID,
			Book:       book.Key,
			MarketType: MarketTotal,
			Side:       side,
			Line:       outcome.Point,
			Price:      *outcome.Price,
			Link:       outcome.Link,
			Sid:        outcome.Sid,
			EventSid:   book.Sid,
		})
	}
	return lines, nil
}

// totalSide maps an outcome name to OVER or UNDER.
func totalSide(name string) (string, error) {
	switch name {
	case "Over":
		return SideOver, nil
	case "Under":
		return SideUnder, nil
	default:
		return "", fmt.Errorf("unknown total outcome %q", name)
	}
}
