package analyzer_test

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/analyzer"
)

func TestComboRulesAnalyzeSlip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		legs         []analyzer.Leg
		wantRule     string
		wantSeverity analyzer.Severity
		wantIDs      []int64
		wantContains []string
	}{
		{
			name: "same team ML and spread",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "ML", "BUF", "", "TEAM", nil, -175),
				leg(2, "Ben", "buf-mia", "SPREAD", "BUF", "", "TEAM", new(-3.5), -110),
			},
			wantRule:     "book_combo_ineligible",
			wantSeverity: analyzer.Danger,
			wantIDs:      []int64{1, 2},
			wantContains: []string{"DraftKings and FanDuel", "BUF ML", "BUF -3.5"},
		},
		{
			name: "opposite spreads in same game",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "SPREAD", "BUF", "", "TEAM", new(-3.5), -110),
				leg(2, "Adam", "buf-mia", "SPREAD", "MIA", "", "TEAM", new(3.5), -110),
			},
			wantRule:     "book_combo_ineligible",
			wantSeverity: analyzer.Danger,
			wantIDs:      []int64{1, 2},
			wantContains: []string{"opposite sides"},
		},
		{
			name: "opposite moneylines in same game",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "ML", "BUF", "", "TEAM", nil, -160),
				leg(2, "Adam", "buf-mia", "ML", "MIA", "", "TEAM", nil, 140),
			},
			wantRule:     "book_combo_ineligible",
			wantSeverity: analyzer.Danger,
			wantIDs:      []int64{1, 2},
			wantContains: []string{"opposite sides"},
		},
		{
			name: "opposite game totals",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "TOTAL", "", "", "OVER", new(47.5), -110),
				leg(2, "Ben", "buf-mia", "TOTAL", "", "", "UNDER", new(47.5), -110),
			},
			wantRule:     "book_combo_ineligible",
			wantSeverity: analyzer.Danger,
			wantIDs:      []int64{1, 2},
			wantContains: []string{"opposite sides"},
		},
		{
			name: "opposite team totals for same team",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "TEAM_TOTAL", "BUF", "", "OVER", new(24.5), -110),
				leg(2, "Ben", "buf-mia", "TEAM_TOTAL", "BUF", "", "UNDER", new(24.5), -110),
			},
			wantRule:     "book_combo_ineligible",
			wantSeverity: analyzer.Danger,
			wantIDs:      []int64{1, 2},
			wantContains: []string{"opposite sides"},
		},
		{
			name: "opposite sides of same player prop",
			legs: []analyzer.Leg{
				withPlayerID(
					leg(1, "Adam", "buf-mia", "PASS_YDS", "BUF", "Josh Allen", "OVER", new(265.5), -110),
					"allen",
				),
				withPlayerID(
					leg(2, "Ben", "buf-mia", "PASS_YDS", "BUF", "Josh Allen", "UNDER", new(265.5), -110),
					"allen",
				),
			},
			wantRule:     "book_combo_ineligible",
			wantSeverity: analyzer.Danger,
			wantIDs:      []int64{1, 2},
			wantContains: []string{"opposite sides"},
		},
		{
			name: "duplicate player prop selection",
			legs: []analyzer.Leg{
				withPlayerID(
					leg(1, "Adam", "buf-mia", "REC_YDS", "BUF", "Khalil Shakir", "OVER", new(52.5), -110),
					"shakir",
				),
				withPlayerID(
					leg(2, "Ben", "buf-mia", "REC_YDS", "BUF", "Khalil Shakir", "OVER", new(52.5), -115),
					"shakir",
				),
			},
			wantRule:     "book_combo_ineligible",
			wantSeverity: analyzer.Danger,
			wantIDs:      []int64{1, 2},
			wantContains: []string{"same selection"},
		},
		{
			name: "duplicate spread selection",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "SPREAD", "BUF", "", "TEAM", new(-3.0), -110),
				leg(2, "Ben", "buf-mia", "SPREAD", "BUF", "", "TEAM", new(-3.0), -115),
			},
			wantRule:     "book_combo_ineligible",
			wantSeverity: analyzer.Danger,
			wantIDs:      []int64{1, 2},
			wantContains: []string{"same selection"},
		},
		{
			name: "mutually exclusive ML and opponent favorite spread",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "ML", "MIA", "", "TEAM", nil, 155),
				leg(2, "Ben", "buf-mia", "SPREAD", "BUF", "", "TEAM", new(-3.5), -110),
			},
			wantRule:     "book_combo_ineligible",
			wantSeverity: analyzer.Danger,
			wantIDs:      []int64{1, 2},
			wantContains: []string{"mutually exclusive"},
		},
		{
			name: "mutually exclusive team total overs vs game total under",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "TEAM_TOTAL", "BUF", "", "OVER", new(24.5), -110),
				leg(2, "Ben", "buf-mia", "TEAM_TOTAL", "MIA", "", "OVER", new(21.5), -110),
				leg(3, "Cal", "buf-mia", "TOTAL", "", "", "UNDER", new(44.5), -110),
			},
			wantRule:     "book_combo_ineligible",
			wantSeverity: analyzer.Danger,
			wantIDs:      []int64{1, 2, 3},
			wantContains: []string{"mutually exclusive totals"},
		},
		{
			name: "mutually exclusive team total unders vs game total over",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "TEAM_TOTAL", "BUF", "", "UNDER", new(21.5), -110),
				leg(2, "Ben", "buf-mia", "TEAM_TOTAL", "MIA", "", "UNDER", new(20.5), -110),
				leg(3, "Cal", "buf-mia", "TOTAL", "", "", "OVER", new(44.5), -110),
			},
			wantRule:     "book_combo_ineligible",
			wantSeverity: analyzer.Danger,
			wantIDs:      []int64{1, 2, 3},
			wantContains: []string{"mutually exclusive totals"},
		},
		{
			name: "restricted kicker pts with game total",
			legs: []analyzer.Leg{
				withPlayerID(
					leg(1, "Adam", "buf-mia", "KICKER_PTS", "BUF", "Tyler Bass", "OVER", new(7.5), -115),
					"bass",
				),
				leg(2, "Ben", "buf-mia", "TOTAL", "", "", "OVER", new(47.5), -110),
			},
			wantRule:     "book_combo_restricted",
			wantSeverity: analyzer.Warn,
			wantIDs:      []int64{1, 2},
			wantContains: []string{"Tyler Bass", "Over 47.5"},
		},
		{
			name: "restricted kicker pts with team total",
			legs: []analyzer.Leg{
				withPlayerID(
					leg(1, "Adam", "buf-mia", "KICKER_PTS", "BUF", "Tyler Bass", "OVER", new(7.5), -115),
					"bass",
				),
				leg(2, "Ben", "buf-mia", "TEAM_TOTAL", "BUF", "", "OVER", new(24.5), -110),
			},
			wantRule:     "book_combo_restricted",
			wantSeverity: analyzer.Warn,
			wantIDs:      []int64{1, 2},
			wantContains: []string{"Tyler Bass", "BUF Team Total Over 24.5"},
		},
		{
			name: "restricted inverse same player pass yds over and pass tds under",
			legs: []analyzer.Leg{
				withPlayerID(
					leg(1, "Adam", "buf-mia", "PASS_YDS", "BUF", "Josh Allen", "OVER", new(265.5), -110),
					"allen",
				),
				withPlayerID(
					leg(2, "Adam", "buf-mia", "PASS_TDS", "BUF", "Josh Allen", "UNDER", new(1.5), -110),
					"allen",
				),
			},
			wantRule:     "book_combo_restricted",
			wantSeverity: analyzer.Warn,
			wantIDs:      []int64{1, 2},
			wantContains: []string{"inverse same-player props"},
		},
		{
			name: "restricted duplicate player milestone",
			legs: []analyzer.Leg{
				withPlayerID(
					leg(1, "Adam", "buf-mia", "PASS_YDS", "BUF", "Josh Allen", "OVER", new(245.5), -110),
					"allen",
				),
				withPlayerID(
					leg(2, "Ben", "buf-mia", "PASS_YDS", "BUF", "Josh Allen", "OVER", new(275.5), 140),
					"allen",
				),
			},
			wantRule:     "book_combo_restricted",
			wantSeverity: analyzer.Warn,
			wantIDs:      []int64{1, 2},
			wantContains: []string{"Pass Yds"},
		},
	}

	g := games()
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := analyzer.AnalyzeSlip(tc.legs, g, nil)
			var matched []analyzer.Warning
			for _, w := range got {
				if w.Rule == tc.wantRule {
					matched = append(matched, w)
				}
			}
			require.Len(t, matched, 1, "all warnings: %+v", got)
			assert.Equal(t, tc.wantSeverity, matched[0].Severity)
			assert.Equal(t, tc.wantIDs, matched[0].LegIDs)
			assert.NotContains(t, matched[0].Message, "don't")
			for _, substr := range tc.wantContains {
				assert.Contains(t, matched[0].Message, substr)
			}
		})
	}
}

func TestComboRulesNoFalsePositives(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		legs []analyzer.Leg
	}{
		{
			name: "valid same-game QB pass yds and WR rec yds",
			legs: []analyzer.Leg{
				withPlayerID(
					leg(1, "Adam", "buf-mia", "PASS_YDS", "BUF", "Josh Allen", "OVER", new(265.5), -110),
					"allen",
				),
				withPlayerID(
					leg(2, "Ben", "buf-mia", "REC_YDS", "BUF", "Khalil Shakir", "OVER", new(52.5), -110),
					"shakir",
				),
			},
		},
		{
			name: "valid spread and game total",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "SPREAD", "BUF", "", "TEAM", new(-3.5), -110),
				leg(2, "Ben", "buf-mia", "TOTAL", "", "", "OVER", new(47.5), -110),
			},
		},
		{
			name: "valid ML and opponent underdog spread",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "ML", "BUF", "", "TEAM", nil, -175),
				leg(2, "Ben", "buf-mia", "SPREAD", "MIA", "", "TEAM", new(3.5), -110),
			},
		},
		{
			name: "feasible team total overs under higher game total",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "TEAM_TOTAL", "BUF", "", "OVER", new(21.5), -110),
				leg(2, "Ben", "buf-mia", "TEAM_TOTAL", "MIA", "", "OVER", new(19.5), -110),
				leg(3, "Cal", "buf-mia", "TOTAL", "", "", "UNDER", new(44.5), -110),
			},
		},
		{
			name: "same market across different games",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "ML", "BUF", "", "TEAM", nil, -160),
				leg(2, "Ben", "dal-phi", "SPREAD", "PHI", "", "TEAM", new(-3.5), -110),
			},
		},
		{
			name: "OTHER market ignored",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "OTHER", "BUF", "", "TEAM", nil, -110),
				leg(2, "Ben", "buf-mia", "OTHER", "BUF", "", "TEAM", nil, -110),
			},
		},
	}

	g := games()
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := analyzer.AnalyzeSlip(tc.legs, g, nil)
			for _, w := range got {
				assert.NotEqual(t, "book_combo_ineligible", w.Rule, "unexpected warning: %+v", w)
				assert.NotEqual(t, "book_combo_restricted", w.Rule, "unexpected warning: %+v", w)
			}
		})
	}
}

func TestCheckComboConflict(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		candidate    analyzer.Leg
		existing     []analyzer.Leg
		wantOK       bool
		wantRule     string
		wantSeverity analyzer.Severity
	}{
		{
			name:      "candidate ML conflicts with existing same-team spread",
			candidate: leg(0, "", "buf-mia", "ML", "BUF", "", "TEAM", nil, -175),
			existing: []analyzer.Leg{
				leg(10, "Adam", "buf-mia", "SPREAD", "BUF", "", "TEAM", new(-3.5), -110),
			},
			wantOK:       true,
			wantRule:     "book_combo_ineligible",
			wantSeverity: analyzer.Danger,
		},
		{
			name: "candidate kicker pts warns against existing total",
			candidate: withPlayerID(
				leg(0, "", "buf-mia", "KICKER_PTS", "BUF", "Tyler Bass", "OVER", new(7.5), -115),
				"bass",
			),
			existing: []analyzer.Leg{
				leg(10, "Adam", "buf-mia", "TOTAL", "", "", "OVER", new(47.5), -110),
			},
			wantOK:       true,
			wantRule:     "book_combo_restricted",
			wantSeverity: analyzer.Warn,
		},
		{
			name:      "short-circuits to highest severity Danger over Warn",
			candidate: leg(0, "", "buf-mia", "TOTAL", "", "", "OVER", new(47.5), -110),
			existing: []analyzer.Leg{
				withPlayerID(
					leg(10, "Adam", "buf-mia", "KICKER_PTS", "BUF", "Tyler Bass", "OVER", new(7.5), -115),
					"bass",
				),
				leg(11, "Ben", "buf-mia", "TOTAL", "", "", "UNDER", new(47.5), -110),
			},
			wantOK:       true,
			wantRule:     "book_combo_ineligible",
			wantSeverity: analyzer.Danger,
		},
		{
			name:      "candidate total under triggers 3-leg team total contradiction",
			candidate: leg(0, "", "buf-mia", "TOTAL", "", "", "UNDER", new(44.5), -110),
			existing: []analyzer.Leg{
				leg(10, "Adam", "buf-mia", "TEAM_TOTAL", "BUF", "", "OVER", new(24.5), -110),
				leg(11, "Ben", "buf-mia", "TEAM_TOTAL", "MIA", "", "OVER", new(21.5), -110),
			},
			wantOK:       true,
			wantRule:     "book_combo_ineligible",
			wantSeverity: analyzer.Danger,
		},
		{
			name: "no conflict for valid same-game QB plus WR",
			candidate: withPlayerID(
				leg(0, "", "buf-mia", "REC_YDS", "BUF", "Khalil Shakir", "OVER", new(52.5), -110),
				"shakir",
			),
			existing: []analyzer.Leg{
				withPlayerID(
					leg(10, "Adam", "buf-mia", "PASS_YDS", "BUF", "Josh Allen", "OVER", new(265.5), -110),
					"allen",
				),
			},
			wantOK: false,
		},
		{
			name:      "skips candidate with empty GameID",
			candidate: leg(0, "", "", "ML", "BUF", "", "TEAM", nil, -175),
			existing: []analyzer.Leg{
				leg(10, "Adam", "buf-mia", "SPREAD", "BUF", "", "TEAM", new(-3.5), -110),
			},
			wantOK: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, ok := analyzer.CheckComboConflict(tc.candidate, tc.existing)
			assert.Equal(t, tc.wantOK, ok)
			if tc.wantOK {
				assert.Equal(t, tc.wantRule, got.Rule)
				assert.Equal(t, tc.wantSeverity, got.Severity)
				assert.NotEmpty(t, got.Message)
				assert.NotContains(t, got.Message, "don't")
			}
		})
	}
}

func TestComboRulesRegisteredInRuleIDsAndDocs(t *testing.T) {
	t.Parallel()

	ids := analyzer.RuleIDs()
	assert.True(t, slices.Contains(ids, "book_combo_ineligible"))
	assert.True(t, slices.Contains(ids, "book_combo_restricted"))

	docs := analyzer.RuleDocs()
	docIDs := make([]string, len(docs))
	for i, d := range docs {
		docIDs[i] = d.ID
	}
	assert.True(t, slices.Contains(docIDs, "book_combo_ineligible"))
	assert.True(t, slices.Contains(docIDs, "book_combo_restricted"))
}
