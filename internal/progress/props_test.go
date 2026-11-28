package progress_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/ingest/espn"
	"github.com/kalverra/parlay-club/internal/parlay"
	"github.com/kalverra/parlay-club/internal/progress"
)

func TestEvaluatePlayerPropsKickoffAnchoring(t *testing.T) {
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
	emptyStatsBox := &espn.BoxScore{
		HasStats: true,
		Players: []espn.PlayerBoxStats{
			{PlayerID: "1", DisplayName: "Josh Allen", TeamAbbrev: "BUF"},
		},
	}

	tests := []struct {
		name string
		leg  db.Leg
	}{
		{
			name: "PASS_YDS OVER",
			leg: db.Leg{
				MarketType: "PASS_YDS",
				PlayerID:   new("1"),
				Side:       new("OVER"),
				Line:       new(255.5),
				Odds:       -110,
				Result:     parlay.Pending,
			},
		},
		{
			name: "PASS_YDS UNDER",
			leg: db.Leg{
				MarketType: "PASS_YDS",
				PlayerID:   new("1"),
				Side:       new("UNDER"),
				Line:       new(255.5),
				Odds:       -115,
				Result:     parlay.Pending,
			},
		},
		{
			name: "RUSH_YDS OVER",
			leg: db.Leg{
				MarketType: "RUSH_YDS",
				PlayerID:   new("1"),
				Side:       new("OVER"),
				Line:       new(42.5),
				Odds:       -110,
				Result:     parlay.Pending,
			},
		},
		{
			name: "REC_YDS OVER",
			leg: db.Leg{
				MarketType: "REC_YDS",
				PlayerID:   new("1"),
				Side:       new("OVER"),
				Line:       new(68.5),
				Odds:       105,
				Result:     parlay.Pending,
			},
		},
		{
			name: "RUSH_ATT OVER",
			leg: db.Leg{
				MarketType: "RUSH_ATT",
				PlayerID:   new("1"),
				Side:       new("OVER"),
				Line:       new(8.5),
				Odds:       -120,
				Result:     parlay.Pending,
			},
		},
		{
			name: "RECEPTIONS OVER",
			leg: db.Leg{
				MarketType: "RECEPTIONS",
				PlayerID:   new("1"),
				Side:       new("OVER"),
				Line:       new(5.5),
				Odds:       -130,
				Result:     parlay.Pending,
			},
		},
		{
			name: "KICKER_PTS OVER",
			leg: db.Leg{
				MarketType: "KICKER_PTS",
				PlayerID:   new("1"),
				Side:       new("OVER"),
				Line:       new(7.5),
				Odds:       -110,
				Result:     parlay.Pending,
			},
		},
		{
			name: "PASS_TDS OVER",
			leg: db.Leg{
				MarketType: "PASS_TDS",
				PlayerID:   new("1"),
				Side:       new("OVER"),
				Line:       new(1.5),
				Odds:       -140,
				Result:     parlay.Pending,
			},
		},
		{
			name: "PASS_TDS UNDER",
			leg: db.Leg{
				MarketType: "PASS_TDS",
				PlayerID:   new("1"),
				Side:       new("UNDER"),
				Line:       new(1.5),
				Odds:       115,
				Result:     parlay.Pending,
			},
		},
		{
			name: "ANYTIME_TD YES",
			leg: db.Leg{
				MarketType: "ANYTIME_TD",
				PlayerID:   new("1"),
				Side:       new("YES"),
				Odds:       135,
				Result:     parlay.Pending,
			},
		},
		{
			name: "ANYTIME_TD NO",
			leg: db.Leg{
				MarketType: "ANYTIME_TD",
				PlayerID:   new("1"),
				Side:       new("NO"),
				Odds:       -165,
				Result:     parlay.Pending,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := progress.EvaluateLeg(tt.leg, kickoffGame, emptyStatsBox)
			assert.Equal(t, progress.StateLive, got.State)
			assert.True(t, got.HasLive)
			assert.True(t, got.Counts)
			assert.InDelta(t, got.PreGameProb, got.LiveProb, 1e-4)
		})
	}
}

func TestEvaluatePlayerPropsYardageNonMonotonicity(t *testing.T) {
	t.Parallel()

	liveGame := &db.Game{
		HomeTeam: "BUF",
		AwayTeam: "KC",
		Status:   db.GameLive,
		Period:   new(3),
		Clock:    new("6:00"),
	}
	box := &espn.BoxScore{
		HasStats: true,
		Players: []espn.PlayerBoxStats{
			{
				PlayerID:    "17",
				DisplayName: "Josh Allen",
				TeamAbbrev:  "BUF",
				Passing:     espn.PassingStats{PassingYards: 280},
				Rushing:     espn.RushingStats{RushingYards: 62},
				Receiving:   espn.ReceivingStats{ReceivingYards: 95},
			},
		},
	}

	tests := []struct {
		name      string
		market    string
		side      string
		line      float64
		wantPct   float64
		wantLabel string
	}{
		{
			name:      "PASS_YDS OVER stays live at 100%",
			market:    "PASS_YDS",
			side:      "OVER",
			line:      250.5,
			wantPct:   100,
			wantLabel: "280 / 250.5 yds",
		},
		{
			name:      "PASS_YDS UNDER stays live at 0%",
			market:    "PASS_YDS",
			side:      "UNDER",
			line:      250.5,
			wantPct:   0,
			wantLabel: "280 / 250.5 yds",
		},
		{
			name:      "RUSH_YDS OVER stays live at 100%",
			market:    "RUSH_YDS",
			side:      "OVER",
			line:      45.5,
			wantPct:   100,
			wantLabel: "62 / 45.5 yds",
		},
		{
			name:      "REC_YDS UNDER stays live at 0%",
			market:    "REC_YDS",
			side:      "UNDER",
			line:      80.5,
			wantPct:   0,
			wantLabel: "95 / 80.5 yds",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			leg := db.Leg{
				MarketType: tt.market,
				PlayerID:   new("17"),
				Side:       new(tt.side),
				Line:       new(tt.line),
				Odds:       -110,
				Result:     parlay.Pending,
			}
			got := progress.EvaluateLeg(leg, liveGame, box)
			assert.Equal(t, progress.StateLive, got.State)
			assert.InDelta(t, tt.wantPct, got.ProgressPct, 1e-6)
			assert.Equal(t, tt.wantLabel, got.StatLabel)
			assert.LessOrEqual(t, got.LiveProb, 0.99)
			assert.GreaterOrEqual(t, got.LiveProb, 0.01)
		})
	}
}

func TestEvaluatePlayerPropsMonotonicEarlySettlement(t *testing.T) {
	t.Parallel()

	liveGame := &db.Game{
		HomeTeam: "BUF",
		AwayTeam: "KC",
		Status:   db.GameLive,
		Period:   new(3),
		Clock:    new("4:00"),
	}
	box := &espn.BoxScore{
		HasStats: true,
		Players: []espn.PlayerBoxStats{
			{
				PlayerID:    "17",
				DisplayName: "Josh Allen",
				TeamAbbrev:  "BUF",
				Passing:     espn.PassingStats{PassingTDs: 3},
				Rushing:     espn.RushingStats{RushingAttempts: 12, RushingTDs: 1},
				Receiving:   espn.ReceivingStats{Receptions: 7},
				Kicking:     espn.KickingStats{FieldGoalsMade: 2, ExtraPointsMade: 3},
			},
		},
	}

	tests := []struct {
		name      string
		market    string
		side      string
		line      *float64
		wantState progress.State
		wantProb  float64
		wantPct   float64
		wantLabel string
	}{
		{
			name:      "RUSH_ATT OVER cashes",
			market:    "RUSH_ATT",
			side:      "OVER",
			line:      new(10.5),
			wantState: progress.StateCashed,
			wantProb:  1.0,
			wantPct:   100,
			wantLabel: "12 / 10.5 att",
		},
		{
			name:      "RECEPTIONS UNDER busts",
			market:    "RECEPTIONS",
			side:      "UNDER",
			line:      new(5.5),
			wantState: progress.StateBusted,
			wantProb:  0.0,
			wantPct:   0,
			wantLabel: "7 / 5.5 rec",
		},
		{
			name:      "KICKER_PTS OVER cashes via FG+XP fallback",
			market:    "KICKER_PTS",
			side:      "OVER",
			line:      new(8.5),
			wantState: progress.StateCashed,
			wantProb:  1.0,
			wantPct:   100,
			wantLabel: "9 / 8.5 pts",
		},
		{
			name:      "PASS_TDS OVER cashes",
			market:    "PASS_TDS",
			side:      "OVER",
			line:      new(2.5),
			wantState: progress.StateCashed,
			wantProb:  1.0,
			wantPct:   100,
			wantLabel: "3 / 2.5 TD",
		},
		{
			name:      "ANYTIME_TD YES cashes",
			market:    "ANYTIME_TD",
			side:      "YES",
			wantState: progress.StateCashed,
			wantProb:  1.0,
			wantPct:   100,
			wantLabel: "1 TD",
		},
		{
			name:      "ANYTIME_TD NO busts",
			market:    "ANYTIME_TD",
			side:      "NO",
			wantState: progress.StateBusted,
			wantProb:  0.0,
			wantPct:   0,
			wantLabel: "1 TD",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			leg := db.Leg{
				MarketType: tt.market,
				PlayerID:   new("17"),
				Side:       new(tt.side),
				Line:       tt.line,
				Odds:       -110,
				Result:     parlay.Pending,
			}
			got := progress.EvaluateLeg(leg, liveGame, box)
			assert.Equal(t, tt.wantState, got.State)
			assert.InDelta(t, tt.wantProb, got.LiveProb, 1e-9)
			assert.InDelta(t, tt.wantPct, got.ProgressPct, 1e-9)
			assert.Equal(t, tt.wantLabel, got.StatLabel)
			assert.True(t, got.Counts)
			assert.True(t, got.HasLive)
		})
	}
}

func TestEvaluatePlayerPropsAbsentAndPendingBoxScore(t *testing.T) {
	t.Parallel()

	liveGame := &db.Game{
		HomeTeam: "BUF",
		AwayTeam: "KC",
		Status:   db.GameLive,
		Period:   new(2),
		Clock:    new("8:10"),
	}

	t.Run("absent player in populated live box score evaluates as 0, never VOID", func(t *testing.T) {
		t.Parallel()
		box := &espn.BoxScore{
			HasStats: true,
			Players: []espn.PlayerBoxStats{
				{PlayerID: "99", DisplayName: "Other Player", TeamAbbrev: "BUF"},
			},
		}
		leg := db.Leg{
			MarketType: "REC_YDS",
			PlayerName: new("Dalton Kincaid (BUF)"),
			Side:       new("OVER"),
			Line:       new(45.5),
			Odds:       -110,
			Result:     parlay.Pending,
		}
		got := progress.EvaluateLeg(leg, liveGame, box)
		require.Equal(t, progress.StateLive, got.State)
		assert.Equal(t, "0 / 45.5 yds", got.StatLabel)
		assert.True(t, got.Counts)
		assert.True(t, got.HasLive)
		assert.Less(t, got.LiveProb, got.PreGameProb)
	})

	t.Run("missing or unpopulated box score shows stats pending at pre-game prob", func(t *testing.T) {
		t.Parallel()
		leg := db.Leg{
			MarketType: "PASS_YDS",
			PlayerID:   new("17"),
			Side:       new("OVER"),
			Line:       new(250.5),
			Odds:       -110,
			Result:     parlay.Pending,
		}
		for _, box := range []*espn.BoxScore{nil, {HasStats: false}} {
			got := progress.EvaluateLeg(leg, liveGame, box)
			assert.Equal(t, progress.StateLive, got.State)
			assert.Equal(t, "Live · stats pending", got.StatLabel)
			assert.Equal(t, "Q2 8:10", got.ClockLabel)
			assert.InDelta(t, got.PreGameProb, got.LiveProb, 1e-9)
			assert.True(t, got.HasLive)
		}
	})

	t.Run("unlinked leg with nil game stays scheduled", func(t *testing.T) {
		t.Parallel()
		leg := db.Leg{
			MarketType: "PASS_YDS",
			PlayerID:   new("17"),
			Side:       new("OVER"),
			Line:       new(250.5),
			Odds:       120,
			Result:     parlay.Pending,
		}
		got := progress.EvaluateLeg(leg, nil, nil)
		assert.Equal(t, progress.StateScheduled, got.State)
		assert.False(t, got.HasLive)
		assert.InDelta(t, parlay.ImpliedProb(120), got.LiveProb, 1e-9)
	})
}

func TestEvaluateSlip(t *testing.T) {
	t.Parallel()

	t.Run("empty slip", func(t *testing.T) {
		t.Parallel()
		got := progress.EvaluateSlip(nil)
		assert.False(t, got.Show)
		assert.Equal(t, progress.StateScheduled, got.State)
	})

	t.Run("all scheduled legs hide live card", func(t *testing.T) {
		t.Parallel()
		got := progress.EvaluateSlip([]progress.LegProgress{
			{State: progress.StateScheduled, PreGameProb: 0.5, LiveProb: 0.5, Counts: true},
			{State: progress.StateScheduled, PreGameProb: 0.5, LiveProb: 0.5, Counts: true},
		})
		assert.False(t, got.Show)
		assert.Equal(t, progress.StateScheduled, got.State)
		assert.InDelta(t, 0.25, got.PreGameProb, 1e-9)
		assert.InDelta(t, 0.25, got.LiveProb, 1e-9)
	})

	t.Run("mixed live cashed scheduled with push and void excluded", func(t *testing.T) {
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
			{State: progress.StateLive, ProgressPct: 60, PreGameProb: 0.5, LiveProb: 0.8, Counts: true, HasLive: true},
			{
				State:       progress.StateScheduled,
				ProgressPct: 0,
				PreGameProb: 0.5,
				LiveProb:    0.5,
				Counts:      true,
				HasLive:     false,
			},
			{State: progress.StatePush, Counts: false, HasLive: true},
			{State: progress.StateVoid, Counts: false, HasLive: true},
		})
		assert.True(t, got.Show)
		assert.Equal(t, progress.StateLive, got.State)
		assert.Equal(t, "on_track", got.Tone)
		assert.Equal(t, "1/3 hit · 1 live · 1 upcoming", got.Summary)
		assert.Equal(t, 1, got.HitCount)
		assert.Equal(t, 1, got.LiveCount)
		assert.Equal(t, 1, got.ScheduledCount)
		assert.Equal(t, 0, got.LostCount)
		assert.Equal(t, 3, got.TotalCount)
		assert.InDelta(t, 0.125, got.PreGameProb, 1e-9)
		assert.InDelta(t, 0.4, got.LiveProb, 1e-9)
		assert.InDelta(t, (100.0+60.0+0.0)/3.0, got.ProgressPct, 1e-9)
	})

	t.Run("busted parlay collapses live prob to 0", func(t *testing.T) {
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
			{State: progress.StateBusted, ProgressPct: 0, PreGameProb: 0.5, LiveProb: 0.0, Counts: true, HasLive: true},
			{State: progress.StateLive, ProgressPct: 80, PreGameProb: 0.5, LiveProb: 0.9, Counts: true, HasLive: true},
		})
		assert.True(t, got.Show)
		assert.Equal(t, progress.StateBusted, got.State)
		assert.Equal(t, "lost", got.Tone)
		assert.Zero(t, got.LiveProb)
		assert.Equal(t, 1, got.LostCount)
		assert.Equal(t, "1/3 hit · 1 lost", got.Summary)
	})

	t.Run("all cashed parlay reaches 100%", func(t *testing.T) {
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
				State:       progress.StateCashed,
				ProgressPct: 100,
				PreGameProb: 0.4,
				LiveProb:    1.0,
				Counts:      true,
				HasLive:     true,
			},
			{State: progress.StateVoid, Counts: false, HasLive: true},
		})
		assert.True(t, got.Show)
		assert.Equal(t, progress.StateCashed, got.State)
		assert.Equal(t, "won", got.Tone)
		assert.InDelta(t, 1.0, got.LiveProb, 1e-9)
		assert.InDelta(t, 100.0, got.ProgressPct, 1e-9)
		assert.Equal(t, "2/2 hit", got.Summary)
	})

	t.Run("all pushed or voided legs", func(t *testing.T) {
		t.Parallel()
		got := progress.EvaluateSlip([]progress.LegProgress{
			{State: progress.StatePush, Counts: false, HasLive: true},
			{State: progress.StateVoid, Counts: false, HasLive: true},
		})
		assert.True(t, got.Show)
		assert.Equal(t, progress.StatePush, got.State)
		assert.Equal(t, "neutral", got.Tone)
		assert.Equal(t, "Push", got.Summary)
	})
}
