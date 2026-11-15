package refresh

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/ingest/nflverse"
	"github.com/kalverra/parlay-club/internal/sandbox"
)

type fakeNFLverseClient struct {
	players     []nflverse.Player
	teamStats   []nflverse.TeamWeek
	playerStats []nflverse.PlayerWeek
	snapCounts  []nflverse.SnapCount
	err         error
	calls       int
}

func (f *fakeNFLverseClient) Players(context.Context) ([]nflverse.Player, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return f.players, nil
}

func (f *fakeNFLverseClient) TeamStats(context.Context, int) ([]nflverse.TeamWeek, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.teamStats, nil
}

func (f *fakeNFLverseClient) PlayerStats(context.Context, int) ([]nflverse.PlayerWeek, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.playerStats, nil
}

func (f *fakeNFLverseClient) SnapCounts(context.Context, int) ([]nflverse.SnapCount, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.snapCounts, nil
}

func TestSyncNFLverseWritesTablesAndPublishesOnce(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.November, 8, 13, 0, 0, 0, time.UTC)
	store, week, _ := nfeloTestStore(t, now)
	publisher := &recordingPublisher{}
	client := &fakeNFLverseClient{
		players: []nflverse.Player{
			{
				GSISID:      "00-0036900",
				PFRID:       "ChasJa00",
				ESPNID:      "4362628",
				DisplayName: "Ja'Marr Chase",
				LatestTeam:  "CIN",
				Position:    "WR",
			},
			{
				GSISID:      "00-0036442",
				PFRID:       "BurrJo01",
				ESPNID:      "3915511",
				DisplayName: "Joe Burrow",
				LatestTeam:  "CIN",
				Position:    "QB",
			},
		},
		teamStats: []nflverse.TeamWeek{
			{
				Season:        2026,
				SeasonType:    nflverse.SeasonTypeRegular,
				Week:          8,
				Team:          "CIN",
				Opponent:      "BAL",
				PassingEPA:    10.5,
				RushingEPA:    2.5,
				Attempts:      36,
				Carries:       24,
				OppPassingEPA: 6.2,
				OppRushingEPA: -1.4,
			},
			// Preseason row should be ignored.
			{
				Season:     2026,
				SeasonType: nflverse.SeasonTypePreseason,
				Week:       1,
				Team:       "CIN",
				Opponent:   "BAL",
			},
		},
		playerStats: []nflverse.PlayerWeek{
			{
				PlayerID:      "00-0036900",
				Team:          "CIN",
				Season:        2026,
				SeasonType:    nflverse.SeasonTypeRegular,
				Week:          8,
				TargetShare:   0.32,
				AirYardsShare: 0.44,
				WOPR:          0.79,
				PassingEPA:    0,
				RushingEPA:    0.5,
				ReceivingEPA:  7.5,
				Targets:       12,
				Carries:       1,
			},
			// Unmatched GSIS ID fails soft.
			{
				PlayerID:   "00-9999999",
				Team:       "CIN",
				Season:     2026,
				SeasonType: nflverse.SeasonTypeRegular,
				Week:       8,
			},
		},
		snapCounts: []nflverse.SnapCount{
			{
				PFRPlayerID:  "ChasJa00",
				Player:       "Ja'Marr Chase",
				Team:         "CIN",
				Season:       2026,
				SeasonType:   nflverse.SeasonTypeRegular,
				Week:         8,
				OffenseSnaps: 58,
				OffensePct:   0.95,
			},
			// Snap-only row (0 targets/carries in player stats) still lands.
			{
				PFRPlayerID:  "BurrJo01",
				Player:       "Joe Burrow",
				Team:         "CIN",
				Season:       2026,
				SeasonType:   nflverse.SeasonTypeRegular,
				Week:         8,
				OffenseSnaps: 61,
				OffensePct:   1.0,
			},
			// Unmatched PFR ID fails soft.
			{
				PFRPlayerID:  "UnknPl00",
				Team:         "CIN",
				Season:       2026,
				SeasonType:   nflverse.SeasonTypeRegular,
				Week:         8,
				OffenseSnaps: 10,
				OffensePct:   0.15,
			},
		},
	}

	service := New(Deps{
		Store:     store,
		NFLverse:  client,
		Publisher: publisher,
		Logger:    zerolog.Nop(),
	})

	changed, err := service.syncNFLverse(t.Context(), week, now)
	require.NoError(t, err)
	assert.True(t, changed)
	assert.Equal(t, []string{fmt.Sprintf("week:%d", week.ID)}, publisher.seen())

	idRow, ok, err := store.NFLversePlayerIDByESPN(t.Context(), "4362628")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "00-0036900", idRow.GSISID)
	assert.Equal(t, "ChasJa00", idRow.PFRID)

	teamRows, err := store.NFLverseTeamWeeksForSeason(t.Context(), 2026, nflverse.SeasonTypeRegular)
	require.NoError(t, err)
	require.Len(t, teamRows, 1)
	assert.Equal(t, db.NFLverseTeamWeek{
		Season:            2026,
		SeasonType:        nflverse.SeasonTypeRegular,
		WeekNum:           8,
		Team:              "CIN",
		OffPassEPA:        10.5,
		OffRushEPA:        2.5,
		DefPassEPAAllowed: 6.2,
		DefRushEPAAllowed: -1.4,
		PlaysPerGame:      60,
		PassRate:          0.6,
		UpdatedAt:         now,
	}, teamRows[0])

	playerRows, err := store.NFLversePlayerWeeksForSeason(t.Context(), 2026, nflverse.SeasonTypeRegular)
	require.NoError(t, err)
	require.Len(t, playerRows, 2)
	assert.Equal(t, db.NFLversePlayerWeek{
		ESPNID:       "3915511",
		Season:       2026,
		SeasonType:   nflverse.SeasonTypeRegular,
		WeekNum:      8,
		Team:         "CIN",
		OffenseSnaps: 61,
		OffensePct:   1.0,
		UpdatedAt:    now,
	}, playerRows[0])
	assert.Equal(t, db.NFLversePlayerWeek{
		ESPNID:        "4362628",
		Season:        2026,
		SeasonType:    nflverse.SeasonTypeRegular,
		WeekNum:       8,
		Team:          "CIN",
		OffenseSnaps:  58,
		OffensePct:    0.95,
		TargetShare:   0.32,
		AirYardsShare: 0.44,
		WOPR:          0.79,
		Carries:       1,
		Targets:       12,
		TotalEPA:      8.0,
		UpdatedAt:     now,
	}, playerRows[1])

	state, err := store.SyncState(t.Context(), NFLverseSource(2026))
	require.NoError(t, err)
	assert.Equal(t, now, state.FetchedAt)

	// Rerun with the same data changes nothing and does not publish.
	changed, err = service.syncNFLverse(t.Context(), week, now.Add(6*time.Hour))
	require.NoError(t, err)
	assert.False(t, changed)
	assert.Len(t, publisher.seen(), 1)
}

func TestNFLverseStale(t *testing.T) {
	t.Parallel()

	// Friday 2026-11-06 14:00 EST (19:00 UTC) -> idle day (24h window).
	friday := time.Date(2026, time.November, 6, 19, 0, 0, 0, time.UTC)
	// Tuesday 2026-11-10 14:00 EST (19:00 UTC) -> active day (6h window).
	tuesday := time.Date(2026, time.November, 10, 19, 0, 0, 0, time.UTC)
	// Sunday 2026-11-08 13:00 UTC -> active day (6h window).
	sunday := time.Date(2026, time.November, 8, 18, 0, 0, 0, time.UTC)

	regWeek := db.Week{ID: 1, Season: 2026, SeasonType: regularSeasonType, WeekNum: 9}
	postWeek := db.Week{ID: 2, Season: 2026, SeasonType: postseasonSeasonType, WeekNum: 1}
	preWeek := db.Week{ID: 3, Season: 2026, SeasonType: 1, WeekNum: 1}

	scheduled := []db.Game{{ID: "g1", Kickoff: sunday, Status: db.GameScheduled}}
	allDone := []db.Game{{ID: "g1", Kickoff: sunday, Status: db.GameFinal}}

	tests := []struct {
		name  string
		last  time.Time
		now   time.Time
		week  db.Week
		games []db.Game
		want  bool
	}{
		{
			name: "never synced in regular season",
			now:  friday, week: regWeek, games: scheduled, want: true,
		},
		{
			name: "never synced in postseason",
			now:  friday, week: postWeek, games: scheduled, want: true,
		},
		{
			name: "skips preseason",
			now:  friday, week: preWeek, games: scheduled, want: false,
		},
		{
			name: "idle day fresh under 24h",
			last: friday.Add(-nflverseIdleAge + time.Minute),
			now:  friday, week: regWeek, games: scheduled, want: false,
		},
		{
			name: "idle day stale at 24h",
			last: friday.Add(-nflverseIdleAge),
			now:  friday, week: regWeek, games: scheduled, want: true,
		},
		{
			name: "active Tuesday fresh under 6h",
			last: tuesday.Add(-nflverseActiveAge + time.Minute),
			now:  tuesday, week: regWeek, games: scheduled, want: false,
		},
		{
			name: "active Tuesday stale at 6h",
			last: tuesday.Add(-nflverseActiveAge),
			now:  tuesday, week: regWeek, games: scheduled, want: true,
		},
		{
			name: "all final and synced more than 12h after latest kickoff",
			last: sunday.Add(13 * time.Hour),
			now:  tuesday, week: regWeek, games: allDone, want: false,
		},
		{
			name: "all final and synced before 12h post-kickoff still refreshes",
			last: sunday.Add(2 * time.Hour),
			now:  tuesday, week: regWeek, games: allDone, want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, nflverseStale(tt.last, tt.now, tt.week, tt.games))
		})
	}
}

func TestNFLverseFailureKeepsRowsAndBacksOff(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.November, 8, 13, 0, 0, 0, time.UTC)
	store, week, game := nfeloTestStore(t, now)

	seeded := db.NFLverseTeamWeek{
		Season:     2026,
		SeasonType: nflverse.SeasonTypeRegular,
		WeekNum:    8,
		Team:       "MIA",
		OffPassEPA: 4.5,
		UpdatedAt:  now.Add(-48 * time.Hour),
	}
	_, err := store.UpsertNFLverseTeamWeeks(t.Context(), []db.NFLverseTeamWeek{seeded})
	require.NoError(t, err)
	require.NoError(t, store.RecordSync(t.Context(), NFLverseSource(2026), now.Add(-48*time.Hour), nil))

	clock := &fakeClock{now: now}
	client := &fakeNFLverseClient{err: errors.New("github unavailable")}
	service := New(Deps{
		Store: store, NFLverse: client, Now: clock.Now, Logger: zerolog.Nop(),
	})

	src := service.nflverseSource(week, []db.Game{game}, now)
	service.runSources(t.Context(), clock.Now(), []source{src})
	service.wait()

	// Seeded row and old sync_state remain intact.
	rows, err := store.NFLverseTeamWeeksForSeason(t.Context(), 2026, nflverse.SeasonTypeRegular)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.InDelta(t, 4.5, rows[0].OffPassEPA, 1e-9)

	state, err := store.SyncState(t.Context(), NFLverseSource(2026))
	require.NoError(t, err)
	assert.Equal(t, now.Add(-48*time.Hour), state.FetchedAt)
	assert.Equal(t, 1, client.calls)

	// Immediate retry is suppressed by failure backoff.
	service.runSources(t.Context(), clock.Now(), []source{src})
	service.wait()
	assert.Equal(t, 1, client.calls)

	// After backoff expires, it retries.
	clock.Advance(failureBackoff)
	service.runSources(t.Context(), clock.Now(), []source{src})
	service.wait()
	assert.Equal(t, 2, client.calls)
}

func TestNFLverseEndToEndWithSandboxFixtures(t *testing.T) {
	t.Parallel()

	registry, err := sandbox.Week9(os.DirFS("../.."))
	require.NoError(t, err)

	transport := sandbox.NewTransport(registry, sandbox.Morning)
	httpClient := &http.Client{Transport: transport}

	now := sandbox.MorningTime()
	store, week, _ := nfeloTestStore(t, now)
	service := New(Deps{
		Store:    store,
		NFLverse: nflverse.NewClient("", httpClient),
		Now:      func() time.Time { return now },
		Logger:   zerolog.Nop(),
	})

	changed, err := service.syncNFLverse(t.Context(), week, now)
	require.NoError(t, err)
	assert.True(t, changed)

	teamRows, err := store.NFLverseTeamWeeksForSeason(t.Context(), 2026, nflverse.SeasonTypeRegular)
	require.NoError(t, err)
	assert.NotEmpty(t, teamRows)

	playerRows, err := store.NFLversePlayerWeeksForSeason(t.Context(), 2026, nflverse.SeasonTypeRegular)
	require.NoError(t, err)
	assert.NotEmpty(t, playerRows)

	state, err := store.SyncState(t.Context(), NFLverseSource(2026))
	require.NoError(t, err)
	assert.Equal(t, now, state.FetchedAt)
}
