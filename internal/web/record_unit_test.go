package web

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/analyzer"
	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/parlay"
)

func TestNewProcessOutcomeView(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		letter    string
		result    parlay.Result
		wantHas   bool
		wantKey   string
		wantLabel string
		wantClass string
	}{
		{
			name:      "grade A won is Clean Hit",
			letter:    "A",
			result:    parlay.Won,
			wantHas:   true,
			wantKey:   "clean_hit",
			wantLabel: "Clean Hit",
			wantClass: "bg-positive-soft text-positive-ink",
		},
		{
			name:      "grade B lost is Tough Beat",
			letter:    "B",
			result:    parlay.Lost,
			wantHas:   true,
			wantKey:   "tough_beat",
			wantLabel: "Tough Beat",
			wantClass: "bg-surface-2 text-ink-muted",
		},
		{
			name:      "grade C won is Total Bailout",
			letter:    "C",
			result:    parlay.Won,
			wantHas:   true,
			wantKey:   "total_bailout",
			wantLabel: "Total Bailout",
			wantClass: "bg-warning-soft text-warning-ink",
		},
		{
			name:      "grade D lost is Book Donation",
			letter:    "D",
			result:    parlay.Lost,
			wantHas:   true,
			wantKey:   "book_donation",
			wantLabel: "Book Donation",
			wantClass: "bg-danger-soft text-danger-ink",
		},
		{
			name:    "push returns empty view",
			letter:  "A",
			result:  parlay.Push,
			wantHas: false,
		},
		{
			name:    "ungraded returns empty view",
			letter:  "",
			result:  parlay.Won,
			wantHas: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := newProcessOutcomeView(analyzer.ClassifyProcessResult(tt.letter, tt.result))
			assert.Equal(t, tt.wantHas, got.Has)
			assert.Equal(t, tt.wantKey, got.Key)
			assert.Equal(t, tt.wantLabel, got.Label)
			assert.Equal(t, tt.wantClass, got.Class)
		})
	}
}

func TestBuildRecordSlipsAndMemberBetsProcessOutcome(t *testing.T) {
	t.Parallel()

	slips := []db.GradedSlip{
		{
			Slip: db.Slip{ID: 10, Name: "Week 1", Status: db.SlipGraded, Result: parlay.Won},
			Week: db.Week{Season: 2025, SeasonType: 2, WeekNum: 1},
		},
		{
			Slip: db.Slip{ID: 20, Name: "Week 2", Status: db.SlipGraded, Result: parlay.Lost},
			Week: db.Week{Season: 2025, SeasonType: 2, WeekNum: 2},
		},
	}
	slipGrades := map[int64]string{
		10: "A",
		20: "D",
	}

	views := buildRecordSlips(slips, slipGrades)
	require.Len(t, views, 2)

	// Newest first: Week 2 (ID 20) then Week 1 (ID 10).
	assert.Equal(t, int64(20), views[0].SlipID)
	assert.Equal(t, "D", views[0].Grade)
	assert.Equal(t, "grade-d", views[0].GradeClass)
	assert.Equal(t, "Book Donation", views[0].ProcessOutcome.Label)

	assert.Equal(t, int64(10), views[1].SlipID)
	assert.Equal(t, "A", views[1].Grade)
	assert.Equal(t, "grade-a", views[1].GradeClass)
	assert.Equal(t, "Clean Hit", views[1].ProcessOutcome.Label)

	loaded := &recordSeasonData{
		season: 2025,
		slips:  slips,
		legs: []db.Leg{
			{ID: 101, SlipID: 10, UserID: 1, MarketType: "ML", Selection: "BUF ML", Odds: -110, Result: parlay.Won},
			{ID: 102, SlipID: 20, UserID: 1, MarketType: "ML", Selection: "NYJ ML", Odds: 130, Result: parlay.Lost},
		},
		legGrades:  map[int64]string{101: "A", 102: "D"},
		slipGrades: slipGrades,
	}
	bets := buildMemberBets(1, loaded)
	require.Len(t, bets, 2)
	assert.Equal(t, "D", bets[0].SlipGrade)
	assert.Equal(t, "grade-d", bets[0].SlipGradeClass)
	assert.Equal(t, "Book Donation", bets[0].SlipProcessOutcome.Label)
	assert.Equal(t, "A", bets[1].SlipGrade)
	assert.Equal(t, "grade-a", bets[1].SlipGradeClass)
	assert.Equal(t, "Clean Hit", bets[1].SlipProcessOutcome.Label)
}
