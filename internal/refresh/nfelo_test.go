package refresh

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/ingest/nfelo"
)

// fakeProjections serves a prepared projection list or an error, and counts the
// fetches.
type fakeProjections struct {
	projections []nfelo.Projection
	err         error
	calls       atomic.Int32
}

// Projections returns the prepared answer and counts the call.
func (f *fakeProjections) Projections(context.Context) ([]nfelo.Projection, error) {
	f.calls.Add(1)
	return f.projections, f.err
}

// nfeloTestStore builds week 9 with the one stored game the projection tests
// use.
func nfeloTestStore(t *testing.T, now time.Time) (*db.Store, db.Week, db.Game) {
	t.Helper()

	store := openTestStore(t)
	week, err := store.UpsertWeek(t.Context(), 2026, 2, 9)
	require.NoError(t, err)
	game, err := store.UpsertGame(t.Context(), storedGame(week.ID, now))
	require.NoError(t, err)
	return store, week, game
}

// bufMiaProjection is nfelo's row for the stored game.
func bufMiaProjection() nfelo.Projection {
	return nfelo.Projection{
		Season:      2026,
		Week:        9,
		Home:        "BUF",
		Away:        "MIA",
		HomeSpread:  -7.5,
		HomeWinProb: 0.72,
		HomeQB:      "Josh Allen",
		AwayQB:      "Tua Tagovailoa",
		HomeQBAdj:   18.5,
		AwayQBAdj:   -4.25,
	}
}

func TestSyncNfeloWritesMatchingRowsAndPublishesOnce(t *testing.T) {
	t.Parallel()

	now := novemberTime(7, 11, 0)
	store, week, game := nfeloTestStore(t, now)

	other := storedGame(week.ID, now)
	other.ID = "402"
	other.HomeTeam = "DAL"
	other.AwayTeam = "PHI"
	other, err := store.UpsertGame(t.Context(), other)
	require.NoError(t, err)

	client := &fakeProjections{projections: []nfelo.Projection{
		bufMiaProjection(),
		// Another season's similar matchup must not land on this week.
		{Season: 2025, Week: 9, Home: "BUF", Away: "MIA", HomeSpread: 2.5, HomeWinProb: 0.4},
		// A game this week that is not on the slate has nowhere to go.
		{Season: 2026, Week: 9, Home: "SF", Away: "SEA", HomeSpread: -1.5, HomeWinProb: 0.54},
	}}
	publisher := &recordingPublisher{}
	service := New(Deps{
		Store: store, Projections: client, Publisher: publisher, Logger: zerolog.Nop(),
	})

	changed, err := service.syncNfelo(
		t.Context(), week, []db.Game{game, other}, now)
	require.NoError(t, err)
	assert.True(t, changed)

	stored, ok, err := store.NfeloForGame(t.Context(), game.ID)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, db.NfeloGame{
		GameID:      game.ID,
		HomeSpread:  -7.5,
		HomeWinProb: 0.72,
		HomeQB:      "Josh Allen",
		AwayQB:      "Tua Tagovailoa",
		HomeQBAdj:   18.5,
		AwayQBAdj:   -4.25,
		UpdatedAt:   now,
	}, stored)

	_, ok, err = store.NfeloForGame(t.Context(), other.ID)
	require.NoError(t, err)
	assert.False(t, ok, "a game with no matching row writes nothing")

	state, err := store.SyncState(t.Context(), NfeloProjectionsSource)
	require.NoError(t, err)
	assert.Equal(t, now, state.FetchedAt)

	assert.Equal(t, []string{fmt.Sprintf("week:%d", week.ID)}, publisher.seen())

	// The same CSV at a later time changes no row and publishes nothing.
	changed, err = service.syncNfelo(
		t.Context(), week, []db.Game{game, other}, now.Add(time.Hour))
	require.NoError(t, err)
	assert.False(t, changed)

	stored, ok, err = store.NfeloForGame(t.Context(), game.ID)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, now, stored.UpdatedAt, "an unchanged CSV leaves updated_at alone")
	assert.Equal(t, []string{fmt.Sprintf("week:%d", week.ID)}, publisher.seen(),
		"the second sync publishes nothing")
}

func TestNfeloStale(t *testing.T) {
	t.Parallel()

	saturday := novemberTime(7, 11, 0)
	sunday := novemberTime(8, 11, 0)
	regular := db.Week{ID: 2026209, Season: 2026, SeasonType: 2, WeekNum: 9}
	playoff := regular
	playoff.SeasonType = 3
	upcoming := []db.Game{{ID: "401", Kickoff: sunday.Add(2 * time.Hour)}}
	kickedOff := []db.Game{{ID: "401", Kickoff: saturday.Add(-time.Minute)}}

	tests := map[string]struct {
		last  time.Time
		now   time.Time
		week  db.Week
		games []db.Game
		want  bool
	}{
		"never fetched": {last: time.Time{}, now: saturday, week: regular, games: upcoming, want: true},
		"idle waits four hours": {
			last:  saturday.Add(-nfeloIdleAge + time.Minute),
			now:   saturday,
			week:  regular,
			games: upcoming,
			want:  false,
		},
		"idle at four hours": {
			last:  saturday.Add(-nfeloIdleAge),
			now:   saturday,
			week:  regular,
			games: upcoming,
			want:  true,
		},
		"game day waits one hour": {
			last:  sunday.Add(-nfeloGameDayAge + time.Minute),
			now:   sunday,
			week:  regular,
			games: upcoming,
			want:  false,
		},
		"game day at one hour": {
			last:  sunday.Add(-nfeloGameDayAge),
			now:   sunday,
			week:  regular,
			games: upcoming,
			want:  true,
		},
		"playoff week":   {last: time.Time{}, now: saturday, week: playoff, games: upcoming, want: false},
		"all kicked off": {last: time.Time{}, now: saturday, week: regular, games: kickedOff, want: false},
		"no games":       {last: time.Time{}, now: saturday, week: regular, games: nil, want: false},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, nfeloStale(tt.last, tt.now, tt.week, tt.games))
		})
	}
}

func TestNfeloSourceSkipsANonRegularSeasonOrKickedOffWeek(t *testing.T) {
	t.Parallel()

	now := novemberTime(7, 11, 0)
	tests := map[string]struct {
		seasonType int
		kickoff    time.Time
	}{
		"playoff week":   {seasonType: 3, kickoff: now.Add(time.Hour)},
		"all kicked off": {seasonType: 2, kickoff: now.Add(-time.Minute)},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			store, week, game := nfeloTestStore(t, now)
			week.SeasonType = tt.seasonType
			game.Kickoff = tt.kickoff

			client := &fakeProjections{projections: []nfelo.Projection{bufMiaProjection()}}
			service := New(Deps{
				Store: store, Projections: client, Logger: zerolog.Nop(),
			})

			service.runSources(t.Context(), now, []source{
				service.nfeloSource(week, []db.Game{game}, now),
			})
			service.wait()

			assert.Zero(t, client.calls.Load())
			_, err := store.SyncState(t.Context(), NfeloProjectionsSource)
			require.ErrorIs(t, err, db.ErrSyncStateNotFound)
		})
	}
}

func TestKickSkipsNfeloWithoutAClient(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	service := New(Deps{
		Store:      store,
		Scoreboard: fakeScoreboard{board: espnWeek()},
		Now:        func() time.Time { return novemberTime(4, 12, 0) },
		Logger:     zerolog.Nop(),
	})

	service.KickContext(t.Context())
	service.wait()

	_, err := store.SyncState(t.Context(), NfeloProjectionsSource)
	require.ErrorIs(t, err, db.ErrSyncStateNotFound,
		"a nil projections client leaves the source off")
}

func TestNfeloFailureKeepsRowsAndBacksOff(t *testing.T) {
	t.Parallel()

	now := novemberTime(7, 11, 0)
	store, week, game := nfeloTestStore(t, now)

	seeded := db.NfeloGame{
		GameID:      game.ID,
		HomeSpread:  -6.5,
		HomeWinProb: 0.68,
		HomeQB:      "Josh Allen",
		AwayQB:      "Tua Tagovailoa",
		HomeQBAdj:   12,
		AwayQBAdj:   -2,
		UpdatedAt:   now.Add(-48 * time.Hour),
	}
	_, err := store.UpsertNfeloGames(t.Context(), []db.NfeloGame{seeded})
	require.NoError(t, err)
	require.NoError(t, store.RecordSync(
		t.Context(), NfeloProjectionsSource, now.Add(-48*time.Hour), nil))

	clock := &fakeClock{now: now}
	client := &fakeProjections{err: errors.New("github is down")}
	service := New(Deps{
		Store: store, Projections: client, Now: clock.Now, Logger: zerolog.Nop(),
	})
	src := service.nfeloSource(week, []db.Game{game}, now)

	service.runSources(t.Context(), clock.Now(), []source{src})
	service.wait()
	assert.Equal(t, int32(1), client.calls.Load())

	stored, ok, err := store.NfeloForGame(t.Context(), game.ID)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, seeded, stored, "a failed fetch keeps the old row")

	state, err := store.SyncState(t.Context(), NfeloProjectionsSource)
	require.NoError(t, err)
	assert.Equal(t, now.Add(-48*time.Hour), state.FetchedAt,
		"a failed fetch is not recorded as fresh")

	clock.Advance(4 * time.Minute)
	service.runSources(t.Context(), clock.Now(), []source{src})
	service.wait()
	assert.Equal(t, int32(1), client.calls.Load(), "the failure pauses the source")

	clock.Advance(time.Minute)
	service.runSources(t.Context(), clock.Now(), []source{src})
	service.wait()
	assert.Equal(t, int32(2), client.calls.Load(), "the source retries after five minutes")
}
