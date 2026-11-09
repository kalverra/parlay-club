package web_test

import (
	"fmt"
	"html"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/db"
)

// TestSlipPanelBannerNamesBothOwners gives one person the spread and another
// the passing over so the banner has to name both.
func TestSlipPanelBannerNamesBothOwners(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	game := addGame(t, fixture, gameKickoff())
	sam, err := fixture.store.CreateUserWithIdentity(
		t.Context(), "Sam", "sam@example.com", "google-sam", "")
	require.NoError(t, err)
	require.NoError(t, fixture.store.AddSlipMember(t.Context(), fixture.slip.ID, sam.ID))

	addStoredLeg(t, fixture, db.Leg{
		UserID:     fixture.user.ID,
		GameID:     new(game.ID),
		MarketType: "SPREAD",
		Selection:  "BUF -7.5",
		Team:       new("BUF"),
		Side:       new("TEAM"),
		Line:       new(-7.5),
		Odds:       -110,
	})
	addStoredLeg(t, fixture, db.Leg{
		UserID:     sam.ID,
		GameID:     new(game.ID),
		MarketType: "PASS_YDS",
		Selection:  "Josh Allen Over 245.5 Pass Yds",
		Team:       new("BUF"),
		PlayerName: new("Josh Allen"),
		Side:       new("OVER"),
		Line:       new(245.5),
		Odds:       -110,
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		fmt.Sprintf("/slips/%d/panel", fixture.slip.ID),
		nil,
	)
	fixture.handler.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	body := recorder.Body.String()
	text := html.UnescapeString(body)
	assert.Contains(t, body, `data-rule="blowout_passing"`)
	assert.Contains(t, body, `data-rule="hook"`)
	assert.Contains(t, text, "Ada's BUF -7.5")
	assert.Contains(t, text, "Sam's Josh Allen Over 245.5 Pass Yds")
	assert.Contains(t, text, "Ada · BUF -7.5", "each card lists the legs with their owners")
	assert.Contains(t, text, "Sam · Josh Allen Over 245.5 Pass Yds")
	assert.Less(t, strings.Index(body, `data-rule="blowout_passing"`),
		strings.Index(body, `data-rule="hook"`), "the stable rule order keeps the spread warning first")
	assert.NotContains(t, body, "Looks clean")
}

func TestSlipPanelBannerLooksCleanWithNoWarnings(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		fmt.Sprintf("/slips/%d/panel", fixture.slip.ID),
		nil,
	)
	fixture.handler.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	body := recorder.Body.String()
	assert.Contains(t, body, "Looks clean")
	assert.NotContains(t, body, "data-rule=")
}

// TestSlipPanelUsesNfeloOnlyWhenFreshAndScheduled checks the two guards that
// keep an old model or a game already underway out of the banner and the leg
// row. The slip is the week-9 style pair: BUF -7.5 and Josh Allen's passing
// over.
func TestSlipPanelUsesNfeloOnlyWhenFreshAndScheduled(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		fetchedAt time.Time
		status    db.GameStatus
		wantModel bool
	}{
		{name: "fresh and scheduled", fetchedAt: time.Now().UTC(), status: db.GameScheduled, wantModel: true},
		{name: "four days old", fetchedAt: time.Now().UTC().Add(-96 * time.Hour), status: db.GameScheduled},
		{name: "live game", fetchedAt: time.Now().UTC(), status: db.GameLive},
		{name: "final game", fetchedAt: time.Now().UTC(), status: db.GameFinal},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			fixture := newSlipFixture(t, 2)
			game := addGame(t, fixture, gameKickoff())
			if test.status != db.GameScheduled {
				game.Status = test.status
				updated, err := fixture.store.UpsertGame(t.Context(), game)
				require.NoError(t, err)
				game = updated
			}

			changed, err := fixture.store.UpsertNfeloGames(t.Context(), []db.NfeloGame{{
				GameID:      game.ID,
				HomeSpread:  -10,
				HomeWinProb: 0.76,
				HomeQB:      "Josh Allen",
				AwayQB:      "Tua Tagovailoa",
				HomeQBAdj:   5,
				AwayQBAdj:   -5,
				UpdatedAt:   test.fetchedAt,
			}})
			require.NoError(t, err)
			require.True(t, changed)
			require.NoError(t, fixture.store.RecordSync(
				t.Context(), "nfelo:projections", test.fetchedAt, nil))

			addStoredLeg(t, fixture, db.Leg{
				UserID:     fixture.user.ID,
				GameID:     new(game.ID),
				MarketType: "SPREAD",
				Selection:  "BUF -7.5",
				Team:       new("BUF"),
				Side:       new("TEAM"),
				Line:       new(-7.5),
				Odds:       -110,
			})
			addStoredLeg(t, fixture, db.Leg{
				UserID:     fixture.user.ID,
				GameID:     new(game.ID),
				MarketType: "PASS_YDS",
				Selection:  "Josh Allen Over 245.5 Pass Yds",
				Team:       new("BUF"),
				PlayerName: new("Josh Allen"),
				Side:       new("OVER"),
				Line:       new(245.5),
				Odds:       -110,
			})

			recorder := httptest.NewRecorder()
			request := httptest.NewRequestWithContext(
				t.Context(),
				http.MethodGet,
				fmt.Sprintf("/slips/%d/panel", fixture.slip.ID),
				nil,
			)
			fixture.handler.ServeHTTP(recorder, request)

			require.Equal(t, http.StatusOK, recorder.Code)
			body := html.UnescapeString(recorder.Body.String())
			require.Contains(t, body, `data-rule="hook"`, "the hook still fires without a model")
			require.Contains(t, body, `data-rule="blowout_passing"`)

			if test.wantModel {
				assert.Contains(t, body, "nfelo has it nearer -10, so the hook matters less.")
				assert.Contains(t, body, "nfelo -10.0", "the spread leg carries the model chip")
				return
			}
			assert.NotContains(t, body, "nfelo has it nearer")
			assert.NotContains(t, body, "nfelo has it closer to")
			assert.NotContains(t, body, "nfelo sees it near the number too")
			assert.NotContains(t, body, "nfelo -10.0", "a stale or live game shows no chip")
		})
	}
}

// TestSlipLegRowShowsModelOnlyForSpreadAndMoneyline covers the chip's two
// formats and its quiet cases. The chip shares the row's wrapping flex line
// with the odds, so at 360 px it moves under the price instead of widening the
// card; no rule here adds a fixed width.
func TestSlipLegRowShowsModelOnlyForSpreadAndMoneyline(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 3)
	game := addGame(t, fixture, gameKickoff())
	changed, err := fixture.store.UpsertNfeloGames(t.Context(), []db.NfeloGame{{
		GameID:      game.ID,
		HomeSpread:  -10,
		HomeWinProb: 0.76,
		HomeQB:      "Josh Allen",
		AwayQB:      "Tua Tagovailoa",
		HomeQBAdj:   5,
		AwayQBAdj:   -5,
		UpdatedAt:   time.Now().UTC(),
	}})
	require.NoError(t, err)
	require.True(t, changed)
	require.NoError(t, fixture.store.RecordSync(
		t.Context(), "nfelo:projections", time.Now().UTC(), nil))

	addStoredLeg(t, fixture, db.Leg{
		UserID:     fixture.user.ID,
		GameID:     new(game.ID),
		MarketType: "ML",
		Selection:  "BUF ML",
		Team:       new("BUF"),
		Side:       new("TEAM"),
		Odds:       -110,
	})
	addStoredLeg(t, fixture, db.Leg{
		UserID:     fixture.user.ID,
		GameID:     new(game.ID),
		MarketType: "SPREAD",
		Selection:  "MIA +7.5",
		Team:       new("MIA"),
		Side:       new("TEAM"),
		Line:       new(7.5),
		Odds:       -110,
	})
	addStoredLeg(t, fixture, db.Leg{
		UserID:     fixture.user.ID,
		GameID:     new(game.ID),
		MarketType: "TOTAL",
		Selection:  "BUF@MIA Over 47.5",
		Side:       new("OVER"),
		Line:       new(47.5),
		Odds:       -110,
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		fmt.Sprintf("/slips/%d/panel", fixture.slip.ID),
		nil,
	)
	fixture.handler.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	body := html.UnescapeString(recorder.Body.String())
	assert.Contains(t, body, "nfelo 76%", "a moneyline shows the model's win probability")
	assert.Contains(t, body, "nfelo +10.0", "an away spread flips to the away team's side")
	assert.Equal(t, 2, strings.Count(body, "nfelo "), "a total gets no chip")
	assert.Contains(t, body, `class="mt-1 flex flex-wrap items-center gap-2 text-xs text-slate-400"`,
		"the chip shares the wrapping row beside the odds")
}

// TestSlipPanelBannerShowsMedianTrap verifies that stored player_gamelogs
// feed the analyzer and render the median_trap warning card on the slip panel.
func TestSlipPanelBannerShowsMedianTrap(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	game := addGame(t, fixture, gameKickoff())

	err := fixture.store.UpsertPlayerGameLog(t.Context(), db.PlayerGameLog{
		AthleteID:   "4430755",
		GamesPlayed: 5,
		StatsJSON:   `{"REC_YDS":[41.0, 41.0, 41.0, 41.0, 126.0]}`,
		UpdatedAt:   time.Now().UTC(),
	})
	require.NoError(t, err)

	addStoredLeg(t, fixture, db.Leg{
		UserID:     fixture.user.ID,
		GameID:     new(game.ID),
		MarketType: "REC_YDS",
		Selection:  "Khalil Shakir Over 52.5 Rec Yds",
		Team:       new("BUF"),
		PlayerID:   new("4430755"),
		PlayerName: new("Khalil Shakir"),
		Side:       new("OVER"),
		Line:       new(52.5),
		Odds:       -110,
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		fmt.Sprintf("/slips/%d/panel", fixture.slip.ID),
		nil,
	)
	fixture.handler.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	body := recorder.Body.String()
	text := html.UnescapeString(body)

	assert.Contains(t, body, `data-rule="median_trap"`)
	assert.Contains(t, text, "Khalil Shakir's last-5 median is 41 yards, so 52.5 is a stretch.")
	assert.Contains(t, text, "His 58-yard average is propped up by one big game.")
	assert.Contains(t, text, "Ada · Khalil Shakir Over 52.5 Rec Yds")
}

// TestSlipPanelBannerMedianTrapQuietCases checks that median_trap stays quiet
// when the line is safe, the player is unmatched, or no gamelogs exist.
func TestSlipPanelBannerMedianTrapQuietCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		playerID  *string
		hasLog    bool
		line      float64
		statsJSON string
	}{
		{
			name:      "line at or below median",
			playerID:  new("4430755"),
			hasLog:    true,
			line:      40.5,
			statsJSON: `{"REC_YDS":[41.0, 41.0, 41.0, 41.0, 126.0]}`,
		},
		{
			name:      "unmatched player",
			playerID:  nil,
			hasLog:    false,
			line:      52.5,
			statsJSON: ``,
		},
		{
			name:      "no gamelog in store",
			playerID:  new("9999999"),
			hasLog:    false,
			line:      52.5,
			statsJSON: ``,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			fixture := newSlipFixture(t, 2)
			game := addGame(t, fixture, gameKickoff())

			if test.hasLog && test.playerID != nil {
				err := fixture.store.UpsertPlayerGameLog(t.Context(), db.PlayerGameLog{
					AthleteID:   *test.playerID,
					GamesPlayed: 5,
					StatsJSON:   test.statsJSON,
					UpdatedAt:   time.Now().UTC(),
				})
				require.NoError(t, err)
			}

			addStoredLeg(t, fixture, db.Leg{
				UserID:     fixture.user.ID,
				GameID:     new(game.ID),
				MarketType: "REC_YDS",
				Selection:  "Khalil Shakir Over Rec Yds",
				Team:       new("BUF"),
				PlayerID:   test.playerID,
				PlayerName: new("Khalil Shakir"),
				Side:       new("OVER"),
				Line:       new(test.line),
				Odds:       -110,
			})

			recorder := httptest.NewRecorder()
			request := httptest.NewRequestWithContext(
				t.Context(),
				http.MethodGet,
				fmt.Sprintf("/slips/%d/panel", fixture.slip.ID),
				nil,
			)
			fixture.handler.ServeHTTP(recorder, request)

			require.Equal(t, http.StatusOK, recorder.Code)
			body := recorder.Body.String()
			assert.NotContains(t, body, `data-rule="median_trap"`)
		})
	}
}

// addStoredLeg writes one leg straight through the store so a test can give
// legs to more than one person without switching the signed-in user.
func addStoredLeg(t *testing.T, fixture *slipFixture, leg db.Leg) db.Leg {
	t.Helper()

	leg.SlipID = fixture.slip.ID
	created, err := fixture.store.CreateLeg(t.Context(), leg)
	require.NoError(t, err)
	return created
}
