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

	"github.com/kalverra/parlay-club/internal/analyzer"
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
func TestNfeloFreshnessUsesTheScenarioClock(t *testing.T) {
	t.Parallel()

	fetchedAt := time.Now().UTC().Add(-96 * time.Hour)
	scenarioNow := fetchedAt.Add(time.Hour)
	fixture := newSlipFixtureAt(t, 2, func() time.Time { return scenarioNow })
	game := addGame(t, fixture, gameKickoff())

	changed, err := fixture.store.UpsertNfeloGames(t.Context(), []db.NfeloGame{{
		GameID:      game.ID,
		HomeSpread:  -10,
		HomeWinProb: 0.76,
		UpdatedAt:   fetchedAt,
	}})
	require.NoError(t, err)
	require.True(t, changed)
	require.NoError(t, fixture.store.RecordSync(
		t.Context(), "nfelo:projections", fetchedAt, nil))

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

	recorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		fmt.Sprintf("/slips/%d/panel", fixture.slip.ID),
		nil,
	)
	fixture.handler.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "nfelo -10.0",
		"a model that is fresh at scenario time still shows")
}

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
	assert.Contains(t, body, `class="mt-1 flex flex-wrap items-center gap-2 text-xs text-ink-muted"`,
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

func TestAnalyzerGuideRendersAllRulesAndAnchors(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		"/analyzer/guide?from=/slips/42",
		nil,
	)
	fixture.handler.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	body := recorder.Body.String()
	assert.Contains(t, body, `href="/slips/42"`)
	assert.Contains(t, body, `id="leg-grades"`)
	assert.Contains(t, body, `id="slip-grade"`)
	assert.Contains(t, body, `id="suggestions"`)
	assert.Contains(t, body, "Grade A")
	assert.Contains(t, body, "Grade F")
	assert.Contains(t, body, "grade-a")
	assert.Contains(t, body, "grade-b")
	assert.Contains(t, body, "grade-c")
	assert.Contains(t, body, "grade-d")
	assert.Contains(t, body, "grade-f")
	assert.Contains(t, body, "sev-warn")
	for _, ruleID := range analyzer.RuleIDs() {
		assert.Contains(t, body, fmt.Sprintf(`id="rule-%s"`, ruleID))
	}
}

func TestAnalyzerGuideRejectsUnsafeBackURL(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)

	for _, raw := range []string{"//evil.com/phish", "https://evil.com/phish", `/\evil.com`} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequestWithContext(
			t.Context(),
			http.MethodGet,
			"/analyzer/guide?from="+raw,
			nil,
		)
		fixture.handler.ServeHTTP(recorder, request)

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.NotContains(t, recorder.Body.String(), raw)
	}
}

func TestSlipPanelAndTrayWireNFLverseGradesAndRuleLinks(t *testing.T) {
	t.Parallel()

	seedNFLverse := func(t *testing.T, fixture *slipFixture, syncedAt time.Time) {
		t.Helper()
		game := addGame(t, fixture, gameKickoff())

		require.NoError(t, fixture.store.RecordSync(t.Context(), "nflverse:season:2026", syncedAt, nil))

		// Make MIA rank #1 toughest pass defense (lowest DefPassEPAAllowed) across 6 teams.
		teamWeeks := []db.NFLverseTeamWeek{
			{
				Season:            2026,
				SeasonType:        2,
				WeekNum:           1,
				Team:              "MIA",
				DefPassEPAAllowed: -0.25,
				PlaysPerGame:      62,
				PassRate:          0.55,
				UpdatedAt:         syncedAt,
			},
			{
				Season:            2026,
				SeasonType:        2,
				WeekNum:           1,
				Team:              "BUF",
				DefPassEPAAllowed: 0.02,
				PlaysPerGame:      64,
				PassRate:          0.58,
				UpdatedAt:         syncedAt,
			},
			{
				Season:            2026,
				SeasonType:        2,
				WeekNum:           1,
				Team:              "KC",
				DefPassEPAAllowed: 0.05,
				PlaysPerGame:      65,
				PassRate:          0.60,
				UpdatedAt:         syncedAt,
			},
			{
				Season:            2026,
				SeasonType:        2,
				WeekNum:           1,
				Team:              "BAL",
				DefPassEPAAllowed: 0.08,
				PlaysPerGame:      63,
				PassRate:          0.50,
				UpdatedAt:         syncedAt,
			},
			{
				Season:            2026,
				SeasonType:        2,
				WeekNum:           1,
				Team:              "SF",
				DefPassEPAAllowed: 0.12,
				PlaysPerGame:      61,
				PassRate:          0.52,
				UpdatedAt:         syncedAt,
			},
			{
				Season:            2026,
				SeasonType:        2,
				WeekNum:           1,
				Team:              "NYJ",
				DefPassEPAAllowed: 0.18,
				PlaysPerGame:      59,
				PassRate:          0.54,
				UpdatedAt:         syncedAt,
			},
		}
		_, err := fixture.store.UpsertNFLverseTeamWeeks(t.Context(), teamWeeks)
		require.NoError(t, err)

		// Player with 4 weeks where last-2 snap share dropped >= 15 pp below season average.
		playerWeeks := []db.NFLversePlayerWeek{
			{
				ESPNID:      "4362628",
				Season:      2026,
				SeasonType:  2,
				WeekNum:     1,
				Team:        "BUF",
				OffensePct:  0.90,
				TargetShare: 0.25,
				UpdatedAt:   syncedAt,
			},
			{
				ESPNID:      "4362628",
				Season:      2026,
				SeasonType:  2,
				WeekNum:     2,
				Team:        "BUF",
				OffensePct:  0.90,
				TargetShare: 0.25,
				UpdatedAt:   syncedAt,
			},
			{
				ESPNID:      "4362628",
				Season:      2026,
				SeasonType:  2,
				WeekNum:     3,
				Team:        "BUF",
				OffensePct:  0.50,
				TargetShare: 0.15,
				UpdatedAt:   syncedAt,
			},
			{
				ESPNID:      "4362628",
				Season:      2026,
				SeasonType:  2,
				WeekNum:     4,
				Team:        "BUF",
				OffensePct:  0.50,
				TargetShare: 0.15,
				UpdatedAt:   syncedAt,
			},
		}
		_, err = fixture.store.UpsertNFLversePlayerWeeks(t.Context(), playerWeeks)
		require.NoError(t, err)

		addStoredLeg(t, fixture, db.Leg{
			UserID:     fixture.user.ID,
			GameID:     new(game.ID),
			MarketType: "REC_YDS",
			Selection:  "Khalil Shakir Over 55.5 Rec Yds",
			Team:       new("BUF"),
			PlayerID:   new("4362628"),
			PlayerName: new("Khalil Shakir"),
			Side:       new("OVER"),
			Line:       new(55.5),
			Odds:       -110,
		})
	}

	t.Run("fresh nflverse triggers rules, grades, and guide links", func(t *testing.T) {
		t.Parallel()

		fixture := newSlipFixture(t, 2)
		seedNFLverse(t, fixture, time.Now().UTC())

		panelRec := httptest.NewRecorder()
		panelReq := httptest.NewRequestWithContext(
			t.Context(),
			http.MethodGet,
			fmt.Sprintf("/slips/%d/panel", fixture.slip.ID),
			nil,
		)
		fixture.handler.ServeHTTP(panelRec, panelReq)
		require.Equal(t, http.StatusOK, panelRec.Code)

		panelBody := panelRec.Body.String()
		assert.Contains(t, panelBody, `data-rule="tough_matchup"`)
		assert.Contains(t, panelBody, `data-rule="shrinking_role"`)
		assert.Contains(t, panelBody, fmt.Sprintf(`/analyzer/guide?from=/slips/%d#rule-tough_matchup`, fixture.slip.ID))
		assert.Contains(
			t,
			panelBody,
			fmt.Sprintf(`/analyzer/guide?from=/slips/%d#rule-shrinking_role`, fixture.slip.ID),
		)
		assert.Contains(t, panelBody, fmt.Sprintf(`/analyzer/guide?from=/slips/%d"`, fixture.slip.ID))
		assert.Contains(t, panelBody, `data-slip-grade`)
		assert.Contains(t, panelBody, `data-leg-grade`)
		assert.Contains(t, panelBody, fmt.Sprintf(`/analyzer/guide?from=/slips/%d#leg-grades`, fixture.slip.ID))
		assert.Contains(t, panelBody, fmt.Sprintf(`/analyzer/guide?from=/slips/%d#slip-grade`, fixture.slip.ID))

		trayRec := httptest.NewRecorder()
		trayReq := httptest.NewRequestWithContext(
			t.Context(),
			http.MethodGet,
			fmt.Sprintf("/slips/%d/tray", fixture.slip.ID),
			nil,
		)
		fixture.handler.ServeHTTP(trayRec, trayReq)
		require.Equal(t, http.StatusOK, trayRec.Code)

		trayBody := trayRec.Body.String()
		assert.Contains(t, trayBody, `data-leg-grade`)
		assert.Contains(t, trayBody, `data-slip-grade`)
		assert.Contains(
			t,
			trayBody,
			fmt.Sprintf(`/analyzer/guide?from=/slips/%d/pick#rule-tough_matchup`, fixture.slip.ID),
		)
		assert.Contains(t, trayBody, fmt.Sprintf(`/analyzer/guide?from=/slips/%d/pick#leg-grades`, fixture.slip.ID))
	})

	t.Run("stale nflverse fails soft without rules", func(t *testing.T) {
		t.Parallel()

		fixture := newSlipFixture(t, 2)
		seedNFLverse(t, fixture, time.Now().UTC().Add(-8*24*time.Hour))

		panelRec := httptest.NewRecorder()
		panelReq := httptest.NewRequestWithContext(
			t.Context(),
			http.MethodGet,
			fmt.Sprintf("/slips/%d/panel", fixture.slip.ID),
			nil,
		)
		fixture.handler.ServeHTTP(panelRec, panelReq)
		require.Equal(t, http.StatusOK, panelRec.Code)

		panelBody := panelRec.Body.String()
		assert.NotContains(t, panelBody, `data-rule="tough_matchup"`)
		assert.NotContains(t, panelBody, `data-rule="shrinking_role"`)
		assert.Contains(t, panelBody, `data-slip-grade`)
		assert.Contains(t, panelBody, `data-leg-grade`)
	})
}

func TestSlipPanelTrayAndGuideRenderSharpFactors(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	game := addGame(t, fixture, gameKickoff())
	now := time.Now().UTC()

	_, err := fixture.store.UpsertOdds(t.Context(), []db.Odds{
		{
			GameID:     game.ID,
			Book:       "lowvig",
			MarketType: "SPREAD",
			Team:       "BUF",
			Side:       "TEAM",
			Line:       new(-3.5),
			Price:      -110,
			FetchedAt:  now,
		},
		{
			GameID:     game.ID,
			Book:       "lowvig",
			MarketType: "SPREAD",
			Team:       "MIA",
			Side:       "TEAM",
			Line:       new(3.5),
			Price:      -110,
			FetchedAt:  now,
		},
		{
			GameID:     game.ID,
			Book:       "pinnacle",
			MarketType: "PASS_YDS",
			Team:       "BUF",
			PlayerName: "Josh Allen",
			Side:       "OVER",
			Line:       new(265.5),
			Price:      -120,
			FetchedAt:  now,
		},
		{
			GameID:     game.ID,
			Book:       "pinnacle",
			MarketType: "PASS_YDS",
			Team:       "BUF",
			PlayerName: "Josh Allen",
			Side:       "UNDER",
			Line:       new(265.5),
			Price:      100,
			FetchedAt:  now,
		},
	})
	require.NoError(t, err)

	addStoredLeg(t, fixture, db.Leg{
		UserID:     fixture.user.ID,
		GameID:     new(game.ID),
		MarketType: "SPREAD",
		Selection:  "BUF -2.5",
		Team:       new("BUF"),
		Side:       new("TEAM"),
		Line:       new(-2.5),
		Odds:       -110,
	})
	addStoredLeg(t, fixture, db.Leg{
		UserID:     fixture.user.ID,
		GameID:     new(game.ID),
		MarketType: "PASS_YDS",
		Selection:  "Josh Allen Over 265.5 Pass Yds",
		Team:       new("BUF"),
		PlayerName: new("Josh Allen"),
		Side:       new("OVER"),
		Line:       new(265.5),
		Odds:       -135,
	})

	panelRec := httptest.NewRecorder()
	panelReq := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		fmt.Sprintf("/slips/%d/panel", fixture.slip.ID),
		nil,
	)
	fixture.handler.ServeHTTP(panelRec, panelReq)
	require.Equal(t, http.StatusOK, panelRec.Code)
	panelBody := html.UnescapeString(panelRec.Body.String())
	assert.Contains(t, panelBody, "Sharp line edge")
	assert.Contains(t, panelBody, "+8")
	assert.Contains(t, panelBody, "Steep price vs. sharp")
	assert.Contains(t, panelBody, "-6")

	trayRec := httptest.NewRecorder()
	trayReq := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		fmt.Sprintf("/slips/%d/tray", fixture.slip.ID),
		nil,
	)
	fixture.handler.ServeHTTP(trayRec, trayReq)
	require.Equal(t, http.StatusOK, trayRec.Code)
	trayBody := html.UnescapeString(trayRec.Body.String())
	assert.Contains(t, trayBody, "Sharp line edge")
	assert.Contains(t, trayBody, "+8")
	assert.Contains(t, trayBody, "Steep price vs. sharp")
	assert.Contains(t, trayBody, "-6")

	guideRec := httptest.NewRecorder()
	guideReq := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		"/analyzer/guide",
		nil,
	)
	fixture.handler.ServeHTTP(guideRec, guideReq)
	require.Equal(t, http.StatusOK, guideRec.Code)
	guideBody := html.UnescapeString(guideRec.Body.String())
	for _, label := range []string{
		"Sharp line edge",
		"Sharp line fade",
		"Sharp price edge",
		"Steep price vs. sharp",
		"+8 (+5 consensus)",
		"-8 (-5 consensus)",
		"+6 (+4 consensus)",
		"-6 (-4 consensus)",
	} {
		assert.Contains(t, guideBody, label)
	}
}
