package refresh

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/kalverra/parlay-club/internal/db"
)

// slateGames is a week with two Sunday kickoffs and one Monday night game.
func slateGames() []db.Game {
	return []db.Game{
		{ID: "sun-early", Kickoff: novemberTime(8, 13, 0)},
		{ID: "sun-late", Kickoff: novemberTime(8, 16, 25)},
		{ID: "mon", Kickoff: novemberTime(9, 20, 15)},
	}
}

// thanksgivingWeekGames is a Thanksgiving week slate: 3 Thanksgiving Thursday
// games, 1 Black Friday game, and 1 Sunday game.
func thanksgivingWeekGames() []db.Game {
	return []db.Game{
		{ID: "tg-early", Kickoff: novemberTime(26, 12, 30)},
		{ID: "tg-mid", Kickoff: novemberTime(26, 16, 30)},
		{ID: "tg-night", Kickoff: novemberTime(26, 20, 20)},
		{ID: "bf", Kickoff: novemberTime(27, 15, 0)},
		{ID: "sun", Kickoff: novemberTime(29, 13, 0)},
	}
}

// christmasGames is a Wednesday Christmas 2024 doubleheader plus a Thursday game.
func christmasGames() []db.Game {
	return []db.Game{
		{ID: "xmas-early", Kickoff: easternTime(2024, time.December, 25, 13, 0)},
		{ID: "xmas-late", Kickoff: easternTime(2024, time.December, 25, 16, 30)},
		{ID: "thu-night", Kickoff: easternTime(2024, time.December, 26, 20, 15)},
	}
}

// playoffWeekendGames is a Wild Card / Divisional playoff slate with two
// Saturday games and two Sunday games (no Thursday or Monday games).
func playoffWeekendGames() []db.Game {
	return []db.Game{
		{ID: "sat-early", Kickoff: easternTime(2026, time.January, 10, 16, 30)},
		{ID: "sat-night", Kickoff: easternTime(2026, time.January, 10, 20, 0)},
		{ID: "sun-early", Kickoff: easternTime(2026, time.January, 11, 13, 0)},
		{ID: "sun-late", Kickoff: easternTime(2026, time.January, 11, 16, 30)},
	}
}

// easternTime builds an Eastern wall-clock time and returns it in UTC, the
// form the database stores.
func easternTime(year int, month time.Month, day, hour, minute int) time.Time {
	return time.Date(year, month, day, hour, minute, 0, 0, eastern).UTC()
}

// novemberTime builds a 2026 November wall-clock time in America/New_York and
// returns it in UTC, the form the database stores.
func novemberTime(day, hour, minute int) time.Time {
	return easternTime(2026, time.November, day, hour, minute)
}

func TestScoreboardStaleRules(t *testing.T) {
	t.Parallel()

	games := slateGames()
	tgGames := thanksgivingWeekGames()
	xmasGames := christmasGames()
	poGames := playoffWeekendGames()

	tests := []struct {
		name  string
		last  time.Time
		now   time.Time
		games []db.Game
		live  bool
		want  bool
	}{
		{
			name: "never fetched",
			now:  novemberTime(4, 12, 0),
			want: true,
		},
		{
			name:  "live game waits five minutes",
			last:  novemberTime(8, 13, 0),
			now:   novemberTime(8, 13, 4),
			games: games,
			live:  true,
			want:  false,
		},
		{
			name:  "live game at five minutes",
			last:  novemberTime(8, 13, 0),
			now:   novemberTime(8, 13, 5),
			games: games,
			live:  true,
			want:  true,
		},
		{
			name:  "game day waits fifteen minutes",
			last:  novemberTime(8, 13, 0),
			now:   novemberTime(8, 13, 14),
			games: games,
			want:  false,
		},
		{
			name:  "game day at fifteen minutes",
			last:  novemberTime(8, 13, 0),
			now:   novemberTime(8, 13, 15),
			games: games,
			want:  true,
		},
		{
			name:  "monday is also a game day when scheduled",
			last:  novemberTime(9, 12, 0),
			now:   novemberTime(9, 12, 15),
			games: games,
			want:  true,
		},
		{
			name: "cold DB falls back to thursday as game day",
			last: novemberTime(5, 12, 0),
			now:  novemberTime(5, 12, 15),
			want: true,
		},
		{
			name: "cold DB thursday waits fifteen minutes",
			last: novemberTime(5, 12, 0),
			now:  novemberTime(5, 12, 14),
			want: false,
		},
		{
			name:  "thanksgiving is a game day",
			last:  novemberTime(26, 12, 0),
			now:   novemberTime(26, 12, 15),
			games: tgGames,
			want:  true,
		},
		{
			name:  "black friday is a game day",
			last:  novemberTime(27, 12, 0),
			now:   novemberTime(27, 12, 15),
			games: tgGames,
			want:  true,
		},
		{
			name:  "christmas wednesday is a game day",
			last:  easternTime(2024, time.December, 25, 12, 0),
			now:   easternTime(2024, time.December, 25, 12, 15),
			games: xmasGames,
			want:  true,
		},
		{
			name:  "playoff saturday is a game day",
			last:  easternTime(2026, time.January, 10, 12, 0),
			now:   easternTime(2026, time.January, 10, 12, 15),
			games: poGames,
			want:  true,
		},
		{
			name:  "playoff thursday with no thursday games waits two hours",
			last:  easternTime(2026, time.January, 8, 12, 0),
			now:   easternTime(2026, time.January, 8, 13, 59),
			games: poGames,
			want:  false,
		},
		{
			name:  "midweek waits two hours",
			last:  novemberTime(4, 12, 0),
			now:   novemberTime(4, 13, 59),
			games: games,
			want:  false,
		},
		{
			name:  "midweek at two hours",
			last:  novemberTime(4, 12, 0),
			now:   novemberTime(4, 14, 0),
			games: games,
			want:  true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, test.want, scoreboardStale(test.last, test.now, test.games, test.live))
		})
	}
}

func TestOddsStaleRules(t *testing.T) {
	t.Parallel()

	games := slateGames()
	tgGames := thanksgivingWeekGames()
	xmasGames := christmasGames()
	poGames := playoffWeekendGames()

	tests := []struct {
		name  string
		last  time.Time
		now   time.Time
		games []db.Game
		want  bool
	}{
		{
			name:  "never fetched with a game ahead",
			now:   novemberTime(4, 12, 0),
			games: games,
			want:  true,
		},
		{
			name: "no games means no fetch",
			now:  novemberTime(4, 12, 0),
			want: false,
		},
		{
			name:  "every game kicked off means no fetch",
			last:  novemberTime(9, 20, 0),
			now:   novemberTime(10, 12, 0),
			games: games,
			want:  false,
		},
		{
			name:  "non-game wednesday with no thursday game waits twenty four hours",
			last:  novemberTime(4, 12, 0),
			now:   novemberTime(5, 11, 59),
			games: games,
			want:  false,
		},
		{
			name:  "non-game day at twenty four hours",
			last:  novemberTime(4, 12, 0),
			now:   novemberTime(5, 12, 0),
			games: games,
			want:  true,
		},
		{
			name:  "eve of sunday games waits twelve hours",
			last:  novemberTime(7, 8, 0),
			now:   novemberTime(7, 19, 59),
			games: games,
			want:  false,
		},
		{
			name:  "eve of sunday games at twelve hours",
			last:  novemberTime(7, 8, 0),
			now:   novemberTime(7, 20, 0),
			games: games,
			want:  true,
		},
		{
			name:  "sunday before the last kickoff waits forty five minutes",
			last:  novemberTime(8, 11, 0),
			now:   novemberTime(8, 11, 44),
			games: games,
			want:  false,
		},
		{
			name:  "sunday at forty five minutes",
			last:  novemberTime(8, 11, 0),
			now:   novemberTime(8, 11, 45),
			games: games,
			want:  true,
		},
		{
			name:  "sunday after the last kickoff with monday game tomorrow waits twelve hours",
			last:  novemberTime(8, 14, 0),
			now:   novemberTime(8, 17, 0),
			games: games,
			want:  false,
		},
		{
			name:  "sunday night after last kickoff at twelve hours from pre-kickoff sync",
			last:  novemberTime(8, 11, 0),
			now:   novemberTime(8, 23, 0),
			games: games,
			want:  true,
		},
		{
			name:  "monday before the night game waits three hours",
			last:  novemberTime(9, 15, 0),
			now:   novemberTime(9, 17, 59),
			games: games,
			want:  false,
		},
		{
			name:  "monday before the night game at three hours",
			last:  novemberTime(9, 15, 0),
			now:   novemberTime(9, 18, 0),
			games: games,
			want:  true,
		},
		{
			name:  "thanksgiving with three upcoming games waits forty five minutes",
			last:  novemberTime(26, 10, 0),
			now:   novemberTime(26, 10, 44),
			games: tgGames,
			want:  false,
		},
		{
			name:  "thanksgiving with three upcoming games at forty five minutes",
			last:  novemberTime(26, 10, 0),
			now:   novemberTime(26, 10, 45),
			games: tgGames,
			want:  true,
		},
		{
			name:  "thanksgiving with one upcoming game left waits three hours",
			last:  novemberTime(26, 17, 0),
			now:   novemberTime(26, 19, 59),
			games: tgGames,
			want:  false,
		},
		{
			name:  "thanksgiving with one upcoming game left at three hours",
			last:  novemberTime(26, 17, 0),
			now:   novemberTime(26, 20, 0),
			games: tgGames,
			want:  true,
		},
		{
			name:  "thanksgiving night after all thursday kickoffs with black friday tomorrow waits twelve hours",
			last:  novemberTime(26, 19, 0),
			now:   novemberTime(26, 23, 0),
			games: tgGames,
			want:  false,
		},
		{
			name:  "black friday single game waits three hours",
			last:  novemberTime(27, 10, 0),
			now:   novemberTime(27, 12, 59),
			games: tgGames,
			want:  false,
		},
		{
			name:  "black friday single game at three hours",
			last:  novemberTime(27, 10, 0),
			now:   novemberTime(27, 13, 0),
			games: tgGames,
			want:  true,
		},
		{
			name:  "christmas wednesday doubleheader waits forty five minutes",
			last:  easternTime(2024, time.December, 25, 11, 0),
			now:   easternTime(2024, time.December, 25, 11, 45),
			games: xmasGames,
			want:  true,
		},
		{
			name:  "playoff saturday doubleheader waits forty five minutes",
			last:  easternTime(2026, time.January, 10, 11, 0),
			now:   easternTime(2026, time.January, 10, 11, 45),
			games: poGames,
			want:  true,
		},
		{
			name:  "friday eve of playoff saturday waits twelve hours",
			last:  easternTime(2026, time.January, 9, 10, 0),
			now:   easternTime(2026, time.January, 9, 21, 59),
			games: poGames,
			want:  false,
		},
		{
			name:  "friday eve of playoff saturday at twelve hours",
			last:  easternTime(2026, time.January, 9, 10, 0),
			now:   easternTime(2026, time.January, 9, 22, 0),
			games: poGames,
			want:  true,
		},
		{
			name:  "non-game tuesday before thanksgiving week waits twenty four hours",
			last:  novemberTime(23, 12, 0),
			now:   novemberTime(24, 11, 59),
			games: tgGames,
			want:  false,
		},
		{
			name:  "non-game tuesday at twenty four hours",
			last:  novemberTime(23, 12, 0),
			now:   novemberTime(24, 12, 0),
			games: tgGames,
			want:  true,
		},
		{
			name:  "wednesday eve of thanksgiving waits twelve hours",
			last:  novemberTime(25, 0, 0),
			now:   novemberTime(25, 11, 59),
			games: tgGames,
			want:  false,
		},
		{
			name:  "wednesday eve of thanksgiving at twelve hours",
			last:  novemberTime(25, 0, 0),
			now:   novemberTime(25, 12, 0),
			games: tgGames,
			want:  true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, test.want, oddsStale(test.last, test.now, test.games))
		})
	}
}

func TestWeatherStaleRules(t *testing.T) {
	t.Parallel()

	venue := "3673"
	kickoff := novemberTime(8, 13, 0)
	game := db.Game{ID: "401", Kickoff: kickoff, VenueID: &venue}

	tests := []struct {
		name string
		last time.Time
		now  time.Time
		game db.Game
		want bool
	}{
		{
			name: "never fetched inside the window",
			now:  novemberTime(7, 11, 0),
			game: game,
			want: true,
		},
		{
			name: "forecast more than six hours out waits thirty minutes",
			last: novemberTime(7, 10, 31),
			now:  novemberTime(7, 11, 0),
			game: game,
			want: false,
		},
		{
			name: "forecast more than six hours out at thirty minutes",
			last: novemberTime(7, 10, 30),
			now:  novemberTime(7, 11, 0),
			game: game,
			want: true,
		},
		{
			name: "forecast within six hours waits fifteen minutes",
			last: novemberTime(8, 10, 46),
			now:  novemberTime(8, 11, 0),
			game: game,
			want: false,
		},
		{
			name: "forecast within six hours at fifteen minutes",
			last: novemberTime(8, 10, 45),
			now:  novemberTime(8, 11, 0),
			game: game,
			want: true,
		},
		{
			name: "forecast at exactly six hours uses fifteen minute window",
			last: novemberTime(8, 6, 45),
			now:  novemberTime(8, 7, 0),
			game: game,
			want: true,
		},
		{
			name: "more than forty eight hours out is skipped",
			now:  novemberTime(6, 12, 59),
			game: game,
			want: false,
		},
		{
			name: "exactly forty eight hours out is due",
			now:  novemberTime(6, 13, 0),
			game: game,
			want: true,
		},
		{
			name: "indoor games get no forecast",
			now:  novemberTime(7, 11, 0),
			game: db.Game{ID: "402", Kickoff: kickoff, VenueID: &venue, Indoor: true},
			want: false,
		},
		{
			name: "unknown venues get no forecast",
			now:  novemberTime(7, 11, 0),
			game: db.Game{ID: "403", Kickoff: kickoff, VenueID: new("no-such-venue")},
			want: false,
		},
		{
			name: "no venue gets no forecast",
			now:  novemberTime(7, 11, 0),
			game: db.Game{ID: "404", Kickoff: kickoff},
			want: false,
		},
		{
			name: "kickoff itself is too late",
			now:  kickoff,
			game: game,
			want: false,
		},
		{
			name: "after kickoff is too late",
			now:  kickoff.Add(time.Minute),
			game: game,
			want: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, test.want, weatherStale(test.last, test.now, test.game))
		})
	}
}

func TestOddsPropsStaleRules(t *testing.T) {
	t.Parallel()

	kickoff := novemberTime(8, 13, 0)
	game := db.Game{ID: "401", Kickoff: kickoff}
	satNightGame := db.Game{ID: "405", Kickoff: easternTime(2026, time.January, 10, 20, 0)}

	tests := []struct {
		name string
		last time.Time
		now  time.Time
		game db.Game
		want bool
	}{
		{
			name: "never fetched before kickoff",
			now:  novemberTime(8, 10, 0),
			game: game,
			want: true,
		},
		{
			name: "game day waits thirty minutes",
			last: novemberTime(8, 9, 0),
			now:  novemberTime(8, 9, 29),
			game: game,
			want: false,
		},
		{
			name: "game day at thirty minutes",
			last: novemberTime(8, 9, 0),
			now:  novemberTime(8, 9, 30),
			game: game,
			want: true,
		},
		{
			name: "playoff saturday morning (>6h before kickoff) uses thirty minute game day window",
			last: easternTime(2026, time.January, 10, 9, 0),
			now:  easternTime(2026, time.January, 10, 9, 30),
			game: satNightGame,
			want: true,
		},
		{
			name: "within six hours of kickoff waits thirty minutes",
			last: novemberTime(8, 7, 0),
			now:  novemberTime(8, 7, 29),
			game: db.Game{ID: "402", Kickoff: novemberTime(8, 13, 0)},
			want: false,
		},
		{
			name: "saturday within six hours of kickoff waits thirty minutes",
			last: novemberTime(7, 20, 0),
			now:  novemberTime(7, 20, 29),
			game: db.Game{ID: "403", Kickoff: novemberTime(8, 1, 0)},
			want: false,
		},
		{
			name: "saturday within six hours of kickoff at thirty minutes",
			last: novemberTime(7, 20, 0),
			now:  novemberTime(7, 20, 30),
			game: db.Game{ID: "403", Kickoff: novemberTime(8, 1, 0)},
			want: true,
		},
		{
			name: "midweek waits two hours",
			last: novemberTime(4, 10, 0),
			now:  novemberTime(4, 11, 59),
			game: game,
			want: false,
		},
		{
			name: "midweek at two hours",
			last: novemberTime(4, 10, 0),
			now:  novemberTime(4, 12, 0),
			game: game,
			want: true,
		},
		{
			name: "at kickoff is skipped",
			last: novemberTime(8, 9, 0),
			now:  kickoff,
			game: game,
			want: false,
		},
		{
			name: "after kickoff is skipped",
			now:  kickoff.Add(10 * time.Minute),
			game: game,
			want: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, test.want, oddsPropsStale(test.last, test.now, test.game))
		})
	}
}

func TestOddsPickLinesStaleRules(t *testing.T) {
	t.Parallel()

	games := slateGames()
	thursdayGames := []db.Game{
		{ID: "thu", Kickoff: novemberTime(5, 20, 15)},
		{ID: "sun", Kickoff: novemberTime(8, 13, 0)},
	}
	tgGames := thanksgivingWeekGames()
	xmasGames := christmasGames()
	poGames := playoffWeekendGames()

	tests := []struct {
		name  string
		last  time.Time
		now   time.Time
		games []db.Game
		want  bool
	}{
		{
			name:  "never fetched with upcoming game",
			now:   novemberTime(4, 12, 0),
			games: games,
			want:  true,
		},
		{
			name:  "no games skips",
			now:   novemberTime(8, 12, 0),
			games: nil,
			want:  false,
		},
		{
			name:  "all games kicked off skips",
			now:   novemberTime(9, 20, 15),
			games: games,
			want:  false,
		},
		{
			name:  "sunday before last sunday kickoff waits fifteen minutes",
			last:  novemberTime(8, 12, 0),
			now:   novemberTime(8, 12, 14),
			games: games,
			want:  false,
		},
		{
			name:  "sunday before last sunday kickoff at fifteen minutes",
			last:  novemberTime(8, 12, 0),
			now:   novemberTime(8, 12, 15),
			games: games,
			want:  true,
		},
		{
			name:  "thursday before thursday kickoff at fifteen minutes",
			last:  novemberTime(5, 19, 0),
			now:   novemberTime(5, 19, 15),
			games: thursdayGames,
			want:  true,
		},
		{
			name:  "monday before monday kickoff at fifteen minutes",
			last:  novemberTime(9, 19, 0),
			now:   novemberTime(9, 19, 15),
			games: games,
			want:  true,
		},
		{
			name:  "black friday before kickoff at fifteen minutes",
			last:  novemberTime(27, 10, 0),
			now:   novemberTime(27, 10, 15),
			games: tgGames,
			want:  true,
		},
		{
			name:  "christmas wednesday before kickoff at fifteen minutes",
			last:  easternTime(2024, time.December, 25, 10, 0),
			now:   easternTime(2024, time.December, 25, 10, 15),
			games: xmasGames,
			want:  true,
		},
		{
			name:  "playoff saturday before kickoff at fifteen minutes",
			last:  easternTime(2026, time.January, 10, 10, 0),
			now:   easternTime(2026, time.January, 10, 10, 15),
			games: poGames,
			want:  true,
		},
		{
			name:  "sunday after last sunday kickoff waits two hours",
			last:  novemberTime(8, 17, 0),
			now:   novemberTime(8, 18, 59),
			games: games,
			want:  false,
		},
		{
			name:  "sunday after last sunday kickoff at two hours",
			last:  novemberTime(8, 17, 0),
			now:   novemberTime(8, 19, 0),
			games: games,
			want:  true,
		},
		{
			name:  "midweek waits two hours",
			last:  novemberTime(4, 12, 0),
			now:   novemberTime(4, 13, 59),
			games: games,
			want:  false,
		},
		{
			name:  "midweek at two hours",
			last:  novemberTime(4, 12, 0),
			now:   novemberTime(4, 14, 0),
			games: games,
			want:  true,
		},
		{
			name:  "within two hours of next kickoff waits fifteen minutes",
			last:  novemberTime(7, 23, 0),
			now:   novemberTime(7, 23, 15),
			games: []db.Game{{ID: "early", Kickoff: novemberTime(8, 1, 0)}},
			want:  true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, test.want, oddsPickLinesStale(test.last, test.now, test.games))
		})
	}
}

func TestPickCooldownStaleRules(t *testing.T) {
	t.Parallel()

	now := novemberTime(8, 12, 0)
	assert.True(t, pickCooldownStale(time.Time{}, now))
	assert.False(t, pickCooldownStale(now.Add(-2*time.Minute+time.Second), now))
	assert.True(t, pickCooldownStale(now.Add(-2*time.Minute), now))
}

func TestRosterStaleRules(t *testing.T) {
	t.Parallel()

	games := slateGames()
	tgGames := thanksgivingWeekGames()
	sunday := novemberTime(8, 12, 0)
	saturday := novemberTime(7, 12, 0)
	blackFriday := novemberTime(27, 12, 0)
	tests := []struct {
		name  string
		last  time.Time
		now   time.Time
		games []db.Game
		want  bool
	}{
		{
			name:  "never fetched",
			now:   sunday,
			games: games,
			want:  true,
		},
		{
			name:  "game day waits 2 hours",
			last:  sunday.Add(-2*time.Hour + time.Minute),
			now:   sunday,
			games: games,
			want:  false,
		},
		{
			name:  "game day at 2 hours",
			last:  sunday.Add(-2 * time.Hour),
			now:   sunday,
			games: games,
			want:  true,
		},
		{
			name:  "black friday game day at 2 hours",
			last:  blackFriday.Add(-2 * time.Hour),
			now:   blackFriday,
			games: tgGames,
			want:  true,
		},
		{
			name:  "idle waits 6 hours",
			last:  saturday.Add(-6*time.Hour + time.Minute),
			now:   saturday,
			games: games,
			want:  false,
		},
		{
			name:  "idle at 6 hours",
			last:  saturday.Add(-6 * time.Hour),
			now:   saturday,
			games: games,
			want:  true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, test.want, rosterStale(test.last, test.now, test.games))
		})
	}
}

func TestSummaryStaleRules(t *testing.T) {
	t.Parallel()

	sunday := novemberTime(8, 12, 0)
	saturday := novemberTime(7, 12, 0)
	scheduledGame := db.Game{ID: "401", Status: db.GameScheduled, Kickoff: novemberTime(8, 13, 0)}
	liveGame := db.Game{ID: "405", Status: db.GameLive, Kickoff: novemberTime(8, 13, 0)}
	saturdayNearKickoff := db.Game{ID: "403", Status: db.GameScheduled, Kickoff: saturday.Add(3 * time.Hour)}
	saturdayFarKickoff := db.Game{ID: "404", Status: db.GameScheduled, Kickoff: saturday.Add(6 * time.Hour)}
	finalGame := db.Game{ID: "402", Status: db.GameFinal}

	tests := []struct {
		name        string
		last        time.Time
		now         time.Time
		game        db.Game
		hasSlipLegs bool
		want        bool
	}{
		{
			name: "never fetched scheduled game",
			now:  sunday,
			game: scheduledGame,
			want: true,
		},
		{
			name:        "final game is skipped even with slip legs",
			now:         sunday,
			game:        finalGame,
			hasSlipLegs: true,
			want:        false,
		},
		{
			name:        "live game with slip legs waits 2 minutes",
			last:        sunday.Add(-2*time.Minute + time.Second),
			now:         sunday,
			game:        liveGame,
			hasSlipLegs: true,
			want:        false,
		},
		{
			name:        "live game with slip legs at 2 minutes",
			last:        sunday.Add(-2 * time.Minute),
			now:         sunday,
			game:        liveGame,
			hasSlipLegs: true,
			want:        true,
		},
		{
			name:        "live game without slip legs on game day waits 15 minutes",
			last:        sunday.Add(-14 * time.Minute),
			now:         sunday,
			game:        liveGame,
			hasSlipLegs: false,
			want:        false,
		},
		{
			name:        "live game without slip legs on game day at 15 minutes",
			last:        sunday.Add(-15 * time.Minute),
			now:         sunday,
			game:        liveGame,
			hasSlipLegs: false,
			want:        true,
		},
		{
			name:        "live game without slip legs off game day waits 1 hour",
			last:        saturday.Add(-59 * time.Minute),
			now:         saturday,
			game:        liveGame,
			hasSlipLegs: false,
			want:        false,
		},
		{
			name:        "scheduled game with slip legs on game day still waits 15 minutes",
			last:        sunday.Add(-14 * time.Minute),
			now:         sunday,
			game:        scheduledGame,
			hasSlipLegs: true,
			want:        false,
		},
		{
			name: "game day waits 15 minutes",
			last: sunday.Add(-14 * time.Minute),
			now:  sunday,
			game: scheduledGame,
			want: false,
		},
		{
			name: "game day at 15 minutes",
			last: sunday.Add(-15 * time.Minute),
			now:  sunday,
			game: scheduledGame,
			want: true,
		},
		{
			name: "near kickoff waits 15 minutes",
			last: saturday.Add(-14 * time.Minute),
			now:  saturday,
			game: saturdayNearKickoff,
			want: false,
		},
		{
			name: "near kickoff at 15 minutes",
			last: saturday.Add(-15 * time.Minute),
			now:  saturday,
			game: saturdayNearKickoff,
			want: true,
		},
		{
			name: "saturday game far from kickoff on saturday still uses 15m game day window",
			last: saturday.Add(-15 * time.Minute),
			now:  saturday,
			game: saturdayFarKickoff,
			want: true,
		},
		{
			name: "idle on non-game day waits 1 hour",
			last: saturday.Add(-59 * time.Minute),
			now:  saturday,
			game: scheduledGame,
			want: false,
		},
		{
			name: "idle on non-game day at 1 hour",
			last: saturday.Add(-time.Hour),
			now:  saturday,
			game: scheduledGame,
			want: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, test.want, summaryStale(test.last, test.now, test.game, test.hasSlipLegs))
		})
	}
}

func TestGameLogStaleRules(t *testing.T) {
	t.Parallel()

	games := slateGames()
	poGames := playoffWeekendGames()
	sunday := novemberTime(8, 12, 0)
	saturday := novemberTime(7, 12, 0)
	playoffSat := easternTime(2026, time.January, 10, 12, 0)
	tests := []struct {
		name  string
		last  time.Time
		now   time.Time
		games []db.Game
		want  bool
	}{
		{
			name:  "never fetched",
			now:   sunday,
			games: games,
			want:  true,
		},
		{
			name:  "game day waits 1 hour",
			last:  sunday.Add(-59 * time.Minute),
			now:   sunday,
			games: games,
			want:  false,
		},
		{
			name:  "game day at 1 hour",
			last:  sunday.Add(-time.Hour),
			now:   sunday,
			games: games,
			want:  true,
		},
		{
			name:  "playoff saturday game day at 1 hour",
			last:  playoffSat.Add(-time.Hour),
			now:   playoffSat,
			games: poGames,
			want:  true,
		},
		{
			name:  "idle waits 4 hours",
			last:  saturday.Add(-4*time.Hour + time.Minute),
			now:   saturday,
			games: games,
			want:  false,
		},
		{
			name:  "idle at 4 hours",
			last:  saturday.Add(-4 * time.Hour),
			now:   saturday,
			games: games,
			want:  true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, test.want, gameLogStale(test.last, test.now, test.games))
		})
	}
}

func TestPrefillStaleRules(t *testing.T) {
	t.Parallel()

	now := novemberTime(8, 9, 30)
	assert.True(t, prefillStale(time.Time{}, now))
	assert.False(t, prefillStale(now.Add(-14*time.Minute), now))
	assert.True(t, prefillStale(now.Add(-15*time.Minute), now))
}

func TestIsGameDay(t *testing.T) {
	t.Parallel()

	tgGames := thanksgivingWeekGames()
	xmasGames := christmasGames()
	poGames := playoffWeekendGames()

	tests := []struct {
		name  string
		games []db.Game
		now   time.Time
		want  bool
	}{
		// Cold DB (nil games) falls back to Thu/Sun/Mon.
		{name: "cold DB Sunday", now: novemberTime(8, 12, 0), want: true},
		{name: "cold DB Monday", now: novemberTime(9, 12, 0), want: true},
		{name: "cold DB Tuesday", now: novemberTime(10, 12, 0), want: false},
		{name: "cold DB Wednesday", now: novemberTime(4, 12, 0), want: false},
		{name: "cold DB Thursday", now: novemberTime(5, 12, 0), want: true},
		{name: "cold DB Friday", now: novemberTime(6, 12, 0), want: false},
		{name: "cold DB Saturday", now: novemberTime(7, 12, 0), want: false},

		// Scoreboard-derived irregular game days and non-game days.
		{name: "Thanksgiving Thursday", games: tgGames, now: novemberTime(26, 9, 0), want: true},
		{name: "Black Friday", games: tgGames, now: novemberTime(27, 9, 0), want: true},
		{name: "Thanksgiving week Tuesday (non-game day)", games: tgGames, now: novemberTime(24, 12, 0), want: false},
		{name: "Thanksgiving week Saturday (non-game day)", games: tgGames, now: novemberTime(28, 12, 0), want: false},
		{name: "Christmas Wednesday", games: xmasGames, now: easternTime(2024, time.December, 25, 10, 0), want: true},
		{
			name:  "Christmas week Tuesday (non-game day)",
			games: xmasGames,
			now:   easternTime(2024, time.December, 24, 12, 0),
			want:  false,
		},
		{name: "Playoff Saturday", games: poGames, now: easternTime(2026, time.January, 10, 10, 0), want: true},
		{name: "Playoff Sunday", games: poGames, now: easternTime(2026, time.January, 11, 10, 0), want: true},
		{
			name:  "Playoff Thursday with no games (non-game day)",
			games: poGames,
			now:   easternTime(2026, time.January, 8, 12, 0),
			want:  false,
		},
		{
			name:  "Playoff Monday with no games (non-game day)",
			games: poGames,
			now:   easternTime(2026, time.January, 12, 12, 0),
			want:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, isGameDay(tt.games, tt.now))
		})
	}
}

func TestGamesOnDateAndUpcomingGamesOnDate(t *testing.T) {
	t.Parallel()

	tgGames := thanksgivingWeekGames()
	// Thanksgiving night game is 20:20 ET on Nov 26 (which is 01:20 UTC on Nov 27).
	// Querying on Nov 26 at 18:00 ET must still include tg-night, and querying on
	// Black Friday (Nov 27) must NOT include tg-night.
	tgMidEvening := novemberTime(26, 18, 0)
	onTG := gamesOnDate(tgGames, tgMidEvening)
	assert.Len(t, onTG, 3)
	upcomingTG := upcomingGamesOnDate(tgGames, tgMidEvening)
	requireGameIDs(t, upcomingTG, []string{"tg-night"})

	bfMorning := novemberTime(27, 8, 0)
	onBF := gamesOnDate(tgGames, bfMorning)
	requireGameIDs(t, onBF, []string{"bf"})
	upcomingBF := upcomingGamesOnDate(tgGames, bfMorning)
	requireGameIDs(t, upcomingBF, []string{"bf"})

	// After Black Friday kickoff, gamesOnDate still returns bf, upcomingGamesOnDate is empty.
	bfAfter := novemberTime(27, 15, 0)
	requireGameIDs(t, gamesOnDate(tgGames, bfAfter), []string{"bf"})
	assert.Empty(t, upcomingGamesOnDate(tgGames, bfAfter))

	// Final games on the date are excluded from upcomingGamesOnDate.
	finalGame := []db.Game{{ID: "done", Status: db.GameFinal, Kickoff: novemberTime(27, 15, 0)}}
	requireGameIDs(t, gamesOnDate(finalGame, bfMorning), []string{"done"})
	assert.Empty(t, upcomingGamesOnDate(finalGame, bfMorning))

	// Non-game Tuesday returns empty for both.
	tuesday := novemberTime(24, 12, 0)
	assert.Empty(t, gamesOnDate(tgGames, tuesday))
	assert.Empty(t, upcomingGamesOnDate(tgGames, tuesday))
}

func requireGameIDs(t *testing.T, games []db.Game, want []string) {
	t.Helper()
	got := make([]string, len(games))
	for i, g := range games {
		got[i] = g.ID
	}
	assert.Equal(t, want, got)
}

func TestGameDayPrefillWindow(t *testing.T) {
	t.Parallel()

	games := slateGames()
	tgGames := thanksgivingWeekGames()
	xmasGames := christmasGames()
	poGames := playoffWeekendGames()
	mondayOnly := []db.Game{
		{ID: "mon", Kickoff: novemberTime(9, 20, 15)},
	}

	tests := []struct {
		name  string
		games []db.Game
		now   time.Time
		want  bool
	}{
		{
			name:  "no games",
			games: nil,
			now:   novemberTime(8, 8, 0),
			want:  false,
		},
		{
			name:  "no sunday games even with monday game",
			games: mondayOnly,
			now:   novemberTime(8, 8, 0),
			want:  false,
		},
		{
			name:  "monday with monday game at 06:00 ET",
			games: games,
			now:   novemberTime(9, 6, 0),
			want:  true,
		},
		{
			name:  "non-game thursday",
			games: games,
			now:   novemberTime(5, 8, 0),
			want:  false,
		},
		{
			name:  "non-game saturday",
			games: games,
			now:   novemberTime(7, 23, 59),
			want:  false,
		},
		{
			name:  "sunday midnight ET before start hour",
			games: games,
			now:   novemberTime(8, 0, 0),
			want:  false,
		},
		{
			name:  "sunday 05:59 ET before start hour",
			games: games,
			now:   novemberTime(8, 5, 59),
			want:  false,
		},
		{
			name:  "sunday 06:00 ET with upcoming sunday games",
			games: games,
			now:   novemberTime(8, 6, 0),
			want:  true,
		},
		{
			name:  "sunday between early and late kickoff",
			games: games,
			now:   novemberTime(8, 16, 24),
			want:  true,
		},
		{
			name:  "sunday at last sunday kickoff with monday game remaining",
			games: games,
			now:   novemberTime(8, 16, 25),
			want:  false,
		},
		{
			name:  "sunday night after last sunday kickoff with monday game remaining",
			games: games,
			now:   novemberTime(8, 20, 0),
			want:  false,
		},
		{
			name:  "thanksgiving 05:59 ET before start hour",
			games: tgGames,
			now:   novemberTime(26, 5, 59),
			want:  false,
		},
		{
			name:  "thanksgiving 06:00 ET triggers window",
			games: tgGames,
			now:   novemberTime(26, 6, 0),
			want:  true,
		},
		{
			name:  "thanksgiving after last kickoff with black friday remaining is false",
			games: tgGames,
			now:   novemberTime(26, 20, 20),
			want:  false,
		},
		{
			name:  "black friday 06:00 ET triggers window",
			games: tgGames,
			now:   novemberTime(27, 6, 0),
			want:  true,
		},
		{
			name:  "black friday after kickoff is false",
			games: tgGames,
			now:   novemberTime(27, 15, 0),
			want:  false,
		},
		{
			name:  "christmas wednesday 06:05 ET triggers window",
			games: xmasGames,
			now:   easternTime(2024, time.December, 25, 6, 5),
			want:  true,
		},
		{
			name:  "playoff saturday 06:05 ET triggers window",
			games: poGames,
			now:   easternTime(2026, time.January, 10, 6, 5),
			want:  true,
		},
		{
			name:  "playoff thursday with no games is false",
			games: poGames,
			now:   easternTime(2026, time.January, 8, 9, 0),
			want:  false,
		},
		{
			name:  "non-game tuesday is false",
			games: tgGames,
			now:   novemberTime(24, 9, 0),
			want:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, gameDayPrefillWindow(tt.games, tt.now))
		})
	}
}

func TestGameDayPrefillStale(t *testing.T) {
	t.Parallel()

	fallBackMidnight := time.Date(2026, time.November, 1, 0, 30, 0, 0, eastern).UTC()
	fallBackMorning := time.Date(2026, time.November, 1, 6, 0, 0, 0, eastern).UTC()
	fallBackEveSat := time.Date(2026, time.October, 31, 23, 30, 0, 0, eastern).UTC()
	springForwardMorning := time.Date(2026, time.March, 8, 6, 0, 0, 0, eastern).UTC()
	springForwardNight := time.Date(2026, time.March, 8, 23, 0, 0, 0, eastern).UTC()

	tests := []struct {
		name string
		last time.Time
		now  time.Time
		want bool
	}{
		{
			name: "zero last time",
			last: time.Time{},
			now:  novemberTime(8, 6, 0),
			want: true,
		},
		{
			name: "same eastern sunday",
			last: novemberTime(8, 6, 0),
			now:  novemberTime(8, 15, 0),
			want: false,
		},
		{
			name: "thanksgiving to black friday is stale on new game date",
			last: novemberTime(26, 6, 5),
			now:  novemberTime(27, 6, 5),
			want: true,
		},
		{
			name: "same black friday date is fresh",
			last: novemberTime(27, 6, 5),
			now:  novemberTime(27, 14, 0),
			want: false,
		},
		{
			name: "saturday before sunday",
			last: novemberTime(7, 23, 59),
			now:  novemberTime(8, 6, 0),
			want: true,
		},
		{
			name: "prior sunday",
			last: novemberTime(1, 6, 0),
			now:  novemberTime(8, 6, 0),
			want: true,
		},
		{
			name: "same eastern sunday across fall-back DST",
			last: fallBackMidnight,
			now:  fallBackMorning,
			want: false,
		},
		{
			name: "saturday late EDT vs sunday morning EST sharing UTC date",
			last: fallBackEveSat,
			now:  fallBackMorning,
			want: true,
		},
		{
			name: "same eastern sunday across spring-forward DST",
			last: springForwardMorning,
			now:  springForwardNight,
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, gameDayPrefillStale(tt.last, tt.now))
		})
	}
}
