package analyzer_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/analyzer"
)

// games is the two-game slate every test shares. BUF visits MIA, DAL visits PHI.
func games() map[string]analyzer.Game {
	return map[string]analyzer.Game{
		"buf-mia": {ID: "buf-mia", Home: "MIA", Away: "BUF"},
		"dal-phi": {ID: "dal-phi", Home: "PHI", Away: "DAL"},
	}
}

// leg builds one analyzer leg with the fields a rule reads.
func leg(
	id int64,
	owner, game, market, team, player, side string,
	line *float64,
	odds int,
) analyzer.Leg {
	return analyzer.Leg{
		ID: id, Owner: owner, GameID: game, Market: market, Team: team,
		PlayerName: player, Side: side, Line: line, Odds: odds,
	}
}

// withPlayerID gives a leg an ESPN athlete ID.
func withPlayerID(leg analyzer.Leg, id string) analyzer.Leg {
	leg.PlayerID = id
	return leg
}

// want is one expected warning.
type want struct {
	rule     string
	severity analyzer.Severity
	ids      []int64
	contains []string
}

// assertWarnings checks the full warning list and the parts of each message
// that matter to the rule.
func assertWarnings(t *testing.T, got []analyzer.Warning, expected []want) {
	t.Helper()
	require.Len(t, got, len(expected))
	for i, want := range expected {
		assert.Equal(t, want.rule, got[i].Rule)
		assert.Equal(t, want.severity, got[i].Severity)
		assert.Equal(t, want.ids, got[i].LegIDs)
		assert.NotEmpty(t, got[i].Message)
		assert.NotContains(t, got[i].Message, "don't")
		for _, part := range want.contains {
			assert.Contains(t, got[i].Message, part)
		}
	}
}

func TestBlowoutPassing(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		legs []analyzer.Leg
		want []want
	}{
		{
			name: "minus seven triggers",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "SPREAD", "BUF", "", "TEAM", new(-7.0), -110),
				leg(2, "Adam", "buf-mia", "PASS_YDS", "BUF", "Josh Allen", "OVER", new(245.5), -110),
			},
			want: []want{{
				rule: "blowout_passing", severity: analyzer.Warn, ids: []int64{1, 2},
				contains: []string{"BUF -7", "Josh Allen Over 245.5 Pass Yds"},
			}},
		},
		{
			name: "past the threshold triggers",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "SPREAD", "BUF", "", "TEAM", new(-10.5), -110),
				leg(2, "Adam", "buf-mia", "REC_YDS", "BUF", "Khalil Shakir", "OVER", new(52.5), -110),
			},
			want: []want{{
				rule: "blowout_passing", severity: analyzer.Warn, ids: []int64{1, 2},
			}},
		},
		{
			name: "minus six and a half stays quiet",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "SPREAD", "BUF", "", "TEAM", new(-6.5), -110),
				leg(2, "Adam", "buf-mia", "PASS_YDS", "BUF", "Josh Allen", "OVER", new(245.5), -110),
			},
		},
		{
			name: "opposing team's receiver stays quiet",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "SPREAD", "BUF", "", "TEAM", new(-7.5), -110),
				leg(2, "Adam", "buf-mia", "REC_YDS", "MIA", "Tyreek Hill", "OVER", new(72.5), -110),
			},
		},
		{
			name: "other game stays quiet",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "SPREAD", "BUF", "", "TEAM", new(-7.5), -110),
				leg(2, "Adam", "dal-phi", "REC_YDS", "BUF", "Khalil Shakir", "OVER", new(52.5), -110),
			},
		},
		{
			name: "one warning lists every matching prop",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "SPREAD", "BUF", "", "TEAM", new(-7.5), -110),
				leg(2, "Adam", "buf-mia", "PASS_YDS", "BUF", "Josh Allen", "OVER", new(245.5), -110),
				leg(3, "Adam", "buf-mia", "REC_YDS", "BUF", "Khalil Shakir", "OVER", new(52.5), -110),
			},
			want: []want{{
				rule: "blowout_passing", severity: analyzer.Warn, ids: []int64{1, 2, 3},
				contains: []string{"Josh Allen Over 245.5 Pass Yds and Khalil Shakir Over 52.5 Rec Yds"},
			}},
		},
		{
			name: "under prop does not count",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "SPREAD", "BUF", "", "TEAM", new(-7.5), -110),
				leg(2, "Adam", "buf-mia", "PASS_YDS", "BUF", "Josh Allen", "UNDER", new(245.5), -110),
			},
		},
		{
			name: "passing touchdowns and receptions are not yardage props",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "SPREAD", "BUF", "", "TEAM", new(-7.5), -110),
				leg(2, "Adam", "buf-mia", "PASS_TDS", "BUF", "Josh Allen", "OVER", new(1.5), -110),
				leg(3, "Adam", "buf-mia", "RECEPTIONS", "BUF", "Khalil Shakir", "OVER", new(4.5), -110),
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			assertWarnings(t, analyzer.AnalyzeSlip(test.legs, games(), nil), test.want)
		})
	}
}

func TestTrailingRB(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		legs []analyzer.Leg
		want []want
	}{
		{
			name: "plus four and a half triggers",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "SPREAD", "MIA", "", "TEAM", new(4.5), -110),
				leg(2, "Adam", "buf-mia", "RUSH_YDS", "MIA", "De'Von Achane", "OVER", new(64.5), -110),
			},
			want: []want{{
				rule: "trailing_rb", severity: analyzer.Danger, ids: []int64{1, 2},
				contains: []string{"MIA +4.5", "De'Von Achane Over 64.5 Rush Yds"},
			}},
		},
		{
			name: "plus six and a half triggers",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "SPREAD", "MIA", "", "TEAM", new(6.5), -110),
				leg(2, "Adam", "buf-mia", "RUSH_ATT", "MIA", "De'Von Achane", "OVER", new(14.5), -110),
			},
			want: []want{{rule: "trailing_rb", severity: analyzer.Danger, ids: []int64{1, 2}}},
		},
		{
			name: "underdog moneyline triggers",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "ML", "MIA", "", "TEAM", nil, 150),
				leg(2, "Adam", "buf-mia", "RUSH_YDS", "MIA", "De'Von Achane", "OVER", new(64.5), -110),
			},
			want: []want{{
				rule: "trailing_rb", severity: analyzer.Danger, ids: []int64{1, 2},
				contains: []string{"MIA ML +150"},
			}},
		},
		{
			name: "plus four stays quiet",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "SPREAD", "MIA", "", "TEAM", new(4.0), -110),
				leg(2, "Adam", "buf-mia", "RUSH_YDS", "MIA", "De'Von Achane", "OVER", new(64.5), -110),
			},
		},
		{
			name: "favorite stays quiet",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "SPREAD", "MIA", "", "TEAM", new(-3.5), -110),
				leg(2, "Adam", "buf-mia", "RUSH_YDS", "MIA", "De'Von Achane", "OVER", new(64.5), -110),
			},
		},
		{
			name: "other team's running back stays quiet",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "SPREAD", "MIA", "", "TEAM", new(6.5), -110),
				leg(2, "Adam", "buf-mia", "RUSH_YDS", "BUF", "James Cook", "OVER", new(64.5), -110),
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			assertWarnings(t, analyzer.AnalyzeSlip(test.legs, games(), nil), test.want)
		})
	}
}

func TestTotalContradiction(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		legs []analyzer.Leg
		want []want
	}{
		{
			name: "under with two scoring legs triggers",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "TOTAL", "", "", "UNDER", new(44.5), -110),
				leg(2, "Adam", "buf-mia", "PASS_YDS", "BUF", "Josh Allen", "OVER", new(245.5), -110),
				leg(3, "Adam", "buf-mia", "ANYTIME_TD", "MIA", "De'Von Achane", "YES", nil, -110),
			},
			want: []want{{
				rule: "total_contradiction", severity: analyzer.Danger, ids: []int64{1, 2, 3},
				contains: []string{"Under 44.5", "Josh Allen Over 245.5 Pass Yds and De'Von Achane Anytime TD"},
			}},
		},
		{
			name: "under with two receiving overs triggers",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "TOTAL", "", "", "UNDER", new(44.5), -110),
				leg(2, "Adam", "buf-mia", "REC_YDS", "BUF", "Khalil Shakir", "OVER", new(52.5), -110),
				leg(3, "Adam", "buf-mia", "RECEPTIONS", "MIA", "Tyreek Hill", "OVER", new(6.5), -110),
			},
			want: []want{{rule: "total_contradiction", severity: analyzer.Danger, ids: []int64{1, 2, 3}}},
		},
		{
			name: "under with one scoring leg stays quiet",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "TOTAL", "", "", "UNDER", new(44.5), -110),
				leg(2, "Adam", "buf-mia", "ANYTIME_TD", "BUF", "James Cook", "YES", nil, -110),
			},
		},
		{
			name: "scoring legs in another game stay quiet",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "TOTAL", "", "", "UNDER", new(44.5), -110),
				leg(2, "Adam", "buf-mia", "PASS_YDS", "BUF", "Josh Allen", "OVER", new(245.5), -110),
				leg(3, "Adam", "dal-phi", "ANYTIME_TD", "DAL", "CeeDee Lamb", "YES", nil, -110),
			},
		},
		{
			name: "total over does not trigger",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "TOTAL", "", "", "OVER", new(44.5), -110),
				leg(2, "Adam", "buf-mia", "PASS_YDS", "BUF", "Josh Allen", "OVER", new(245.5), -110),
				leg(3, "Adam", "buf-mia", "ANYTIME_TD", "MIA", "De'Von Achane", "YES", nil, -110),
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			assertWarnings(t, analyzer.AnalyzeSlip(test.legs, games(), nil), test.want)
		})
	}
}

func TestStackClockDrain(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		legs []analyzer.Leg
		want []want
	}{
		{
			name: "favorite spread rush over and opponent team total under triggers",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "SPREAD", "BUF", "", "TEAM", new(-6.5), -110),
				leg(2, "Adam", "buf-mia", "RUSH_YDS", "BUF", "James Cook", "OVER", new(64.5), -110),
				leg(3, "Adam", "buf-mia", "TEAM_TOTAL", "MIA", "", "UNDER", new(20.5), -110),
			},
			want: []want{
				{
					rule:     "stack_clock_drain",
					severity: analyzer.Info,
					ids:      []int64{1, 2, 3},
					contains: []string{
						"BUF -6.5 covers",
						"James Cook grinds the clock",
						"MIA Team Total Under 20.5 stays quiet",
					},
				},
			},
		},
		{
			name: "missing rush over stays quiet",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "SPREAD", "BUF", "", "TEAM", new(-6.5), -110),
				leg(2, "Adam", "buf-mia", "TEAM_TOTAL", "MIA", "", "UNDER", new(20.5), -110),
			},
		},
		{
			name: "missing team total under stays quiet",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "SPREAD", "BUF", "", "TEAM", new(-6.5), -110),
				leg(2, "Adam", "buf-mia", "RUSH_YDS", "BUF", "James Cook", "OVER", new(64.5), -110),
			},
		},
		{
			name: "team total under on the favorite stays quiet",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "SPREAD", "BUF", "", "TEAM", new(-6.5), -110),
				leg(2, "Adam", "buf-mia", "RUSH_YDS", "BUF", "James Cook", "OVER", new(64.5), -110),
				leg(3, "Adam", "buf-mia", "TEAM_TOTAL", "BUF", "", "UNDER", new(24.5), -110),
			},
		},
		{
			name: "moneyline favorite is not a spread stack",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "ML", "BUF", "", "TEAM", nil, -150),
				leg(2, "Adam", "buf-mia", "RUSH_YDS", "BUF", "James Cook", "OVER", new(64.5), -110),
				leg(3, "Adam", "buf-mia", "TEAM_TOTAL", "MIA", "", "UNDER", new(20.5), -110),
			},
		},
		{
			name: "team total over stays quiet",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "SPREAD", "BUF", "", "TEAM", new(-6.5), -110),
				leg(2, "Adam", "buf-mia", "RUSH_YDS", "BUF", "James Cook", "OVER", new(64.5), -110),
				leg(3, "Adam", "buf-mia", "TEAM_TOTAL", "MIA", "", "OVER", new(20.5), -110),
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			assertWarnings(t, analyzer.AnalyzeSlip(test.legs, games(), nil), test.want)
		})
	}
}

func TestStackShootout(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		legs []analyzer.Leg
		want []want
	}{
		{
			name: "pass over receiver over and total over triggers",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "PASS_YDS", "BUF", "Josh Allen", "OVER", new(245.5), -110),
				leg(2, "Adam", "buf-mia", "REC_YDS", "BUF", "Khalil Shakir", "OVER", new(52.5), -110),
				leg(3, "Adam", "buf-mia", "TOTAL", "", "", "OVER", new(47.5), -110),
			},
			want: []want{
				{
					rule:     "stack_shootout",
					severity: analyzer.Info,
					ids:      []int64{1, 2, 3},
					contains: []string{
						"Josh Allen Over 245.5 Pass Yds",
						"Khalil Shakir Over 52.5 Rec Yds",
						"game Over 47.5",
					},
				},
			},
		},
		{
			name: "receptions count as a receiving over",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "PASS_YDS", "BUF", "Josh Allen", "OVER", new(245.5), -110),
				leg(2, "Adam", "buf-mia", "RECEPTIONS", "BUF", "Khalil Shakir", "OVER", new(4.5), -110),
				leg(3, "Adam", "buf-mia", "TOTAL", "", "", "OVER", new(47.5), -110),
			},
			want: []want{{rule: "stack_shootout", severity: analyzer.Info, ids: []int64{1, 2, 3}}},
		},
		{
			name: "same player is not a receiver stack",
			legs: []analyzer.Leg{
				withPlayerID(
					leg(1, "Adam", "buf-mia", "PASS_YDS", "BUF", "Josh Allen", "OVER", new(245.5), -110),
					"allen",
				),
				withPlayerID(
					leg(2, "Adam", "buf-mia", "REC_YDS", "BUF", "Josh Allen", "OVER", new(20.5), -110),
					"allen",
				),
				leg(3, "Adam", "buf-mia", "TOTAL", "", "", "OVER", new(47.5), -110),
			},
		},
		{
			name: "receiver on the other team stays quiet",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "PASS_YDS", "BUF", "Josh Allen", "OVER", new(245.5), -110),
				leg(2, "Adam", "buf-mia", "REC_YDS", "MIA", "Tyreek Hill", "OVER", new(72.5), -110),
				leg(3, "Adam", "buf-mia", "TOTAL", "", "", "OVER", new(47.5), -110),
			},
		},
		{
			name: "total under is not a shootout",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "PASS_YDS", "BUF", "Josh Allen", "OVER", new(245.5), -110),
				leg(2, "Adam", "buf-mia", "REC_YDS", "BUF", "Khalil Shakir", "OVER", new(52.5), -110),
				leg(3, "Adam", "buf-mia", "TOTAL", "", "", "UNDER", new(47.5), -110),
			},
			// The under and the two overs are a total_contradiction, but the
			// missing total over means this is not a shootout stack.
			want: []want{{rule: "total_contradiction", severity: analyzer.Danger, ids: []int64{1, 2, 3}}},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			assertWarnings(t, analyzer.AnalyzeSlip(test.legs, games(), nil), test.want)
		})
	}
}

func TestSkippedLegs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		legs []analyzer.Leg
	}{
		{name: "empty slip"},
		{
			name: "other leg",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "OTHER", "BUF", "", "", new(-7.5), -500),
			},
		},
		{
			name: "other legs do not feed a total under",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "TOTAL", "", "", "UNDER", new(44.5), -110),
				leg(2, "Adam", "buf-mia", "OTHER", "BUF", "", "", nil, -110),
				leg(3, "Adam", "buf-mia", "OTHER", "MIA", "", "", nil, -110),
			},
		},
		{
			name: "leg with no game",
			legs: []analyzer.Leg{
				leg(1, "Adam", "", "PASS_YDS", "BUF", "Josh Allen", "OVER", new(245.5), -110),
			},
		},
		{
			name: "leg with an unknown game",
			legs: []analyzer.Leg{
				leg(1, "Adam", "missing", "PASS_YDS", "BUF", "Josh Allen", "OVER", new(245.5), -110),
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			assert.Empty(t, analyzer.AnalyzeSlip(test.legs, games(), nil))
		})
	}
}

func TestOutputOrder(t *testing.T) {
	t.Parallel()
	legs := []analyzer.Leg{
		leg(1, "Adam", "buf-mia", "TOTAL", "", "", "UNDER", new(44.5), -110),
		leg(2, "Adam", "buf-mia", "SPREAD", "BUF", "", "TEAM", new(-7.5), -110),
		leg(3, "Adam", "buf-mia", "PASS_YDS", "BUF", "Josh Allen", "OVER", new(245.5), -110),
		leg(4, "Adam", "buf-mia", "REC_YDS", "BUF", "Khalil Shakir", "OVER", new(52.5), -110),
	}

	got := analyzer.AnalyzeSlip(legs, games(), nil)
	require.Len(t, got, 2)
	assert.Equal(t, "total_contradiction", got[0].Rule)
	assert.Equal(t, analyzer.Danger, got[0].Severity)
	assert.Equal(t, []int64{1, 3, 4}, got[0].LegIDs)
	assert.Equal(t, "blowout_passing", got[1].Rule)
	assert.Equal(t, analyzer.Warn, got[1].Severity)
	assert.Equal(t, []int64{2, 3, 4}, got[1].LegIDs)

	assert.Equal(t, got, analyzer.AnalyzeSlip(legs, games(), nil), "same input must give the same output")
}

func TestOutputOrderWithinSeverity(t *testing.T) {
	t.Parallel()
	legs := []analyzer.Leg{
		leg(1, "Adam", "buf-mia", "SPREAD", "BUF", "", "TEAM", new(-7.5), -110),
		leg(2, "Adam", "buf-mia", "PASS_YDS", "BUF", "Josh Allen", "OVER", new(245.5), -110),
		leg(3, "Adam", "dal-phi", "SPREAD", "DAL", "", "TEAM", new(-7.0), -110),
		leg(4, "Adam", "dal-phi", "REC_YDS", "DAL", "CeeDee Lamb", "OVER", new(72.5), -110),
	}

	got := analyzer.AnalyzeSlip(legs, games(), nil)
	require.Len(t, got, 2)
	assert.Equal(t, []int64{1, 2}, got[0].LegIDs)
	assert.Equal(t, []int64{3, 4}, got[1].LegIDs)
}

func TestMessagesNameOwners(t *testing.T) {
	t.Parallel()
	legs := []analyzer.Leg{
		leg(1, "Adam", "buf-mia", "SPREAD", "BUF", "", "TEAM", new(-7.5), -110),
		leg(2, "Sam", "buf-mia", "PASS_YDS", "BUF", "Josh Allen", "OVER", new(245.5), -110),
	}

	got := analyzer.AnalyzeSlip(legs, games(), nil)
	require.Len(t, got, 1)
	assert.Contains(t, got[0].Message, "Adam's BUF -7.5")
	assert.Contains(t, got[0].Message, "Sam's Josh Allen Over 245.5 Pass Yds")
}

func TestMessagesKeepOneOwnerQuiet(t *testing.T) {
	t.Parallel()
	legs := []analyzer.Leg{
		leg(1, "Adam", "buf-mia", "SPREAD", "BUF", "", "TEAM", new(-7.5), -110),
		leg(2, "Adam", "buf-mia", "PASS_YDS", "BUF", "Josh Allen", "OVER", new(245.5), -110),
	}

	got := analyzer.AnalyzeSlip(legs, games(), nil)
	require.Len(t, got, 1)
	assert.NotContains(t, got[0].Message, "Adam's")
	assert.Contains(t, got[0].Message, "BUF -7.5")
}
