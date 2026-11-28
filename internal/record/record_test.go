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

func TestImpliedLiftAndLegStreaks(t *testing.T) {
	t.Parallel()

	t0 := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	members := []record.Member{
		{ID: 1, Name: "Ada"},
		{ID: 2, Name: "Sam"},
		{ID: 3, Name: "Ben"},
	}

	// Pass slips out of chronological order; Ada has:
	// - Slip 1 (Week 1): WON (+100), PUSH (-110)
	// - Slip 2 (Week 2): LOST (-110) -> resets win streak
	// - Slip 3 (Week 3): WON (-110), VOID (-110), WON (+150)
	// - Slip 4 (Week 4): WON (+200), PUSH (+100), WON (-150), WON (-110)
	// Ada's active win streak: Slip 3 (2 wins, VOID skipped) + Slip 4 (3 wins, PUSH skipped) = 5!
	// Ada's decided legs: 7 (5 WON, 2 LOST).
	// Sam has 5 decided legs (below MinDecidedLegs -> ImpliedLift nil).
	// Ben has 6 decided legs with invalid Odds == 0 -> ImpliedLift nil.
	slips := []record.Slip{
		{ID: 4, Season: 2026, SeasonType: 2, WeekNum: 4, CreatedAt: t0, Status: "GRADED", Result: parlay.Push},
		{ID: 2, Season: 2026, SeasonType: 2, WeekNum: 2, CreatedAt: t0, Status: "GRADED", Result: parlay.Lost},
		{ID: 1, Season: 2026, SeasonType: 2, WeekNum: 1, CreatedAt: t0, Status: "GRADED", Result: parlay.Push},
		{ID: 3, Season: 2026, SeasonType: 2, WeekNum: 3, CreatedAt: t0, Status: "GRADED", Result: parlay.Won},
	}

	adaOdds := []int{100, -110, -110, 150, 200, -150, -110}
	legs := []record.Leg{
		// Slip 1 (pass leg ID 2 before leg ID 1 to verify Leg.ID ordering within a slip)
		{ID: 2, SlipID: 1, UserID: 1, Odds: -110, Result: parlay.Push},
		{ID: 1, SlipID: 1, UserID: 1, Odds: 100, Result: parlay.Won},
		// Slip 2
		{ID: 3, SlipID: 2, UserID: 1, Odds: -110, Result: parlay.Lost},
		// Slip 3
		{ID: 6, SlipID: 3, UserID: 1, Odds: 150, Result: parlay.Won},
		{ID: 5, SlipID: 3, UserID: 1, Odds: -110, Result: parlay.Void},
		{ID: 4, SlipID: 3, UserID: 1, Odds: -110, Result: parlay.Won},
		// Slip 4
		{ID: 7, SlipID: 4, UserID: 1, Odds: 200, Result: parlay.Won},
		{ID: 8, SlipID: 4, UserID: 1, Odds: 100, Result: parlay.Push},
		{ID: 9, SlipID: 4, UserID: 1, Odds: -150, Result: parlay.Won},
		{ID: 10, SlipID: 4, UserID: 1, Odds: -110, Result: parlay.Won},

		// Sam: 5 decided legs (< MinDecidedLegs)
		{ID: 11, SlipID: 1, UserID: 2, Odds: -110, Result: parlay.Won},
		{ID: 12, SlipID: 2, UserID: 2, Odds: -110, Result: parlay.Won},
		{ID: 13, SlipID: 3, UserID: 2, Odds: -110, Result: parlay.Won},
		{ID: 14, SlipID: 4, UserID: 2, Odds: -110, Result: parlay.Won},
		{ID: 15, SlipID: 4, UserID: 2, Odds: -110, Result: parlay.Lost},

		// Ben: 6 decided legs, all with invalid Odds == 0
		{ID: 16, SlipID: 1, UserID: 3, Odds: 0, Result: parlay.Won},
		{ID: 17, SlipID: 1, UserID: 3, Odds: 0, Result: parlay.Won},
		{ID: 18, SlipID: 2, UserID: 3, Odds: 0, Result: parlay.Lost},
		{ID: 19, SlipID: 3, UserID: 3, Odds: 0, Result: parlay.Won},
		{ID: 20, SlipID: 4, UserID: 3, Odds: 0, Result: parlay.Won},
		{ID: 21, SlipID: 4, UserID: 3, Odds: 0, Result: parlay.Lost},
	}

	book := record.Build(2026, slips, legs, members, nil)
	byUser := rowsByUser(book.Leaderboard)

	ada := byUser[1]
	assert.Equal(t, 5, ada.CurrentWinStreak)
	assert.Equal(t, 0, ada.CurrentLossStreak)
	require.NotNil(t, ada.HitPct)
	require.NotNil(t, ada.ImpliedLift)

	var sumImplied float64
	for _, o := range adaOdds {
		sumImplied += 100.0 / parlay.AmericanToDecimal(o)
	}
	wantLift := *ada.HitPct - (sumImplied / float64(len(adaOdds)))
	assert.InDelta(t, wantLift, *ada.ImpliedLift, 1e-9)

	sam := byUser[2]
	assert.Nil(t, sam.ImpliedLift, "ImpliedLift must be nil below MinDecidedLegs")
	assert.Equal(t, 0, sam.CurrentWinStreak)
	assert.Equal(t, 1, sam.CurrentLossStreak)

	ben := byUser[3]
	assert.NotNil(t, ben.HitPct)
	assert.Nil(t, ben.ImpliedLift, "ImpliedLift must be nil when no decided legs have valid odds")
}

func TestAllFourteenTagsBoundaries(t *testing.T) {
	t.Parallel()

	type legSpec struct {
		odds     int
		result   parlay.Result
		grade    string
		netVotes int
		soloKill bool
	}

	buildForLegs := func(specs []legSpec) record.MemberRow {
		t0 := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
		var slips []record.Slip
		var legs []record.Leg
		for i, sp := range specs {
			slipID := int64(i + 1)
			slipRes := parlay.Won
			switch sp.result {
			case parlay.Lost:
				slipRes = parlay.Lost
			case parlay.Push:
				slipRes = parlay.Push
			}
			slips = append(slips, record.Slip{
				ID:         slipID,
				Season:     2026,
				SeasonType: 2,
				WeekNum:    i + 1,
				CreatedAt:  t0,
				Status:     "GRADED",
				Result:     slipRes,
			})
			legs = append(legs, record.Leg{
				ID:       int64(i*2 + 1),
				SlipID:   slipID,
				UserID:   1,
				Odds:     sp.odds,
				Result:   sp.result,
				Grade:    sp.grade,
				NetVotes: sp.netVotes,
			})
			// Add a teammate leg on lost slips unless this leg is a solo kill.
			if sp.result == parlay.Lost && !sp.soloKill {
				legs = append(legs, record.Leg{
					ID:     int64(i*2 + 2),
					SlipID: slipID,
					UserID: 2,
					Odds:   -110,
					Result: parlay.Lost,
				})
			}
		}
		book := record.Build(2026, slips, legs, []record.Member{
			{ID: 1, Name: "Ada"},
			{ID: 2, Name: "Sam"},
		}, nil)
		return rowsByUser(book.Leaderboard)[1]
	}

	repeatLegs := func(n int, sp legSpec) []legSpec {
		out := make([]legSpec, n)
		for i := range out {
			out[i] = sp
		}
		return out
	}

	tests := []struct {
		name     string
		tagKey   string
		wantTone record.TagTone
		on       []legSpec
		off      []legSpec
	}{
		{
			name:     "natural",
			tagKey:   "natural",
			wantTone: record.TagTonePraise,
			// 3 wins of 5 D/F = 60.0% (exact threshold)
			on: append(
				repeatLegs(3, legSpec{odds: -110, result: parlay.Won, grade: "D"}),
				repeatLegs(2, legSpec{odds: -110, result: parlay.Lost, grade: "F"})...,
			),
			// 3 wins of 6 D/F = 50.0% (< 60.0%)
			off: append(
				repeatLegs(3, legSpec{odds: -110, result: parlay.Won, grade: "D"}),
				repeatLegs(3, legSpec{odds: -110, result: parlay.Lost, grade: "F"})...,
			),
		},
		{
			name:     "ball_knower",
			tagKey:   "ball_knower",
			wantTone: record.TagTonePraise,
			// 31/50 at +100 (50.0% implied exact) -> 62.0% hit -> +12.0 pts
			on: append(
				repeatLegs(31, legSpec{odds: 100, result: parlay.Won}),
				repeatLegs(19, legSpec{odds: 100, result: parlay.Lost})...,
			),
			// 30/50 at +100 -> 60.0% hit -> +10.0 pts (< +12.0)
			off: append(
				repeatLegs(30, legSpec{odds: 100, result: parlay.Won}),
				repeatLegs(20, legSpec{odds: 100, result: parlay.Lost})...,
			),
		},
		{
			name:     "film_room_god",
			tagKey:   "film_room_god",
			wantTone: record.TagTonePraise,
			// 3/4 A/B = 75.0%
			on: append(
				repeatLegs(3, legSpec{odds: -110, result: parlay.Won, grade: "A"}),
				legSpec{odds: -110, result: parlay.Lost, grade: "B"},
			),
			// 3/3 A/B (< 4 min sample)
			off: repeatLegs(3, legSpec{odds: -110, result: parlay.Won, grade: "A"}),
		},
		{
			name:     "sniper",
			tagKey:   "sniper",
			wantTone: record.TagTonePraise,
			// 2/4 at +120 = 50.0%
			on: append(
				repeatLegs(2, legSpec{odds: 120, result: parlay.Won}),
				repeatLegs(2, legSpec{odds: 120, result: parlay.Lost})...,
			),
			// 2/2 at +120 (< 3 min sample)
			off: repeatLegs(2, legSpec{odds: 120, result: parlay.Won}),
		},
		{
			name:     "the_backpack",
			tagKey:   "the_backpack",
			wantTone: record.TagTonePraise,
			// 13/20 = 65.0%, 0 solo kills
			on: append(
				repeatLegs(13, legSpec{odds: -110, result: parlay.Won}),
				repeatLegs(7, legSpec{odds: -110, result: parlay.Lost})...,
			),
			// 7/8 = 87.5%, but 1 solo kill -> disqualified
			off: append(
				repeatLegs(7, legSpec{odds: -110, result: parlay.Won}),
				legSpec{odds: -110, result: parlay.Lost, soloKill: true},
			),
		},
		{
			name:     "heater",
			tagKey:   "heater",
			wantTone: record.TagTonePraise,
			// 5-leg active win streak (with PUSH in middle)
			on: []legSpec{
				{odds: -110, result: parlay.Lost},
				{odds: -110, result: parlay.Won},
				{odds: -110, result: parlay.Won},
				{odds: -110, result: parlay.Push},
				{odds: -110, result: parlay.Won},
				{odds: -110, result: parlay.Won},
				{odds: -110, result: parlay.Won},
			},
			// 4-leg active win streak (below 5)
			off: repeatLegs(4, legSpec{odds: -110, result: parlay.Won}),
		},
		{
			name:     "told_you_so",
			tagKey:   "told_you_so",
			wantTone: record.TagTonePraise,
			// 3/5 net-downvoted = 60.0%
			on: append(
				repeatLegs(3, legSpec{odds: -110, result: parlay.Won, netVotes: -1}),
				repeatLegs(2, legSpec{odds: -110, result: parlay.Lost, netVotes: -2})...,
			),
			// 2/4 net-downvoted = 50.0% (< 60.0%)
			off: append(
				repeatLegs(2, legSpec{odds: -110, result: parlay.Won, netVotes: -1}),
				repeatLegs(2, legSpec{odds: -110, result: parlay.Lost, netVotes: -1})...,
			),
		},
		{
			name:     "parlay_terrorist",
			tagKey:   "parlay_terrorist",
			wantTone: record.TagToneInsult,
			on:       []legSpec{{odds: -110, result: parlay.Lost, soloKill: true}},
			off:      []legSpec{{odds: -110, result: parlay.Lost, soloKill: false}},
		},
		{
			name:     "certified_donkey",
			tagKey:   "certified_donkey",
			wantTone: record.TagToneInsult,
			// 19/50 at +100 (50.0% implied exact) -> 38.0% hit -> -12.0 pts
			on: append(
				repeatLegs(19, legSpec{odds: 100, result: parlay.Won}),
				repeatLegs(31, legSpec{odds: 100, result: parlay.Lost})...,
			),
			// 20/50 at +100 -> 40.0% hit -> -10.0 pts (> -12.0)
			off: append(
				repeatLegs(20, legSpec{odds: 100, result: parlay.Won}),
				repeatLegs(30, legSpec{odds: 100, result: parlay.Lost})...,
			),
		},
		{
			name:     "glue_eater",
			tagKey:   "glue_eater",
			wantTone: record.TagToneInsult,
			// 7/20 A/B = 35.0%
			on: append(
				repeatLegs(7, legSpec{odds: -110, result: parlay.Won, grade: "A"}),
				repeatLegs(13, legSpec{odds: -110, result: parlay.Lost, grade: "B"})...,
			),
			// 2/5 A/B = 40.0% (> 35.0%)
			off: append(
				repeatLegs(2, legSpec{odds: -110, result: parlay.Won, grade: "A"}),
				repeatLegs(3, legSpec{odds: -110, result: parlay.Lost, grade: "B"})...,
			),
		},
		{
			name:     "vegas_favorite_idiot",
			tagKey:   "vegas_favorite_idiot",
			wantTone: record.TagToneInsult,
			// 1/5 at +120 = 20.0%
			on: append(
				[]legSpec{{odds: 120, result: parlay.Won}},
				repeatLegs(4, legSpec{odds: 120, result: parlay.Lost})...,
			),
			// 1/4 at +120 = 25.0% (> 20.0%)
			off: append(
				[]legSpec{{odds: 120, result: parlay.Won}},
				repeatLegs(3, legSpec{odds: 120, result: parlay.Lost})...,
			),
		},
		{
			name:     "scared_money",
			tagKey:   "scared_money",
			wantTone: record.TagToneInsult,
			on:       repeatLegs(6, legSpec{odds: -220, result: parlay.Won}),
			off:      repeatLegs(6, legSpec{odds: -219, result: parlay.Won}),
		},
		{
			name:     "stinker",
			tagKey:   "stinker",
			wantTone: record.TagToneInsult,
			// 5-leg active loss streak (with VOID in middle)
			on: []legSpec{
				{odds: -110, result: parlay.Won},
				{odds: -110, result: parlay.Lost},
				{odds: -110, result: parlay.Lost},
				{odds: -110, result: parlay.Void},
				{odds: -110, result: parlay.Lost},
				{odds: -110, result: parlay.Lost},
				{odds: -110, result: parlay.Lost},
			},
			// 5-leg loss streak broken by final WIN
			off: append(
				repeatLegs(5, legSpec{odds: -110, result: parlay.Lost}),
				legSpec{odds: -110, result: parlay.Won},
			),
		},
		{
			name:     "fraud",
			tagKey:   "fraud",
			wantTone: record.TagToneInsult,
			// 7/20 net-upvoted = 35.0%
			on: append(
				repeatLegs(7, legSpec{odds: -110, result: parlay.Won, netVotes: 1}),
				repeatLegs(13, legSpec{odds: -110, result: parlay.Lost, netVotes: 2})...,
			),
			// 2/5 net-upvoted = 40.0% (> 35.0%)
			off: append(
				repeatLegs(2, legSpec{odds: -110, result: parlay.Won, netVotes: 1}),
				repeatLegs(3, legSpec{odds: -110, result: parlay.Lost, netVotes: 1})...,
			),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			onRow := buildForLegs(tc.on)
			tag, ok := findTag(onRow.Tags, tc.tagKey)
			require.True(t, ok, "expected tag %q to fire on qualifying input", tc.tagKey)
			assert.Equal(t, tc.wantTone, tag.Tone)
			assert.NotEmpty(t, tag.Label)
			assert.NotEmpty(t, tag.Rule)
			assert.NotEmpty(t, tag.Explanation)

			offRow := buildForLegs(tc.off)
			_, offOK := findTag(offRow.Tags, tc.tagKey)
			assert.False(t, offOK, "expected tag %q not to fire below threshold", tc.tagKey)
		})
	}
}

func TestTagOrderingPraiseBeforeInsultAndTerroristFirst(t *testing.T) {
	t.Parallel()

	t0 := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	// Member 1 earns:
	// - Praise: natural, heater (5-leg win streak at end, all D-graded)
	// - Insult: parlay_terrorist (1 solo kill early), glue_eater (0/4 A-graded early)
	// - Scared money too: all odds -250
	slips := make([]record.Slip, 0, 10)
	legs := make([]record.Leg, 0, 10)
	addSlip := func(id int64, res parlay.Result, leg record.Leg) {
		slips = append(slips, record.Slip{
			ID: id, Season: 2026, SeasonType: 2, WeekNum: int(id),
			CreatedAt: t0, Status: "GRADED", Result: res,
		})
		leg.ID = id
		leg.SlipID = id
		leg.UserID = 1
		legs = append(legs, leg)
	}

	// 4 lost A-graded legs (1 solo kill for parlay_terrorist, 3 with a second loser)
	addSlip(1, parlay.Lost, record.Leg{Odds: -250, Result: parlay.Lost, Grade: "A"})
	for id := int64(2); id <= 4; id++ {
		addSlip(id, parlay.Lost, record.Leg{Odds: -250, Result: parlay.Lost, Grade: "A"})
		legs = append(legs, record.Leg{ID: 100 + id, SlipID: id, UserID: 2, Odds: -110, Result: parlay.Lost})
	}
	// 5 won D-graded legs at the end -> natural + heater
	for id := int64(5); id <= 9; id++ {
		addSlip(id, parlay.Won, record.Leg{Odds: -250, Result: parlay.Won, Grade: "D"})
	}

	book := record.Build(2026, slips, legs, []record.Member{
		{ID: 1, Name: "Ada"},
		{ID: 2, Name: "Sam"},
	}, nil)
	ada := rowsByUser(book.Leaderboard)[1]

	gotKeys := make([]string, 0, len(ada.Tags))
	for _, tag := range ada.Tags {
		gotKeys = append(gotKeys, tag.Key)
	}
	assert.Equal(t, []string{
		"natural",
		"heater",
		"parlay_terrorist",
		"certified_donkey",
		"glue_eater",
		"scared_money",
	}, gotKeys)
}

func rowsByUser(rows []record.MemberRow) map[int64]record.MemberRow {
	out := make(map[int64]record.MemberRow, len(rows))
	for _, r := range rows {
		out[r.UserID] = r
	}
	return out
}

func findTag(tags []record.Tag, key string) (record.Tag, bool) {
	for _, tag := range tags {
		if tag.Key == key {
			return tag, true
		}
	}
	return record.Tag{}, false
}
