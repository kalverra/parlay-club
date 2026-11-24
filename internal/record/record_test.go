package record_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/parlay"
	"github.com/kalverra/parlay-club/internal/record"
)

func TestBuildEmptyAndIgnoresUngradedSlips(t *testing.T) {
	t.Parallel()

	t0 := time.Date(2026, 11, 1, 13, 0, 0, 0, time.UTC)
	slips := []record.Slip{
		{ID: 1, Season: 2026, SeasonType: 2, WeekNum: 1, CreatedAt: t0, Status: "OPEN", Result: parlay.Pending},
		{ID: 2, Season: 2026, SeasonType: 2, WeekNum: 1, CreatedAt: t0, Status: "LOCKED", Result: parlay.Pending},
	}
	legs := []record.Leg{
		{SlipID: 1, UserID: 1, Odds: -110, Result: parlay.Won},
	}
	wagers := []record.Wager{
		{SlipID: 1, UserID: 1, Stake: 25, Odds: 200},
	}

	book := record.Build(2026, slips, legs, []record.Member{{ID: 1, Name: "Ada"}}, wagers)
	assert.Equal(t, 2026, book.Season)
	assert.Zero(t, book.TotalSlips)
	assert.Zero(t, book.Wins)
	assert.Zero(t, book.Losses)
	assert.Zero(t, book.Pushes)
	assert.Zero(t, book.UnitsPL)
	assert.Empty(t, book.Leaderboard)
}

func TestBuildStreaksAndPushReset(t *testing.T) {
	t.Parallel()

	t0 := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	// Deliberately pass slips out of chronological order.
	// Chronological sequence:
	// W1 (WON) -> W2 (WON) -> W3 (PUSH, resets) -> W4 (WON) -> W5 (LOST) -> W6 (LOST) -> W7 (LOST)
	// LongestWin = 2, LongestLoss = 3.
	slips := []record.Slip{
		{ID: 7, Season: 2026, SeasonType: 2, WeekNum: 7, CreatedAt: t0, Status: "GRADED", Result: parlay.Lost},
		{ID: 3, Season: 2026, SeasonType: 2, WeekNum: 3, CreatedAt: t0, Status: "GRADED", Result: parlay.Push},
		{ID: 1, Season: 2026, SeasonType: 2, WeekNum: 1, CreatedAt: t0, Status: "GRADED", Result: parlay.Won},
		{ID: 6, Season: 2026, SeasonType: 2, WeekNum: 6, CreatedAt: t0, Status: "GRADED", Result: parlay.Lost},
		{ID: 4, Season: 2026, SeasonType: 2, WeekNum: 4, CreatedAt: t0, Status: "GRADED", Result: parlay.Won},
		{
			ID:         2,
			Season:     2026,
			SeasonType: 2,
			WeekNum:    1,
			CreatedAt:  t0.Add(time.Hour),
			Status:     "GRADED",
			Result:     parlay.Won,
		},
		{ID: 5, Season: 2026, SeasonType: 2, WeekNum: 5, CreatedAt: t0, Status: "GRADED", Result: parlay.Lost},
	}

	legs := make([]record.Leg, 0, len(slips))
	for _, s := range slips {
		legs = append(legs, record.Leg{
			SlipID: s.ID,
			UserID: 1,
			Odds:   100, // +100 -> 2.0 decimal -> +1 unit on WON, -1 on LOST, 0 on PUSH
			Result: s.Result,
		})
	}

	book := record.Build(2026, slips, legs, []record.Member{{ID: 1, Name: "Ada"}}, nil)
	assert.Equal(t, 7, book.TotalSlips)
	assert.Equal(t, 3, book.Wins)
	assert.Equal(t, 3, book.Losses)
	assert.Equal(t, 1, book.Pushes)
	assert.Equal(t, 2, book.LongestWin)
	assert.Equal(t, 3, book.LongestLoss)
	assert.Equal(t, 0, book.CurrentWin)
	assert.Equal(t, 3, book.CurrentLoss)
	assert.InDelta(t, 0.0, book.UnitsPL, 1e-9)
}

func TestBuildLeaderboardThresholdAvgOddsTerroristTiesAndDollarPL(t *testing.T) {
	t.Parallel()

	t0 := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	members := []record.Member{
		{ID: 1, Name: "Ada", AvatarColor: "sky"},
		{ID: 2, Name: "Sam", AvatarColor: "rose"},
		{ID: 3, Name: "Ben", AvatarColor: "amber"},
		{ID: 4, Name: "Zoe", AvatarColor: "emerald"}, // 0 legs -> omitted from leaderboard
	}

	// Build 7 graded slips:
	// - Slip 1 (WON): Ada WON (+100), Sam WON (+100), Ben VOID (-110) -> combined +300 (4.0 dec) -> +3.0u
	// - Slip 2 (WON): Ada WON (-110), Sam WON (-110)
	// - Slip 3 (WON): Ada WON (-110), Sam WON (-110)
	// - Slip 4 (LOST): Ada LOST (-110), Sam WON (-110) -> Ada solo loser (Terrorist +1)
	// - Slip 5 (LOST): Ada WON (-110), Sam LOST (-110) -> Sam solo loser (Terrorist +1)
	// - Slip 6 (LOST): Ada LOST (-110), Sam LOST (-110) -> Multi-loser (NO Terrorist!)
	// - Slip 7 (PUSH): Ada PUSH (+200), Sam PUSH (+200), Ben WON (+100), Ben PUSH (+100)
	//
	// Totals:
	// - Ada: 4 WON, 2 LOST (6 decided -> 66.7%), 1 PUSH, 0 VOID; Terrorist = 1 (TIED)
	// - Sam: 4 WON, 2 LOST (6 decided -> 66.7%), 1 PUSH, 0 VOID; Terrorist = 1 (TIED)
	// - Ben: 1 WON, 0 LOST (1 decided -> <6, HitPct nil), 1 PUSH, 1 VOID; Terrorist = 0
	slips := []record.Slip{
		{ID: 1, Season: 2026, SeasonType: 2, WeekNum: 1, CreatedAt: t0, Status: "GRADED", Result: parlay.Won},
		{ID: 2, Season: 2026, SeasonType: 2, WeekNum: 2, CreatedAt: t0, Status: "GRADED", Result: parlay.Won},
		{ID: 3, Season: 2026, SeasonType: 2, WeekNum: 3, CreatedAt: t0, Status: "GRADED", Result: parlay.Won},
		{ID: 4, Season: 2026, SeasonType: 2, WeekNum: 4, CreatedAt: t0, Status: "GRADED", Result: parlay.Lost},
		{ID: 5, Season: 2026, SeasonType: 2, WeekNum: 5, CreatedAt: t0, Status: "GRADED", Result: parlay.Lost},
		{ID: 6, Season: 2026, SeasonType: 2, WeekNum: 6, CreatedAt: t0, Status: "GRADED", Result: parlay.Lost},
		{ID: 7, Season: 2026, SeasonType: 2, WeekNum: 7, CreatedAt: t0, Status: "GRADED", Result: parlay.Push},
	}

	legs := []record.Leg{
		// Slip 1
		{SlipID: 1, UserID: 1, Odds: 100, Result: parlay.Won},
		{SlipID: 1, UserID: 2, Odds: 100, Result: parlay.Won},
		{SlipID: 1, UserID: 3, Odds: -110, Result: parlay.Void},
		// Slip 2
		{SlipID: 2, UserID: 1, Odds: -110, Result: parlay.Won},
		{SlipID: 2, UserID: 2, Odds: -110, Result: parlay.Won},
		// Slip 3
		{SlipID: 3, UserID: 1, Odds: -110, Result: parlay.Won},
		{SlipID: 3, UserID: 2, Odds: -110, Result: parlay.Won},
		// Slip 4
		{SlipID: 4, UserID: 1, Odds: -110, Result: parlay.Lost},
		{SlipID: 4, UserID: 2, Odds: -110, Result: parlay.Won},
		// Slip 5
		{SlipID: 5, UserID: 1, Odds: -110, Result: parlay.Won},
		{SlipID: 5, UserID: 2, Odds: -110, Result: parlay.Lost},
		// Slip 6
		{SlipID: 6, UserID: 1, Odds: -110, Result: parlay.Lost},
		{SlipID: 6, UserID: 2, Odds: -110, Result: parlay.Lost},
		// Slip 7
		{SlipID: 7, UserID: 1, Odds: 200, Result: parlay.Push},
		{SlipID: 7, UserID: 2, Odds: 200, Result: parlay.Push},
		{SlipID: 7, UserID: 3, Odds: 100, Result: parlay.Won},
		{SlipID: 7, UserID: 3, Odds: 100, Result: parlay.Push},
	}

	// Wagers:
	// - Ada: Slip 1 ($10 @ +300 with +50% boost -> +$45), Slip 4 ($20 LOST -> -$20) => +$25.00
	// - Sam: Slip 7 ($50 PUSH -> $0.00, non-nil!)
	// - Ben: no wagers => DollarPL == nil
	wagers := []record.Wager{
		{SlipID: 1, UserID: 1, Stake: 10, Odds: 300, BoostPct: 50},
		{SlipID: 4, UserID: 1, Stake: 20, Odds: 250, BoostPct: 0},
		{SlipID: 7, UserID: 2, Stake: 50, Odds: 400, BoostPct: 0},
	}

	book := record.Build(2026, slips, legs, members, wagers)
	require.Len(t, book.Leaderboard, 3)

	// Ada and Sam both have 66.7% (4/6); Ada sorts before Sam alphabetically.
	adaRow := book.Leaderboard[0]
	samRow := book.Leaderboard[1]
	benRow := book.Leaderboard[2]

	assert.Equal(t, "Ada", adaRow.Name)
	require.NotNil(t, adaRow.HitPct)
	assert.InDelta(t, 4.0/6.0*100, *adaRow.HitPct, 1e-6)
	assert.Equal(t, 1, adaRow.TerroristCount)
	assert.True(t, adaRow.ParlayTerrorist)
	require.NotNil(t, adaRow.DollarPL)
	assert.InDelta(t, 25.0, *adaRow.DollarPL, 1e-6)

	// Ada's avg odds across 1x 2.0, 5x 1.90909..., 1x 3.0 = 14.545454.../7 = 2.077922 -> +108
	wantAdaAvg := parlay.DecimalToAmerican((2.0 + 5*parlay.AmericanToDecimal(-110) + 3.0) / 7.0)
	assert.Equal(t, wantAdaAvg, adaRow.AvgLegOdds)

	assert.Equal(t, "Sam", samRow.Name)
	require.NotNil(t, samRow.HitPct)
	assert.Equal(t, 1, samRow.TerroristCount)
	assert.True(t, samRow.ParlayTerrorist, "tied leaders both get the Parlay Terrorist badge")
	require.NotNil(t, samRow.DollarPL, "a logged wager on a PUSH slip yields $0.00, not nil")
	assert.InDelta(t, 0.0, *samRow.DollarPL, 1e-9)

	assert.Equal(t, "Ben", benRow.Name)
	assert.Nil(t, benRow.HitPct, "fewer than 6 decided legs yields nil HitPct and sorts last")
	assert.Equal(t, 1, benRow.DecidedLegs)
	assert.Equal(t, 1, benRow.VoidLegs)
	assert.Equal(t, 100, benRow.AvgLegOdds, "VOID leg excluded from avg leg odds")
	assert.False(t, benRow.ParlayTerrorist)
	assert.Nil(t, benRow.DollarPL, "zero logged wagers yields nil DollarPL")
}

func TestBuildNoTerroristBadgeWhenZeroLosses(t *testing.T) {
	t.Parallel()

	t0 := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	slips := []record.Slip{
		{ID: 1, Season: 2026, SeasonType: 2, WeekNum: 1, CreatedAt: t0, Status: "GRADED", Result: parlay.Won},
	}
	legs := []record.Leg{
		{SlipID: 1, UserID: 1, Odds: -110, Result: parlay.Won},
	}

	book := record.Build(2026, slips, legs, []record.Member{{ID: 1, Name: "Ada"}}, nil)
	require.Len(t, book.Leaderboard, 1)
	assert.Zero(t, book.Leaderboard[0].TerroristCount)
	assert.False(t, book.Leaderboard[0].ParlayTerrorist)
}
