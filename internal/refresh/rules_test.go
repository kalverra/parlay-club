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

// novemberTime builds a 2026 November wall-clock time in America/New_York and
// returns it in UTC, the form the database stores.
func novemberTime(day, hour, minute int) time.Time {
	return time.Date(2026, time.November, day, hour, minute, 0, 0, eastern).UTC()
}

func TestScoreboardStaleRules(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		last time.Time
		now  time.Time
		live bool
		want bool
	}{
		{
			name: "never fetched",
			now:  novemberTime(4, 12, 0),
			want: true,
		},
		{
			name: "live game waits ten minutes",
			last: novemberTime(8, 13, 0),
			now:  novemberTime(8, 13, 9),
			live: true,
			want: false,
		},
		{
			name: "live game at ten minutes",
			last: novemberTime(8, 13, 0),
			now:  novemberTime(8, 13, 10),
			live: true,
			want: true,
		},
		{
			name: "game day waits thirty minutes",
			last: novemberTime(8, 13, 0),
			now:  novemberTime(8, 13, 29),
			want: false,
		},
		{
			name: "game day at thirty minutes",
			last: novemberTime(8, 13, 0),
			now:  novemberTime(8, 13, 30),
			want: true,
		},
		{
			name: "monday is also a game day",
			last: novemberTime(9, 12, 0),
			now:  novemberTime(9, 12, 30),
			want: true,
		},
		{
			name: "thursday is also a game day",
			last: novemberTime(5, 12, 0),
			now:  novemberTime(5, 12, 30),
			want: true,
		},
		{
			name: "thursday waits thirty minutes",
			last: novemberTime(5, 12, 0),
			now:  novemberTime(5, 12, 29),
			want: false,
		},
		{
			name: "midweek waits six hours",
			last: novemberTime(4, 12, 0),
			now:  novemberTime(4, 17, 59),
			want: false,
		},
		{
			name: "midweek at six hours",
			last: novemberTime(4, 12, 0),
			now:  novemberTime(4, 18, 0),
			want: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, test.want, scoreboardStale(test.last, test.now, test.live))
		})
	}
}

func TestOddsStaleRules(t *testing.T) {
	t.Parallel()

	games := slateGames()
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
			name:  "wednesday waits twenty four hours",
			last:  novemberTime(4, 12, 0),
			now:   novemberTime(5, 11, 59),
			games: games,
			want:  false,
		},
		{
			name:  "wednesday at twenty four hours",
			last:  novemberTime(4, 12, 0),
			now:   novemberTime(5, 12, 0),
			games: games,
			want:  true,
		},
		{
			name:  "saturday waits twelve hours",
			last:  novemberTime(7, 8, 0),
			now:   novemberTime(7, 19, 59),
			games: games,
			want:  false,
		},
		{
			name:  "saturday at twelve hours",
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
			name:  "sunday after the last kickoff waits three hours",
			last:  novemberTime(8, 14, 30),
			now:   novemberTime(8, 17, 0),
			games: games,
			want:  false,
		},
		{
			name:  "sunday after the last kickoff at three hours",
			last:  novemberTime(8, 14, 0),
			now:   novemberTime(8, 17, 0),
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
			name: "forecast waits an hour",
			last: novemberTime(7, 10, 1),
			now:  novemberTime(7, 11, 0),
			game: game,
			want: false,
		},
		{
			name: "forecast at an hour",
			last: novemberTime(7, 10, 0),
			now:  novemberTime(7, 11, 0),
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
			name: "props wait two hours",
			last: novemberTime(8, 9, 0),
			now:  novemberTime(8, 10, 59),
			game: game,
			want: false,
		},
		{
			name: "props at two hours",
			last: novemberTime(8, 9, 0),
			now:  novemberTime(8, 11, 0),
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

func TestRosterStaleRules(t *testing.T) {
	t.Parallel()

	now := novemberTime(8, 12, 0)
	tests := []struct {
		name string
		last time.Time
		now  time.Time
		want bool
	}{
		{
			name: "never fetched",
			now:  now,
			want: true,
		},
		{
			name: "roster waits 24 hours",
			last: now.Add(-23 * time.Hour),
			now:  now,
			want: false,
		},
		{
			name: "roster at 24 hours",
			last: now.Add(-24 * time.Hour),
			now:  now,
			want: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, test.want, rosterStale(test.last, test.now))
		})
	}
}

func TestSummaryStaleRules(t *testing.T) {
	t.Parallel()

	now := novemberTime(8, 12, 0)
	scheduledGame := db.Game{ID: "401", Status: db.GameScheduled}
	finalGame := db.Game{ID: "402", Status: db.GameFinal}

	tests := []struct {
		name string
		last time.Time
		now  time.Time
		game db.Game
		want bool
	}{
		{
			name: "never fetched scheduled game",
			now:  now,
			game: scheduledGame,
			want: true,
		},
		{
			name: "final game is skipped",
			now:  now,
			game: finalGame,
			want: false,
		},
		{
			name: "summary waits two hours",
			last: now.Add(-1 * time.Hour),
			now:  now,
			game: scheduledGame,
			want: false,
		},
		{
			name: "summary at two hours",
			last: now.Add(-2 * time.Hour),
			now:  now,
			game: scheduledGame,
			want: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, test.want, summaryStale(test.last, test.now, test.game))
		})
	}
}

func TestGameLogStaleRules(t *testing.T) {
	t.Parallel()

	now := novemberTime(8, 12, 0)
	tests := []struct {
		name string
		last time.Time
		now  time.Time
		want bool
	}{
		{
			name: "never fetched",
			now:  now,
			want: true,
		},
		{
			name: "gamelog waits 12 hours",
			last: now.Add(-11 * time.Hour),
			now:  now,
			want: false,
		},
		{
			name: "gamelog at 12 hours",
			last: now.Add(-12 * time.Hour),
			now:  now,
			want: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, test.want, gameLogStale(test.last, test.now))
		})
	}
}

func TestGameDay(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		now  time.Time
		want bool
	}{
		{name: "Sunday", now: novemberTime(8, 12, 0), want: true},
		{name: "Monday", now: novemberTime(9, 12, 0), want: true},
		{name: "Tuesday", now: novemberTime(10, 12, 0), want: false},
		{name: "Wednesday", now: novemberTime(4, 12, 0), want: false},
		{name: "Thursday", now: novemberTime(5, 12, 0), want: true},
		{name: "Friday", now: novemberTime(6, 12, 0), want: false},
		{name: "Saturday", now: novemberTime(7, 12, 0), want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, gameDay(tt.now))
		})
	}
}
