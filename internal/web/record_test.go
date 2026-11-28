package web_test

import (
	"fmt"
	"html"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/parlay"
	assets "github.com/kalverra/parlay-club/web"
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
	f.handler.ServeHTTP(rec, httptest.NewRequestWithContext(ctx, http.MethodGet, "/record", nil))
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
	assert.Contains(t, body, "Ball Knower")
	assert.Contains(t, body, "Heater")
	assert.Contains(t, body, "Bob")
	assert.Contains(t, body, "7-0")
	assert.Contains(t, body, "0-1")
	assert.Contains(t, body, fmt.Sprintf(`href="/record/members/%d?season=2025"`, f.user.ID))
	assert.Contains(t, body, fmt.Sprintf(`href="/record/members/%d?season=2025"`, bob.ID))
	assert.Contains(t, body, `data-tag-explain`)
	assert.Contains(t, body, `data-tag-label="Parlay Terrorist"`)
	assert.Contains(t, body, `data-tag-tone="insult"`)
	assert.Contains(t, body, `data-tag-rule=`)
	assert.Contains(t, body, `data-tag-label="Heater"`)
	assert.Contains(t, body, `data-tag-tone="praise"`)
	assert.Contains(t, body, `<dialog id="tag-explain-dialog" class="leg-sheet"`)
	assert.Contains(t, body, `data-close-tag-explain`)

	// Tag buttons must never be nested inside member <a> links.
	for rest := body; ; {
		idx := strings.Index(rest, `href="/record/members/`)
		if idx < 0 {
			break
		}
		rest = rest[idx:]
		end := strings.Index(rest, "</a>")
		require.Positive(t, end)
		assert.NotContains(t, rest[:end], "<button")
		rest = rest[end:]
	}

	// ?season=2026 switches to 2026 and shows the empty state for 2026.
	rec2026 := httptest.NewRecorder()
	f.handler.ServeHTTP(rec2026, httptest.NewRequestWithContext(ctx, http.MethodGet, "/record?season=2026", nil))
	require.Equal(t, http.StatusOK, rec2026.Code)
	body2026 := html.UnescapeString(rec2026.Body.String())
	assert.Contains(t, body2026, "2026 season")
	assert.Contains(t, body2026, "No graded slips yet for the 2026 season.")
}

func TestRecordBookVotesAndEvalGradesFailSoft(t *testing.T) {
	t.Parallel()

	f := newSlipFixture(t, 2)
	ctx := t.Context()

	bob, err := f.store.CreateUserWithIdentity(ctx, "Bob", "bob-votes@example.com", "google-bob-votes", "")
	require.NoError(t, err)
	_, err = f.store.JoinByToken(ctx, bob.ID, f.group.InviteToken)
	require.NoError(t, err)

	wk, err := f.store.UpsertWeek(ctx, 2025, 2, 3)
	require.NoError(t, err)
	slip, err := f.store.CreateSlip(
		ctx, f.group.ID, wk.ID, nil, "Week 3", db.SlateScopeFullWeek, 1, []int64{f.user.ID, bob.ID},
	)
	require.NoError(t, err)

	// Ada wins 3 downvoted picks -> earns "Told You So" via VotesForGroupSeason.
	for i := range 3 {
		leg, err := f.store.CreateLeg(ctx, db.Leg{
			SlipID: slip.ID, UserID: f.user.ID, MarketType: "ML", Selection: fmt.Sprintf("Underdog %d", i), Odds: -110,
		})
		require.NoError(t, err)
		require.NoError(t, f.store.SetLegResult(ctx, leg.ID, parlay.Won, f.user.Name))
		require.NoError(t, f.store.SetLegVote(ctx, leg.ID, bob.ID, -1, time.Now()))
	}
	require.NoError(t, f.store.SetSlipGrade(ctx, slip.ID, parlay.Won, db.SlipGraded))

	// Even without archived pre-kickoff games/odds (so EvalCache has no grades),
	// GET /record fails soft and renders non-grade tags like "Told You So".
	rec := httptest.NewRecorder()
	f.handler.ServeHTTP(rec, httptest.NewRequestWithContext(ctx, http.MethodGet, "/record?season=2025", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	body := html.UnescapeString(rec.Body.String())
	assert.Contains(t, body, `data-tag-label="Told You So"`)
	assert.Contains(t, body, `Won 3 of 3 net-downvoted picks (100.0%).`)
}

func TestRecordBookMatchesAppJSTagExplainIDs(t *testing.T) {
	t.Parallel()

	source, err := fs.ReadFile(assets.Static, "static/app.js")
	require.NoError(t, err)

	matches := regexp.MustCompile(`getElementById\("(tag-explain-[^"]+)"\)`).FindAllStringSubmatch(string(source), -1)
	require.NotEmpty(t, matches)

	f := newSlipFixture(t, 2)
	rec := httptest.NewRecorder()
	f.handler.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/record", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()

	assert.Contains(t, body, `<dialog id="tag-explain-dialog" class="leg-sheet"`)
	assert.Contains(t, body, `data-close-tag-explain`)
	for _, match := range matches {
		id := match[1]
		assert.Contains(
			t,
			body,
			`id="`+id+`"`,
			"app.js references #%s which is missing from rendered /record page",
			id,
		)
	}
}

func TestRecordMemberHistoryScopingAndRendering(t *testing.T) {
	t.Parallel()

	f := newSlipFixture(t, 2)
	ctx := t.Context()

	bob, err := f.store.CreateUserWithIdentity(ctx, "Bob", "bob-member@example.com", "google-bob-member", "")
	require.NoError(t, err)
	_, err = f.store.JoinByToken(ctx, bob.ID, f.group.InviteToken)
	require.NoError(t, err)

	// Outsider belongs only to a different group and must never be accessible via f.group.
	outsider, err := f.store.CreateUserWithIdentity(ctx, "Outsider", "outsider@example.com", "google-outsider", "")
	require.NoError(t, err)
	otherGroup, err := f.store.Create(ctx, outsider.ID, "Other Crew")
	require.NoError(t, err)

	wkReg1, err := f.store.UpsertWeek(ctx, 2025, 2, 1)
	require.NoError(t, err)
	wkReg2, err := f.store.UpsertWeek(ctx, 2025, 2, 2)
	require.NoError(t, err)
	wkWildCard, err := f.store.UpsertWeek(ctx, 2025, 3, 1)
	require.NoError(t, err)

	// Slip 1 (Week 1): Bob has 1 winning leg.
	slip1, err := f.store.CreateSlip(
		ctx, f.group.ID, wkReg1.ID, nil, "Week 1", db.SlateScopeFullWeek, 1, []int64{f.user.ID, bob.ID},
	)
	require.NoError(t, err)
	leg1, err := f.store.CreateLeg(ctx, db.Leg{
		SlipID: slip1.ID, UserID: bob.ID, MarketType: "ML", Selection: "BUF ML", Odds: -115,
	})
	require.NoError(t, err)
	require.NoError(t, f.store.SetLegResult(ctx, leg1.ID, parlay.Won, f.user.Name))
	require.NoError(t, f.store.SetSlipGrade(ctx, slip1.ID, parlay.Won, db.SlipGraded))

	// Slip 2 (Week 2): Bob is the sole losing leg on a lost slip + receives a downvote.
	slip2, err := f.store.CreateSlip(
		ctx, f.group.ID, wkReg2.ID, nil, "Week 2", db.SlateScopeFullWeek, 1, []int64{f.user.ID, bob.ID},
	)
	require.NoError(t, err)
	legAdaWin, err := f.store.CreateLeg(ctx, db.Leg{
		SlipID: slip2.ID, UserID: f.user.ID, MarketType: "SPREAD", Selection: "KC -3.5", Odds: -110,
	})
	require.NoError(t, err)
	require.NoError(t, f.store.SetLegResult(ctx, legAdaWin.ID, parlay.Won, f.user.Name))
	legBobSoleLoss, err := f.store.CreateLeg(ctx, db.Leg{
		SlipID: slip2.ID, UserID: bob.ID, MarketType: "PASS_YDS", Selection: "Josh Allen Over 255.5", Odds: 120,
	})
	require.NoError(t, err)
	require.NoError(t, f.store.SetLegResult(ctx, legBobSoleLoss.ID, parlay.Lost, f.user.Name))
	require.NoError(t, f.store.SetLegVote(ctx, legBobSoleLoss.ID, f.user.ID, -1, time.Now()))
	require.NoError(t, f.store.SetSlipGrade(ctx, slip2.ID, parlay.Lost, db.SlipGraded))

	// Slip 3 (Wild Card): newest slip, Bob wins a leg.
	slip3, err := f.store.CreateSlip(
		ctx, f.group.ID, wkWildCard.ID, nil, "Wild Card Parlay", db.SlateScopeFullWeek, 1, []int64{bob.ID},
	)
	require.NoError(t, err)
	leg3, err := f.store.CreateLeg(ctx, db.Leg{
		SlipID: slip3.ID, UserID: bob.ID, MarketType: "TOTAL", Selection: "Over 47.5", Odds: -105,
	})
	require.NoError(t, err)
	require.NoError(t, f.store.SetLegResult(ctx, leg3.ID, parlay.Won, bob.Name))
	require.NoError(t, f.store.SetSlipGrade(ctx, slip3.ID, parlay.Won, db.SlipGraded))

	// Other group's slip for outsider must not leak.
	otherSlip, err := f.store.CreateSlip(
		ctx, otherGroup.ID, wkReg1.ID, nil, "Secret Slip", db.SlateScopeFullWeek, 1, []int64{outsider.ID},
	)
	require.NoError(t, err)
	otherLeg, err := f.store.CreateLeg(ctx, db.Leg{
		SlipID: otherSlip.ID, UserID: outsider.ID, MarketType: "ML", Selection: "MIA ML", Odds: 150,
	})
	require.NoError(t, err)
	require.NoError(t, f.store.SetLegResult(ctx, otherLeg.ID, parlay.Won, outsider.Name))
	require.NoError(t, f.store.SetSlipGrade(ctx, otherSlip.ID, parlay.Won, db.SlipGraded))

	// 400 on bad member ID or bad season.
	badID := httptest.NewRecorder()
	f.handler.ServeHTTP(badID, httptest.NewRequestWithContext(ctx, http.MethodGet, "/record/members/abc", nil))
	assert.Equal(t, http.StatusBadRequest, badID.Code)

	badSeason := httptest.NewRecorder()
	f.handler.ServeHTTP(
		badSeason,
		httptest.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("/record/members/%d?season=abc", bob.ID), nil),
	)
	assert.Equal(t, http.StatusBadRequest, badSeason.Code)

	// 404 when probing a user outside the active group.
	notFound := httptest.NewRecorder()
	outsiderURL := fmt.Sprintf("/record/members/%d?season=2025", outsider.ID)
	f.handler.ServeHTTP(notFound, httptest.NewRequestWithContext(ctx, http.MethodGet, outsiderURL, nil))
	assert.Equal(t, http.StatusNotFound, notFound.Code)

	// 200 on Bob's member history for 2025.
	rec := httptest.NewRecorder()
	f.handler.ServeHTTP(
		rec,
		httptest.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("/record/members/%d?season=2025", bob.ID), nil),
	)
	require.Equal(t, http.StatusOK, rec.Code)
	body := html.UnescapeString(rec.Body.String())

	assert.Contains(t, body, `href="/record?season=2025"`)
	assert.Contains(t, body, fmt.Sprintf(`action="/record/members/%d"`, bob.ID))
	assert.Contains(t, body, "Bob")
	assert.Contains(t, body, "2025 season")
	assert.Contains(t, body, "2-1")
	assert.Contains(t, body, "3 graded legs")
	assert.Contains(t, body, `data-tag-label="Parlay Terrorist"`)
	assert.Contains(t, body, `<dialog id="tag-explain-dialog" class="leg-sheet"`)
	assert.Contains(t, body, "Sole killer")
	assert.Contains(t, body, "-1 votes")
	assert.Contains(t, body, "Pass Yds")
	assert.Contains(t, body, "+120")
	assert.Contains(t, body, fmt.Sprintf(`href="/slips/%d"`, slip1.ID))
	assert.Contains(t, body, fmt.Sprintf(`href="/slips/%d"`, slip2.ID))
	assert.Contains(t, body, fmt.Sprintf(`href="/slips/%d"`, slip3.ID))
	assert.NotContains(t, body, "Secret Slip")
	assert.NotContains(t, body, "KC -3.5")

	// Verify newest-first ordering: Wild Card (slip3) -> Week 2 (slip2) -> Week 1 (slip1).
	idx3 := strings.Index(body, "Over 47.5")
	idx2 := strings.Index(body, "Josh Allen Over 255.5")
	idx1 := strings.Index(body, "BUF ML")
	require.Positive(t, idx3)
	require.Positive(t, idx2)
	require.Positive(t, idx1)
	assert.Less(t, idx3, idx2)
	assert.Less(t, idx2, idx1)

	// Empty state for a season with no graded legs for Bob.
	rec2026 := httptest.NewRecorder()
	f.handler.ServeHTTP(
		rec2026,
		httptest.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("/record/members/%d?season=2026", bob.ID), nil),
	)
	require.Equal(t, http.StatusOK, rec2026.Code)
	assert.Contains(t, html.UnescapeString(rec2026.Body.String()), "No graded legs for Bob in the 2026 season.")
}
