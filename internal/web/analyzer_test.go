package web_test

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/analyzer"
	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/live"
	"github.com/kalverra/parlay-club/internal/slips"
	webui "github.com/kalverra/parlay-club/internal/web"
	assets "github.com/kalverra/parlay-club/web"
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
	assert.Contains(t, body, `id="pillars"`)
	assert.Contains(t, body, `id="leg-grades"`)
	assert.Contains(t, body, `id="slip-grade"`)
	assert.Contains(t, body, `id="suggestions"`)
	assert.Contains(t, body, `data-guide-pillar="price_vig"`)
	assert.Contains(t, body, `data-guide-pillar="script_fit"`)
	assert.Contains(t, body, `data-guide-pillar="stability_risk"`)
	assert.Contains(t, body, `data-guide-pillar="context_traps"`)
	assert.Contains(t, body, `data-guide-uncertainty`)
	assert.Contains(t, body, "Active engine: v2")
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

// TestDocsGradingServesGuideUnderTheDocsTab covers both URLs that land on the
// guide. The docs alias and the legacy analyzer path must render the same
// rubric, mark the Docs tab, and highlight the Grading & Rules switcher while
// forwarding a safe back link.
func TestDocsGradingServesGuideUnderTheDocsTab(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)

	for _, path := range []string{"/docs/grading?from=/slips/42", "/analyzer/guide?from=/slips/42"} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil)
		fixture.handler.ServeHTTP(recorder, request)

		require.Equal(t, http.StatusOK, recorder.Code, path)
		body := recorder.Body.String()

		assert.Regexp(t, `<a\s+href="/docs"\s+class="flex flex-col[^>]*aria-current="page"`, body, path)
		assert.Contains(t, body, `aria-current="page">Grading &amp; Rules</a`, path)
		assert.Contains(t, body, `href="/slips/42"`, path)
		assert.Contains(t, body, `href="/docs?from=%2Fslips%2F42"`, path)
		assert.Contains(t, body, `href="/docs/glossary?from=%2Fslips%2F42"`, path)
		assert.Contains(t, body, `href="/docs/evals?from=%2Fslips%2F42"`, path)
		assert.Contains(t, body, `id="leg-grades"`, path)
		assert.Contains(t, body, `id="slip-grade"`, path)
		for _, ruleID := range analyzer.RuleIDs() {
			assert.Contains(t, body, fmt.Sprintf(`id="rule-%s"`, ruleID), path)
		}
	}
}

// TestDocsGradingStripsUnsafeBackURL keeps the docs alias on the same
// open-redirect guard as the analyzer path.
func TestDocsGradingStripsUnsafeBackURL(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		"/docs/grading?from=https://evil.example/phish",
		nil,
	)
	fixture.handler.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	body := recorder.Body.String()
	assert.NotContains(t, body, "evil.example")
	assert.NotContains(t, body, "&larr; Back")
	assert.NotContains(t, body, "from=", "no switcher link forwards a stripped back link")
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
		assert.Contains(t, panelBody, fmt.Sprintf(`/docs/grading?from=/slips/%d#rule-tough_matchup`, fixture.slip.ID))
		assert.Contains(
			t,
			panelBody,
			fmt.Sprintf(`/docs/grading?from=/slips/%d#rule-shrinking_role`, fixture.slip.ID),
		)
		assert.Contains(t, panelBody, fmt.Sprintf(`/docs/grading?from=/slips/%d"`, fixture.slip.ID))
		assert.Contains(t, panelBody, `data-slip-grade`)
		assert.Contains(t, panelBody, `data-leg-grade`)
		assert.Contains(t, panelBody, fmt.Sprintf(`/docs/grading?from=/slips/%d#leg-grades`, fixture.slip.ID))
		assert.Contains(t, panelBody, fmt.Sprintf(`/docs/grading?from=/slips/%d#slip-grade`, fixture.slip.ID))

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
			fmt.Sprintf(`/docs/grading?from=/slips/%d/pick#rule-tough_matchup`, fixture.slip.ID),
		)
		assert.Contains(t, trayBody, fmt.Sprintf(`/docs/grading?from=/slips/%d/pick#leg-grades`, fixture.slip.ID))
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
	assert.Contains(t, panelBody, "data-grade-pillars")
	assert.Contains(t, panelBody, "Price & Vig")
	assert.Contains(t, panelBody, "Getting BUF -2.5 while LowVig sits at -3.5.")
	assert.Contains(t, panelBody, "Paying -135 vs. fair price")

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
	assert.Contains(t, trayBody, "data-grade-pillars")
	assert.Contains(t, trayBody, "Price & Vig")
	assert.Contains(t, trayBody, "Getting BUF -2.5 while LowVig sits at -3.5.")
	assert.Contains(t, trayBody, "Paying -135 vs. fair price")

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

func TestGradeBreakdownsRenderFourPillarsAndV1FallbackTemplates(t *testing.T) {
	t.Parallel()

	renderer, err := webui.NewRenderer(assets.Templates, false)
	require.NoError(t, err)

	v2LegGrade := webui.LegGradeView(analyzer.LegGrade{
		LegID:          11,
		Letter:         "B",
		Score:          77,
		CalibratedProb: 0.54,
		Uncertain:      true,
		Pillars: []analyzer.PillarScore{
			{
				Key:     analyzer.PillarPriceVig,
				Label:   analyzer.PillarLabelPriceVig,
				Score:   86,
				Tone:    analyzer.ToneGood,
				Summary: "Getting +105 vs. fair price near -108.",
				Reasons: []string{"Getting +105 vs. fair price near -108."},
			},
			{
				Key:     analyzer.PillarScriptFit,
				Label:   analyzer.PillarLabelScriptFit,
				Score:   75,
				Tone:    analyzer.ToneNeutral,
				Summary: "Neutral single-leg game script.",
				Reasons: []string{"Neutral single-leg game script."},
			},
			{
				Key:     analyzer.PillarStability,
				Label:   analyzer.PillarLabelStability,
				Score:   66,
				Tone:    analyzer.ToneWarn,
				Summary: "Moderate week-to-week stat volatility.",
				Reasons: []string{"Moderate week-to-week stat volatility."},
			},
			{
				Key:     analyzer.PillarContextTraps,
				Label:   analyzer.PillarLabelContextTraps,
				Score:   52,
				Tone:    analyzer.ToneBad,
				Summary: "Player is listed Questionable on the injury report.",
				Reasons: []string{
					"Player is listed Questionable on the injury report.",
					"Model and market see this differently — check role news.",
				},
			},
		},
	})

	v2SlipGrade := webui.SlipGradeView(analyzer.SlipGrade{
		Letter:         "B",
		Score:          79,
		Summary:        "Strong 2-leg correlated ticket",
		CalibratedProb: 0.31,
		Uncertain:      true,
		CorrMultiplier: 1.16,
		LegGrades:      []analyzer.LegGrade{{LegID: 11, Letter: "B"}, {LegID: 12, Letter: "A"}},
		Pillars: []analyzer.PillarScore{
			{
				Key:     analyzer.PillarPriceVig,
				Label:   analyzer.PillarLabelPriceVig,
				Score:   68,
				Tone:    analyzer.ToneNeutral,
				Summary: "Combined payout +260 sits close to fair ticket odds near +255.",
				Reasons: []string{
					"Combined payout +260 sits close to fair ticket odds near +255.",
					"This stack fits the script, but steep leg prices tax the correlation.",
				},
			},
			{
				Key:     analyzer.PillarScriptFit,
				Label:   analyzer.PillarLabelScriptFit,
				Score:   92,
				Tone:    analyzer.ToneGood,
				Summary: "Josh Allen Pass Yds and Khalil Shakir Rec Yds rise together.",
				Reasons: []string{"Josh Allen Pass Yds and Khalil Shakir Rec Yds rise together."},
			},
			{
				Key:     analyzer.PillarStability,
				Label:   analyzer.PillarLabelStability,
				Score:   76,
				Tone:    analyzer.ToneNeutral,
				Summary: "Balanced leg stability across the ticket.",
				Reasons: []string{"Balanced leg stability across the ticket."},
			},
			{
				Key:     analyzer.PillarContextTraps,
				Label:   analyzer.PillarLabelContextTraps,
				Score:   55,
				Tone:    analyzer.ToneBad,
				Summary: "Khalil Shakir is listed Questionable.",
				Reasons: []string{"Khalil Shakir is listed Questionable."},
			},
		},
	})

	t.Run("v2 four pillars, why bullets, uncertainty badge, and corr summary", func(t *testing.T) {
		t.Parallel()

		panelHTML, err := renderer.RenderPartial("slip-panel", webui.SlipPage{
			Slip: &webui.SlipView{
				ID:     7,
				Name:   "Sunday Ticket",
				Status: "OPEN",
				Members: []webui.SlipMemberView{{
					User: webui.User{Name: "Ada"},
					Legs: []webui.LegView{{
						ID:        11,
						Selection: "Khalil Shakir Over 52.5 Rec Yds",
						Odds:      "+105",
						Grade:     v2LegGrade,
					}},
				}},
				Analyzer: webui.AnalyzerView{
					Clean: true,
					Grade: v2SlipGrade,
				},
			},
		})
		require.NoError(t, err)

		trayHTML, err := renderer.RenderPartial("pick-tray", webui.PickDockView{
			SlipID: 7,
			Picked: 1,
			Limit:  2,
			Legs: []webui.TrayLegView{{
				ID:        11,
				Selection: "Khalil Shakir Over 52.5 Rec Yds",
				Odds:      "+105",
				Grade:     v2LegGrade,
			}},
			Grade: v2SlipGrade,
		})
		require.NoError(t, err)

		highlightsHTML, err := renderer.RenderPartial("slip-panel", webui.SlipPage{
			Slip: &webui.SlipView{
				ID:     7,
				Name:   "Sunday Ticket",
				Status: "OPEN",
			},
			Highlights: webui.HighlightsView{
				Has:    true,
				SlipID: 7,
				Legs: []webui.LegHighlightView{{
					Text:  "Khalil Shakir Over 52.5 Rec Yds",
					Price: "+105",
					Odds:  105,
					Grade: v2LegGrade,
				}},
				Parlays: []webui.ParlayHighlightView{{
					Label:        "2-leg BUF stack",
					CombinedOdds: "+260",
					LegCount:     2,
					Grade:        v2SlipGrade,
				}},
			},
		})
		require.NoError(t, err)
		assert.Contains(t, string(highlightsHTML), "data-highlight-leg")
		assert.Contains(t, string(highlightsHTML), "data-highlight-parlay")

		for name, raw := range map[string]string{
			"slip-panel":       string(panelHTML),
			"pick-tray":        string(trayHTML),
			"highlights-strip": string(highlightsHTML),
		} {
			body := html.UnescapeString(raw)
			assert.Contains(t, body, "data-grade-summary", name)
			assert.Contains(t, body, "Overall", name)
			assert.Contains(t, body, "Hit chance", name)
			assert.Contains(t, body, "Payout value", name)
			assert.Contains(t, body, "~54%", name)
			assert.Contains(t, body, "~31%", name)
			assert.Contains(t, body, "Strong value", name)
			assert.Contains(t, body, "Slight juice", name)
			assert.NotContains(t, body, "+EV", name)
			assert.Contains(t, body, "data-grade-pillars", name)
			assert.Contains(t, body, `data-pillar="price_vig"`, name)
			assert.Contains(t, body, `data-pillar="script_fit"`, name)
			assert.Contains(t, body, `data-pillar="stability_risk"`, name)
			assert.Contains(t, body, `data-pillar="context_traps"`, name)
			assert.Contains(t, body, "Price & Vig", name)
			assert.Contains(t, body, "Script Fit", name)
			assert.Contains(t, body, "Stability & Tail Risk", name)
			assert.Contains(t, body, "Context & Traps", name)
			assert.Contains(t, body, "bg-positive", name)
			assert.Contains(t, body, "bg-warning", name)
			assert.Contains(t, body, "bg-danger", name)
			assert.Contains(t, body, "bg-ink-subtle", name)
			assert.Contains(t, body, "data-grade-why", name)
			assert.Contains(t, body, "data-uncertainty-badge", name)
			assert.Contains(t, body, "High uncertainty", name)
			assert.Contains(t, body, "data-corr-summary", name)
			assert.Contains(t, body, "Script multiplier: 1.16x (legs rise together)", name)
			assert.Contains(t, body, "Getting +105 vs. fair price near -108.", name)
			assert.Contains(t, body, "Player is listed Questionable on the injury report.", name)
			assert.NotContains(t, body, "Baseline 75", name)
		}
	})

	t.Run("v1 fallback renders factor list without pillar markup", func(t *testing.T) {
		t.Parallel()

		v1LegGrade := webui.LegGradeView(analyzer.LegGrade{
			LegID:  11,
			Letter: "A",
			Score:  85,
			Factors: []analyzer.GradeFactor{{
				Rule:  "hit_rate",
				Label: "Strong L5 hit rate (5/5)",
				Delta: 10,
			}},
		})
		v1SlipGrade := webui.SlipGradeView(analyzer.SlipGrade{
			Letter:  "B",
			Score:   80,
			Summary: "2 graded legs",
			Factors: []analyzer.GradeFactor{{
				Rule:   "stack_shootout",
				Label:  "Positive stack",
				Delta:  8,
				Detail: "Shootout stack aligned.",
			}},
		})

		panelHTML, err := renderer.RenderPartial("slip-panel", webui.SlipPage{
			Slip: &webui.SlipView{
				ID:     7,
				Name:   "Sunday Ticket",
				Status: "OPEN",
				Members: []webui.SlipMemberView{{
					User: webui.User{Name: "Ada"},
					Legs: []webui.LegView{{
						ID:        11,
						Selection: "Josh Allen Over 255.5 Pass Yds",
						Odds:      "-110",
						Grade:     v1LegGrade,
					}},
				}},
				Analyzer: webui.AnalyzerView{
					Clean: true,
					Grade: v1SlipGrade,
				},
			},
		})
		require.NoError(t, err)

		body := html.UnescapeString(string(panelHTML))
		assert.Contains(t, body, "Strong L5 hit rate (5/5)")
		assert.Contains(t, body, "+10")
		assert.Contains(t, body, "Positive stack — Shootout stack aligned.")
		assert.Contains(t, body, "+8")
		assert.NotContains(t, body, "data-grade-summary")
		assert.NotContains(t, body, "data-grade-pillars")
		assert.NotContains(t, body, "data-grade-why")
		assert.NotContains(t, body, "data-uncertainty-badge")
		assert.NotContains(t, body, "data-corr-summary")
	})
}

func TestGradeViewSummaryFields(t *testing.T) {
	t.Parallel()

	t.Run("v2 leg computes hit chance and payout value", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name      string
			prob      float64
			price     int
			tone      string
			wantHit   string
			wantLabel string
			wantClass string
		}{
			{
				name:      "strong value",
				prob:      0.614,
				price:     88,
				tone:      analyzer.ToneGood,
				wantHit:   "~61%",
				wantLabel: "Strong value",
				wantClass: "text-positive-ink",
			},
			{
				name:      "fair value",
				prob:      0.502,
				price:     75,
				tone:      analyzer.ToneNeutral,
				wantHit:   "~50%",
				wantLabel: "Fair value",
				wantClass: "text-ink-muted",
			},
			{
				name:      "slight juice",
				prob:      0.002,
				price:     62,
				tone:      analyzer.ToneWarn,
				wantHit:   "<1%",
				wantLabel: "Slight juice",
				wantClass: "text-warning-ink",
			},
			{
				name:      "steep juice",
				prob:      0.997,
				price:     45,
				tone:      analyzer.ToneBad,
				wantHit:   ">99%",
				wantLabel: "Steep juice",
				wantClass: "text-danger-ink",
			},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				gv := webui.LegGradeView(analyzer.LegGrade{
					LegID:          1,
					Letter:         "B",
					Score:          78,
					CalibratedProb: tc.prob,
					Pillars: []analyzer.PillarScore{{
						Key:   analyzer.PillarPriceVig,
						Label: analyzer.PillarLabelPriceVig,
						Score: tc.price,
						Tone:  tc.tone,
					}},
				})
				assert.Equal(t, tc.wantHit, gv.HitChance)
				assert.Equal(t, tc.wantLabel, gv.PayoutLabel)
				assert.Contains(t, gv.PayoutToneClass, tc.wantClass)
				assert.NotContains(t, gv.PayoutLabel, "+EV")
			})
		}
	})

	t.Run("v2 multi-leg slip computes joint hit chance and payout value", func(t *testing.T) {
		t.Parallel()

		gv := webui.SlipGradeView(analyzer.SlipGrade{
			Letter:         "A",
			Score:          86,
			Summary:        "2-leg correlated ticket",
			CalibratedProb: 0.342,
			CorrMultiplier: 1.12,
			LegGrades:      []analyzer.LegGrade{{LegID: 1, Letter: "A"}, {LegID: 2, Letter: "B"}},
			Pillars: []analyzer.PillarScore{{
				Key:   analyzer.PillarPriceVig,
				Label: analyzer.PillarLabelPriceVig,
				Score: 86,
				Tone:  analyzer.ToneGood,
			}},
		})
		assert.Equal(t, "~34%", gv.HitChance)
		assert.Equal(t, "Strong value", gv.PayoutLabel)
		assert.Contains(t, gv.PayoutToneClass, "text-positive-ink")
		assert.NotContains(t, gv.PayoutLabel, "+EV")
	})

	t.Run("v1 fallback leaves hit chance and payout value empty", func(t *testing.T) {
		t.Parallel()

		legV1 := webui.LegGradeView(analyzer.LegGrade{
			LegID:  1,
			Letter: "B",
			Score:  75,
			Factors: []analyzer.GradeFactor{{
				Rule:  "hit_rate",
				Label: "Hit rate",
				Delta: 5,
			}},
		})
		assert.Empty(t, legV1.HitChance)
		assert.Empty(t, legV1.PayoutLabel)
		assert.Empty(t, legV1.PayoutToneClass)

		slipV1 := webui.SlipGradeView(analyzer.SlipGrade{
			Letter:  "B",
			Score:   75,
			Summary: "1 graded leg",
		})
		assert.Empty(t, slipV1.HitChance)
		assert.Empty(t, slipV1.PayoutLabel)
		assert.Empty(t, slipV1.PayoutToneClass)
	})
}

func TestSlipPanelTrayAndGuideWithEngineV2Handler(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixtureWithEngine(t, 2, analyzer.EngineV2)
	game := addGame(t, fixture, gameKickoff())
	now := time.Now().UTC()

	_, err := fixture.store.UpsertOdds(t.Context(), []db.Odds{
		{
			GameID:     game.ID,
			Book:       "pinnacle",
			MarketType: "PASS_YDS",
			Team:       "BUF",
			PlayerName: "Josh Allen",
			Side:       "OVER",
			Line:       new(255.5),
			Price:      -115,
			FetchedAt:  now,
		},
		{
			GameID:     game.ID,
			Book:       "pinnacle",
			MarketType: "PASS_YDS",
			Team:       "BUF",
			PlayerName: "Josh Allen",
			Side:       "UNDER",
			Line:       new(255.5),
			Price:      -105,
			FetchedAt:  now,
		},
		{
			GameID:     game.ID,
			Book:       "pinnacle",
			MarketType: "REC_YDS",
			Team:       "BUF",
			PlayerName: "Khalil Shakir",
			Side:       "OVER",
			Line:       new(52.5),
			Price:      -110,
			FetchedAt:  now,
		},
		{
			GameID:     game.ID,
			Book:       "pinnacle",
			MarketType: "REC_YDS",
			Team:       "BUF",
			PlayerName: "Khalil Shakir",
			Side:       "UNDER",
			Line:       new(52.5),
			Price:      -110,
			FetchedAt:  now,
		},
	})
	require.NoError(t, err)

	injuriesJSON := `[
		{
			"team": {"id": "2", "displayName": "Buffalo Bills", "abbreviation": "BUF"},
			"injuries": [
				{
					"status": "Questionable",
					"athlete": {"id": "4430755", "fullName": "Khalil Shakir", "position": {"abbreviation": "WR"}},
					"details": {"type": "Ankle"}
				}
			]
		}
	]`
	_, err = fixture.store.SetGameInjuries(t.Context(), game.ID, injuriesJSON)
	require.NoError(t, err)
	require.NoError(t, fixture.store.RecordSync(t.Context(), "espn:injuries:"+game.ID, now, nil))

	addStoredLeg(t, fixture, db.Leg{
		UserID:     fixture.user.ID,
		GameID:     new(game.ID),
		MarketType: "PASS_YDS",
		Selection:  "Josh Allen Over 255.5 Pass Yds",
		Team:       new("BUF"),
		PlayerID:   new("3918298"),
		PlayerName: new("Josh Allen"),
		Side:       new("OVER"),
		Line:       new(255.5),
		Odds:       -110,
	})
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

	for _, path := range []string{
		fmt.Sprintf("/slips/%d/panel", fixture.slip.ID),
		fmt.Sprintf("/slips/%d/tray", fixture.slip.ID),
	} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil)
		fixture.handler.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code, path)

		body := html.UnescapeString(rec.Body.String())
		assert.Contains(t, body, "data-leg-grade", path)
		assert.Contains(t, body, "data-slip-grade", path)
		assert.Contains(t, body, "data-grade-summary", path)
		assert.Contains(t, body, "Overall", path)
		assert.Contains(t, body, "Hit chance", path)
		assert.Contains(t, body, "Payout value", path)
		assert.NotContains(t, body, "+EV", path)
		assert.Contains(t, body, "data-grade-pillars", path)
		assert.Contains(t, body, "Price & Vig", path)
		assert.Contains(t, body, "Script Fit", path)
		assert.Contains(t, body, "Stability & Tail Risk", path)
		assert.Contains(t, body, "Context & Traps", path)
		assert.Contains(t, body, "data-grade-why", path)
		assert.Contains(t, body, "data-uncertainty-badge", path)
		assert.Contains(t, body, "High uncertainty", path)
		assert.Contains(t, body, "data-corr-summary", path)
		assert.Contains(t, body, "Script multiplier:", path)
		assert.Contains(t, body, "w-72 max-w-full", path)
	}

	guideRec := httptest.NewRecorder()
	guideReq := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		fmt.Sprintf("/analyzer/guide?from=/slips/%d", fixture.slip.ID),
		nil,
	)
	fixture.handler.ServeHTTP(guideRec, guideReq)
	require.Equal(t, http.StatusOK, guideRec.Code)

	guideBody := html.UnescapeString(guideRec.Body.String())
	assert.Contains(t, guideBody, "Active engine: v2")
	assert.Contains(t, guideBody, "Four-Pillar Grading (v2)")
	assert.Contains(t, guideBody, "Power de-vigging")
	assert.Contains(t, guideBody, "Gaussian copula")
	assert.Contains(t, guideBody, "SGP correlation tax")
	assert.Contains(t, guideBody, "High uncertainty badge")
}

func newSlipFixtureWithEngine(t *testing.T, legsPerMember int, engine analyzer.GradingEngine) *slipFixture {
	t.Helper()

	database, err := db.Open(filepath.Join(t.TempDir(), "parlay.db"))
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, database.Close()) })
	require.NoError(t, db.Migrate(t.Context(), database))

	store := db.NewStore(database)
	t.Cleanup(func() { assert.NoError(t, store.Close()) })
	user, err := store.CreateUserWithIdentity(t.Context(), "Ada", "ada@example.com", "google-ada", "")
	require.NoError(t, err)
	group, err := store.Create(t.Context(), user.ID, "Sunday Parlay Club")
	require.NoError(t, err)
	week, err := store.UpsertWeek(t.Context(), 2026, 2, 9)
	require.NoError(t, err)
	slip, err := store.CreateSlip(
		t.Context(),
		group.ID,
		week.ID,
		nil,
		"Week 9",
		db.SlateScopeFullWeek,
		legsPerMember,
		[]int64{user.ID},
	)
	require.NoError(t, err)

	renderer, err := webui.NewRenderer(assets.Templates, false)
	require.NoError(t, err)

	hub := &live.Hub{}
	service := slips.New(store, hub, nil)
	stream := live.NewStream(hub, func(int64) bool { return false })

	handler := webui.NewWithDeps(webui.Dependencies{
		Logger:        zerolog.Nop(),
		Renderer:      renderer,
		GradingEngine: engine,
		UserFrom: func(context.Context) (db.User, bool) {
			return user, true
		},
		GroupsFrom: func(context.Context) []webui.GroupOption {
			return []webui.GroupOption{{ID: group.ID, Name: group.Name, Current: true}}
		},
		GroupFrom: func(context.Context) (db.Group, bool) {
			return group, true
		},
		Store:  store,
		Slips:  service,
		Stream: stream,
	})

	return &slipFixture{
		handler: handler,
		store:   store,
		hub:     hub,
		user:    user,
		group:   group,
		week:    week,
		slip:    slip,
	}
}
