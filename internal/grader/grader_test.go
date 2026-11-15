package grader_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kalverra/parlay-club/internal/grader"
	"github.com/kalverra/parlay-club/internal/ingest/espn"
	"github.com/kalverra/parlay-club/internal/parlay"
)

func sampleBox() *espn.BoxScore {
	return &espn.BoxScore{
		Completed: true,
		HasStats:  true,
		Players: []espn.PlayerBoxStats{
			{
				PlayerID:    "3918298",
				DisplayName: "Josh Allen",
				TeamAbbrev:  "BUF",
				Passing:     espn.PassingStats{PassingYards: 280, PassingTDs: 2},
				Rushing:     espn.RushingStats{RushingAttempts: 6, RushingYards: 32, RushingTDs: 0},
			},
			{
				PlayerID:    "4379399",
				DisplayName: "James Cook III",
				TeamAbbrev:  "BUF",
				Rushing:     espn.RushingStats{RushingAttempts: 18, RushingYards: 95, RushingTDs: 1},
				// Appears in rushing only; receiving stats are 0 (played, not DNP).
			},
			{
				PlayerID:    "4241457",
				DisplayName: "Tyreek Hill",
				TeamAbbrev:  "MIA",
				Receiving:   espn.ReceivingStats{Receptions: 7, ReceivingYards: 112, ReceivingTDs: 1},
			},
			{
				PlayerID:    "4035687",
				DisplayName: "Braxton Berrios",
				TeamAbbrev:  "MIA",
				OtherTDs:    1, // punt/kick return TD
			},
			{
				PlayerID:    "3051909",
				DisplayName: "Tyler Bass",
				TeamAbbrev:  "BUF",
				Kicking:     espn.KickingStats{FieldGoalsMade: 2, ExtraPointsMade: 3, TotalKickingPoints: 9},
			},
			{
				PlayerID:    "4360234",
				DisplayName: "Jason Sanders",
				TeamAbbrev:  "MIA",
				// TotalKickingPoints omitted (0): fallback 2*3 + 4 = 10.
				Kicking: espn.KickingStats{FieldGoalsMade: 2, ExtraPointsMade: 4},
			},
		},
	}
}

func finalGame(homeScore int) grader.GameScore {
	return grader.GameScore{
		HomeTeam:  "BUF",
		AwayTeam:  "MIA",
		HomeScore: new(homeScore),
		AwayScore: new(20),
		Final:     true,
	}
}

func TestGradeLeg_GameAndTeamMarkets(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		leg  grader.LegInput
		game grader.GameScore
		want parlay.Result
	}{
		// Non-final
		{
			name: "non-final game returns pending",
			leg:  grader.LegInput{Market: "ML", Team: "BUF", Side: "TEAM"},
			game: grader.GameScore{
				HomeTeam:  "BUF",
				AwayTeam:  "MIA",
				HomeScore: new(14),
				AwayScore: new(7),
				Final:     false,
			},
			want: parlay.Pending,
		},
		// SPREAD
		{
			name: "spread home favorite wins",
			leg:  grader.LegInput{Market: "SPREAD", Team: "BUF", Side: "TEAM", Line: new(-3.5)},
			game: finalGame(27),
			want: parlay.Won,
		},
		{
			name: "spread home favorite loses",
			leg:  grader.LegInput{Market: "SPREAD", Team: "BUF", Side: "TEAM", Line: new(-7.5)},
			game: finalGame(27),
			want: parlay.Lost,
		},
		{
			name: "spread integer push",
			leg:  grader.LegInput{Market: "SPREAD", Team: "BUF", Side: "TEAM", Line: new(float64(-7))},
			game: finalGame(27),
			want: parlay.Push,
		},
		{
			name: "spread away underdog wins",
			leg:  grader.LegInput{Market: "SPREAD", Team: "MIA", Side: "TEAM", Line: new(7.5)},
			game: finalGame(27),
			want: parlay.Won,
		},
		{
			name: "spread away underdog loses",
			leg:  grader.LegInput{Market: "SPREAD", Team: "MIA", Side: "TEAM", Line: new(6.5)},
			game: finalGame(27),
			want: parlay.Lost,
		},
		{
			name: "spread missing line",
			leg:  grader.LegInput{Market: "SPREAD", Team: "BUF", Side: "TEAM"},
			game: finalGame(27),
			want: parlay.NeedsGrading,
		},
		{
			name: "spread unmatched team",
			leg:  grader.LegInput{Market: "SPREAD", Team: "KC", Side: "TEAM", Line: new(-3.5)},
			game: finalGame(27),
			want: parlay.NeedsGrading,
		},
		{
			name: "spread missing scores",
			leg:  grader.LegInput{Market: "SPREAD", Team: "BUF", Side: "TEAM", Line: new(-3.5)},
			game: grader.GameScore{HomeTeam: "BUF", AwayTeam: "MIA", Final: true},
			want: parlay.NeedsGrading,
		},
		// ML
		{
			name: "ml home wins",
			leg:  grader.LegInput{Market: "ML", Team: "BUF", Side: "TEAM"},
			game: finalGame(27),
			want: parlay.Won,
		},
		{
			name: "ml away loses",
			leg:  grader.LegInput{Market: "ML", Team: "MIA", Side: "TEAM"},
			game: finalGame(27),
			want: parlay.Lost,
		},
		{
			name: "ml tie pushes",
			leg:  grader.LegInput{Market: "ML", Team: "BUF", Side: "TEAM"},
			game: finalGame(20),
			want: parlay.Push,
		},
		{
			name: "ml unmatched team",
			leg:  grader.LegInput{Market: "ML", Team: "NYJ", Side: "TEAM"},
			game: finalGame(27),
			want: parlay.NeedsGrading,
		},
		{
			name: "ml missing scores",
			leg:  grader.LegInput{Market: "ML", Team: "BUF", Side: "TEAM"},
			game: grader.GameScore{HomeTeam: "BUF", AwayTeam: "MIA", Final: true},
			want: parlay.NeedsGrading,
		},
		// TOTAL
		{
			name: "total over wins",
			leg:  grader.LegInput{Market: "TOTAL", Side: "OVER", Line: new(46.5)},
			game: finalGame(27),
			want: parlay.Won,
		},
		{
			name: "total over loses",
			leg:  grader.LegInput{Market: "TOTAL", Side: "OVER", Line: new(47.5)},
			game: finalGame(27),
			want: parlay.Lost,
		},
		{
			name: "total under wins",
			leg:  grader.LegInput{Market: "TOTAL", Side: "UNDER", Line: new(47.5)},
			game: finalGame(27),
			want: parlay.Won,
		},
		{
			name: "total under loses",
			leg:  grader.LegInput{Market: "TOTAL", Side: "UNDER", Line: new(46.5)},
			game: finalGame(27),
			want: parlay.Lost,
		},
		{
			name: "total integer push",
			leg:  grader.LegInput{Market: "TOTAL", Side: "OVER", Line: new(float64(47))},
			game: finalGame(27),
			want: parlay.Push,
		},
		{
			name: "total missing line",
			leg:  grader.LegInput{Market: "TOTAL", Side: "OVER"},
			game: finalGame(27),
			want: parlay.NeedsGrading,
		},
		{
			name: "total invalid side",
			leg:  grader.LegInput{Market: "TOTAL", Side: "TEAM", Line: new(47.5)},
			game: finalGame(27),
			want: parlay.NeedsGrading,
		},
		// TEAM_TOTAL
		{
			name: "team total home over wins",
			leg:  grader.LegInput{Market: "TEAM_TOTAL", Team: "BUF", Side: "OVER", Line: new(26.5)},
			game: finalGame(27),
			want: parlay.Won,
		},
		{
			name: "team total away under wins",
			leg:  grader.LegInput{Market: "TEAM_TOTAL", Team: "MIA", Side: "UNDER", Line: new(20.5)},
			game: finalGame(27),
			want: parlay.Won,
		},
		{
			name: "team total away over loses",
			leg:  grader.LegInput{Market: "TEAM_TOTAL", Team: "MIA", Side: "OVER", Line: new(20.5)},
			game: finalGame(27),
			want: parlay.Lost,
		},
		{
			name: "team total integer push",
			leg:  grader.LegInput{Market: "TEAM_TOTAL", Team: "MIA", Side: "UNDER", Line: new(float64(20))},
			game: finalGame(27),
			want: parlay.Push,
		},
		{
			name: "team total unmatched team",
			leg:  grader.LegInput{Market: "TEAM_TOTAL", Team: "NE", Side: "OVER", Line: new(20.5)},
			game: finalGame(27),
			want: parlay.NeedsGrading,
		},
		// OTHER & unknown
		{
			name: "other always needs grading",
			leg:  grader.LegInput{Market: "OTHER"},
			game: finalGame(27),
			want: parlay.NeedsGrading,
		},
		{
			name: "unknown market needs grading",
			leg:  grader.LegInput{Market: "FIRST_TD", Team: "BUF"},
			game: finalGame(27),
			want: parlay.NeedsGrading,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, grader.GradeLeg(tt.leg, tt.game, nil))
		})
	}
}

func TestGradeLeg_PlayerProps(t *testing.T) {
	t.Parallel()

	game := finalGame(27)
	box := sampleBox()

	tests := []struct {
		name string
		leg  grader.LegInput
		box  *espn.BoxScore
		want parlay.Result
	}{
		// PASS_YDS (Josh Allen: 280)
		{
			name: "pass yds over wins",
			leg:  grader.LegInput{Market: "PASS_YDS", PlayerID: "3918298", Team: "BUF", Side: "OVER", Line: new(279.5)},
			box:  box,
			want: parlay.Won,
		},
		{
			name: "pass yds over loses",
			leg:  grader.LegInput{Market: "PASS_YDS", PlayerID: "3918298", Team: "BUF", Side: "OVER", Line: new(280.5)},
			box:  box,
			want: parlay.Lost,
		},
		{
			name: "pass yds integer push",
			leg: grader.LegInput{
				Market:   "PASS_YDS",
				PlayerID: "3918298",
				Team:     "BUF",
				Side:     "UNDER",
				Line:     new(float64(280)),
			},
			box:  box,
			want: parlay.Push,
		},
		// PASS_TDS (Josh Allen: 2)
		{
			name: "pass tds over wins",
			leg:  grader.LegInput{Market: "PASS_TDS", PlayerID: "3918298", Team: "BUF", Side: "OVER", Line: new(1.5)},
			box:  box,
			want: parlay.Won,
		},
		{
			name: "pass tds under loses",
			leg:  grader.LegInput{Market: "PASS_TDS", PlayerID: "3918298", Team: "BUF", Side: "UNDER", Line: new(1.5)},
			box:  box,
			want: parlay.Lost,
		},
		{
			name: "pass tds push",
			leg: grader.LegInput{
				Market:   "PASS_TDS",
				PlayerID: "3918298",
				Team:     "BUF",
				Side:     "OVER",
				Line:     new(float64(2)),
			},
			box:  box,
			want: parlay.Push,
		},
		// RUSH_YDS (James Cook III: 95)
		{
			name: "rush yds over wins",
			leg:  grader.LegInput{Market: "RUSH_YDS", PlayerID: "4379399", Team: "BUF", Side: "OVER", Line: new(89.5)},
			box:  box,
			want: parlay.Won,
		},
		{
			name: "rush yds under wins",
			leg:  grader.LegInput{Market: "RUSH_YDS", PlayerID: "4379399", Team: "BUF", Side: "UNDER", Line: new(99.5)},
			box:  box,
			want: parlay.Won,
		},
		{
			name: "rush yds push",
			leg: grader.LegInput{
				Market:   "RUSH_YDS",
				PlayerID: "4379399",
				Team:     "BUF",
				Side:     "OVER",
				Line:     new(float64(95)),
			},
			box:  box,
			want: parlay.Push,
		},
		// RUSH_ATT (James Cook III: 18)
		{
			name: "rush att over wins",
			leg:  grader.LegInput{Market: "RUSH_ATT", PlayerID: "4379399", Team: "BUF", Side: "OVER", Line: new(17.5)},
			box:  box,
			want: parlay.Won,
		},
		{
			name: "rush att under loses",
			leg:  grader.LegInput{Market: "RUSH_ATT", PlayerID: "4379399", Team: "BUF", Side: "UNDER", Line: new(17.5)},
			box:  box,
			want: parlay.Lost,
		},
		{
			name: "rush att push",
			leg: grader.LegInput{
				Market:   "RUSH_ATT",
				PlayerID: "4379399",
				Team:     "BUF",
				Side:     "UNDER",
				Line:     new(float64(18)),
			},
			box:  box,
			want: parlay.Push,
		},
		// REC_YDS (Tyreek Hill: 112)
		{
			name: "rec yds over wins",
			leg:  grader.LegInput{Market: "REC_YDS", PlayerID: "4241457", Team: "MIA", Side: "OVER", Line: new(99.5)},
			box:  box,
			want: parlay.Won,
		},
		{
			name: "rec yds over loses",
			leg:  grader.LegInput{Market: "REC_YDS", PlayerID: "4241457", Team: "MIA", Side: "OVER", Line: new(115.5)},
			box:  box,
			want: parlay.Lost,
		},
		// RECEPTIONS (Tyreek Hill: 7)
		{
			name: "receptions over wins",
			leg:  grader.LegInput{Market: "RECEPTIONS", PlayerID: "4241457", Team: "MIA", Side: "OVER", Line: new(6.5)},
			box:  box,
			want: parlay.Won,
		},
		{
			name: "receptions push",
			leg: grader.LegInput{
				Market:   "RECEPTIONS",
				PlayerID: "4241457",
				Team:     "MIA",
				Side:     "OVER",
				Line:     new(float64(7)),
			},
			box:  box,
			want: parlay.Push,
		},
		// KICKER_PTS (Tyler Bass: 9 via TotalKickingPoints; Jason Sanders: 10 via fallback 2*3+4)
		{
			name: "kicker pts totalKickingPoints over wins",
			leg:  grader.LegInput{Market: "KICKER_PTS", PlayerID: "3051909", Team: "BUF", Side: "OVER", Line: new(8.5)},
			box:  box,
			want: parlay.Won,
		},
		{
			name: "kicker pts fallback calculation over wins",
			leg:  grader.LegInput{Market: "KICKER_PTS", PlayerID: "4360234", Team: "MIA", Side: "OVER", Line: new(9.5)},
			box:  box,
			want: parlay.Won,
		},
		{
			name: "kicker pts push",
			leg: grader.LegInput{
				Market:   "KICKER_PTS",
				PlayerID: "4360234",
				Team:     "MIA",
				Side:     "UNDER",
				Line:     new(float64(10)),
			},
			box:  box,
			want: parlay.Push,
		},
		// ANYTIME_TD
		{
			name: "anytime td rushing td wins",
			leg:  grader.LegInput{Market: "ANYTIME_TD", PlayerID: "4379399", Team: "BUF", Side: "YES"},
			box:  box,
			want: parlay.Won,
		},
		{
			name: "anytime td receiving td wins",
			leg:  grader.LegInput{Market: "ANYTIME_TD", PlayerID: "4241457", Team: "MIA", Side: "YES"},
			box:  box,
			want: parlay.Won,
		},
		{
			name: "anytime td return/other td wins",
			leg:  grader.LegInput{Market: "ANYTIME_TD", PlayerID: "4035687", Team: "MIA", Side: "YES"},
			box:  box,
			want: parlay.Won,
		},
		{
			name: "anytime td excludes passing tds",
			leg:  grader.LegInput{Market: "ANYTIME_TD", PlayerID: "3918298", Team: "BUF", Side: "YES"},
			box:  box,
			want: parlay.Lost,
		},
		{
			name: "anytime td side NO wins when 0 rush/rec/other tds",
			leg:  grader.LegInput{Market: "ANYTIME_TD", PlayerID: "3918298", Team: "BUF", Side: "NO"},
			box:  box,
			want: parlay.Won,
		},
		{
			name: "anytime td side NO loses when scored",
			leg:  grader.LegInput{Market: "ANYTIME_TD", PlayerID: "4379399", Team: "BUF", Side: "NO"},
			box:  box,
			want: parlay.Lost,
		},
		{
			name: "anytime td empty side defaults to YES",
			leg:  grader.LegInput{Market: "ANYTIME_TD", PlayerID: "4379399", Team: "BUF"},
			box:  box,
			want: parlay.Won,
		},
		// Player active in another category (0, not VOID)
		{
			name: "player active in rushing has 0 receptions (not void)",
			leg: grader.LegInput{
				Market:   "RECEPTIONS",
				PlayerID: "4379399",
				Team:     "BUF",
				Side:     "UNDER",
				Line:     new(0.5),
			},
			box:  box,
			want: parlay.Won,
		},
		{
			name: "player active in rushing loses over 0.5 receptions",
			leg:  grader.LegInput{Market: "RECEPTIONS", PlayerID: "4379399", Team: "BUF", Side: "OVER", Line: new(0.5)},
			box:  box,
			want: parlay.Lost,
		},
		// Name fallback matching & team narrowing
		{
			name: "name fallback matches normalized name without suffix",
			leg: grader.LegInput{
				Market:     "RUSH_YDS",
				PlayerName: "james cook",
				Team:       "BUF",
				Side:       "OVER",
				Line:       new(89.5),
			},
			box:  box,
			want: parlay.Won,
		},
		{
			name: "name fallback narrowed by team avoids wrong team",
			leg: grader.LegInput{
				Market:     "RUSH_YDS",
				PlayerName: "James Cook",
				Team:       "MIA",
				Side:       "OVER",
				Line:       new(89.5),
			},
			box:  box,
			want: parlay.Void,
		},
		// DNP vs missing stats
		{
			name: "absent player in populated box score is void (DNP)",
			leg: grader.LegInput{
				Market:     "REC_YDS",
				PlayerID:   "9999999",
				PlayerName: "Keon Coleman",
				Team:       "BUF",
				Side:       "OVER",
				Line:       new(39.5),
			},
			box:  box,
			want: parlay.Void,
		},
		{
			name: "empty box score (HasStats false) returns needs grading",
			leg:  grader.LegInput{Market: "PASS_YDS", PlayerID: "3918298", Team: "BUF", Side: "OVER", Line: new(245.5)},
			box:  &espn.BoxScore{Completed: true, HasStats: false},
			want: parlay.NeedsGrading,
		},
		{
			name: "nil box score returns needs grading",
			leg:  grader.LegInput{Market: "PASS_YDS", PlayerID: "3918298", Team: "BUF", Side: "OVER", Line: new(245.5)},
			box:  nil,
			want: parlay.NeedsGrading,
		},
		{
			name: "prop with no player id and no player name returns needs grading",
			leg:  grader.LegInput{Market: "PASS_YDS", Team: "BUF", Side: "OVER", Line: new(245.5)},
			box:  box,
			want: parlay.NeedsGrading,
		},
		{
			name: "prop with missing line returns needs grading",
			leg:  grader.LegInput{Market: "PASS_YDS", PlayerID: "3918298", Team: "BUF", Side: "OVER"},
			box:  box,
			want: parlay.NeedsGrading,
		},
		{
			name: "prop with invalid side returns needs grading",
			leg:  grader.LegInput{Market: "PASS_YDS", PlayerID: "3918298", Team: "BUF", Side: "YES", Line: new(245.5)},
			box:  box,
			want: parlay.NeedsGrading,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, grader.GradeLeg(tt.leg, game, tt.box))
		})
	}
}

func TestIsPlayerMarket(t *testing.T) {
	t.Parallel()

	playerMarkets := []string{
		"PASS_YDS", "PASS_TDS", "RUSH_YDS", "RUSH_ATT",
		"REC_YDS", "RECEPTIONS", "ANYTIME_TD", "KICKER_PTS",
	}
	for _, m := range playerMarkets {
		assert.True(t, grader.IsPlayerMarket(m), m)
	}

	nonPlayerMarkets := []string{"SPREAD", "ML", "TOTAL", "TEAM_TOTAL", "OTHER", ""}
	for _, m := range nonPlayerMarkets {
		assert.False(t, grader.IsPlayerMarket(m), m)
	}
}
