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
			name: "live game waits five minutes",
			last: novemberTime(8, 13, 0),
			now:  novemberTime(8, 13, 4),
			live: true,
			want: false,
		},
		{
			name: "live game at five minutes",
			last: novemberTime(8, 13, 0),
			now:  novemberTime(8, 13, 5),
			live: true,
			want: true,
		},
		{
			name: "game day waits fifteen minutes",
			last: novemberTime(8, 13, 0),
			now:  novemberTime(8, 13, 14),
			want: false,
		},
		{
			name: "game day at fifteen minutes",
			last: novemberTime(8, 13, 0),
			now:  novemberTime(8, 13, 15),
			want: true,
		},
		{
			name: "monday is also a game day",
			last: novemberTime(9, 12, 0),
			now:  novemberTime(9, 12, 15),
			want: true,
		},
		{
			name: "thursday is also a game day",
			last: novemberTime(5, 12, 0),
			now:  novemberTime(5, 12, 15),
			want: true,
		},
		{
			name: "thursday waits fifteen minutes",
			last: novemberTime(5, 12, 0),
			now:  novemberTime(5, 12, 14),
			want: false,
		},
		{
			name: "midweek waits two hours",
			last: novemberTime(4, 12, 0),
			now:  novemberTime(4, 13, 59),
			want: false,
		},
		{
			name: "midweek at two hours",
			last: novemberTime(4, 12, 0),
			now:  novemberTime(4, 14, 0),
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

	sunday := novemberTime(8, 12, 0)
	saturday := novemberTime(7, 12, 0)
	tests := []struct {
		name string
		last time.Time
		now  time.Time
		want bool
	}{
		{
			name: "never fetched",
			now:  sunday,
			want: true,
		},
		{
			name: "game day waits 2 hours",
			last: sunday.Add(-2*time.Hour + time.Minute),
			now:  sunday,
			want: false,
		},
		{
			name: "game day at 2 hours",
			last: sunday.Add(-2 * time.Hour),
			now:  sunday,
			want: true,
		},
		{
			name: "idle waits 6 hours",
			last: saturday.Add(-6*time.Hour + time.Minute),
			now:  saturday,
			want: false,
		},
		{
			name: "idle at 6 hours",
			last: saturday.Add(-6 * time.Hour),
			now:  saturday,
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

	sunday := novemberTime(8, 12, 0)
	saturday := novemberTime(7, 12, 0)
	scheduledGame := db.Game{ID: "401", Status: db.GameScheduled, Kickoff: novemberTime(8, 13, 0)}
	saturdayNearKickoff := db.Game{ID: "403", Status: db.GameScheduled, Kickoff: saturday.Add(3 * time.Hour)}
	saturdayFarKickoff := db.Game{ID: "404", Status: db.GameScheduled, Kickoff: saturday.Add(6 * time.Hour)}
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
			now:  sunday,
			game: scheduledGame,
			want: true,
		},
		{
			name: "final game is skipped",
			now:  sunday,
			game: finalGame,
			want: false,
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
			name: "near kickoff on non-game-day waits 15 minutes",
			last: saturday.Add(-14 * time.Minute),
			now:  saturday,
			game: saturdayNearKickoff,
			want: false,
		},
		{
			name: "near kickoff on non-game-day at 15 minutes",
			last: saturday.Add(-15 * time.Minute),
			now:  saturday,
			game: saturdayNearKickoff,
			want: true,
		},
		{
			name: "idle waits 1 hour",
			last: saturday.Add(-59 * time.Minute),
			now:  saturday,
			game: saturdayFarKickoff,
			want: false,
		},
		{
			name: "idle at 1 hour",
			last: saturday.Add(-time.Hour),
			now:  saturday,
			game: saturdayFarKickoff,
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

	sunday := novemberTime(8, 12, 0)
	saturday := novemberTime(7, 12, 0)
	tests := []struct {
		name string
		last time.Time
		now  time.Time
		want bool
	}{
		{
			name: "never fetched",
			now:  sunday,
			want: true,
		},
		{
			name: "game day waits 1 hour",
			last: sunday.Add(-59 * time.Minute),
			now:  sunday,
			want: false,
		},
		{
			name: "game day at 1 hour",
			last: sunday.Add(-time.Hour),
			now:  sunday,
			want: true,
		},
		{
			name: "idle waits 4 hours",
			last: saturday.Add(-4*time.Hour + time.Minute),
			now:  saturday,
			want: false,
		},
		{
			name: "idle at 4 hours",
			last: saturday.Add(-4 * time.Hour),
			now:  saturday,
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

func TestPrefillStaleRules(t *testing.T) {
	t.Parallel()

	now := novemberTime(8, 9, 30)
	assert.True(t, prefillStale(time.Time{}, now))
	assert.False(t, prefillStale(now.Add(-14*time.Minute), now))
	assert.True(t, prefillStale(now.Add(-15*time.Minute), now))
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
