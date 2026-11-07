package refresh

import (
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/db"
)

func TestLiveWindowRule(t *testing.T) {
	t.Parallel()

	now := novemberTime(8, 12, 30)

	tests := []struct {
		name  string
		games []db.Game
		want  bool
	}{
		{name: "no games", want: false},
		{
			name:  "live game",
			games: []db.Game{{ID: "live", Status: db.GameLive, Kickoff: now.Add(-time.Hour)}},
			want:  true,
		},
		{
			name:  "kickoff in thirty minutes",
			games: []db.Game{{ID: "soon", Status: db.GameScheduled, Kickoff: now.Add(30 * time.Minute)}},
			want:  true,
		},
		{
			name:  "kickoff in thirty-one minutes",
			games: []db.Game{{ID: "later", Status: db.GameScheduled, Kickoff: now.Add(31 * time.Minute)}},
			want:  false,
		},
		{
			name:  "final game near kickoff",
			games: []db.Game{{ID: "final", Status: db.GameFinal, Kickoff: now.Add(10 * time.Minute)}},
			want:  false,
		},
		{
			name:  "scheduled game at kickoff",
			games: []db.Game{{ID: "started", Status: db.GameScheduled, Kickoff: now}},
			want:  false,
		},
		{
			name: "one game in the window is enough",
			games: []db.Game{
				{ID: "later", Status: db.GameScheduled, Kickoff: now.Add(4 * time.Hour)},
				{ID: "live", Status: db.GameLive, Kickoff: now.Add(-time.Hour)},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, liveWindow(tt.games, now))
		})
	}
}

func TestServiceLiveWindowReadsTheWeek(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	now := novemberTime(8, 12, 0)

	liveWeek, err := store.UpsertWeek(t.Context(), 2026, 2, 9)
	require.NoError(t, err)
	_, err = store.UpsertGame(t.Context(), liveTestGame(
		liveWeek.ID, "soon", now.Add(15*time.Minute), db.GameScheduled))
	require.NoError(t, err)

	idleWeek, err := store.UpsertWeek(t.Context(), 2026, 2, 10)
	require.NoError(t, err)
	_, err = store.UpsertGame(t.Context(), liveTestGame(
		idleWeek.ID, "later", now.Add(3*time.Hour), db.GameScheduled))
	require.NoError(t, err)

	service := New(Deps{
		Store:  store,
		Now:    func() time.Time { return now },
		Logger: zerolog.Nop(),
	})

	assert.True(t, service.LiveWindow(liveWeek.ID))
	assert.False(t, service.LiveWindow(idleWeek.ID))
	assert.False(t, service.LiveWindow(999999), "a week with no games is not live")
}

// liveTestGame is a stored game with the given kickoff and status.
func liveTestGame(weekID int64, id string, kickoff time.Time, status db.GameStatus) db.Game {
	game := storedGame(weekID, kickoff)
	game.ID = id
	game.Kickoff = kickoff
	game.Status = status
	return game
}
