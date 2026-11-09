package refresh

import (
	"time"

	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/ingest/weather"
)

// eastern is the zone the game-day and weekly odds rules use. Loading it
// cannot fail because time/tzdata is embedded in refresh.go.
var eastern = mustLoadLocation("America/New_York")

// mustLoadLocation loads a zone or panics: the zone database ships with the
// binary.
func mustLoadLocation(name string) *time.Location {
	location, err := time.LoadLocation(name)
	if err != nil {
		panic("load time zone " + name + ": " + err.Error())
	}
	return location
}

// scoreboardStale reports whether the slate is older than its window. A source
// that has never run is always stale.
func scoreboardStale(last, now time.Time, live bool) bool {
	if last.IsZero() {
		return true
	}
	return !now.Before(last.Add(scoreboardAge(now, live)))
}

// scoreboardAge is the slate's freshness window: 10 minutes while a game is
// live, 30 minutes on a Sunday or Monday, and 6 hours otherwise.
func scoreboardAge(now time.Time, live bool) time.Duration {
	switch {
	case live:
		return scoreboardLiveAge
	case gameDay(now):
		return scoreboardGameDayAge
	default:
		return scoreboardIdleAge
	}
}

// gameDay reports whether now falls on Sunday or Monday in America/New_York.
func gameDay(now time.Time) bool {
	switch now.In(eastern).Weekday() {
	case time.Sunday, time.Monday:
		return true
	default:
		return false
	}
}

// anyLive reports whether any game is in progress.
func anyLive(games []db.Game) bool {
	for _, game := range games {
		if game.Status == db.GameLive {
			return true
		}
	}
	return false
}

// oddsStale reports whether the week's lines are older than the budget allows.
// It skips a week with no games and a week whose games have all kicked off.
func oddsStale(last, now time.Time, games []db.Game) bool {
	if len(games) == 0 || allKickedOff(games, now) {
		return false
	}
	if last.IsZero() {
		return true
	}
	return !now.Before(last.Add(oddsAge(now, games)))
}

// oddsAge is the lines freshness window from the budget: 24 hours Monday
// through Friday, 12 hours on Saturday, 45 minutes on Sunday before the last
// Sunday kickoff, and 3 hours once only the Monday game remains.
func oddsAge(now time.Time, games []db.Game) time.Duration {
	switch now.In(eastern).Weekday() {
	case time.Sunday:
		lastSunday := lastKickoffOn(games, time.Sunday)
		if !lastSunday.IsZero() && now.Before(lastSunday) {
			return oddsSundayAge
		}
		return oddsLateAge
	case time.Monday:
		return oddsLateAge
	case time.Saturday:
		return oddsSaturdayAge
	default:
		return oddsWeekdayAge
	}
}

// regularSeasonType is ESPN's season_type for the regular season.
const regularSeasonType = 2

// nfeloStale reports whether the week's model projections are due. It skips a
// week that is not regular season, a week with no games, and a week whose games
// have all kicked off.
func nfeloStale(last, now time.Time, week db.Week, games []db.Game) bool {
	if week.SeasonType != regularSeasonType || len(games) == 0 || allKickedOff(games, now) {
		return false
	}
	if last.IsZero() {
		return true
	}
	return !now.Before(last.Add(nfeloAge))
}

// allKickedOff reports whether every game has reached its kickoff time.
func allKickedOff(games []db.Game, now time.Time) bool {
	for _, game := range games {
		if now.Before(game.Kickoff) {
			return false
		}
	}
	return true
}

// lastKickoffOn returns the latest kickoff among games that fall on weekday,
// or the zero time when the week has none.
func lastKickoffOn(games []db.Game, weekday time.Weekday) time.Time {
	var latest time.Time
	for _, game := range games {
		if game.Kickoff.In(eastern).Weekday() != weekday {
			continue
		}
		if game.Kickoff.After(latest) {
			latest = game.Kickoff
		}
	}
	return latest
}

// weatherStale reports whether one game's forecast is due. It skips an indoor
// game, an unknown venue, a kickoff more than 48 hours away, and a game that
// has already started. Inside the window the forecast is good for an hour.
func weatherStale(last, now time.Time, game db.Game) bool {
	if game.Indoor || !knownVenue(game) {
		return false
	}
	if now.Before(game.Kickoff.Add(-weatherLookahead)) {
		return false
	}
	if !now.Before(game.Kickoff) {
		return false
	}
	if last.IsZero() {
		return true
	}
	return !now.Before(last.Add(weatherAge))
}

// knownVenue reports whether the game's venue has coordinates.
func knownVenue(game db.Game) bool {
	if game.VenueID == nil {
		return false
	}
	_, ok := weather.VenueByID(*game.VenueID)
	return ok
}
