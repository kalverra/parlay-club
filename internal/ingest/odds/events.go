package odds

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Event is one NFL game in Parlay API's event list. It carries no odds; the
// next sync step matches it to an ESPN game by team names and kickoff.
type Event struct {
	ID           string
	CommenceTime time.Time
	HomeTeam     string
	AwayTeam     string
}

// Events fetches the free NFL event list. Request or parse failures are errors;
// an empty list returns an empty slice.
func (c *Client) Events(ctx context.Context) ([]Event, Credits, error) {
	var raw []eventResponse
	credits, err := c.get(ctx, eventsPath, nil, &raw)
	if err != nil {
		return nil, Credits{}, fmt.Errorf("events: %w", err)
	}

	events := make([]Event, 0, len(raw))
	for _, item := range raw {
		// A fixture whose kickoff is not yet reported cannot match an ESPN
		// game window, so skip it instead of failing the whole list.
		if strings.TrimSpace(item.CommenceTime) == "" {
			continue
		}
		commence, err := time.Parse(time.RFC3339, item.CommenceTime)
		if err != nil {
			return nil, Credits{}, fmt.Errorf("event %s commence time %q: %w", item.ID, item.CommenceTime, err)
		}
		events = append(events, Event{
			ID:           item.ID,
			CommenceTime: commence.UTC(),
			HomeTeam:     item.HomeTeam,
			AwayTeam:     item.AwayTeam,
		})
	}
	return events, credits, nil
}

// eventResponse is the shared slice of Parlay API's event JSON used by both
// the event list and the game-lines response. The event list has no bookmakers.
type eventResponse struct {
	ID           string              `json:"id"`
	CommenceTime string              `json:"commence_time"`
	HomeTeam     string              `json:"home_team"`
	AwayTeam     string              `json:"away_team"`
	Bookmakers   []bookmakerResponse `json:"bookmakers"`
}

// bookmakerResponse is one book's markets for an event. Sid is the book's own
// ID for the event, used by books whose combined link needs an event page.
type bookmakerResponse struct {
	Key     string           `json:"key"`
	Sid     string           `json:"sid"`
	Markets []marketResponse `json:"markets"`
}

// marketResponse is one market, such as h2h or totals, with its outcomes.
type marketResponse struct {
	Key      string            `json:"key"`
	Outcomes []outcomeResponse `json:"outcomes"`
}

// outcomeResponse is one priced selection. Point is set for spreads and totals
// and absent for moneylines. Description names the player or team a prop prices
// and is empty on game lines. Link and Sid are sent only when the request asks
// for them with includeLinks and includeSids, and a book may omit either.
type outcomeResponse struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Price       *int     `json:"price"`
	Point       *float64 `json:"point"`
	Link        string   `json:"link"`
	Sid         string   `json:"sid"`
}
