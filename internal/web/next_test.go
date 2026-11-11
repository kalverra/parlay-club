package web_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/parlay"
)

// hubNow is the injected clock the hub tests run on: Sunday at noon UTC.
var hubNow = time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)

// hubFixture builds a slip fixture on the fixed hub clock.
func hubFixture(t *testing.T, legsPerMember int) *slipFixture {
	t.Helper()

	return newSlipFixtureAt(t, legsPerMember, func() time.Time { return hubNow })
}

// hubBody serves the hub and returns its body.
func hubBody(t *testing.T, fixture *slipFixture, slipID int64) string {
	t.Helper()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		fmt.Sprintf("/slips/%d", slipID),
		nil,
	)
	fixture.handler.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)
	return recorder.Body.String()
}

// addStepLeg writes one leg straight through the store. The step function
// only counts legs, so the market does not matter.
func addStepLeg(t *testing.T, fixture *slipFixture, userID int64) db.Leg {
	t.Helper()

	leg, err := fixture.store.CreateLeg(t.Context(), db.Leg{
		SlipID:     fixture.slip.ID,
		UserID:     userID,
		MarketType: "ML",
		Selection:  "BUF ML",
		Odds:       -110,
	})
	require.NoError(t, err)
	return leg
}

// addSam puts a second person in the group and on the slip.
func addSam(t *testing.T, fixture *slipFixture) db.User {
	t.Helper()

	sam, err := fixture.store.CreateUserWithIdentity(
		t.Context(), "Sam", "sam@example.com", "google-sam", "")
	require.NoError(t, err)
	_, err = fixture.store.JoinByToken(t.Context(), sam.ID, fixture.group.InviteToken)
	require.NoError(t, err)
	require.NoError(t, fixture.store.AddSlipMember(t.Context(), fixture.slip.ID, sam.ID))
	return sam
}

func TestHubBannerPicksAndShowsTheDeadline(t *testing.T) {
	t.Parallel()

	fixture := hubFixture(t, 2)
	kickoff := hubNow.Add(90 * time.Minute)
	addGame(t, fixture, kickoff)

	body := hubBody(t, fixture, fixture.slip.ID)

	assert.Contains(t, body, "Pick 2 more")
	assert.Contains(t, body, "Pick your legs")
	assert.Contains(t, body, `href="/games"`)
	assert.Contains(t, body, "Picks close for MIA @ BUF at")
	assert.Contains(t, body, fmt.Sprintf(`datetime="%s"`, kickoff.Format(time.RFC3339)))
}

func TestHubBannerAsksForTheLock(t *testing.T) {
	t.Parallel()

	fixture := hubFixture(t, 2)
	addStepLeg(t, fixture, fixture.user.ID)
	addStepLeg(t, fixture, fixture.user.ID)

	body := hubBody(t, fixture, fixture.slip.ID)

	assert.Contains(t, body, "Ready to lock in")
	assert.Contains(t, body, fmt.Sprintf(`hx-post="/slips/%d/lock"`, fixture.slip.ID))
	assert.Contains(t, body, "Lock in")
}

func TestHubBannerWaitsOnTheRest(t *testing.T) {
	t.Parallel()

	fixture := hubFixture(t, 2)
	addSam(t, fixture)
	addStepLeg(t, fixture, fixture.user.ID)
	addStepLeg(t, fixture, fixture.user.ID)
	require.NoError(t, fixture.store.SetSlipMemberLock(
		t.Context(), fixture.slip.ID, fixture.user.ID, &hubNow))

	body := hubBody(t, fixture, fixture.slip.ID)

	assert.Contains(t, body, "Waiting on Sam")
	assert.NotContains(t, body, fmt.Sprintf(`hx-post="/slips/%d/lock"`, fixture.slip.ID),
		"a locked viewer has no lock action")
}

func TestHubBannerSaysReadyToPlace(t *testing.T) {
	t.Parallel()

	fixture := hubFixture(t, 2)
	sam := addSam(t, fixture)
	addStepLeg(t, fixture, fixture.user.ID)
	addStepLeg(t, fixture, fixture.user.ID)
	require.NoError(t, fixture.store.SetSlipMemberLock(
		t.Context(), fixture.slip.ID, fixture.user.ID, &hubNow))
	require.NoError(t, fixture.store.SetSlipMemberLock(
		t.Context(), fixture.slip.ID, sam.ID, &hubNow))
	require.NoError(t, fixture.store.SetSlipStatus(t.Context(), fixture.slip.ID, db.SlipLocked))

	body := hubBody(t, fixture, fixture.slip.ID)

	assert.Contains(t, body, "Ready to place")
	assert.Contains(t, body, "Everyone is locked in.")
}

func TestHubBannerShowsTheGrade(t *testing.T) {
	t.Parallel()

	fixture := hubFixture(t, 1)
	addStepLeg(t, fixture, fixture.user.ID)
	require.NoError(t, fixture.store.SetSlipGrade(
		t.Context(), fixture.slip.ID, parlay.Won, db.SlipGraded))

	body := hubBody(t, fixture, fixture.slip.ID)

	assert.Contains(t, body, ">Won</h2>")
	assert.Contains(t, body, "This slip is graded.")
	assert.NotContains(t, body, `href="/games"`, "a graded slip has no pick action")
}

func TestHubBannerClosesWhenEveryGameStarted(t *testing.T) {
	t.Parallel()

	fixture := hubFixture(t, 2)
	addGame(t, fixture, hubNow.Add(-2*time.Hour))

	body := hubBody(t, fixture, fixture.slip.ID)

	assert.Contains(t, body, "Picks are closed")
	assert.Contains(t, body, "Every game this week has kicked off.")
	assert.NotContains(t, body, `href="/games"`)
}

func TestHubBannerLetsAGroupMemberJoin(t *testing.T) {
	t.Parallel()

	fixture := hubFixture(t, 2)
	sam := addSam(t, fixture)
	other, err := fixture.store.CreateSlip(
		t.Context(), fixture.group.ID, fixture.week.ID, "Sam's slip", 2, []int64{sam.ID})
	require.NoError(t, err)

	body := hubBody(t, fixture, other.ID)
	assert.Contains(t, body, "Join this slip")
	assert.Contains(t, body, fmt.Sprintf(`hx-post="/slips/%d/members"`, other.ID))
	assert.Contains(t, body, fmt.Sprintf(`name="user_id" value="%d"`, fixture.user.ID))

	joined := postForm(t, fixture.handler, fmt.Sprintf("/slips/%d/members", other.ID), url.Values{
		"user_id": {strconv.FormatInt(fixture.user.ID, 10)},
	})
	require.Equal(t, http.StatusOK, joined.Code, joined.Body.String())
	assert.Contains(t, joined.Body.String(), "Pick 2 more", "joining moves the viewer on to picking")
}

func TestHubBannerHidesTheDeadlineOnceEveryoneLocks(t *testing.T) {
	t.Parallel()

	fixture := hubFixture(t, 1)
	addGame(t, fixture, hubNow.Add(90*time.Minute))
	addStepLeg(t, fixture, fixture.user.ID)
	require.NoError(t, fixture.store.SetSlipMemberLock(
		t.Context(), fixture.slip.ID, fixture.user.ID, &hubNow))
	require.NoError(t, fixture.store.SetSlipStatus(t.Context(), fixture.slip.ID, db.SlipLocked))

	body := hubBody(t, fixture, fixture.slip.ID)

	assert.Contains(t, body, "Ready to place")
	assert.NotContains(t, body, "Picks close for")
}
