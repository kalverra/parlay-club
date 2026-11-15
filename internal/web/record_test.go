package web_test

import (
	"fmt"
	"html"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/parlay"
)

func TestRecordBookEmptyStateAndInvalidSeason(t *testing.T) {
	t.Parallel()

	f := newSlipFixture(t, 2)

	rec := httptest.NewRecorder()
	f.handler.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/record", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	body := html.UnescapeString(rec.Body.String())
	assert.Contains(t, body, "Record Book")
	assert.Contains(t, body, "No graded slips yet for the 2026 season.")

	bad := httptest.NewRecorder()
	f.handler.ServeHTTP(bad, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/record?season=abc", nil))
	assert.Equal(t, http.StatusBadRequest, bad.Code)
}

func TestRecordBookSeasonSelectionLeaderboardAndGroupIsolation(t *testing.T) {
	t.Parallel()

	f := newSlipFixture(t, 2)
	ctx := t.Context()

	// Create a second member in the group.
	bob, err := f.store.CreateUserWithIdentity(ctx, "Bob", "bob@example.com", "google-bob", "")
	require.NoError(t, err)
	_, err = f.store.JoinByToken(ctx, bob.ID, f.group.InviteToken)
	require.NoError(t, err)

	// Create a second group and a graded slip in it to verify group isolation.
	otherGroup, err := f.store.Create(ctx, bob.ID, "Other Crew")
	require.NoError(t, err)

	// 2025 season: 2 graded slips (1 WON, 1 LOST where Bob is sole loser).
	wk2025_1, err := f.store.UpsertWeek(ctx, 2025, 2, 1)
	require.NoError(t, err)
	wk2025_2, err := f.store.UpsertWeek(ctx, 2025, 2, 2)
	require.NoError(t, err)

	slip25Win, err := f.store.CreateSlip(
		ctx, f.group.ID, wk2025_1.ID, nil, "Week 1", db.SlateScopeFullWeek, 1, []int64{f.user.ID, bob.ID},
	)
	require.NoError(t, err)
	for i := range 6 {
		l, err := f.store.CreateLeg(ctx, db.Leg{
			SlipID: slip25Win.ID, UserID: f.user.ID, MarketType: "ML", Selection: fmt.Sprintf("Pick %d", i), Odds: -110,
		})
		require.NoError(t, err)
		require.NoError(t, f.store.SetLegResult(ctx, l.ID, parlay.Won, f.user.Name))
	}
	require.NoError(t, f.store.SetSlipGrade(ctx, slip25Win.ID, parlay.Won, db.SlipGraded))

	slip25Loss, err := f.store.CreateSlip(
		ctx, f.group.ID, wk2025_2.ID, nil, "Week 2", db.SlateScopeFullWeek, 1, []int64{f.user.ID, bob.ID},
	)
	require.NoError(t, err)
	lAdaWin, err := f.store.CreateLeg(ctx, db.Leg{
		SlipID: slip25Loss.ID, UserID: f.user.ID, MarketType: "ML", Selection: "BUF ML", Odds: 100,
	})
	require.NoError(t, err)
	require.NoError(t, f.store.SetLegResult(ctx, lAdaWin.ID, parlay.Won, f.user.Name))
	lBobLoss, err := f.store.CreateLeg(ctx, db.Leg{
		SlipID: slip25Loss.ID, UserID: bob.ID, MarketType: "ML", Selection: "NYJ ML", Odds: 120,
	})
	require.NoError(t, err)
	require.NoError(t, f.store.SetLegResult(ctx, lBobLoss.ID, parlay.Lost, bob.Name))
	require.NoError(t, f.store.SetSlipGrade(ctx, slip25Loss.ID, parlay.Lost, db.SlipGraded))

	// Log wagers for Ada on 2025 slips; Bob logs none ("–").
	_, err = f.store.UpsertWager(ctx, db.Wager{SlipID: slip25Win.ID, UserID: f.user.ID, Stake: 20, Odds: 200})
	require.NoError(t, err)
	_, err = f.store.UpsertWager(ctx, db.Wager{SlipID: slip25Loss.ID, UserID: f.user.ID, Stake: 10, Odds: 300})
	require.NoError(t, err)

	// Other group's graded slip in 2025 must not leak into f.group.
	otherSlip, err := f.store.CreateSlip(
		ctx, otherGroup.ID, wk2025_1.ID, nil, "Other", db.SlateScopeFullWeek, 1, []int64{bob.ID},
	)
	require.NoError(t, err)
	require.NoError(t, f.store.SetSlipGrade(ctx, otherSlip.ID, parlay.Won, db.SlipGraded))

	// Default /record should pick 2025 because 2026 only has an OPEN slip (f.slip).
	rec := httptest.NewRecorder()
	f.handler.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/record", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	body := html.UnescapeString(rec.Body.String())

	assert.Contains(t, body, "2025 season")
	assert.Contains(t, body, `<option value="2025" selected>2025 Season</option>`)
	assert.Contains(t, body, `<option value="2026">2026 Season</option>`)
	assert.Contains(t, body, "1-1-0")
	assert.Contains(t, body, "2 graded slips")
	assert.Contains(t, body, "+$30.00")
	assert.Contains(t, body, "100.0%")
	assert.Contains(t, body, "Parlay Terrorist")
	assert.Contains(t, body, "Bob")
	assert.Contains(t, body, "7-0")
	assert.Contains(t, body, "0-1")

	// ?season=2026 switches to 2026 and shows the empty state for 2026.
	rec2026 := httptest.NewRecorder()
	req2026 := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/record?season=2026", nil)
	f.handler.ServeHTTP(rec2026, req2026)
	require.Equal(t, http.StatusOK, rec2026.Code)
	body2026 := html.UnescapeString(rec2026.Body.String())
	assert.Contains(t, body2026, "2026 season")
	assert.Contains(t, body2026, "No graded slips yet for the 2026 season.")
}
