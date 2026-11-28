package progress_test

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/parlay"
	"github.com/kalverra/parlay-club/internal/progress"
)

func TestRemainingFractionAndClockLabel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		status    db.GameStatus
		period    *int
		clock     *string
		wantFrac  float64
		wantLabel string
	}{
		{
			name:      "scheduled",
			status:    db.GameScheduled,
			wantFrac:  1.0,
			wantLabel: "",
		},
		{
			name:      "final",
			status:    db.GameFinal,
			period:    new(4),
			clock:     new("0:00"),
			wantFrac:  0.0,
			wantLabel: "Final",
		},
		{
			name:      "kickoff Q1 15:00",
			status:    db.GameLive,
			period:    new(1),
			clock:     new("15:00"),
			wantFrac:  1.0,
			wantLabel: "Q1 15:00",
		},
		{
			name:      "Q3 8:42",
			status:    db.GameLive,
			period:    new(3),
			clock:     new("8:42"),
			wantFrac:  (900.0 + 522.0) / 3600.0,
			wantLabel: "Q3 8:42",
		},
		{
			name:      "halftime label",
			status:    db.GameLive,
			period:    new(2),
			clock:     new("Halftime"),
			wantFrac:  0.50,
			wantLabel: "Halftime",
		},
		{
			name:      "end of 1st",
			status:    db.GameLive,
			period:    new(1),
			clock:     new("End of 1st"),
			wantFrac:  0.75,
			wantLabel: "End of 1st",
		},
		{
			name:      "end of 3rd",
			status:    db.GameLive,
			period:    new(3),
			clock:     new("End of 3rd"),
			wantFrac:  0.25,
			wantLabel: "End of 3rd",
		},
		{
			name:      "OT 10:00",
			status:    db.GameLive,
			period:    new(5),
			clock:     new("10:00"),
			wantFrac:  600.0 / 3600.0,
			wantLabel: "OT 10:00",
		},
		{
			name:      "OT 15:00 clamped to max OT",
			status:    db.GameLive,
			period:    new(5),
			clock:     new("15:00"),
			wantFrac:  0.167,
			wantLabel: "OT 15:00",
		},
		{
			name:      "2OT 0:05 clamped to min live",
			status:    db.GameLive,
			period:    new(6),
			clock:     new("0:05"),
			wantFrac:  0.005,
			wantLabel: "2OT 0:05",
		},
		{
			name:      "missing clock and period",
			status:    db.GameLive,
			wantFrac:  0.50,
			wantLabel: "Live",
		},
		{
			name:      "missing clock with period",
			status:    db.GameLive,
			period:    new(2),
			wantFrac:  0.50,
			wantLabel: "Q2",
		},
		{
			name:      "unparseable clock",
			status:    db.GameLive,
			period:    new(3),
			clock:     new("not-a-clock"),
			wantFrac:  0.50,
			wantLabel: "Q3 not-a-clock",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotFrac := progress.RemainingFraction(tt.status, tt.period, tt.clock)
			assert.InDelta(t, tt.wantFrac, gotFrac, 1e-6)
			gotLabel := progress.ClockLabel(tt.status, tt.period, tt.clock)
			assert.Equal(t, tt.wantLabel, gotLabel)
		})
	}
}

func TestEvaluateLegKickoffAnchoring(t *testing.T) {
	t.Parallel()

	kickoffGame := &db.Game{
		HomeTeam:  "BUF",
		AwayTeam:  "KC",
		Status:    db.GameLive,
		HomeScore: new(0),
		AwayScore: new(0),
		Period:    new(1),
		Clock:     new("15:00"),
	}

	tests := []struct {
		name string
		leg  db.Leg
	}{
		{
			name: "spread favorite",
			leg: db.Leg{
				MarketType: "SPREAD",
				Team:       new("BUF"),
				Line:       new(-3.5),
				Odds:       -110,
				Result:     parlay.Pending,
			},
		},
		{
			name: "spread underdog",
			leg: db.Leg{
				MarketType: "SPREAD",
				Team:       new("KC"),
				Line:       new(7.5),
				Odds:       -115,
				Result:     parlay.Pending,
			},
		},
		{
			name: "moneyline underdog",
			leg: db.Leg{
				MarketType: "ML",
				Team:       new("KC"),
				Odds:       165,
				Result:     parlay.Pending,
			},
		},
		{
			name: "total over",
			leg: db.Leg{
				MarketType: "TOTAL",
				Side:       new("OVER"),
				Line:       new(48.5),
				Odds:       -110,
				Result:     parlay.Pending,
			},
		},
		{
			name: "total under",
			leg: db.Leg{
				MarketType: "TOTAL",
				Side:       new("UNDER"),
				Line:       new(48.5),
				Odds:       -110,
				Result:     parlay.Pending,
			},
		},
		{
			name: "team total over",
			leg: db.Leg{
				MarketType: "TEAM_TOTAL",
				Team:       new("BUF"),
				Side:       new("OVER"),
				Line:       new(26.5),
				Odds:       -120,
				Result:     parlay.Pending,
			},
		},
		{
			name: "team total under",
			leg: db.Leg{
				MarketType: "TEAM_TOTAL",
				Team:       new("KC"),
				Side:       new("UNDER"),
				Line:       new(23.5),
				Odds:       105,
				Result:     parlay.Pending,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := progress.EvaluateLeg(tt.leg, kickoffGame, nil)
			assert.Equal(t, progress.StateLive, got.State)
			assert.True(t, got.HasLive)
			assert.True(t, got.Counts)
			assert.InDelta(t, got.PreGameProb, got.LiveProb, 1e-6)
		})
	}
}

func TestEvaluateLegGameMarketsConvergenceAndLabels(t *testing.T) {
	t.Parallel()

	t.Run("spread leading late converges near 0.99", func(t *testing.T) {
		t.Parallel()
		game := &db.Game{
			HomeTeam:  "BUF",
			AwayTeam:  "KC",
			Status:    db.GameLive,
			HomeScore: new(28),
			AwayScore: new(14),
			Period:    new(4),
			Clock:     new("1:00"),
		}
		leg := db.Leg{
			MarketType: "SPREAD",
			Team:       new("BUF"),
			Line:       new(-3.5),
			Odds:       -110,
			Result:     parlay.Pending,
		}
		got := progress.EvaluateLeg(leg, game, nil)
		assert.Equal(t, progress.StateLive, got.State)
		assert.Equal(t, "on_track", got.Tone)
		assert.Equal(t, "BUF 28-14 (-3.5)", got.StatLabel)
		assert.Equal(t, "Q4 1:00", got.ClockLabel)
		assert.Greater(t, got.LiveProb, 0.95)
		assert.LessOrEqual(t, got.LiveProb, 0.99)
		assert.InDelta(t, got.LiveProb*100, got.ProgressPct, 1e-6)
	})

	t.Run("moneyline trailing late converges near 0.01", func(t *testing.T) {
		t.Parallel()
		game := &db.Game{
			HomeTeam:  "BUF",
			AwayTeam:  "KC",
			Status:    db.GameLive,
			HomeScore: new(10),
			AwayScore: new(27),
			Period:    new(4),
			Clock:     new("1:30"),
		}
		leg := db.Leg{
			MarketType: "ML",
			Team:       new("BUF"),
			Odds:       -150,
			Result:     parlay.Pending,
		}
		got := progress.EvaluateLeg(leg, game, nil)
		assert.Equal(t, progress.StateLive, got.State)
		assert.Equal(t, "trailing", got.Tone)
		assert.Equal(t, "BUF 10-27", got.StatLabel)
		assert.Less(t, got.LiveProb, 0.05)
		assert.GreaterOrEqual(t, got.LiveProb, 0.01)
	})

	t.Run("total under cushion progress bar", func(t *testing.T) {
		t.Parallel()
		game := &db.Game{
			HomeTeam:  "BUF",
			AwayTeam:  "KC",
			Status:    db.GameLive,
			HomeScore: new(10),
			AwayScore: new(10),
			Period:    new(2),
			Clock:     new("Halftime"),
		}
		leg := db.Leg{
			MarketType: "TOTAL",
			Side:       new("UNDER"),
			Line:       new(50.0),
			Odds:       -110,
			Result:     parlay.Pending,
		}
		got := progress.EvaluateLeg(leg, game, nil)
		assert.Equal(t, progress.StateLive, got.State)
		assert.Equal(t, "Halftime", got.ClockLabel)
		assert.Equal(t, "20 / 50 pts", got.StatLabel)
		assert.InDelta(t, 60.0, got.ProgressPct, 1e-6)
	})
}

func TestEvaluateLegMonotonicEarlyCashAndBust(t *testing.T) {
	t.Parallel()

	liveGame := &db.Game{
		HomeTeam:  "BUF",
		AwayTeam:  "KC",
		Status:    db.GameLive,
		HomeScore: new(28),
		AwayScore: new(21),
		Period:    new(3),
		Clock:     new("4:15"),
	}

	t.Run("total over cashes early", func(t *testing.T) {
		t.Parallel()
		leg := db.Leg{
			MarketType: "TOTAL",
			Side:       new("OVER"),
			Line:       new(44.5),
			Odds:       -110,
			Result:     parlay.Pending,
		}
		got := progress.EvaluateLeg(leg, liveGame, nil)
		assert.Equal(t, progress.StateCashed, got.State)
		assert.Equal(t, "won", got.Tone)
		assert.InDelta(t, 1.0, got.LiveProb, 1e-9)
		assert.InDelta(t, 100.0, got.ProgressPct, 1e-9)
		assert.True(t, got.Counts)
		assert.True(t, got.HasLive)
	})

	t.Run("total under busts early", func(t *testing.T) {
		t.Parallel()
		leg := db.Leg{
			MarketType: "TOTAL",
			Side:       new("UNDER"),
			Line:       new(44.5),
			Odds:       -110,
			Result:     parlay.Pending,
		}
		got := progress.EvaluateLeg(leg, liveGame, nil)
		assert.Equal(t, progress.StateBusted, got.State)
		assert.Equal(t, "lost", got.Tone)
		assert.Zero(t, got.LiveProb)
		assert.Zero(t, got.ProgressPct)
		assert.True(t, got.Counts)
		assert.True(t, got.HasLive)
	})

	t.Run("team total over cashes early", func(t *testing.T) {
		t.Parallel()
		leg := db.Leg{
			MarketType: "TEAM_TOTAL",
			Team:       new("BUF"),
			Side:       new("OVER"),
			Line:       new(24.5),
			Odds:       -115,
			Result:     parlay.Pending,
		}
		got := progress.EvaluateLeg(leg, liveGame, nil)
		assert.Equal(t, progress.StateCashed, got.State)
		assert.Equal(t, "won", got.Tone)
		assert.InDelta(t, 1.0, got.LiveProb, 1e-9)
		assert.InDelta(t, 100.0, got.ProgressPct, 1e-9)
		assert.Equal(t, "BUF 28 / 24.5 pts", got.StatLabel)
	})

	t.Run("team total under busts early", func(t *testing.T) {
		t.Parallel()
		leg := db.Leg{
			MarketType: "TEAM_TOTAL",
			Team:       new("BUF"),
			Side:       new("UNDER"),
			Line:       new(24.5),
			Odds:       -105,
			Result:     parlay.Pending,
		}
		got := progress.EvaluateLeg(leg, liveGame, nil)
		assert.Equal(t, progress.StateBusted, got.State)
		assert.Equal(t, "lost", got.Tone)
		assert.Zero(t, got.LiveProb)
		assert.Zero(t, got.ProgressPct)
	})

	t.Run("exactly on line stays live", func(t *testing.T) {
		t.Parallel()
		leg := db.Leg{
			MarketType: "TOTAL",
			Side:       new("OVER"),
			Line:       new(49.0),
			Odds:       -110,
			Result:     parlay.Pending,
		}
		got := progress.EvaluateLeg(leg, liveGame, nil)
		assert.Equal(t, progress.StateLive, got.State)
		assert.False(t, math.IsNaN(got.LiveProb))
		assert.Less(t, got.LiveProb, 1.0)
	})
}

func TestEvaluateLegSettledAndFallbacks(t *testing.T) {
	t.Parallel()

	finalGame := &db.Game{
		HomeTeam: "BUF",
		AwayTeam: "KC",
		Status:   db.GameFinal,
	}

	settledCases := []struct {
		result   parlay.Result
		wantSt   progress.State
		wantPct  float64
		wantProb float64
		wantCnt  bool
		wantTone string
	}{
		{parlay.Won, progress.StateCashed, 100, 1.0, true, "won"},
		{parlay.Lost, progress.StateBusted, 0, 0.0, true, "lost"},
		{parlay.Push, progress.StatePush, 0, 0.0, false, "neutral"},
		{parlay.Void, progress.StateVoid, 0, 0.0, false, "neutral"},
	}

	for _, tc := range settledCases {
		t.Run(string(tc.result), func(t *testing.T) {
			t.Parallel()
			leg := db.Leg{MarketType: "SPREAD", Odds: -110, Result: tc.result}
			got := progress.EvaluateLeg(leg, finalGame, nil)
			assert.Equal(t, tc.wantSt, got.State)
			assert.InDelta(t, tc.wantPct, got.ProgressPct, 1e-9)
			assert.InDelta(t, tc.wantProb, got.LiveProb, 1e-9)
			assert.Equal(t, tc.wantCnt, got.Counts)
			assert.Equal(t, tc.wantTone, got.Tone)
			assert.Equal(t, "Final", got.ClockLabel)
			assert.True(t, got.HasLive)
		})
	}

	t.Run("scheduled game", func(t *testing.T) {
		t.Parallel()
		sched := &db.Game{Status: db.GameScheduled}
		leg := db.Leg{MarketType: "ML", Odds: 150, Result: parlay.Pending}
		got := progress.EvaluateLeg(leg, sched, nil)
		assert.Equal(t, progress.StateScheduled, got.State)
		assert.False(t, got.HasLive)
		assert.InDelta(t, parlay.ImpliedProb(150), got.LiveProb, 1e-9)
	})

	t.Run("OTHER market on live game", func(t *testing.T) {
		t.Parallel()
		live := &db.Game{Status: db.GameLive, Period: new(2), Clock: new("5:00")}
		leg := db.Leg{MarketType: "OTHER", Odds: 200, Result: parlay.Pending}
		got := progress.EvaluateLeg(leg, live, nil)
		assert.Equal(t, progress.StateUnavailable, got.State)
		assert.False(t, got.HasLive)
		assert.InDelta(t, parlay.ImpliedProb(200), got.LiveProb, 1e-9)
	})

	t.Run("missing scores on live game", func(t *testing.T) {
		t.Parallel()
		live := &db.Game{HomeTeam: "BUF", AwayTeam: "KC", Status: db.GameLive}
		leg := db.Leg{MarketType: "SPREAD", Team: new("BUF"), Line: new(-3.5), Odds: -110, Result: parlay.Pending}
		got := progress.EvaluateLeg(leg, live, nil)
		require.Equal(t, progress.StateUnavailable, got.State)
		assert.False(t, got.HasLive)
		assert.InDelta(t, parlay.ImpliedProb(-110), got.LiveProb, 1e-9)
	})
}

func TestEvaluateLegGameProgressAndProbDelta(t *testing.T) {
	t.Parallel()

	t.Run("game progress pct across scheduled live and final", func(t *testing.T) {
		t.Parallel()
		leg := db.Leg{MarketType: "ML", Team: new("BUF"), Odds: -110, Result: parlay.Pending}

		assert.Zero(t, progress.EvaluateLeg(leg, nil, nil).GameProgressPct)
		assert.Zero(t, progress.EvaluateLeg(leg, &db.Game{Status: db.GameScheduled}, nil).GameProgressPct)

		q1Start := &db.Game{
			HomeTeam: "BUF", AwayTeam: "KC", Status: db.GameLive,
			HomeScore: new(0), AwayScore: new(0), Period: new(1), Clock: new("15:00"),
		}
		assert.InDelta(t, 0.0, progress.EvaluateLeg(leg, q1Start, nil).GameProgressPct, 1e-6)

		halftime := &db.Game{
			HomeTeam: "BUF", AwayTeam: "KC", Status: db.GameLive,
			HomeScore: new(14), AwayScore: new(14), Period: new(2), Clock: new("Halftime"),
		}
		assert.InDelta(t, 50.0, progress.EvaluateLeg(leg, halftime, nil).GameProgressPct, 1e-6)

		q3 := &db.Game{
			HomeTeam: "BUF", AwayTeam: "KC", Status: db.GameLive,
			HomeScore: new(21), AwayScore: new(17), Period: new(3), Clock: new("8:42"),
		}
		wantQ3 := (1.0 - (900.0+522.0)/3600.0) * 100.0
		assert.InDelta(t, wantQ3, progress.EvaluateLeg(leg, q3, nil).GameProgressPct, 1e-6)

		ot := &db.Game{
			HomeTeam: "BUF", AwayTeam: "KC", Status: db.GameLive,
			HomeScore: new(24), AwayScore: new(24), Period: new(5), Clock: new("10:00"),
		}
		wantOT := (1.0 - 600.0/3600.0) * 100.0
		assert.InDelta(t, wantOT, progress.EvaluateLeg(leg, ot, nil).GameProgressPct, 1e-6)

		finalGame := &db.Game{HomeTeam: "BUF", AwayTeam: "KC", Status: db.GameFinal}
		assert.InDelta(t, 100.0, progress.EvaluateLeg(leg, finalGame, nil).GameProgressPct, 1e-6)
	})

	t.Run("prob delta and trend across every leg state", func(t *testing.T) {
		t.Parallel()
		pre := parlay.ImpliedProb(-110)

		sched := progress.EvaluateLeg(
			db.Leg{MarketType: "ML", Team: new("BUF"), Odds: -110, Result: parlay.Pending},
			&db.Game{Status: db.GameScheduled},
			nil,
		)
		assert.InDelta(t, pre, sched.PreGameProb, 1e-9)
		assert.InDelta(t, pre, sched.LiveProb, 1e-9)
		assert.Zero(t, sched.ProbDelta)
		assert.Equal(t, "flat", sched.ProbTrend)

		liveUp := progress.EvaluateLeg(
			db.Leg{MarketType: "SPREAD", Team: new("BUF"), Line: new(-3.5), Odds: -110, Result: parlay.Pending},
			&db.Game{
				HomeTeam: "BUF", AwayTeam: "KC", Status: db.GameLive,
				HomeScore: new(28), AwayScore: new(10), Period: new(4), Clock: new("2:00"),
			},
			nil,
		)
		assert.InDelta(t, liveUp.LiveProb-liveUp.PreGameProb, liveUp.ProbDelta, 1e-9)
		assert.Greater(t, liveUp.ProbDelta, 0.005)
		assert.Equal(t, "up", liveUp.ProbTrend)

		liveDown := progress.EvaluateLeg(
			db.Leg{MarketType: "ML", Team: new("BUF"), Odds: -110, Result: parlay.Pending},
			&db.Game{
				HomeTeam: "BUF", AwayTeam: "KC", Status: db.GameLive,
				HomeScore: new(7), AwayScore: new(24), Period: new(4), Clock: new("2:00"),
			},
			nil,
		)
		assert.InDelta(t, liveDown.LiveProb-liveDown.PreGameProb, liveDown.ProbDelta, 1e-9)
		assert.Less(t, liveDown.ProbDelta, -0.005)
		assert.Equal(t, "down", liveDown.ProbTrend)

		liveFlat := progress.EvaluateLeg(
			db.Leg{MarketType: "OTHER", Odds: -110, Result: parlay.Pending},
			&db.Game{Status: db.GameLive, Period: new(2), Clock: new("Halftime")},
			nil,
		)
		assert.Equal(t, progress.StateUnavailable, liveFlat.State)
		assert.InDelta(t, 50.0, liveFlat.GameProgressPct, 1e-6)
		assert.Zero(t, liveFlat.ProbDelta)
		assert.Equal(t, "flat", liveFlat.ProbTrend)

		cashed := progress.EvaluateLeg(
			db.Leg{MarketType: "ML", Team: new("BUF"), Odds: -110, Result: parlay.Won},
			&db.Game{Status: db.GameFinal},
			nil,
		)
		assert.InDelta(t, 1.0, cashed.LiveProb, 1e-9)
		assert.InDelta(t, 1.0-pre, cashed.ProbDelta, 1e-9)
		assert.Equal(t, "up", cashed.ProbTrend)
		assert.InDelta(t, 100.0, cashed.GameProgressPct, 1e-6)

		busted := progress.EvaluateLeg(
			db.Leg{MarketType: "ML", Team: new("BUF"), Odds: -110, Result: parlay.Lost},
			&db.Game{Status: db.GameFinal},
			nil,
		)
		assert.Zero(t, busted.LiveProb)
		assert.InDelta(t, -pre, busted.ProbDelta, 1e-9)
		assert.Equal(t, "down", busted.ProbTrend)

		pushed := progress.EvaluateLeg(
			db.Leg{MarketType: "SPREAD", Odds: -110, Result: parlay.Push},
			&db.Game{Status: db.GameFinal},
			nil,
		)
		assert.Zero(t, pushed.ProbDelta)
		assert.Equal(t, "flat", pushed.ProbTrend)

		voided := progress.EvaluateLeg(
			db.Leg{MarketType: "SPREAD", Odds: -110, Result: parlay.Void},
			&db.Game{Status: db.GameFinal},
			nil,
		)
		assert.Zero(t, voided.ProbDelta)
		assert.Equal(t, "flat", voided.ProbTrend)
	})
}

func TestEvaluateSlipScheduledCountAndProbDelta(t *testing.T) {
	t.Parallel()

	t.Run("mixed hit live and upcoming summary and trend up", func(t *testing.T) {
		t.Parallel()
		got := progress.EvaluateSlip([]progress.LegProgress{
			{
				State:       progress.StateCashed,
				ProgressPct: 100,
				PreGameProb: 0.5,
				LiveProb:    1.0,
				Counts:      true,
				HasLive:     true,
			},
			{
				State:       progress.StateLive,
				ProgressPct: 70,
				PreGameProb: 0.5,
				LiveProb:    0.8,
				Counts:      true,
				HasLive:     true,
			},
			{
				State:       progress.StateLive,
				ProgressPct: 60,
				PreGameProb: 0.5,
				LiveProb:    0.7,
				Counts:      true,
				HasLive:     true,
			},
			{State: progress.StateScheduled, PreGameProb: 0.5, LiveProb: 0.5, Counts: true},
		})
		assert.Equal(t, "1/4 hit · 2 live · 1 upcoming", got.Summary)
		assert.Equal(t, 1, got.ScheduledCount)
		assert.InDelta(t, 0.28-0.0625, got.ProbDelta, 1e-9)
		assert.Equal(t, "up", got.ProbTrend)
	})

	t.Run("hit and upcoming with no live legs", func(t *testing.T) {
		t.Parallel()
		got := progress.EvaluateSlip([]progress.LegProgress{
			{
				State:       progress.StateCashed,
				ProgressPct: 100,
				PreGameProb: 0.5,
				LiveProb:    1.0,
				Counts:      true,
				HasLive:     true,
			},
			{State: progress.StateScheduled, PreGameProb: 0.5, LiveProb: 0.5, Counts: true},
		})
		assert.Equal(t, "1/2 hit · 1 upcoming", got.Summary)
		assert.Equal(t, 1, got.ScheduledCount)
		assert.InDelta(t, 0.25, got.ProbDelta, 1e-9)
		assert.Equal(t, "up", got.ProbTrend)
	})

	t.Run("deadband treats tiny prob delta as flat", func(t *testing.T) {
		t.Parallel()
		got := progress.EvaluateSlip([]progress.LegProgress{
			{
				State:       progress.StateLive,
				ProgressPct: 50,
				PreGameProb: 0.500,
				LiveProb:    0.503,
				Counts:      true,
				HasLive:     true,
			},
		})
		assert.InDelta(t, 0.003, got.ProbDelta, 1e-9)
		assert.Equal(t, "flat", got.ProbTrend)
	})

	t.Run("trailing live slip trends down", func(t *testing.T) {
		t.Parallel()
		got := progress.EvaluateSlip([]progress.LegProgress{
			{State: progress.StateLive, ProgressPct: 20, PreGameProb: 0.5, LiveProb: 0.2, Counts: true, HasLive: true},
		})
		assert.InDelta(t, -0.3, got.ProbDelta, 1e-9)
		assert.Equal(t, "down", got.ProbTrend)
	})

	t.Run("all scheduled counts upcoming legs with flat trend", func(t *testing.T) {
		t.Parallel()
		got := progress.EvaluateSlip([]progress.LegProgress{
			{State: progress.StateScheduled, PreGameProb: 0.5, LiveProb: 0.5, Counts: true},
			{State: progress.StateScheduled, PreGameProb: 0.5, LiveProb: 0.5, Counts: true},
		})
		assert.Equal(t, 2, got.ScheduledCount)
		assert.Zero(t, got.ProbDelta)
		assert.Equal(t, "flat", got.ProbTrend)
	})
}
