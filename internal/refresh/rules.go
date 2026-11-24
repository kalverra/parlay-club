package refresh

import (
	"time"

	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/ingest/nflverse"
	"github.com/kalverra/parlay-club/internal/ingest/weather"
	"github.com/kalverra/parlay-club/internal/tz"
)

// eastern is the zone the game-day and weekly odds rules use.
var eastern = tz.Eastern

// scoreboardStale reports whether the slate is older than its window. A source
// that has never run is always stale.
func scoreboardStale(last, now time.Time, games []db.Game, live bool) bool {
	if last.IsZero() {
		return true
	}
	return !now.Before(last.Add(scoreboardAge(now, games, live)))
}

// scoreboardAge is the slate's freshness window: 5 minutes while a game is
// live, 15 minutes on a scheduled game day, and 2 hours otherwise.
func scoreboardAge(now time.Time, games []db.Game, live bool) time.Duration {
	switch {
	case live:
		return scoreboardLiveAge
	case isGameDay(games, now):
		return scoreboardGameDayAge
	default:
		return scoreboardIdleAge
	}
}

// sameEasternDate reports whether a and b fall on the same calendar date in
// America/New_York.
func sameEasternDate(a, b time.Time) bool {
	ay, am, ad := a.In(eastern).Date()
	by, bm, bd := b.In(eastern).Date()
	return ay == by && am == bm && ad == bd
}

// gamesOnDate returns games whose kickoff falls on now's calendar date in
// America/New_York.
func gamesOnDate(games []db.Game, now time.Time) []db.Game {
	var onDate []db.Game
	for _, game := range games {
		if !game.Kickoff.IsZero() && sameEasternDate(game.Kickoff, now) {
			onDate = append(onDate, game)
		}
	}
	return onDate
}

// upcomingGamesOnDate returns games on now's Eastern calendar date that have
// not yet reached kickoff or finished.
func upcomingGamesOnDate(games []db.Game, now time.Time) []db.Game {
	var upcoming []db.Game
	for _, game := range gamesOnDate(games, now) {
		if game.Status != db.GameFinal && now.Before(game.Kickoff) {
			upcoming = append(upcoming, game)
		}
	}
	return upcoming
}

// isGameDay reports whether any game in games has a kickoff on now's calendar
// date in America/New_York. When games is empty (cold DB before the first
// scoreboard fetch), it falls back to Thursday, Sunday, or Monday.
func isGameDay(games []db.Game, now time.Time) bool {
	if len(games) == 0 {
		switch now.In(eastern).Weekday() {
		case time.Thursday, time.Sunday, time.Monday:
			return true
		default:
			return false
		}
	}
	return len(gamesOnDate(games, now)) > 0
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

// oddsAge is the lines freshness window from the budget: 45 minutes on
// multi-game days (2+ upcoming games today in ET), 3 hours on single-game days
// (1 upcoming game today), 12 hours on the eve of a game day (tomorrow ET has
// games), and 24 hours otherwise.
func oddsAge(now time.Time, games []db.Game) time.Duration {
	upcoming := upcomingGamesOnDate(games, now)
	switch {
	case len(upcoming) >= 2:
		return oddsGameDayAge
	case len(upcoming) == 1:
		return oddsLateAge
	}
	nowET := now.In(eastern)
	tomorrowET := time.Date(nowET.Year(), nowET.Month(), nowET.Day()+1, 12, 0, 0, 0, eastern)
	if len(gamesOnDate(games, tomorrowET)) > 0 {
		return oddsEveAge
	}
	return oddsWeekdayAge
}

// oddsPickLinesStale reports whether the week's lines are stale for a player
// actively picking on an open slip. It skips a week with no games and a week
// whose games have all kicked off. Inside the window lines are good for 15
// minutes on game days (before the last kickoff on that day) or within 2 hours
// of an upcoming kickoff, and 2 hours otherwise.
func oddsPickLinesStale(last, now time.Time, games []db.Game) bool {
	if len(games) == 0 || allKickedOff(games, now) {
		return false
	}
	if last.IsZero() {
		return true
	}
	return !now.Before(last.Add(oddsPickLinesAge(now, games)))
}

// oddsPickLinesAge returns 15 minutes when today has upcoming games or within 2
// hours of an upcoming kickoff, and 2 hours otherwise.
func oddsPickLinesAge(now time.Time, games []db.Game) time.Duration {
	if len(upcomingGamesOnDate(games, now)) > 0 {
		return oddsPickLinesGameDayAge
	}
	if withinNextKickoff(games, now, oddsPickLinesLookahead) {
		return oddsPickLinesGameDayAge
	}
	return oddsPickLinesIdleAge
}

// withinNextKickoff reports whether any upcoming game starts within lookahead.
func withinNextKickoff(games []db.Game, now time.Time, lookahead time.Duration) bool {
	for _, game := range games {
		if now.Before(game.Kickoff) && !now.Before(game.Kickoff.Add(-lookahead)) {
			return true
		}
	}
	return false
}

// pickCooldownStale reports whether a manual pick-time refresh is outside its
// 2-minute cooldown.
func pickCooldownStale(last, now time.Time) bool {
	if last.IsZero() {
		return true
	}
	return !now.Before(last.Add(pickRefreshCooldown))
}

// nfeloStale reports whether the week's model projections are due. It skips a
// week that is not regular season, a week with no games, and a week whose games
// have all kicked off. Inside the window projections are good for 1 hour on
// game days and 4 hours otherwise.
func nfeloStale(last, now time.Time, week db.Week, games []db.Game) bool {
	if week.SeasonType != nflverse.SeasonTypeRegular || len(games) == 0 || allKickedOff(games, now) {
		return false
	}
	if last.IsZero() {
		return true
	}
	return !now.Before(last.Add(gameDayAge(games, now, nfeloGameDayAge, nfeloIdleAge)))
}

// nflverseStale reports whether the season's nflverse team/player/snap sync is
// due. It runs during regular and postseason weeks, syncs every 6 hours on
// Tuesday/Wednesday/game days and every 24 hours otherwise, and stops once all
// games in the week are final and the last sync ran more than 12 hours after
// the latest kickoff.
func nflverseStale(last, now time.Time, week db.Week, games []db.Game) bool {
	if week.Season <= 0 ||
		(week.SeasonType != nflverse.SeasonTypeRegular && week.SeasonType != nflverse.SeasonTypePostseason) {
		return false
	}
	if last.IsZero() {
		return true
	}
	if len(games) > 0 && allFinal(games) {
		latest := latestKickoff(games)
		if !latest.IsZero() && last.After(latest.Add(nflversePostFinalWindow)) {
			return false
		}
	}
	return !now.Before(last.Add(nflverseAge(games, now)))
}

// nflverseAge is the nflverse freshness window: 6 hours on Tuesday, Wednesday,
// and game days (when weekly updates land or lineups shift) and 24 hours
// otherwise.
func nflverseAge(games []db.Game, now time.Time) time.Duration {
	switch now.In(eastern).Weekday() {
	case time.Tuesday, time.Wednesday:
		return nflverseActiveAge
	}
	if isGameDay(games, now) {
		return nflverseActiveAge
	}
	return nflverseIdleAge
}

// allFinal reports whether every game in the slice is final.
func allFinal(games []db.Game) bool {
	for _, game := range games {
		if game.Status != db.GameFinal {
			return false
		}
	}
	return true
}

// latestKickoff returns the latest kickoff time among games.
func latestKickoff(games []db.Game) time.Time {
	var latest time.Time
	for _, game := range games {
		if game.Kickoff.After(latest) {
			latest = game.Kickoff
		}
	}
	return latest
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
// it has kicked off. Inside the window props are good for 30 minutes on the
// game's day or within 6 hours of kickoff, and 2 hours otherwise.
func oddsPropsStale(last, now time.Time, game db.Game) bool {
	if !now.Before(game.Kickoff) {
		return false
	}
	if last.IsZero() {
		return true
	}
	return !now.Before(last.Add(oddsPropsAgeFor(now, game)))
}

// oddsPropsAgeFor returns the props freshness window for game at now.
func oddsPropsAgeFor(now time.Time, game db.Game) time.Duration {
	if !game.Kickoff.IsZero() && sameEasternDate(game.Kickoff, now) {
		return oddsPropsGameDayAge
	}
	if !game.Kickoff.IsZero() && !now.Before(game.Kickoff.Add(-oddsPropsNearKickoff)) {
		return oddsPropsGameDayAge
	}
	return oddsPropsAge
}

// rosterStale reports whether a team's roster is due. Rosters are good for 2
// hours on game days and 6 hours otherwise.
func rosterStale(last, now time.Time, games []db.Game) bool {
	if last.IsZero() {
		return true
	}
	return !now.Before(last.Add(gameDayAge(games, now, rosterGameDayAge, rosterIdleAge)))
}

// summaryStale reports whether a game's summary is due. It skips games that
// are final. Inside the window summaries are good for 15 minutes on the game's
// day or within 4 hours of kickoff, and 1 hour otherwise.
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
	if !game.Kickoff.IsZero() && sameEasternDate(game.Kickoff, now) {
		return summaryGameDayAge
	}
	if !game.Kickoff.IsZero() && !now.Before(game.Kickoff.Add(-summaryNearKickoff)) && now.Before(game.Kickoff) {
		return summaryGameDayAge
	}
	return summaryIdleAge
}

// gameLogStale reports whether an athlete's game log is due. Game logs are good
// for 1 hour on game days and 4 hours otherwise.
func gameLogStale(last, now time.Time, games []db.Game) bool {
	if last.IsZero() {
		return true
	}
	return !now.Before(last.Add(gameDayAge(games, now, gameLogGameDayAge, gameLogIdleAge)))
}

// prefillStale reports whether a pre-fill managed source is older than the
// 15-minute idempotency window.
func prefillStale(last, now time.Time) bool {
	if last.IsZero() {
		return true
	}
	return !now.Before(last.Add(prefillIdempotencyAge))
}

// gameDayPrefillWindow reports whether now falls at or after 6:00 AM
// America/New_York on a day with at least one upcoming game.
func gameDayPrefillWindow(games []db.Game, now time.Time) bool {
	if now.In(eastern).Hour() < prefillStartHour {
		return false
	}
	return len(upcomingGamesOnDate(games, now)) > 0
}

// gameDayPrefillStale reports whether the one-shot game-day pre-fill has not
// yet run on now's calendar date in America/New_York.
func gameDayPrefillStale(last, now time.Time) bool {
	if last.IsZero() {
		return true
	}
	return !sameEasternDate(last, now)
}

// gameDayAge returns gd on a scheduled game day in America/New_York and idle
// otherwise.
func gameDayAge(games []db.Game, now time.Time, gd, idle time.Duration) time.Duration {
	if isGameDay(games, now) {
		return gd
	}
	return idle
}
