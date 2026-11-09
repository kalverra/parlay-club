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

// weatherGames is the shared slate with a forecast on BUF at MIA and no
// forecast on DAL at PHI.
func weatherGames(weather *analyzer.Weather, indoor bool) map[string]analyzer.Game {
	return map[string]analyzer.Game{
		"buf-mia": {ID: "buf-mia", Home: "MIA", Away: "BUF", Indoor: indoor, Weather: weather},
		"dal-phi": {ID: "dal-phi", Home: "PHI", Away: "DAL"},
	}
}

// nfeloGames is the shared slate with an nfelo model on BUF at MIA.
func nfeloGames(nfelo *analyzer.Nfelo) map[string]analyzer.Game {
	return map[string]analyzer.Game{
		"buf-mia": {ID: "buf-mia", Home: "MIA", Away: "BUF", Nfelo: nfelo},
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
	rule        string
	severity    analyzer.Severity
	ids         []int64
	contains    []string
	notContains []string
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
		for _, part := range want.notContains {
			assert.NotContains(t, got[i].Message, part)
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
				leg(1, "Adam", "buf-mia", "SPREAD", "BUF", "", "TEAM", new(-8.5), -110),
				leg(2, "Adam", "buf-mia", "REC_YDS", "MIA", "Tyreek Hill", "OVER", new(72.5), -110),
			},
		},
		{
			name: "other game stays quiet",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "SPREAD", "BUF", "", "TEAM", new(-8.5), -110),
				leg(2, "Adam", "dal-phi", "REC_YDS", "BUF", "Khalil Shakir", "OVER", new(52.5), -110),
			},
		},
		{
			name: "team missing from the game falls back to the other team",
			legs: []analyzer.Leg{
				leg(1, "", "buf-mia", "SPREAD", "KC", "", "TEAM", new(-8.5), -110),
				leg(2, "Sam", "buf-mia", "PASS_YDS", "KC", "Patrick Mahomes", "OVER", new(285.5), -110),
			},
			want: []want{{
				rule: "blowout_passing", severity: analyzer.Warn, ids: []int64{1, 2},
				contains: []string{"KC -8.5", "the other team"},
			}},
		},
		{
			name: "one warning lists every matching prop",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "SPREAD", "BUF", "", "TEAM", new(-8.5), -110),
				leg(2, "Adam", "buf-mia", "PASS_YDS", "BUF", "Josh Allen", "OVER", new(245.5), -110),
				leg(3, "Adam", "buf-mia", "REC_YDS", "BUF", "Khalil Shakir", "OVER", new(52.5), -110),
				leg(4, "Adam", "buf-mia", "REC_YDS", "BUF", "Keon Coleman", "OVER", new(41.5), -110),
			},
			want: []want{{
				rule: "blowout_passing", severity: analyzer.Warn, ids: []int64{1, 2, 3, 4},
				contains: []string{
					"Josh Allen Over 245.5 Pass Yds, Khalil Shakir Over 52.5 Rec Yds, and Keon Coleman Over 41.5 Rec Yds",
				},
			}},
		},
		{
			name: "under prop does not count",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "SPREAD", "BUF", "", "TEAM", new(-8.5), -110),
				leg(2, "Adam", "buf-mia", "PASS_YDS", "BUF", "Josh Allen", "UNDER", new(245.5), -110),
			},
		},
		{
			name: "passing touchdowns and receptions are not yardage props",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "SPREAD", "BUF", "", "TEAM", new(-8.5), -110),
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
			name: "two rush overs fight the script",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "SPREAD", "MIA", "", "TEAM", new(4.5), -110),
				leg(2, "Adam", "buf-mia", "RUSH_YDS", "MIA", "De'Von Achane", "OVER", new(64.5), -110),
				leg(3, "Adam", "buf-mia", "RUSH_ATT", "MIA", "De'Von Achane", "OVER", new(14.5), -110),
			},
			want: []want{{
				rule: "trailing_rb", severity: analyzer.Danger, ids: []int64{1, 2, 3},
				contains: []string{"De'Von Achane Over 64.5 Rush Yds and De'Von Achane Over 14.5 Rush Att fight"},
			}},
		},
		{
			name: "plus six and a half triggers the hook too",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "SPREAD", "MIA", "", "TEAM", new(6.5), -110),
				leg(2, "Adam", "buf-mia", "RUSH_ATT", "MIA", "De'Von Achane", "OVER", new(14.5), -110),
			},
			want: []want{
				{rule: "trailing_rb", severity: analyzer.Danger, ids: []int64{1, 2}},
				{
					rule: "hook", severity: analyzer.Warn, ids: []int64{1},
					contains: []string{"MIA +6.5", "Landing on exactly 7", "Buying to +7"},
				},
			},
		},
		{
			name: "mixed owners name both",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "SPREAD", "MIA", "", "TEAM", new(4.5), -110),
				leg(2, "Sam", "buf-mia", "RUSH_YDS", "MIA", "De'Von Achane", "OVER", new(64.5), -110),
			},
			want: []want{{
				rule: "trailing_rb", severity: analyzer.Danger, ids: []int64{1, 2},
				contains: []string{"Adam's MIA +4.5", "Sam's De'Von Achane Over 64.5 Rush Yds"},
			}},
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
			name: "favorite stays quiet but sits on the hook",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "SPREAD", "MIA", "", "TEAM", new(-3.5), -110),
				leg(2, "Adam", "buf-mia", "RUSH_YDS", "MIA", "De'Von Achane", "OVER", new(64.5), -110),
			},
			want: []want{{
				rule: "hook", severity: analyzer.Warn, ids: []int64{1},
				contains: []string{"MIA -3.5", "switching to the ML"},
			}},
		},
		{
			name: "other team's running back stays quiet but the hook fires",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "SPREAD", "MIA", "", "TEAM", new(6.5), -110),
				leg(2, "Adam", "buf-mia", "RUSH_YDS", "BUF", "James Cook", "OVER", new(64.5), -110),
			},
			want: []want{{rule: "hook", severity: analyzer.Warn, ids: []int64{1}}},
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
				leg(1, "Adam", "buf-mia", "TOTAL", "", "", "UNDER", new(45.5), -110),
				leg(2, "Adam", "buf-mia", "PASS_YDS", "BUF", "Josh Allen", "OVER", new(245.5), -110),
				leg(3, "Adam", "buf-mia", "ANYTIME_TD", "MIA", "De'Von Achane", "YES", nil, -110),
			},
			want: []want{{
				rule: "total_contradiction", severity: analyzer.Danger, ids: []int64{1, 2, 3},
				contains: []string{"Under 45.5", "Josh Allen Over 245.5 Pass Yds and De'Von Achane Anytime TD"},
			}},
		},
		{
			name: "under with two receiving overs triggers",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "TOTAL", "", "", "UNDER", new(45.5), -110),
				leg(2, "Adam", "buf-mia", "REC_YDS", "BUF", "Khalil Shakir", "OVER", new(52.5), -110),
				leg(3, "Adam", "buf-mia", "RECEPTIONS", "MIA", "Tyreek Hill", "OVER", new(6.5), -110),
			},
			want: []want{{rule: "total_contradiction", severity: analyzer.Danger, ids: []int64{1, 2, 3}}},
		},
		{
			name: "mixed owners are named",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "TOTAL", "", "", "UNDER", new(45.5), -110),
				leg(2, "Sam", "buf-mia", "PASS_YDS", "BUF", "Josh Allen", "OVER", new(245.5), -110),
				leg(3, "Adam", "buf-mia", "ANYTIME_TD", "MIA", "De'Von Achane", "YES", nil, -110),
			},
			want: []want{{
				rule: "total_contradiction", severity: analyzer.Danger, ids: []int64{1, 2, 3},
				contains: []string{
					"Adam's Under 45.5",
					"Sam's Josh Allen Over 245.5 Pass Yds and Adam's De'Von Achane Anytime TD",
				},
			}},
		},
		{
			name: "kicking overs do not need points",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "TOTAL", "", "", "UNDER", new(45.5), -110),
				leg(2, "Adam", "buf-mia", "ANYTIME_TD", "BUF", "James Cook", "YES", nil, -110),
				leg(3, "Adam", "buf-mia", "KICKER_PTS", "BUF", "Tyler Bass", "OVER", new(6.5), -110),
			},
		},
		{
			name: "under with one scoring leg stays quiet",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "TOTAL", "", "", "UNDER", new(45.5), -110),
				leg(2, "Adam", "buf-mia", "ANYTIME_TD", "BUF", "James Cook", "YES", nil, -110),
			},
		},
		{
			name: "scoring legs in another game stay quiet",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "TOTAL", "", "", "UNDER", new(45.5), -110),
				leg(2, "Adam", "buf-mia", "PASS_YDS", "BUF", "Josh Allen", "OVER", new(245.5), -110),
				leg(3, "Adam", "dal-phi", "ANYTIME_TD", "DAL", "CeeDee Lamb", "YES", nil, -110),
			},
		},
		{
			name: "total over does not trigger",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "TOTAL", "", "", "OVER", new(45.5), -110),
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
			name: "mixed owners are named",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "SPREAD", "BUF", "", "TEAM", new(-6.5), -110),
				leg(2, "Sam", "buf-mia", "RUSH_YDS", "BUF", "James Cook", "OVER", new(64.5), -110),
				leg(3, "Adam", "buf-mia", "TEAM_TOTAL", "MIA", "", "UNDER", new(20.5), -110),
			},
			want: []want{{
				rule: "stack_clock_drain", severity: analyzer.Info, ids: []int64{1, 2, 3},
				contains: []string{
					"Adam's BUF -6.5 covers",
					"Sam's James Cook grinds the clock",
					"MIA Team Total Under 20.5 stays quiet",
				},
			}},
		},
		{
			name: "two rush overs and two team totals stack",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "SPREAD", "BUF", "", "TEAM", new(-6.5), -110),
				leg(2, "Adam", "buf-mia", "RUSH_YDS", "BUF", "James Cook", "OVER", new(64.5), -110),
				leg(3, "Adam", "buf-mia", "RUSH_ATT", "BUF", "James Cook", "OVER", new(14.5), -110),
				leg(4, "Adam", "buf-mia", "TEAM_TOTAL", "MIA", "", "UNDER", new(20.5), -110),
				leg(5, "Adam", "buf-mia", "TEAM_TOTAL", "MIA", "", "UNDER", new(10.5), -110),
			},
			want: []want{{
				rule: "stack_clock_drain", severity: analyzer.Info, ids: []int64{1, 2, 3, 4, 5},
				contains: []string{
					"James Cook and James Cook grind the clock",
					"MIA Team Total Under 20.5 and MIA Team Total Under 10.5 stay quiet",
				},
			}},
		},
		{
			name: "spread team missing from the game stays quiet",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "SPREAD", "KC", "", "TEAM", new(-6.5), -110),
				leg(2, "Adam", "buf-mia", "RUSH_YDS", "KC", "Isiah Pacheco", "OVER", new(64.5), -110),
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
				leg(3, "Adam", "buf-mia", "TOTAL", "", "", "OVER", new(48.5), -110),
			},
			want: []want{
				{
					rule:     "stack_shootout",
					severity: analyzer.Info,
					ids:      []int64{1, 2, 3},
					contains: []string{
						"Josh Allen Over 245.5 Pass Yds",
						"Khalil Shakir Over 52.5 Rec Yds",
						"game Over 48.5",
					},
				},
			},
		},
		{
			name: "receptions count as a receiving over",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "PASS_YDS", "BUF", "Josh Allen", "OVER", new(245.5), -110),
				leg(2, "Adam", "buf-mia", "RECEPTIONS", "BUF", "Khalil Shakir", "OVER", new(4.5), -110),
				leg(3, "Adam", "buf-mia", "TOTAL", "", "", "OVER", new(48.5), -110),
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
				leg(3, "Adam", "buf-mia", "TOTAL", "", "", "OVER", new(48.5), -110),
			},
		},
		{
			name: "players matched by name are not a receiver stack",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "PASS_YDS", "BUF", "Josh Allen", "OVER", new(245.5), -110),
				leg(2, "Adam", "buf-mia", "REC_YDS", "BUF", "Josh Allen", "OVER", new(20.5), -110),
				leg(3, "Adam", "buf-mia", "TOTAL", "", "", "OVER", new(48.5), -110),
			},
		},
		{
			name: "one missing player ID falls back to the name",
			legs: []analyzer.Leg{
				withPlayerID(
					leg(1, "Adam", "buf-mia", "PASS_YDS", "BUF", "Josh Allen", "OVER", new(245.5), -110),
					"allen",
				),
				leg(2, "Adam", "buf-mia", "REC_YDS", "BUF", "Josh Allen", "OVER", new(20.5), -110),
				leg(3, "Adam", "buf-mia", "TOTAL", "", "", "OVER", new(48.5), -110),
			},
		},
		{
			name: "two unnamed players are different receivers",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "PASS_YDS", "BUF", "", "OVER", new(245.5), -110),
				leg(2, "Adam", "buf-mia", "REC_YDS", "BUF", "", "OVER", new(20.5), -110),
				leg(3, "Adam", "buf-mia", "TOTAL", "", "", "OVER", new(48.5), -110),
			},
			want: []want{{
				rule: "stack_shootout", severity: analyzer.Info, ids: []int64{1, 2, 3},
				contains: []string{"BUF Over 245.5 Pass Yds", "game Over 48.5"},
			}},
		},
		{
			name: "receiver on the other team stays quiet",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "PASS_YDS", "BUF", "Josh Allen", "OVER", new(245.5), -110),
				leg(2, "Adam", "buf-mia", "REC_YDS", "MIA", "Tyreek Hill", "OVER", new(72.5), -110),
				leg(3, "Adam", "buf-mia", "TOTAL", "", "", "OVER", new(48.5), -110),
			},
		},
		{
			name: "total under is not a shootout",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "PASS_YDS", "BUF", "Josh Allen", "OVER", new(245.5), -110),
				leg(2, "Adam", "buf-mia", "REC_YDS", "BUF", "Khalil Shakir", "OVER", new(52.5), -110),
				leg(3, "Adam", "buf-mia", "TOTAL", "", "", "UNDER", new(48.5), -110),
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

func TestWind(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		legs  []analyzer.Leg
		games map[string]analyzer.Game
		want  []want
	}{
		{
			name:  "fifteen mph is a warn",
			games: weatherGames(&analyzer.Weather{WindMPH: 15}, false),
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "PASS_YDS", "BUF", "Josh Allen", "OVER", new(245.5), -110),
			},
			want: []want{{
				rule: "wind", severity: analyzer.Warn, ids: []int64{1},
				contains: []string{"15 mph winds", "Josh Allen Over 245.5 Pass Yds gets tougher"},
			}},
		},
		{
			name:  "just under twenty is still a warn",
			games: weatherGames(&analyzer.Weather{WindMPH: 19.9}, false),
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "PASS_YDS", "BUF", "Josh Allen", "OVER", new(245.5), -110),
			},
			want: []want{{rule: "wind", severity: analyzer.Warn, ids: []int64{1}}},
		},
		{
			name:  "gusts to twenty five warn",
			games: weatherGames(&analyzer.Weather{WindMPH: 5, GustMPH: 25}, false),
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "REC_YDS", "BUF", "Khalil Shakir", "OVER", new(52.5), -110),
			},
			want: []want{{
				rule: "wind", severity: analyzer.Warn, ids: []int64{1},
				contains: []string{"gusts to 25 mph"},
			}},
		},
		{
			name:  "twenty mph is danger",
			games: weatherGames(&analyzer.Weather{WindMPH: 20}, false),
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "PASS_TDS", "BUF", "Josh Allen", "OVER", new(1.5), -110),
			},
			want: []want{{
				rule: "wind", severity: analyzer.Danger, ids: []int64{1},
				contains: []string{"20 mph winds", "Pass TDs"},
			}},
		},
		{
			name:  "just under both thresholds stays quiet",
			games: weatherGames(&analyzer.Weather{WindMPH: 14.9, GustMPH: 24.9}, false),
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "PASS_YDS", "BUF", "Josh Allen", "OVER", new(245.5), -110),
			},
		},
		{
			name:  "indoor stays quiet",
			games: weatherGames(&analyzer.Weather{WindMPH: 30}, true),
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "PASS_YDS", "BUF", "Josh Allen", "OVER", new(245.5), -110),
			},
		},
		{
			name:  "missing weather stays quiet",
			games: weatherGames(nil, false),
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "PASS_YDS", "BUF", "Josh Allen", "OVER", new(245.5), -110),
			},
		},
		{
			name:  "one warning per game lists every flagged leg",
			games: weatherGames(&analyzer.Weather{WindMPH: 22}, false),
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "PASS_YDS", "BUF", "Josh Allen", "OVER", new(245.5), -110),
				leg(2, "Adam", "buf-mia", "KICKER_PTS", "BUF", "", "OVER", new(6.5), -110),
				leg(3, "Adam", "buf-mia", "TOTAL", "", "", "OVER", new(48.5), -110),
			},
			want: []want{{
				rule: "wind", severity: analyzer.Danger, ids: []int64{1, 2, 3},
				contains: []string{"BUF Over 6.5 Kicking Pts", "get tougher"},
			}},
		},
		{
			name:  "mixed owners are named",
			games: weatherGames(&analyzer.Weather{WindMPH: 22}, false),
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "PASS_YDS", "BUF", "Josh Allen", "OVER", new(245.5), -110),
				leg(2, "Sam", "buf-mia", "KICKER_PTS", "BUF", "Tyler Bass", "OVER", new(6.5), -110),
			},
			want: []want{{
				rule: "wind", severity: analyzer.Danger, ids: []int64{1, 2},
				contains: []string{"Adam's Josh Allen Over 245.5 Pass Yds", "Sam's Tyler Bass Over 6.5 Kicking Pts"},
			}},
		},
		{
			name:  "unders and team totals are not flagged",
			games: weatherGames(&analyzer.Weather{WindMPH: 22}, false),
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "PASS_YDS", "BUF", "Josh Allen", "UNDER", new(245.5), -110),
				leg(2, "Adam", "buf-mia", "REC_YDS", "MIA", "Tyreek Hill", "UNDER", new(72.5), -110),
				leg(3, "Adam", "buf-mia", "TEAM_TOTAL", "BUF", "", "OVER", new(24.5), -110),
			},
		},
		{
			name: "each windy game gets its own warning",
			games: map[string]analyzer.Game{
				"buf-mia": {ID: "buf-mia", Home: "MIA", Away: "BUF", Weather: &analyzer.Weather{WindMPH: 22}},
				"dal-phi": {ID: "dal-phi", Home: "PHI", Away: "DAL", Weather: &analyzer.Weather{WindMPH: 22}},
			},
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "PASS_YDS", "BUF", "Josh Allen", "OVER", new(245.5), -110),
				leg(2, "Adam", "dal-phi", "REC_YDS", "DAL", "CeeDee Lamb", "OVER", new(72.5), -110),
			},
			want: []want{
				{rule: "wind", severity: analyzer.Danger, ids: []int64{1}},
				{rule: "wind", severity: analyzer.Danger, ids: []int64{2}},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			assertWarnings(t, analyzer.AnalyzeSlip(test.legs, test.games, nil), test.want)
		})
	}
}

func TestFakeWeather(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		legs    []analyzer.Leg
		weather *analyzer.Weather
		indoor  bool
		want    []want
	}{
		{
			name:    "light rain without wind and an under triggers",
			weather: &analyzer.Weather{PrecipIn: 0.05, WindMPH: 9},
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "TOTAL", "", "", "UNDER", new(45.5), -110),
			},
			want: []want{{
				rule: "fake_weather", severity: analyzer.Info, ids: []int64{1},
				contains: []string{"Under 45.5 gets no extra help"},
			}},
		},
		{
			name:    "a tenth of an inch is too much",
			weather: &analyzer.Weather{PrecipIn: 0.1, WindMPH: 5},
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "TOTAL", "", "", "UNDER", new(45.5), -110),
			},
		},
		{
			name:    "ten mph is too much wind",
			weather: &analyzer.Weather{PrecipIn: 0.05, WindMPH: 10},
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "TOTAL", "", "", "UNDER", new(45.5), -110),
			},
		},
		{
			name:    "no rain stays quiet",
			weather: &analyzer.Weather{PrecipIn: 0, WindMPH: 5},
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "TOTAL", "", "", "UNDER", new(45.5), -110),
			},
		},
		{
			name:    "an over stays quiet",
			weather: &analyzer.Weather{PrecipIn: 0.05, WindMPH: 5},
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "TOTAL", "", "", "OVER", new(45.5), -110),
			},
		},
		{
			name:    "indoor stays quiet",
			weather: &analyzer.Weather{PrecipIn: 0.05, WindMPH: 5},
			indoor:  true,
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "TOTAL", "", "", "UNDER", new(45.5), -110),
			},
		},
		{
			name:    "missing weather stays quiet",
			weather: nil,
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "TOTAL", "", "", "UNDER", new(45.5), -110),
			},
		},
		{
			name:    "a windy day is not fake weather",
			weather: &analyzer.Weather{PrecipIn: 0.05, WindMPH: 20},
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "TOTAL", "", "", "UNDER", new(45.5), -110),
			},
		},
		{
			name:    "a missing line still reads",
			weather: &analyzer.Weather{PrecipIn: 0.05, WindMPH: 5},
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "TOTAL", "", "", "UNDER", nil, -110),
			},
			want: []want{{
				rule: "fake_weather", severity: analyzer.Info, ids: []int64{1},
				contains: []string{"Under gets no extra help"},
			}},
		},
		{
			name:    "two unders get one warning",
			weather: &analyzer.Weather{PrecipIn: 0.05, WindMPH: 5},
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "TOTAL", "", "", "UNDER", new(45.5), -110),
				leg(2, "Sam", "buf-mia", "TOTAL", "", "", "UNDER", new(45.5), -110),
			},
			want: []want{{
				rule: "fake_weather", severity: analyzer.Info, ids: []int64{1, 2},
				contains: []string{"Adam's Under 45.5 and Sam's Under 45.5 get no extra help"},
			}},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			assertWarnings(t, analyzer.AnalyzeSlip(test.legs, weatherGames(test.weather, test.indoor), nil), test.want)
		})
	}
}

func TestHook(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		legs []analyzer.Leg
		want []want
	}{
		{
			name: "minus three and a half triggers",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "SPREAD", "BUF", "", "TEAM", new(-3.5), -110),
			},
			want: []want{{
				rule: "hook", severity: analyzer.Warn, ids: []int64{1},
				contains: []string{"BUF -3.5", "About 15% of games", "switching to the ML"},
			}},
		},
		{
			name: "minus seven and a half triggers",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "SPREAD", "BUF", "", "TEAM", new(-7.5), -110),
			},
			want: []want{{
				rule: "hook", severity: analyzer.Warn, ids: []int64{1},
				contains: []string{"BUF -7.5", "Landing on exactly 7", "Buying to -7 or switching to the ML"},
			}},
		},
		{
			name: "plus two and a half triggers",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "SPREAD", "MIA", "", "TEAM", new(2.5), -110),
			},
			want: []want{{
				rule: "hook", severity: analyzer.Warn, ids: []int64{1},
				contains: []string{"MIA +2.5", "Landing on exactly 3", "Buying to +3 is worth a look"},
			}},
		},
		{
			name: "plus six and a half triggers",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "SPREAD", "MIA", "", "TEAM", new(6.5), -110),
			},
			want: []want{{
				rule: "hook", severity: analyzer.Warn, ids: []int64{1},
				contains: []string{"MIA +6.5", "Landing on exactly 7", "Buying to +7 is worth a look"},
			}},
		},
		{
			name: "minus three is off the hook",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "SPREAD", "BUF", "", "TEAM", new(-3.0), -110),
			},
		},
		{
			name: "minus four is off the hook",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "SPREAD", "BUF", "", "TEAM", new(-4.0), -110),
			},
		},
		{
			name: "plus three and a half is off the hook",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "SPREAD", "MIA", "", "TEAM", new(3.5), -110),
			},
		},
		{
			name: "minus six and a half is off the hook",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "SPREAD", "BUF", "", "TEAM", new(-6.5), -110),
			},
		},
		{
			name: "a spread without a line is quiet",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "SPREAD", "BUF", "", "TEAM", nil, -110),
			},
		},
		{
			name: "a moneyline with a line is quiet",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "ML", "BUF", "", "TEAM", new(-3.5), -110),
			},
		},
		{
			name: "two hooks get two warnings",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "SPREAD", "BUF", "", "TEAM", new(-3.5), -110),
				leg(2, "Adam", "buf-mia", "SPREAD", "MIA", "", "TEAM", new(2.5), -110),
			},
			want: []want{
				{rule: "hook", severity: analyzer.Warn, ids: []int64{1}},
				{rule: "hook", severity: analyzer.Warn, ids: []int64{2}},
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

// TestHookWithNfelo pins the three cushions on a favorite and an underdog. The
// cushion is the leg line minus nfelo's spread for the same team.
func TestHookWithNfelo(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		leg      analyzer.Leg
		nfelo    *analyzer.Nfelo
		contains []string
	}{
		{
			name:     "favorite cushion plus two and a half",
			leg:      leg(1, "Adam", "buf-mia", "SPREAD", "BUF", "", "TEAM", new(-7.5), -110),
			nfelo:    &analyzer.Nfelo{HomeSpread: 10},
			contains: []string{"nfelo has it nearer -10, so the hook matters less."},
		},
		{
			name:     "favorite on the number",
			leg:      leg(1, "Adam", "buf-mia", "SPREAD", "BUF", "", "TEAM", new(-7.5), -110),
			nfelo:    &analyzer.Nfelo{HomeSpread: 7.5},
			contains: []string{"nfelo sees it near the number too (-7.5), so the hook is live."},
		},
		{
			name:     "favorite asking more than the model",
			leg:      leg(1, "Adam", "buf-mia", "SPREAD", "BUF", "", "TEAM", new(-7.5), -110),
			nfelo:    &analyzer.Nfelo{HomeSpread: 5},
			contains: []string{"nfelo has it closer to -5, so this asks for more than the model does."},
		},
		{
			name:     "underdog cushion plus two and a half",
			leg:      leg(1, "Adam", "buf-mia", "SPREAD", "MIA", "", "TEAM", new(6.5), -110),
			nfelo:    &analyzer.Nfelo{HomeSpread: 4},
			contains: []string{"nfelo has it nearer +4, so the hook matters less."},
		},
		{
			name:     "underdog on the number",
			leg:      leg(1, "Adam", "buf-mia", "SPREAD", "MIA", "", "TEAM", new(6.5), -110),
			nfelo:    &analyzer.Nfelo{HomeSpread: 6.5},
			contains: []string{"nfelo sees it near the number too (+6.5), so the hook is live."},
		},
		{
			name:     "underdog asking more than the model",
			leg:      leg(1, "Adam", "buf-mia", "SPREAD", "MIA", "", "TEAM", new(6.5), -110),
			nfelo:    &analyzer.Nfelo{HomeSpread: 9},
			contains: []string{"nfelo has it closer to +9, so this asks for more than the model does."},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got := analyzer.AnalyzeSlip([]analyzer.Leg{test.leg}, nfeloGames(test.nfelo), nil)
			assertWarnings(t, got, []want{{
				rule: "hook", severity: analyzer.Warn, ids: []int64{test.leg.ID},
				contains: test.contains,
			}})
		})
	}
}

func TestTotalKeyNumber(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		legs []analyzer.Leg
		want []want
	}{
		{
			name: "forty three is a key number",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "TOTAL", "", "", "UNDER", new(43.0), -110),
			},
			want: []want{{
				rule: "total_key_number", severity: analyzer.Info, ids: []int64{1},
				contains: []string{"43 is one of the most common NFL final totals", "half point either way"},
			}},
		},
		{
			name: "forty three and a half sits between two keys",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "TOTAL", "", "", "OVER", new(43.5), -110),
			},
			want: []want{{
				rule: "total_key_number", severity: analyzer.Info, ids: []int64{1},
				contains: []string{"43.5 sits right between 43 and 44"},
			}},
		},
		{
			name: "thirty six and a half is near thirty seven",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "TOTAL", "", "", "UNDER", new(36.5), -110),
			},
			want: []want{{
				rule: "total_key_number", severity: analyzer.Info, ids: []int64{1},
				contains: []string{"36.5 is half a point off 37"},
			}},
		},
		{
			name: "fifty one and a half is near fifty one",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "TOTAL", "", "", "OVER", new(51.5), -110),
			},
			want: []want{{
				rule: "total_key_number", severity: analyzer.Info, ids: []int64{1},
				contains: []string{"51.5 is half a point off 51"},
			}},
		},
		{
			name: "every key number triggers",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "TOTAL", "", "", "OVER", new(37.0), -110),
				leg(2, "Adam", "buf-mia", "TOTAL", "", "", "OVER", new(41.0), -110),
				leg(3, "Adam", "buf-mia", "TOTAL", "", "", "OVER", new(44.0), -110),
				leg(4, "Adam", "buf-mia", "TOTAL", "", "", "OVER", new(47.0), -110),
				leg(5, "Adam", "buf-mia", "TOTAL", "", "", "OVER", new(51.0), -110),
			},
			want: []want{
				{rule: "total_key_number", severity: analyzer.Info, ids: []int64{1}},
				{rule: "total_key_number", severity: analyzer.Info, ids: []int64{2}},
				{rule: "total_key_number", severity: analyzer.Info, ids: []int64{3}},
				{rule: "total_key_number", severity: analyzer.Info, ids: []int64{4}},
				{rule: "total_key_number", severity: analyzer.Info, ids: []int64{5}},
			},
		},
		{
			name: "forty five and a half is between keys",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "TOTAL", "", "", "OVER", new(45.5), -110),
			},
		},
		{
			name: "forty nine is between keys",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "TOTAL", "", "", "UNDER", new(49.0), -110),
			},
		},
		{
			name: "a team total is not a game total",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "TEAM_TOTAL", "BUF", "", "OVER", new(43.5), -110),
			},
		},
		{
			name: "a total without a line is quiet",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "TOTAL", "", "", "OVER", nil, -110),
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

func TestChalk(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		legs []analyzer.Leg
		want []want
	}{
		{
			name: "minus two thirty triggers",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "ML", "BUF", "", "TEAM", nil, -230),
			},
			want: []want{{
				rule: "chalk", severity: analyzer.Warn, ids: []int64{1},
				contains: []string{"This -230 leg", "1.43×", "about 30%"},
			}},
		},
		{
			name: "minus five hundred triggers",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "ML", "BUF", "", "TEAM", nil, -500),
			},
			want: []want{{
				rule: "chalk", severity: analyzer.Warn, ids: []int64{1},
				contains: []string{"This -500 leg", "1.2×", "about 17%"},
			}},
		},
		{
			name: "minus three hundred triggers on a prop",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "PASS_YDS", "BUF", "Josh Allen", "OVER", new(245.5), -300),
			},
			want: []want{{
				rule: "chalk", severity: analyzer.Warn, ids: []int64{1},
				contains: []string{"1.33×", "about 25%"},
			}},
		},
		{
			name: "minus two twenty nine is quiet",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "ML", "BUF", "", "TEAM", nil, -229),
			},
		},
		{
			name: "a plus price is quiet",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "ML", "MIA", "", "TEAM", nil, 150),
			},
		},
		{
			name: "two chalk legs get two warnings",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "ML", "BUF", "", "TEAM", nil, -300),
				leg(2, "Adam", "dal-phi", "ML", "PHI", "", "TEAM", nil, -350),
			},
			want: []want{
				{rule: "chalk", severity: analyzer.Warn, ids: []int64{1}},
				{rule: "chalk", severity: analyzer.Warn, ids: []int64{2}},
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

// TestChalkWithNfelo checks that only a moneyline leg picks up the model's win
// probability. A spread at the same price keeps the plain chalk sentence.
func TestChalkWithNfelo(t *testing.T) {
	t.Parallel()
	nfelo := &analyzer.Nfelo{HomeWinProb: 0.24}
	tests := []struct {
		name string
		leg  analyzer.Leg
		want []want
	}{
		{
			name: "away moneyline shows the percentage",
			leg:  leg(1, "Adam", "buf-mia", "ML", "BUF", "", "TEAM", nil, -300),
			want: []want{{
				rule: "chalk", severity: analyzer.Warn, ids: []int64{1},
				contains: []string{"nfelo has BUF at about 76% to win."},
			}},
		},
		{
			name: "home moneyline shows the percentage",
			leg:  leg(1, "Adam", "buf-mia", "ML", "MIA", "", "TEAM", nil, -300),
			want: []want{{
				rule: "chalk", severity: analyzer.Warn, ids: []int64{1},
				contains: []string{"nfelo has MIA at about 24% to win."},
			}},
		},
		{
			name: "spread keeps the plain sentence",
			leg:  leg(1, "Adam", "buf-mia", "SPREAD", "BUF", "", "TEAM", new(-3.0), -300),
			want: []want{{
				rule: "chalk", severity: analyzer.Warn, ids: []int64{1},
				notContains: []string{"nfelo"},
			}},
		},
		{
			name: "prop keeps the plain sentence",
			leg:  leg(1, "Adam", "buf-mia", "PASS_YDS", "BUF", "Josh Allen", "OVER", new(245.5), -300),
			want: []want{{
				rule: "chalk", severity: analyzer.Warn, ids: []int64{1},
				notContains: []string{"nfelo"},
			}},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			assertWarnings(t, analyzer.AnalyzeSlip([]analyzer.Leg{test.leg}, nfeloGames(nfelo), nil), test.want)
		})
	}
}

// TestQBDrag checks the threshold, the markets it covers, the legs it ignores,
// and the single warning per game and team.
func TestQBDrag(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		legs  []analyzer.Leg
		nfelo *analyzer.Nfelo
		want  []want
	}{
		{
			name: "minus twenty nine point nine stays quiet",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "SPREAD", "MIA", "", "TEAM", new(-3.0), -110),
			},
			nfelo: &analyzer.Nfelo{HomeQB: "Tua Tagovailoa", HomeQBAdj: -29.9},
		},
		{
			name: "minus thirty fires",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "SPREAD", "MIA", "", "TEAM", new(-3.0), -110),
			},
			nfelo: &analyzer.Nfelo{HomeQB: "Tua Tagovailoa", HomeQBAdj: -30},
			want: []want{{
				rule: "qb_drag", severity: analyzer.Info, ids: []int64{1},
				contains: []string{
					"nfelo marks down MIA's QB, Tua Tagovailoa, by about 1.2 points.",
					"MIA -3",
				},
			}},
		},
		{
			name: "a favorite spread and a passing over get one warning for both owners",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "SPREAD", "MIA", "", "TEAM", new(-3.0), -110),
				leg(2, "Sam", "buf-mia", "PASS_YDS", "MIA", "Tua Tagovailoa", "OVER", new(245.5), -110),
			},
			nfelo: &analyzer.Nfelo{HomeQB: "Tua Tagovailoa", HomeQBAdj: -35},
			want: []want{{
				rule: "qb_drag", severity: analyzer.Info, ids: []int64{1, 2},
				contains: []string{
					"by about 1.4 points.",
					"Adam's MIA -3",
					"Sam's Tua Tagovailoa Over 245.5 Pass Yds",
				},
			}},
		},
		{
			name: "an away favorite and a receiving over fire",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "ML", "BUF", "", "TEAM", nil, -200),
				leg(2, "Sam", "buf-mia", "REC_YDS", "BUF", "Keon Coleman", "OVER", new(52.5), -110),
			},
			nfelo: &analyzer.Nfelo{AwayQB: "Josh Allen", AwayQBAdj: -40},
			want: []want{{
				rule: "qb_drag", severity: analyzer.Info, ids: []int64{1, 2},
				contains: []string{
					"nfelo marks down BUF's QB, Josh Allen, by about 1.6 points.",
					"Adam's BUF ML",
					"Sam's Keon Coleman Over 52.5 Rec Yds",
				},
			}},
		},
		{
			name: "an underdog spread stays quiet",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "SPREAD", "MIA", "", "TEAM", new(3.0), -110),
			},
			nfelo: &analyzer.Nfelo{HomeQB: "Tua Tagovailoa", HomeQBAdj: -35},
		},
		{
			name: "an underdog moneyline stays quiet",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "ML", "MIA", "", "TEAM", nil, 150),
			},
			nfelo: &analyzer.Nfelo{HomeQB: "Tua Tagovailoa", HomeQBAdj: -35},
		},
		{
			name: "a team total under stays quiet",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "TEAM_TOTAL", "MIA", "", "UNDER", new(20.5), -110),
			},
			nfelo: &analyzer.Nfelo{HomeQB: "Tua Tagovailoa", HomeQBAdj: -35},
		},
		{
			name: "a team total over fires",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "TEAM_TOTAL", "MIA", "", "OVER", new(20.5), -110),
			},
			nfelo: &analyzer.Nfelo{HomeQB: "Tua Tagovailoa", HomeQBAdj: -35},
			want: []want{{
				rule: "qb_drag", severity: analyzer.Info, ids: []int64{1},
				contains: []string{"MIA Team Total Over 20.5"},
			}},
		},
		{
			name: "a rush over stays quiet",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "RUSH_YDS", "MIA", "De'Von Achane", "OVER", new(60.5), -110),
			},
			nfelo: &analyzer.Nfelo{HomeQB: "Tua Tagovailoa", HomeQBAdj: -35},
		},
		{
			name: "no nfelo stays quiet",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "SPREAD", "MIA", "", "TEAM", new(-3.0), -110),
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			assertWarnings(t, analyzer.AnalyzeSlip(test.legs, nfeloGames(test.nfelo), nil), test.want)
		})
	}
}

// TestNilNfeloKeepsTheOldSentences pins the hook and chalk text for a game with
// no model, so the new sentences never leak into a world without nfelo data.
func TestNilNfeloKeepsTheOldSentences(t *testing.T) {
	t.Parallel()
	legs := []analyzer.Leg{
		leg(1, "Adam", "buf-mia", "SPREAD", "BUF", "", "TEAM", new(-7.5), -110),
		leg(2, "Adam", "buf-mia", "ML", "BUF", "", "TEAM", nil, -300),
	}

	got := analyzer.AnalyzeSlip(legs, games(), nil)
	require.Len(t, got, 2)
	assert.Equal(t, "hook", got[0].Rule)
	assert.Equal(t, "BUF -7.5 is on the hook. Landing on exactly 7 loses by the hook. "+
		"Buying to -7 or switching to the ML is worth a look.", got[0].Message)
	assert.Equal(t, "chalk", got[1].Rule)
	assert.Equal(t, "This -300 leg only bumps the payout 1.33× but still busts the slip "+
		"about 25% of the time.", got[1].Message)
}

func TestOutputOrderAcrossRuleSets(t *testing.T) {
	t.Parallel()
	legs := []analyzer.Leg{
		leg(1, "Adam", "buf-mia", "SPREAD", "BUF", "", "TEAM", new(-7.5), -110),
		leg(2, "Adam", "buf-mia", "PASS_YDS", "BUF", "Josh Allen", "OVER", new(245.5), -110),
		leg(3, "Adam", "buf-mia", "TOTAL", "", "", "UNDER", new(43.5), -110),
		leg(4, "Adam", "buf-mia", "ML", "BUF", "", "TEAM", nil, -300),
	}

	got := analyzer.AnalyzeSlip(legs, games(), nil)
	require.Len(t, got, 4)
	assert.Equal(t, "blowout_passing", got[0].Rule)
	assert.Equal(t, analyzer.Warn, got[0].Severity)
	assert.Equal(t, []int64{1, 2}, got[0].LegIDs)
	assert.Equal(t, "hook", got[1].Rule)
	assert.Equal(t, analyzer.Warn, got[1].Severity)
	assert.Equal(t, []int64{1}, got[1].LegIDs)
	assert.Equal(t, "chalk", got[2].Rule)
	assert.Equal(t, analyzer.Warn, got[2].Severity)
	assert.Equal(t, []int64{4}, got[2].LegIDs)
	assert.Equal(t, "total_key_number", got[3].Rule)
	assert.Equal(t, analyzer.Info, got[3].Severity)
	assert.Equal(t, []int64{3}, got[3].LegIDs)
	assert.Equal(t, got, analyzer.AnalyzeSlip(legs, games(), nil), "same input must give the same output")
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
				leg(1, "Adam", "buf-mia", "TOTAL", "", "", "UNDER", new(45.5), -110),
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
		leg(1, "Adam", "buf-mia", "TOTAL", "", "", "UNDER", new(45.5), -110),
		leg(2, "Adam", "buf-mia", "SPREAD", "BUF", "", "TEAM", new(-8.5), -110),
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
		leg(1, "Adam", "buf-mia", "SPREAD", "BUF", "", "TEAM", new(-8.5), -110),
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
		leg(1, "Adam", "buf-mia", "SPREAD", "BUF", "", "TEAM", new(-8.5), -110),
		leg(2, "Sam", "buf-mia", "PASS_YDS", "BUF", "Josh Allen", "OVER", new(245.5), -110),
	}

	got := analyzer.AnalyzeSlip(legs, games(), nil)
	require.Len(t, got, 1)
	assert.Contains(t, got[0].Message, "Adam's BUF -8.5")
	assert.Contains(t, got[0].Message, "Sam's Josh Allen Over 245.5 Pass Yds")
}

func TestMessagesKeepOneOwnerQuiet(t *testing.T) {
	t.Parallel()
	legs := []analyzer.Leg{
		leg(1, "Adam", "buf-mia", "SPREAD", "BUF", "", "TEAM", new(-8.5), -110),
		leg(2, "Adam", "buf-mia", "PASS_YDS", "BUF", "Josh Allen", "OVER", new(245.5), -110),
	}

	got := analyzer.AnalyzeSlip(legs, games(), nil)
	require.Len(t, got, 1)
	assert.NotContains(t, got[0].Message, "Adam's")
	assert.Contains(t, got[0].Message, "BUF -8.5")
}

func TestMedianTrap(t *testing.T) {
	t.Parallel()

	shakirStats := analyzer.PlayerStats{
		Games: map[string][]float64{
			// 5 games: median = 41, mean = 58
			"REC_YDS": {41, 41, 41, 41, 126},
		},
	}

	tests := []struct {
		name  string
		legs  []analyzer.Leg
		stats map[string]analyzer.PlayerStats
		want  []want
	}{
		{
			name: "line over L5 median with line under mean appends mean sentence",
			legs: []analyzer.Leg{
				withPlayerID(leg(1, "Adam", "buf-mia", "REC_YDS", "BUF", "Shakir", "OVER", new(52.5), -110), "shakir"),
			},
			stats: map[string]analyzer.PlayerStats{"shakir": shakirStats},
			want: []want{{
				rule:     "median_trap",
				severity: analyzer.Warn,
				ids:      []int64{1},
				contains: []string{
					"Shakir's last-5 median is 41 yards, so 52.5 is a stretch.",
					"His 58-yard average is propped up by one big game.",
				},
			}},
		},
		{
			name: "line over L5 median but above mean omits mean sentence",
			legs: []analyzer.Leg{
				withPlayerID(leg(1, "Adam", "buf-mia", "REC_YDS", "BUF", "Shakir", "OVER", new(60.0), -110), "shakir"),
			},
			stats: map[string]analyzer.PlayerStats{"shakir": shakirStats},
			want: []want{{
				rule:     "median_trap",
				severity: analyzer.Warn,
				ids:      []int64{1},
				contains: []string{
					"Shakir's last-5 median is 41 yards, so 60 is a stretch.",
				},
				notContains: []string{
					"propped up by one big game",
					"average",
				},
			}},
		},
		{
			name: "line equal to median does not trigger",
			legs: []analyzer.Leg{
				withPlayerID(leg(1, "Adam", "buf-mia", "REC_YDS", "BUF", "Shakir", "OVER", new(41.0), -110), "shakir"),
			},
			stats: map[string]analyzer.PlayerStats{"shakir": shakirStats},
			want:  nil,
		},
		{
			name: "line below median does not trigger",
			legs: []analyzer.Leg{
				withPlayerID(leg(1, "Adam", "buf-mia", "REC_YDS", "BUF", "Shakir", "OVER", new(38.5), -110), "shakir"),
			},
			stats: map[string]analyzer.PlayerStats{"shakir": shakirStats},
			want:  nil,
		},
		{
			name: "fewer than 3 games does not trigger",
			legs: []analyzer.Leg{
				withPlayerID(leg(1, "Adam", "buf-mia", "REC_YDS", "BUF", "Shakir", "OVER", new(52.5), -110), "shakir"),
			},
			stats: map[string]analyzer.PlayerStats{
				"shakir": {
					Games: map[string][]float64{
						"REC_YDS": {41, 126},
					},
				},
			},
			want: nil,
		},
		{
			name: "unmatched player does not trigger",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "REC_YDS", "BUF", "Shakir", "OVER", new(52.5), -110),
			},
			stats: map[string]analyzer.PlayerStats{"shakir": shakirStats},
			want:  nil,
		},
		{
			name: "under side does not trigger",
			legs: []analyzer.Leg{
				withPlayerID(leg(1, "Adam", "buf-mia", "REC_YDS", "BUF", "Shakir", "UNDER", new(52.5), -110), "shakir"),
			},
			stats: map[string]analyzer.PlayerStats{"shakir": shakirStats},
			want:  nil,
		},
		{
			name: "receptions market does not trigger",
			legs: []analyzer.Leg{
				withPlayerID(
					leg(1, "Adam", "buf-mia", "RECEPTIONS", "BUF", "Shakir", "OVER", new(5.5), -110),
					"shakir",
				),
			},
			stats: map[string]analyzer.PlayerStats{
				"shakir": {
					Games: map[string][]float64{
						"RECEPTIONS": {3, 3, 3, 3, 10},
					},
				},
			},
			want: nil,
		},
		{
			name: "pass tds market does not trigger",
			legs: []analyzer.Leg{
				withPlayerID(
					leg(1, "Adam", "buf-mia", "PASS_TDS", "BUF", "Josh Allen", "OVER", new(2.5), -110),
					"allen",
				),
			},
			stats: map[string]analyzer.PlayerStats{
				"allen": {
					Games: map[string][]float64{
						"PASS_TDS": {1, 1, 1, 2, 4},
					},
				},
			},
			want: nil,
		},
		{
			name: "rush yards over triggers",
			legs: []analyzer.Leg{
				withPlayerID(
					leg(1, "Adam", "buf-mia", "RUSH_YDS", "BUF", "James Cook", "OVER", new(75.5), -110),
					"cook",
				),
			},
			stats: map[string]analyzer.PlayerStats{
				"cook": {
					Games: map[string][]float64{
						// median = 60, mean = 80
						"RUSH_YDS": {60, 60, 60, 70, 150},
					},
				},
			},
			want: []want{{
				rule:     "median_trap",
				severity: analyzer.Warn,
				ids:      []int64{1},
				contains: []string{
					"James Cook's last-5 median is 60 yards, so 75.5 is a stretch.",
					"His 80-yard average is propped up by one big game.",
				},
			}},
		},
		{
			name: "pass yards over triggers",
			legs: []analyzer.Leg{
				withPlayerID(
					leg(1, "Adam", "buf-mia", "PASS_YDS", "BUF", "Josh Allen", "OVER", new(255.5), -110),
					"allen",
				),
			},
			stats: map[string]analyzer.PlayerStats{
				"allen": {
					Games: map[string][]float64{
						// 3 games: median = 220, mean = 230
						"PASS_YDS": {210, 220, 260},
					},
				},
			},
			want: []want{{
				rule:     "median_trap",
				severity: analyzer.Warn,
				ids:      []int64{1},
				contains: []string{
					"Josh Allen's last-5 median is 220 yards, so 255.5 is a stretch.",
				},
				notContains: []string{
					"propped up by one big game",
				},
			}},
		},
		{
			name: "six games uses last 5 for median and all 6 for season mean",
			legs: []analyzer.Leg{
				withPlayerID(leg(1, "Adam", "buf-mia", "REC_YDS", "BUF", "Shakir", "OVER", new(50.5), -110), "shakir"),
			},
			stats: map[string]analyzer.PlayerStats{
				"shakir": {
					Games: map[string][]float64{
						// 6 games: L5 are {40, 42, 41, 43, 39} -> median = 41
						// 6th game was 200 -> season mean = 405 / 6 = 67.5 -> rounded 68
						"REC_YDS": {40, 42, 41, 43, 39, 200},
					},
				},
			},
			want: []want{{
				rule:     "median_trap",
				severity: analyzer.Warn,
				ids:      []int64{1},
				contains: []string{
					"Shakir's last-5 median is 41 yards, so 50.5 is a stretch.",
					"His 68-yard average is propped up by one big game.",
				},
			}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got := analyzer.AnalyzeSlip(test.legs, games(), test.stats)
			assertWarnings(t, got, test.want)
		})
	}
}
