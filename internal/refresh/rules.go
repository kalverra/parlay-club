package refresh

import (
	"time"

	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/ingest/weather"
	"github.com/kalverra/parlay-club/internal/tz"
)

// eastern is the zone the game-day and weekly odds rules use.
var eastern = tz.Eastern

// scoreboardStale reports whether the slate is older than its window. A source
// that has never run is always stale.
func scoreboardStale(last, now time.Time, live bool) bool {
	if last.IsZero() {
		return true
	}
	return !now.Before(last.Add(scoreboardAge(now, live)))
}

// scoreboardAge is the slate's freshness window: 5 minutes while a game is
// live, 15 minutes on a Thursday, Sunday, or Monday, and 2 hours otherwise.
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

// gameDay reports whether now falls on Thursday, Sunday, or Monday in America/New_York.
func gameDay(now time.Time) bool {
	switch now.In(eastern).Weekday() {
	case time.Thursday, time.Sunday, time.Monday:
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
// have all kicked off. Inside the window projections are good for 1 hour on
// game days and 4 hours otherwise.
func nfeloStale(last, now time.Time, week db.Week, games []db.Game) bool {
	if week.SeasonType != regularSeasonType || len(games) == 0 || allKickedOff(games, now) {
		return false
	}
	if last.IsZero() {
		return true
	}
	return !now.Before(last.Add(gameDayAge(now, nfeloGameDayAge, nfeloIdleAge)))
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
// has already started. Inside the window the forecast is good for 15 minutes
// within 6 hours of kickoff and 30 minutes otherwise.
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
	age := weatherAge
	if !now.Before(game.Kickoff.Add(-weatherNearLookahead)) {
		age = weatherNearAge
	}
	return !now.Before(last.Add(age))
}

// knownVenue reports whether the game's venue has coordinates.
func knownVenue(game db.Game) bool {
	if game.VenueID == nil {
		return false
	}
	_, ok := weather.VenueByID(*game.VenueID)
	return ok
}

// oddsPropsStale reports whether a game's props are due. It skips a game once
// it has kicked off. Inside the window props are good for two hours.
func oddsPropsStale(last, now time.Time, game db.Game) bool {
	if !now.Before(game.Kickoff) {
		return false
	}
	if last.IsZero() {
		return true
	}
	return !now.Before(last.Add(oddsPropsAge))
}

// rosterStale reports whether a team's roster is due. Rosters are good for 2
// hours on game days and 6 hours otherwise.
func rosterStale(last, now time.Time) bool {
	if last.IsZero() {
		return true
	}
	return !now.Before(last.Add(gameDayAge(now, rosterGameDayAge, rosterIdleAge)))
}

// summaryStale reports whether a game's summary is due. It skips games that
// are final. Inside the window summaries are good for 15 minutes on game days
// or within 4 hours of kickoff, and 1 hour otherwise.
func summaryStale(last, now time.Time, game db.Game) bool {
	if game.Status == db.GameFinal {
		return false
	}
	if last.IsZero() {
		return true
	}
	return !now.Before(last.Add(summaryAgeFor(now, game)))
}

// summaryAgeFor returns the summary freshness window for game at now.
func summaryAgeFor(now time.Time, game db.Game) time.Duration {
	if gameDay(now) {
		return summaryGameDayAge
	}
	if !game.Kickoff.IsZero() && !now.Before(game.Kickoff.Add(-summaryNearKickoff)) && now.Before(game.Kickoff) {
		return summaryGameDayAge
	}
	return summaryIdleAge
}

// gameLogStale reports whether an athlete's game log is due. Game logs are good
// for 1 hour on game days and 4 hours otherwise.
func gameLogStale(last, now time.Time) bool {
	if last.IsZero() {
		return true
	}
	return !now.Before(last.Add(gameDayAge(now, gameLogGameDayAge, gameLogIdleAge)))
}

// prefillStale reports whether a pre-fill managed source is older than the
// 15-minute idempotency window.
func prefillStale(last, now time.Time) bool {
	if last.IsZero() {
		return true
	}
	return !now.Before(last.Add(prefillIdempotencyAge))
}

// gameDayAge returns gd on Thursday, Sunday, or Monday in America/New_York and
// idle otherwise.
func gameDayAge(now time.Time, gd, idle time.Duration) time.Duration {
	if gameDay(now) {
		return gd
	}
	return idle
}
