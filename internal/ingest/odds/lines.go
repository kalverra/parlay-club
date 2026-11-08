package odds

import (
	"context"
	"errors"
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

// The Odds API market keys this client reads.
const (
	marketH2H     = "h2h"
	marketSpreads = "spreads"
	marketTotals  = "totals"
)

// Line is one game-line row from one book, shaped like the odds table. Team is
// The Odds API's display name; the caller maps it to an ESPN abbreviation. Line
// is nil for a moneyline and set for a spread or total. Price is American odds.
// Link and Sid are the book's deep link and selection ID for this outcome, and
// are empty when the book supplied none.
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
}

// Lines fetches every book's moneyline, spread, and total for the events The
// Odds API returns, and maps them into one row per selection. Unknown markets
// are ignored so the same client can serve later prop requests; a malformed
// moneyline, spread, or total is an error rather than a guess.
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

// parseLines maps the game-line markets of every event and book into rows.
func parseLines(raw []eventResponse) ([]Line, error) {
	var lines []Line
	for _, event := range raw {
		for _, book := range event.Bookmakers {
			for _, market := range book.Markets {
				mapped, err := mapMarket(event.ID, book.Key, market)
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
func mapMarket(eventID, book string, market marketResponse) ([]Line, error) {
	switch market.Key {
	case marketH2H:
		return mapTeamMarket(eventID, book, market, MarketML, false)
	case marketSpreads:
		return mapTeamMarket(eventID, book, market, MarketSpread, true)
	case marketTotals:
		return mapTotalMarket(eventID, book, market)
	default:
		return nil, nil
	}
}

// mapTeamMarket maps a moneyline or spread. Both outcomes name teams; a spread
// must carry a point.
func mapTeamMarket(
	eventID, book string,
	market marketResponse,
	marketType string,
	needsPoint bool,
) ([]Line, error) {
	lines := make([]Line, 0, len(market.Outcomes))
	for _, outcome := range market.Outcomes {
		price, err := outcomePrice(outcome)
		if err != nil {
			return nil, fmt.Errorf("outcome %q: %w", outcome.Name, err)
		}
		if outcome.Name == "" {
			return nil, fmt.Errorf("outcome %q has no name", outcome.Name)
		}
		line, err := outcomePoint(outcome, needsPoint)
		if err != nil {
			return nil, fmt.Errorf("outcome %q: %w", outcome.Name, err)
		}

		lines = append(lines, Line{
			EventID:    eventID,
			Book:       book,
			MarketType: marketType,
			Team:       outcome.Name,
			Side:       SideTeam,
			Line:       line,
			Price:      price,
			Link:       outcome.Link,
			Sid:        outcome.Sid,
		})
	}
	return lines, nil
}

// mapTotalMarket maps a total's Over and Under. Each outcome carries the point
// it prices, and the team column stays empty.
func mapTotalMarket(eventID, book string, market marketResponse) ([]Line, error) {
	lines := make([]Line, 0, len(market.Outcomes))
	for _, outcome := range market.Outcomes {
		price, err := outcomePrice(outcome)
		if err != nil {
			return nil, fmt.Errorf("total outcome %q: %w", outcome.Name, err)
		}
		line, err := outcomePoint(outcome, true)
		if err != nil {
			return nil, fmt.Errorf("total outcome %q: %w", outcome.Name, err)
		}
		side, err := totalSide(outcome.Name)
		if err != nil {
			return nil, err
		}

		lines = append(lines, Line{
			EventID:    eventID,
			Book:       book,
			MarketType: MarketTotal,
			Side:       side,
			Line:       line,
			Price:      price,
			Link:       outcome.Link,
			Sid:        outcome.Sid,
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

// outcomePrice requires a price on every outcome.
func outcomePrice(outcome outcomeResponse) (int, error) {
	if outcome.Price == nil {
		return 0, errors.New("price is missing")
	}
	return *outcome.Price, nil
}

// outcomePoint returns the line for a market that needs one, and nil otherwise.
func outcomePoint(outcome outcomeResponse, needed bool) (*float64, error) {
	if !needed {
		return nil, nil
	}
	if outcome.Point == nil {
		return nil, errors.New("point is missing")
	}
	return outcome.Point, nil
}
