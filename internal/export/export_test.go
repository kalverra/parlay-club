package export_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/export"
)

// testNow is the injected clock every test runs on.
var testNow = time.Date(2026, 11, 8, 16, 0, 0, 0, time.UTC)

// upcoming and kickedOff bracket testNow for the kickoff rule.
var (
	upcoming  = testNow.Add(2 * time.Hour)
	kickedOff = testNow.Add(-time.Hour)
)

// lockedSlip is the slip an export is allowed on.
func lockedSlip() db.Slip {
	return db.Slip{ID: 7, Name: "Week 9", Status: db.SlipLocked}
}

// text reads an optional leg field the way the odds table stores it.
func text(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// field keeps a blank string absent.
func field(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

// testGame is the game every built leg belongs to.
const testGame = "game-1"

// makeLeg builds one export leg for testGame.
func makeLeg(
	id int64,
	kickoff *time.Time,
	market, team, player, side string,
	point *float64,
) export.Leg {
	return export.Leg{
		db.Leg{
			ID:         id,
			GameID:     field(testGame),
			MarketType: market,
			Selection:  fmt.Sprintf("leg %d", id),
			Team:       field(team),
			PlayerName: field(player),
			Side:       field(side),
			Line:       point,
			Odds:       -110,
		},
		kickoff,
	}
}

// row builds one stored odds row. A blank link or sid stays NULL.
func row(
	game, book, market, team, player, side string,
	point *float64,
	price int,
	link, sid string,
) db.Odds {
	return db.Odds{
		GameID:     game,
		Book:       book,
		MarketType: market,
		Team:       team,
		PlayerName: player,
		Side:       side,
		Line:       point,
		Price:      price,
		Link:       field(link),
		Sid:        field(sid),
		FetchedAt:  testNow,
	}
}

// linkedIDs returns the IDs of the linked legs in order.
func linkedIDs(plan export.BookPlan) []int64 {
	ids := make([]int64, 0, len(plan.Linked))
	for _, link := range plan.Linked {
		ids = append(ids, link.Leg.ID)
	}
	return ids
}

// manualIDs returns the IDs of the manual legs in order.
func manualIDs(plan export.BookPlan) []int64 {
	ids := make([]int64, 0, len(plan.Manual))
	for _, manual := range plan.Manual {
		ids = append(ids, manual.Leg.ID)
	}
	return ids
}

// TestPlanLinksEveryMarketType checks the exact-key match on every market the
// slip can hold, for both books.
func TestPlanLinksEveryMarketType(t *testing.T) {
	t.Parallel()

	const game = "game-1"
	kickoff := upcoming
	legs := []export.Leg{
		makeLeg(1, &kickoff, "SPREAD", "BUF", "", "TEAM", new(-3.5)),
		makeLeg(2, &kickoff, "ML", "BUF", "", "TEAM", nil),
		makeLeg(3, &kickoff, "TOTAL", "", "", "OVER", new(47.5)),
		makeLeg(4, &kickoff, "TEAM_TOTAL", "BUF", "", "OVER", new(27.5)),
		makeLeg(5, &kickoff, "PASS_YDS", "BUF", "Josh Allen", "OVER", new(245.5)),
		makeLeg(6, &kickoff, "ANYTIME_TD", "BUF", "James Cook", "YES", nil),
	}

	for _, book := range []string{export.DraftKings, export.FanDuel} {
		t.Run(book, func(t *testing.T) {
			t.Parallel()

			rows := make([]db.Odds, 0, len(legs))
			for _, leg := range legs {
				rows = append(rows, row(
					game,
					book,
					leg.MarketType,
					text(leg.Team),
					text(leg.PlayerName),
					text(leg.Side),
					leg.Line,
					-110-int(leg.ID),
					fmt.Sprintf("https://%s/%d", book, leg.ID),
					fmt.Sprintf("sid-%d", leg.ID),
				))
			}

			plan := export.Plan(lockedSlip(), legs, rows, book, testNow)

			assert.Equal(t, book, plan.Book)
			assert.Empty(t, plan.Manual)
			require.Len(t, plan.Linked, len(legs))
			for i, link := range plan.Linked {
				assert.Equal(t, legs[i].ID, link.Leg.ID)
				assert.Equal(t, fmt.Sprintf("https://%s/%d", book, legs[i].ID), link.URL)
				assert.Equal(t, legs[i].Line, link.Line)
				assert.Equal(t, -110-int(legs[i].ID), link.Price)
				assert.Equal(t, fmt.Sprintf("sid-%d", legs[i].ID), text(link.Sid))
			}
		})
	}
}

// TestPlanManualReasons checks every fallback the design names.
func TestPlanManualReasons(t *testing.T) {
	t.Parallel()

	const game = "game-1"
	prop := makeLeg(11, &upcoming, "PASS_YDS", "BUF", "Josh Allen", "OVER", new(245.5))
	spread := makeLeg(12, &upcoming, "SPREAD", "BUF", "", "TEAM", new(-3.5))
	started := makeLeg(13, &kickedOff, "SPREAD", "BUF", "", "TEAM", new(-3.5))
	other := export.Leg{
		db.Leg{ID: 14, MarketType: "OTHER", Selection: "Coin toss", Odds: -110},
		nil,
	}
	withLink := func(market, team, player, side string, point *float64) db.Odds {
		return row(game, export.DraftKings, market, team, player, side, point, -110, "https://link", "sid")
	}
	withoutLink := func(market, team, player, side string, point *float64) db.Odds {
		return row(game, export.DraftKings, market, team, player, side, point, -110, "", "")
	}
	fanDuelSpread := row(game, export.FanDuel, "SPREAD", "BUF", "", "TEAM", new(-3.5), -110,
		"https://fd", "sid")

	tests := map[string]struct {
		leg        export.Leg
		odds       []db.Odds
		book       string
		wantReason export.Reason
		wantLine   *float64
	}{
		"a prop with no link": {
			leg:        prop,
			odds:       []db.Odds{withoutLink("PASS_YDS", "BUF", "Josh Allen", "OVER", new(245.5))},
			wantReason: export.ReasonNoLink,
		},
		"a row with an empty link counts as no link": {
			leg:        spread,
			odds:       []db.Odds{withoutLink("SPREAD", "BUF", "", "TEAM", new(-3.5))},
			wantReason: export.ReasonNoLink,
		},
		"a game that has started": {
			leg:        started,
			odds:       []db.Odds{withLink("SPREAD", "BUF", "", "TEAM", new(-3.5))},
			wantReason: export.ReasonGameStarted,
		},
		"a game kicking off exactly now counts as started": {
			leg:        makeLeg(16, &testNow, "SPREAD", "BUF", "", "TEAM", new(-3.5)),
			odds:       []db.Odds{withLink("SPREAD", "BUF", "", "TEAM", new(-3.5))},
			wantReason: export.ReasonGameStarted,
		},
		"a started game wins over a missing link": {
			leg:        started,
			odds:       []db.Odds{withoutLink("SPREAD", "BUF", "", "TEAM", new(-3.5))},
			wantReason: export.ReasonGameStarted,
		},
		"a leg with no odds row for the book": {
			leg:        spread,
			wantReason: export.ReasonNoLine,
		},
		"a hand-typed OTHER leg with no game": {
			leg:        other,
			wantReason: export.ReasonNoLine,
		},
		"a leg matching only the other book": {
			leg:        spread,
			odds:       []db.Odds{fanDuelSpread},
			book:       export.DraftKings,
			wantReason: export.ReasonNoLine,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			book := test.book
			if book == "" {
				book = export.DraftKings
			}
			plan := export.Plan(lockedSlip(), []export.Leg{test.leg}, test.odds, book, testNow)

			require.Empty(t, plan.Linked)
			require.Len(t, plan.Manual, 1)
			manual := plan.Manual[0]
			assert.Equal(t, test.leg.ID, manual.Leg.ID)
			assert.Equal(t, test.wantReason, manual.Reason)
			assert.Equal(t, test.wantLine, manual.Line)
		})
	}
}

// TestPlanDoesNotMatchADifferentSelection checks each part of the odds key.
func TestPlanDoesNotMatchADifferentSelection(t *testing.T) {
	t.Parallel()

	const game = "game-1"
	leg := makeLeg(21, &upcoming, "SPREAD", "BUF", "", "TEAM", new(-3.5))

	tests := map[string]db.Odds{
		"market": row(game, export.DraftKings, "ML", "BUF", "", "TEAM", nil, -110, "https://link", "sid"),
		"team":   row(game, export.DraftKings, "SPREAD", "MIA", "", "TEAM", new(-3.5), -110, "https://link", "sid"),
		"player": row(
			game,
			export.DraftKings,
			"SPREAD",
			"BUF",
			"Josh Allen",
			"TEAM",
			new(-3.5),
			-110,
			"https://link",
			"sid",
		),
		"side": row(game, export.DraftKings, "SPREAD", "BUF", "", "OVER", new(-3.5), -110, "https://link", "sid"),
		"game": row("game-2", export.DraftKings, "SPREAD", "BUF", "", "TEAM", new(-3.5), -110, "https://link", "sid"),
	}

	for name, stored := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			plan := export.Plan(lockedSlip(), []export.Leg{leg}, []db.Odds{stored}, export.DraftKings, testNow)

			require.Empty(t, plan.Linked)
			require.Len(t, plan.Manual, 1)
			assert.Equal(t, export.ReasonNoLine, plan.Manual[0].Reason)
		})
	}
}

// TestPlanLinksThroughPriceDrift keeps the sportsbook's price from blocking a
// link. The person sees the book's price and decides.
func TestPlanLinksThroughPriceDrift(t *testing.T) {
	t.Parallel()

	leg := makeLeg(31, &upcoming, "SPREAD", "BUF", "", "TEAM", new(-3.5))
	stored := row("game-1", export.DraftKings, "SPREAD", "BUF", "", "TEAM", new(-3.5), 120,
		"https://link", "sid")

	plan := export.Plan(lockedSlip(), []export.Leg{leg}, []db.Odds{stored}, export.DraftKings, testNow)

	require.Empty(t, plan.Manual)
	require.Len(t, plan.Linked, 1)
	assert.Equal(t, 120, plan.Linked[0].Price)
}

// TestPlanPlacesEveryLegInExactlyOneList runs one mixed slip through Plan and
// checks no leg is dropped or repeated.
func TestPlanPlacesEveryLegInExactlyOneList(t *testing.T) {
	t.Parallel()

	const game = "game-1"
	legs := []export.Leg{
		makeLeg(41, &upcoming, "SPREAD", "BUF", "", "TEAM", new(-3.5)),
		makeLeg(42, &upcoming, "PASS_YDS", "BUF", "Josh Allen", "OVER", new(245.5)),
		makeLeg(43, &upcoming, "SPREAD", "BUF", "", "TEAM", new(-4.5)),
		makeLeg(44, &kickedOff, "ML", "BUF", "", "TEAM", nil),
		makeLeg(45, &upcoming, "TOTAL", "", "", "OVER", new(47.5)),
		{db.Leg{ID: 46, MarketType: "OTHER", Selection: "Coin toss", Odds: -110}, nil},
		makeLeg(47, &upcoming, "ML", "MIA", "", "TEAM", nil),
	}
	odds := []db.Odds{
		row(game, export.DraftKings, "SPREAD", "BUF", "", "TEAM", new(-3.5), -110, "https://dk/41", "sid-41"),
		row(game, export.DraftKings, "PASS_YDS", "BUF", "Josh Allen", "OVER", new(245.5), -110, "", ""),
		row(game, export.DraftKings, "SPREAD", "BUF", "", "TEAM", new(-4.5), -110, "https://dk/43", "sid-43"),
		row(game, export.DraftKings, "ML", "BUF", "", "TEAM", nil, -110, "https://dk/44", "sid-44"),
		row(game, export.FanDuel, "ML", "MIA", "", "TEAM", nil, -110, "https://fd/47", "sid-47"),
	}

	plan := export.Plan(lockedSlip(), legs, odds, export.DraftKings, testNow)

	require.Len(t, plan.Linked, 2)
	require.Len(t, plan.Manual, len(legs)-2)
	assert.Equal(t, []int64{41, 43}, linkedIDs(plan))
	assert.Equal(t, []int64{42, 44, 45, 46, 47}, manualIDs(plan))

	seen := make(map[int64]int, len(legs))
	for _, id := range linkedIDs(plan) {
		seen[id]++
	}
	for _, id := range manualIDs(plan) {
		seen[id]++
	}
	for _, leg := range legs {
		assert.Equal(t, 1, seen[leg.ID], "leg %d", leg.ID)
	}
}

// TestPlanLinksMovedLinesAndNameVariants checks that SameSelection resolves
// player name variants (prod Leg 19) and moved lines (prod Leg 20) to live
// sportsbook links with LineMoved set accurately and combined URLs intact.
func TestPlanLinksMovedLinesAndNameVariants(t *testing.T) {
	t.Parallel()

	const game = "game-1"

	t.Run("matches generational suffix variant without flagging drift", func(t *testing.T) {
		t.Parallel()

		leg := makeLeg(19, &upcoming, "RUSH_YDS", "MIA", "Ollie Gordon", "OVER", new(17.5))
		odds := []db.Odds{
			row(game, export.DraftKings, "RUSH_YDS", "MIA", "Ollie Gordon II", "OVER", new(17.5), -110,
				"https://sportsbook.draftkings.com/event/123?outcomes=dk-gordon", "0ML#gordon"),
		}

		plan := export.Plan(lockedSlip(), []export.Leg{leg}, odds, export.DraftKings, testNow)
		require.Empty(t, plan.Manual)
		require.Len(t, plan.Linked, 1)
		assert.Equal(t, "https://sportsbook.draftkings.com/event/123?outcomes=dk-gordon", plan.Linked[0].URL)
		assert.Equal(t, new(17.5), plan.Linked[0].Line)
		assert.False(t, plan.Linked[0].LineMoved)
	})

	t.Run("links moved prop line with LineMoved and keeps combined DraftKings URL", func(t *testing.T) {
		t.Parallel()

		bryce := makeLeg(20, &upcoming, "PASS_YDS", "CAR", "Bryce Young", "OVER", new(248.5))
		gordon := makeLeg(19, &upcoming, "RUSH_YDS", "MIA", "Ollie Gordon", "OVER", new(17.5))

		dkRow := func(market, team, player, side string, point *float64, sid string) db.Odds {
			o := row(game, export.DraftKings, market, team, player, side, point, -115,
				"https://sportsbook.draftkings.com/event/dk-evt?outcomes="+sid, sid)
			o.EventSid = new("dk-evt")
			return o
		}
		odds := []db.Odds{
			dkRow("PASS_YDS", "CAR", "Bryce Young", "OVER", new(254.5), "0ML#bryce"),
			dkRow("RUSH_YDS", "MIA", "Ollie Gordon II", "OVER", new(17.5), "0ML#gordon"),
		}

		plan := export.Plan(lockedSlip(), []export.Leg{bryce, gordon}, odds, export.DraftKings, testNow)
		require.Empty(t, plan.Manual)
		require.Len(t, plan.Linked, 2)

		assert.True(t, plan.Linked[0].LineMoved)
		assert.Equal(t, new(254.5), plan.Linked[0].Line)
		assert.Equal(t, new(248.5), plan.Linked[0].Leg.Line)
		assert.False(t, plan.Linked[1].LineMoved)
		assert.Equal(t,
			"https://sportsbook.draftkings.com/event/dk-evt?outcomes=0ML%23bryce,0ML%23gordon",
			plan.Combined,
		)
	})

	t.Run("links moved spread and total lines with LineMoved true", func(t *testing.T) {
		t.Parallel()

		spread := makeLeg(12, &upcoming, "SPREAD", "BUF", "", "TEAM", new(-3.5))
		total := makeLeg(15, &upcoming, "TOTAL", "", "", "OVER", new(47.5))
		odds := []db.Odds{
			row(game, export.DraftKings, "SPREAD", "BUF", "", "TEAM", new(-4.5), -110, "https://dk/spread", "sid-s"),
			row(game, export.DraftKings, "TOTAL", "", "", "OVER", new(48.5), -110, "https://dk/total", "sid-t"),
		}

		plan := export.Plan(lockedSlip(), []export.Leg{spread, total}, odds, export.DraftKings, testNow)
		require.Empty(t, plan.Manual)
		require.Len(t, plan.Linked, 2)
		assert.True(t, plan.Linked[0].LineMoved)
		assert.Equal(t, new(-4.5), plan.Linked[0].Line)
		assert.True(t, plan.Linked[1].LineMoved)
		assert.Equal(t, new(48.5), plan.Linked[1].Line)
	})
}

// fakeCombiner records what it was asked to combine.
type fakeCombiner struct {
	url   string
	ok    bool
	calls int
	got   []export.LegLink
}

// Combine records the linked legs and returns the fixed answer.
func (c *fakeCombiner) Combine(linked []export.LegLink) (string, bool) {
	c.calls++
	c.got = append([]export.LegLink(nil), linked...)
	return c.url, c.ok
}

// TestPlanUsesAVerifiedCombiner checks the one-tap path and the fallback.
func TestPlanUsesAVerifiedCombiner(t *testing.T) {
	t.Parallel()

	const game = "game-1"
	legs := []export.Leg{
		makeLeg(51, &upcoming, "SPREAD", "BUF", "", "TEAM", new(-3.5)),
		makeLeg(52, &upcoming, "ML", "BUF", "", "TEAM", nil),
	}
	odds := []db.Odds{
		row(game, export.DraftKings, "SPREAD", "BUF", "", "TEAM", new(-3.5), -110, "https://dk/51", "sid-51"),
		row(game, export.DraftKings, "ML", "BUF", "", "TEAM", nil, -110, "https://dk/52", "sid-52"),
	}

	t.Run("a verified format gives one URL for every linked leg", func(t *testing.T) {
		t.Parallel()

		combiner := &fakeCombiner{url: "https://combined", ok: true}
		plan := export.PlanWithCombiner(
			lockedSlip(), legs, odds, export.DraftKings, testNow, combiner)

		assert.Equal(t, "https://combined", plan.Combined)
		require.Len(t, plan.Linked, len(legs), "per-leg links stay available")
		require.Equal(t, 1, combiner.calls)
		assert.Equal(t, []int64{51, 52}, linkedIDs(export.BookPlan{Linked: combiner.got}))
	})

	t.Run("an unverified format leaves the per-leg links", func(t *testing.T) {
		t.Parallel()

		combiner := &fakeCombiner{url: "https://guessed", ok: false}
		plan := export.PlanWithCombiner(
			lockedSlip(), legs, odds, export.DraftKings, testNow, combiner)

		assert.Empty(t, plan.Combined)
		require.Len(t, plan.Linked, len(legs))
		assert.Equal(t, "https://dk/51", plan.Linked[0].URL)
		assert.Equal(t, "https://dk/52", plan.Linked[1].URL)
	})

	t.Run("no combiner leaves Combined empty", func(t *testing.T) {
		t.Parallel()

		plan := export.PlanWithCombiner(
			lockedSlip(), legs, odds, export.DraftKings, testNow, nil)

		assert.Empty(t, plan.Combined)
		require.Len(t, plan.Linked, len(legs))
	})

	t.Run("manual legs are not sent to the combiner", func(t *testing.T) {
		t.Parallel()

		mixedLegs := append([]export.Leg{
			makeLeg(53, &upcoming, "TOTAL", "", "", "OVER", new(47.5)),
		}, legs...)
		combiner := &fakeCombiner{url: "https://combined", ok: true}

		plan := export.PlanWithCombiner(
			lockedSlip(), mixedLegs, odds, export.DraftKings, testNow, combiner)

		require.Len(t, plan.Manual, 1)
		require.Equal(t, 1, combiner.calls)
		assert.Equal(t, []int64{51, 52}, linkedIDs(export.BookPlan{Linked: combiner.got}))
		assert.Equal(t, "https://combined", plan.Combined)
	})

	t.Run("no linked legs means no combined URL", func(t *testing.T) {
		t.Parallel()

		combiner := &fakeCombiner{url: "https://combined", ok: true}
		plan := export.PlanWithCombiner(
			lockedSlip(),
			[]export.Leg{makeLeg(54, &upcoming, "ML", "BUF", "", "TEAM", nil)},
			nil,
			export.DraftKings,
			testNow,
			combiner,
		)

		assert.Empty(t, plan.Combined)
		assert.Zero(t, combiner.calls)
	})
}

// TestPlanCombinesDraftKingsSelections checks the verified DraftKings format
// against the sids from the saved probe: two selections share one event page,
// with their sids comma-separated in the outcomes parameter.
func TestPlanCombinesDraftKingsSelections(t *testing.T) {
	t.Parallel()

	const game = "game-1"
	ml := makeLeg(71, &upcoming, "ML", "ATL", "", "TEAM", nil)
	under := makeLeg(72, &upcoming, "TOTAL", "", "", "UNDER", new(52.5))
	odds := []db.Odds{
		row(
			game,
			export.DraftKings,
			"ML",
			"ATL",
			"",
			"TEAM",
			nil,
			145,
			"https://sportsbook.draftkings.com/?outcomes=0ML84695561_1",
			"0ML84695561_1",
		),
		row(
			game,
			export.DraftKings,
			"TOTAL",
			"",
			"",
			"UNDER",
			new(52.5),
			-112,
			"https://sportsbook.draftkings.com/?outcomes=0OU84695561U5250_3",
			"0OU84695561U5250_3",
		),
	}

	odds[0].EventSid = new("34118128")
	odds[1].EventSid = new("34118128")

	plan := export.Plan(lockedSlip(), []export.Leg{ml, under}, odds, export.DraftKings, testNow)

	require.Empty(t, plan.Manual)
	require.Len(t, plan.Linked, 2, "per-leg links stay available")
	assert.Equal(t,
		"https://sportsbook.draftkings.com/event/34118128"+
			"?outcomes=0ML84695561_1,0OU84695561U5250_3",
		plan.Combined)
}

// TestPlanCombinesDraftKingsPropSids checks that a prop sid's # is escaped and
// stays inside the one outcomes parameter.
func TestPlanCombinesDraftKingsPropSids(t *testing.T) {
	t.Parallel()

	const game = "game-1"
	ml := makeLeg(73, &upcoming, "ML", "ATL", "", "TEAM", nil)
	prop := makeLeg(74, &upcoming, "PASS_YDS", "CIN", "Joe Burrow", "OVER", new(278.5))
	odds := []db.Odds{
		row(
			game,
			export.DraftKings,
			"ML",
			"ATL",
			"",
			"TEAM",
			nil,
			145,
			"https://sportsbook.draftkings.com/?outcomes=0ML84695561_1",
			"0ML84695561_1",
		),
		row(
			game,
			export.DraftKings,
			"PASS_YDS",
			"CIN",
			"Joe Burrow",
			"OVER",
			new(278.5),
			-112,
			"https://sportsbook.draftkings.com/?outcomes=0QA385017402%232346593166_13L88808Q1466938236Q20",
			"0QA385017402#2346593166_13L88808Q1466938236Q20",
		),
	}

	odds[0].EventSid = new("34118128")
	odds[1].EventSid = new("34118128")

	plan := export.Plan(lockedSlip(), []export.Leg{ml, prop}, odds, export.DraftKings, testNow)

	assert.Equal(t,
		"https://sportsbook.draftkings.com/event/34118128?outcomes=0ML84695561_1,"+
			"0QA385017402%232346593166_13L88808Q1466938236Q20",
		plan.Combined)
}

// TestPlanKeepsDraftKingsPerLegWhenACombinedLinkCannotBeBuilt checks the
// DraftKings fallback for a missing sid, a missing event id, legs from
// different events, and a single linked leg.
func TestPlanKeepsDraftKingsPerLegWhenACombinedLinkCannotBeBuilt(t *testing.T) {
	t.Parallel()

	const game = "game-1"
	ml := makeLeg(75, &upcoming, "ML", "ATL", "", "TEAM", nil)
	under := makeLeg(76, &upcoming, "TOTAL", "", "", "UNDER", new(52.5))
	base := []db.Odds{
		row(
			game,
			export.DraftKings,
			"ML",
			"ATL",
			"",
			"TEAM",
			nil,
			145,
			"https://sportsbook.draftkings.com/?outcomes=0ML84695561_1",
			"0ML84695561_1",
		),
		row(
			game,
			export.DraftKings,
			"TOTAL",
			"",
			"",
			"UNDER",
			new(52.5),
			-112,
			"https://sportsbook.draftkings.com/?outcomes=0OU84695561U5250_3",
			"0OU84695561U5250_3",
		),
	}

	base[0].EventSid = new("34118128")
	base[1].EventSid = new("34118128")

	t.Run("a missing sid", func(t *testing.T) {
		t.Parallel()

		odds := append([]db.Odds(nil), base...)
		odds[1].Sid = nil
		plan := export.Plan(lockedSlip(), []export.Leg{ml, under}, odds, export.DraftKings, testNow)

		assert.Empty(t, plan.Combined)
		require.Len(t, plan.Linked, 2)
	})

	t.Run("a missing event id", func(t *testing.T) {
		t.Parallel()

		odds := append([]db.Odds(nil), base...)
		odds[1].EventSid = nil
		plan := export.Plan(lockedSlip(), []export.Leg{ml, under}, odds, export.DraftKings, testNow)

		assert.Empty(t, plan.Combined)
		require.Len(t, plan.Linked, 2)
	})

	t.Run("legs from different events", func(t *testing.T) {
		t.Parallel()

		odds := append([]db.Odds(nil), base...)
		odds[1].EventSid = new("other-event")
		plan := export.Plan(lockedSlip(), []export.Leg{ml, under}, odds, export.DraftKings, testNow)

		assert.Empty(t, plan.Combined)
		require.Len(t, plan.Linked, 2)
	})

	t.Run("one linked leg stays a per-leg tap", func(t *testing.T) {
		t.Parallel()

		plan := export.Plan(lockedSlip(), []export.Leg{ml}, base[:1], export.DraftKings, testNow)

		assert.Empty(t, plan.Combined)
		require.Len(t, plan.Linked, 1)
	})
}

// TestPlanCombinesFanDuelSelections checks the verified FanDuel format against
// the sids from the saved probe: two selections become one indexed betslip URL.
func TestPlanCombinesFanDuelSelections(t *testing.T) {
	t.Parallel()

	const game = "game-1"
	ml := makeLeg(71, &upcoming, "ML", "ATL", "", "TEAM", nil)
	under := makeLeg(72, &upcoming, "TOTAL", "", "", "UNDER", new(52.5))
	odds := []db.Odds{
		row(
			game,
			export.FanDuel,
			"ML",
			"ATL",
			"",
			"TEAM",
			nil,
			140,
			"https://sportsbook.fanduel.com/addToBetslip?marketId=42.578189645&selectionId=50199",
			"50199",
		),
		row(
			game,
			export.FanDuel,
			"TOTAL",
			"",
			"",
			"UNDER",
			new(52.5),
			-110,
			"https://sportsbook.fanduel.com/addToBetslip?marketId=42.578189648&selectionId=7017917",
			"7017917",
		),
	}

	plan := export.Plan(lockedSlip(), []export.Leg{ml, under}, odds, export.FanDuel, testNow)

	require.Empty(t, plan.Manual)
	require.Len(t, plan.Linked, 2, "per-leg links stay available")
	assert.Equal(t, "https://sportsbook.fanduel.com/addToBetslip"+
		"?marketId%5B0%5D=42.578189645&selectionId%5B0%5D=50199"+
		"&marketId%5B1%5D=42.578189648&selectionId%5B1%5D=7017917",
		plan.Combined)
}

// TestPlanKeepsFanDuelPerLegWhenACombinedLinkCannotBeBuilt checks the fallback:
// a leg without the data FanDuel's format needs leaves the per-leg taps alone.
func TestPlanKeepsFanDuelPerLegWhenACombinedLinkCannotBeBuilt(t *testing.T) {
	t.Parallel()

	const game = "game-1"
	ml := makeLeg(81, &upcoming, "ML", "ATL", "", "TEAM", nil)
	under := makeLeg(82, &upcoming, "TOTAL", "", "", "UNDER", new(52.5))
	base := []db.Odds{
		row(
			game,
			export.FanDuel,
			"ML",
			"ATL",
			"",
			"TEAM",
			nil,
			140,
			"https://sportsbook.fanduel.com/addToBetslip?marketId=42.578189645&selectionId=50199",
			"50199",
		),
		row(
			game,
			export.FanDuel,
			"TOTAL",
			"",
			"",
			"UNDER",
			new(52.5),
			-110,
			"https://sportsbook.fanduel.com/addToBetslip?marketId=42.578189648&selectionId=7017917",
			"7017917",
		),
	}

	t.Run("a missing sid", func(t *testing.T) {
		t.Parallel()

		odds := append([]db.Odds(nil), base...)
		odds[1].Sid = nil
		plan := export.Plan(lockedSlip(), []export.Leg{ml, under}, odds, export.FanDuel, testNow)

		assert.Empty(t, plan.Combined)
		require.Len(t, plan.Linked, 2)
	})

	t.Run("a link without a market id", func(t *testing.T) {
		t.Parallel()

		odds := append([]db.Odds(nil), base...)
		odds[1].Link = new("https://sportsbook.fanduel.com/addToBetslip?selectionId=7017917")
		plan := export.Plan(lockedSlip(), []export.Leg{ml, under}, odds, export.FanDuel, testNow)

		assert.Empty(t, plan.Combined)
		require.Len(t, plan.Linked, 2)
	})

	t.Run("a malformed link", func(t *testing.T) {
		t.Parallel()

		odds := append([]db.Odds(nil), base...)
		odds[0].Link = new("https://%zz")
		plan := export.Plan(lockedSlip(), []export.Leg{ml, under}, odds, export.FanDuel, testNow)

		assert.Empty(t, plan.Combined)
	})

	t.Run("one linked leg stays a per-leg tap", func(t *testing.T) {
		t.Parallel()

		plan := export.Plan(lockedSlip(), []export.Leg{ml}, base[:1], export.FanDuel, testNow)

		assert.Empty(t, plan.Combined)
		require.Len(t, plan.Linked, 1)
	})

	t.Run("the stored sid wins over the link's selection id", func(t *testing.T) {
		t.Parallel()

		odds := append([]db.Odds(nil), base...)
		odds[1].Sid = new("from-row")
		plan := export.Plan(lockedSlip(), []export.Leg{ml, under}, odds, export.FanDuel, testNow)

		assert.Contains(t, plan.Combined, "selectionId%5B1%5D=from-row")
	})

	t.Run("a sid is escaped", func(t *testing.T) {
		t.Parallel()

		odds := append([]db.Odds(nil), base...)
		odds[1].Sid = new("a#b c")
		plan := export.Plan(lockedSlip(), []export.Leg{ml, under}, odds, export.FanDuel, testNow)

		assert.Contains(t, plan.Combined, "selectionId%5B1%5D=a%23b+c")
	})
}

// TestPlanOnlyCoversALockedSlip checks that an open or graded slip gets no
// export at all.
func TestPlanOnlyCoversALockedSlip(t *testing.T) {
	t.Parallel()

	leg := makeLeg(61, &upcoming, "SPREAD", "BUF", "", "TEAM", new(-3.5))
	odds := []db.Odds{
		row("game-1", export.DraftKings, "SPREAD", "BUF", "", "TEAM", new(-3.5), -110, "https://link", "sid"),
	}

	for _, status := range []db.SlipStatus{db.SlipOpen, db.SlipGraded} {
		t.Run(string(status), func(t *testing.T) {
			t.Parallel()

			slip := lockedSlip()
			slip.Status = status
			plan := export.Plan(slip, []export.Leg{leg}, odds, export.DraftKings, testNow)

			assert.Empty(t, plan.Linked)
			assert.Empty(t, plan.Manual)
			assert.Empty(t, plan.Combined)
		})
	}
}
