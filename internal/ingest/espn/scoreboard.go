package espn

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Status is where a game sits in its lifecycle. These strings match the games
// table.
type Status string

// The lifecycle states a game can have.
const (
	Scheduled Status = "SCHEDULED"
	Live      Status = "LIVE"
	Final     Status = "FINAL"
)

// Season identifies an NFL season and its phase: 2 is the regular season and 3
// is the postseason.
type Season struct {
	Year int
	Type int
}

// Week identifies a week within a season.
type Week struct {
	Number int
}

// Team is one side of a game.
type Team struct {
	ID           string
	Abbreviation string
	Name         string
	Logo         string
	Score        *int
}

// Venue is where a game is played.
type Venue struct {
	ID     string
	Name   string
	Indoor bool
}

// Game is one game from a scoreboard, in UTC.
// Period and Clock are populated only when Status is Live.
type Game struct {
	ID      string
	Kickoff time.Time
	Status  Status
	Period  *int
	Clock   *string
	Home    Team
	Away    Team
	Venue   Venue
}

// Scoreboard is one week's slate.
type Scoreboard struct {
	Season Season
	Week   Week
	Games  []Game
}

// ScoreboardParams narrows the request to one week. The zero value asks ESPN
// for the current week.
type ScoreboardParams struct {
	Season     int
	SeasonType int
	Week       int
}

// query returns the ESPN query parameters for these params, or nil for the
// current week.
func (p ScoreboardParams) query() url.Values {
	if p.Season == 0 {
		return nil
	}

	query := url.Values{}
	query.Set("dates", strconv.Itoa(p.Season))
	if p.SeasonType != 0 {
		query.Set("seasontype", strconv.Itoa(p.SeasonType))
	}
	if p.Week != 0 {
		query.Set("week", strconv.Itoa(p.Week))
	}
	return query
}

// Scoreboard fetches a week's scoreboard and parses all games. Request or
// parse failures are errors; a week with no games returns an empty slice.
func (c *Client) Scoreboard(ctx context.Context, params ScoreboardParams) (Scoreboard, error) {
	var raw scoreboardResponse
	if err := c.get(ctx, "/scoreboard", params.query(), &raw); err != nil {
		return Scoreboard{}, fmt.Errorf("scoreboard: %w", err)
	}

	board, err := parseScoreboard(raw)
	if err != nil {
		return Scoreboard{}, fmt.Errorf("scoreboard: %w", err)
	}
	return board, nil
}

// scoreboardResponse is the slice of ESPN's scoreboard JSON this app reads.
type scoreboardResponse struct {
	Season struct {
		Year int `json:"year"`
		Type int `json:"type"`
	} `json:"season"`
	Week struct {
		Number int `json:"number"`
	} `json:"week"`
	Events []eventResponse `json:"events"`
}

// eventResponse is one event in a scoreboard.
type eventResponse struct {
	ID           string                `json:"id"`
	Date         string                `json:"date"`
	Status       statusResponse        `json:"status"`
	Competitions []competitionResponse `json:"competitions"`
}

// statusResponse is the state ESPN puts on an event.
type statusResponse struct {
	DisplayClock string `json:"displayClock"`
	Period       int    `json:"period"`
	Type         struct {
		Name        string `json:"name"`
		State       string `json:"state"`
		ShortDetail string `json:"shortDetail"`
	} `json:"type"`
}

// competitionResponse holds the competitors and venue for an event. ESPN sends
// one competition per NFL event.
type competitionResponse struct {
	Competitors []competitorResponse `json:"competitors"`
	Venue       venueResponse        `json:"venue"`
}

// competitorResponse is one side of a competition.
type competitorResponse struct {
	HomeAway string       `json:"homeAway"`
	Team     teamResponse `json:"team"`
	Score    string       `json:"score"`
}

// teamResponse is the slice of a team this app reads.
type teamResponse struct {
	ID           string `json:"id"`
	Abbreviation string `json:"abbreviation"`
	DisplayName  string `json:"displayName"`
	Logo         string `json:"logo"`
}

// venueResponse is the slice of a venue this app reads.
type venueResponse struct {
	ID       string `json:"id"`
	FullName string `json:"fullName"`
	Indoor   bool   `json:"indoor"`
}

// parseScoreboard turns ESPN's response into the app's shape, keeping games on
// any day of the week.
func parseScoreboard(raw scoreboardResponse) (Scoreboard, error) {
	board := Scoreboard{
		Season: Season{Year: raw.Season.Year, Type: raw.Season.Type},
		Week:   Week{Number: raw.Week.Number},
		Games:  make([]Game, 0, len(raw.Events)),
	}

	for _, event := range raw.Events {
		kickoff, err := parseKickoff(event.Date)
		if err != nil {
			return Scoreboard{}, fmt.Errorf("event %s kickoff: %w", event.ID, err)
		}

		game, err := parseEvent(event, kickoff)
		if err != nil {
			return Scoreboard{}, err
		}
		board.Games = append(board.Games, game)
	}
	return board, nil
}

// parseKickoff reads ESPN's kickoff time, which usually has minute precision
// but may include seconds.
func parseKickoff(raw string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339, raw)
	if err == nil {
		return parsed.UTC(), nil
	}
	parsed, err = time.Parse("2006-01-02T15:04Z07:00", raw)
	if err == nil {
		return parsed.UTC(), nil
	}
	return time.Time{}, err
}

// parseEvent turns one event into a game.
func parseEvent(event eventResponse, kickoff time.Time) (Game, error) {
	status, err := parseState(event.Status.Type.State)
	if err != nil {
		return Game{}, fmt.Errorf("event %s: %w", event.ID, err)
	}
	if len(event.Competitions) == 0 {
		return Game{}, fmt.Errorf("event %s: no competition", event.ID)
	}

	competition := event.Competitions[0]
	game := Game{
		ID:      event.ID,
		Kickoff: kickoff,
		Status:  status,
		Venue: Venue{
			ID:     competition.Venue.ID,
			Name:   competition.Venue.FullName,
			Indoor: competition.Venue.Indoor,
		},
	}
	if status == Live {
		game.Period, game.Clock = parseLiveClock(event.Status)
	}

	for _, competitor := range competition.Competitors {
		team, err := parseTeam(competitor, status)
		if err != nil {
			return Game{}, fmt.Errorf("event %s: %w", event.ID, err)
		}
		switch competitor.HomeAway {
		case "home":
			game.Home = team
		case "away":
			game.Away = team
		default:
			return Game{}, fmt.Errorf("event %s: unknown homeAway %q", event.ID, competitor.HomeAway)
		}
	}

	if game.Home.ID == "" || game.Away.ID == "" {
		return Game{}, fmt.Errorf("event %s: missing home or away team", event.ID)
	}
	return game, nil
}

// parseState maps ESPN's event state onto the app's status.
func parseState(state string) (Status, error) {
	switch state {
	case "pre":
		return Scheduled, nil
	case "in":
		return Live, nil
	case "post":
		return Final, nil
	default:
		return "", fmt.Errorf("unknown event state %q", state)
	}
}

// parseLiveClock extracts the current period and display clock (or normalized
// break label such as "Halftime" or "End of 1st") for an in-progress game.
func parseLiveClock(status statusResponse) (*int, *string) {
	var periodPtr *int
	if status.Period > 0 {
		p := status.Period
		periodPtr = &p
	}

	shortDetail := strings.TrimSpace(status.Type.ShortDetail)
	typeName := strings.TrimSpace(status.Type.Name)
	displayClock := strings.TrimSpace(status.DisplayClock)

	var clockStr string
	switch {
	case strings.EqualFold(typeName, "STATUS_HALFTIME") ||
		strings.EqualFold(shortDetail, "Halftime") ||
		strings.EqualFold(shortDetail, "Half"):
		clockStr = "Halftime"
	case strings.HasPrefix(strings.ToLower(shortDetail), "end of "):
		clockStr = shortDetail
	case strings.EqualFold(typeName, "STATUS_END_PERIOD"):
		if shortDetail != "" {
			clockStr = shortDetail
		} else if status.Period > 0 {
			clockStr = "End of " + ordinalPeriod(status.Period)
		}
	case displayClock != "":
		clockStr = displayClock
	}

	if clockStr == "" {
		return periodPtr, nil
	}
	return periodPtr, &clockStr
}

func ordinalPeriod(period int) string {
	switch period {
	case 1:
		return "1st"
	case 2:
		return "2nd"
	case 3:
		return "3rd"
	case 4:
		return "4th"
	case 5:
		return "OT"
	default:
		return fmt.Sprintf("%dOT", period-4)
	}
}

// parseTeam turns one competitor into a team. A scheduled game has no score,
// even when ESPN reports a placeholder zero.
func parseTeam(competitor competitorResponse, status Status) (Team, error) {
	team := Team{
		ID:           competitor.Team.ID,
		Abbreviation: competitor.Team.Abbreviation,
		Name:         competitor.Team.DisplayName,
		Logo:         normalizeHTTPSURL(competitor.Team.Logo),
	}
	if status == Scheduled || competitor.Score == "" {
		return team, nil
	}

	score, err := strconv.Atoi(competitor.Score)
	if err != nil {
		return Team{}, fmt.Errorf("team %s score %q: %w", team.ID, competitor.Score, err)
	}
	team.Score = &score
	return team, nil
}
