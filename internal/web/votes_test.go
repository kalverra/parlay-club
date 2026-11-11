package web_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/db"
)

// addVotableLeg puts Sam on the slip with one leg Ada can vote on.
func addVotableLeg(t *testing.T, fixture *slipFixture) db.Leg {
	t.Helper()

	sam := addSam(t, fixture)
	leg, err := fixture.store.CreateLeg(t.Context(), db.Leg{
		SlipID:     fixture.slip.ID,
		UserID:     sam.ID,
		MarketType: "ML",
		Selection:  "MIA ML",
	})
	require.NoError(t, err)
	return leg
}

// getPanel fetches the slip panel partial as the signed-in person.
func getPanel(t *testing.T, fixture *slipFixture) *httptest.ResponseRecorder {
	t.Helper()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		fmt.Sprintf("/slips/%d/panel", fixture.slip.ID),
		nil,
	)
	fixture.handler.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)
	return recorder
}

// vote posts one vote through the handler.
func vote(t *testing.T, fixture *slipFixture, legID int64, value int) *httptest.ResponseRecorder {
	t.Helper()

	return postForm(t, fixture.handler, fmt.Sprintf("/legs/%d/vote", legID), url.Values{
		"value": {strconv.Itoa(value)},
	})
}

func TestLegPanelOffersVotesOnSomeoneElsesLeg(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	leg := addVotableLeg(t, fixture)

	body := getPanel(t, fixture).Body.String()
	assert.Contains(t, body, fmt.Sprintf(`hx-post="/legs/%d/vote"`, leg.ID))
	assert.Contains(t, body, "&uarr; 0")
	assert.Contains(t, body, "&darr; 0")
	assert.Contains(t, body, `data-votes`)

	response := vote(t, fixture, leg.ID, 1)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	assert.Contains(t, response.Body.String(), "&uarr; 1", "the actor's panel shows the new tally")
	assert.Contains(t, getPanel(t, fixture).Body.String(), "&uarr; 1")

	votes, err := fixture.store.VotesForSlip(t.Context(), fixture.slip.ID)
	require.NoError(t, err)
	require.Len(t, votes, 1)
	assert.Equal(t, fixture.user.ID, votes[0].UserID)
	assert.Equal(t, 1, votes[0].Value)
}

func TestTappingTheSameVoteAgainClearsIt(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	leg := addVotableLeg(t, fixture)

	require.Equal(t, http.StatusOK, vote(t, fixture, leg.ID, 1).Code)
	response := vote(t, fixture, leg.ID, 0)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	assert.Contains(t, response.Body.String(), "&uarr; 0")
	assert.Contains(t, response.Body.String(), "&darr; 0")

	votes, err := fixture.store.VotesForSlip(t.Context(), fixture.slip.ID)
	require.NoError(t, err)
	assert.Empty(t, votes)
}

func TestVoteOnYourOwnLegAnswers422(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	response := addLeg(t, fixture.handler, fixture.slip.ID, "BUF ML")
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	legs, err := fixture.store.LegsForSlip(t.Context(), fixture.slip.ID)
	require.NoError(t, err)
	require.Len(t, legs, 1)

	body := getPanel(t, fixture).Body.String()
	assert.NotContains(t, body, fmt.Sprintf(`hx-post="/legs/%d/vote"`, legs[0].ID),
		"the owner sees the tally only")

	refused := vote(t, fixture, legs[0].ID, 1)
	assert.Equal(t, http.StatusUnprocessableEntity, refused.Code)
	assert.Contains(t, refused.Body.String(), "you can't vote on your own pick")
}

func TestVoteBeforeEveryoneLocksInStillWorks(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 1)
	leg := addVotableLeg(t, fixture)
	require.Equal(t, http.StatusOK, addLeg(t, fixture.handler, fixture.slip.ID, "BUF ML").Code)

	response := vote(t, fixture, leg.ID, -1)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	assert.Contains(t, response.Body.String(), "&darr; 1")
}

func TestVoteAfterEveryoneLocksInAnswers422(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 1)
	leg := addVotableLeg(t, fixture)
	require.Equal(t, http.StatusOK, addLeg(t, fixture.handler, fixture.slip.ID, "BUF ML").Code)

	now := time.Now()
	require.NoError(t, fixture.store.SetSlipMemberLock(t.Context(), fixture.slip.ID, fixture.user.ID, &now))
	members, err := fixture.store.SlipMembers(t.Context(), fixture.slip.ID)
	require.NoError(t, err)
	for _, member := range members {
		require.NoError(t, fixture.store.SetSlipMemberLock(t.Context(), fixture.slip.ID, member.User.ID, &now))
	}
	require.NoError(t, fixture.store.SetSlipStatus(t.Context(), fixture.slip.ID, db.SlipLocked))

	body := getPanel(t, fixture).Body.String()
	assert.NotContains(t, body, fmt.Sprintf(`hx-post="/legs/%d/vote"`, leg.ID),
		"a locked slip keeps the tally but takes no votes")

	refused := vote(t, fixture, leg.ID, 1)
	assert.Equal(t, http.StatusUnprocessableEntity, refused.Code)
	assert.Contains(t, refused.Body.String(), "votes closed once everyone locked in")
}

func TestVoteRejectsAnUnknownValue(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	leg := addVotableLeg(t, fixture)

	refused := vote(t, fixture, leg.ID, 2)
	assert.Equal(t, http.StatusUnprocessableEntity, refused.Code)
	assert.Contains(t, refused.Body.String(), "vote must be up, down, or clear")
}

func TestVoteSignalsTheOtherBrowser(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	leg := addVotableLeg(t, fixture)
	server := httptest.NewServer(fixture.handler)
	t.Cleanup(server.Close)

	request, err := http.NewRequestWithContext(
		t.Context(), http.MethodGet,
		fmt.Sprintf("%s/slips/%d/events", server.URL, fixture.slip.ID), nil)
	require.NoError(t, err)
	stream, err := server.Client().Do(request)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, stream.StatusCode)
	t.Cleanup(func() { assert.NoError(t, stream.Body.Close()) })

	form := url.Values{"value": {"1"}}
	request, err = http.NewRequestWithContext(
		t.Context(), http.MethodPost,
		fmt.Sprintf("%s/legs/%d/vote", server.URL, leg.ID),
		strings.NewReader(form.Encode()))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Hx-Request", "true")
	response, err := server.Client().Do(request)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.StatusCode)
	require.NoError(t, response.Body.Close())

	assert.Equal(t, "slip-updated", waitForEvent(t, stream.Body),
		"the other browser refetches its panel from the signal")
}
