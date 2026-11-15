package web_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/db"
)

func TestSlipPageRendersMyWagerDefaults(t *testing.T) {
	t.Parallel()

	f := newSlipFixture(t, 2)
	_, err := f.store.CreateLeg(t.Context(), db.Leg{
		SlipID:     f.slip.ID,
		UserID:     f.user.ID,
		MarketType: "ML",
		Selection:  "BUF ML",
		Odds:       150,
	})
	require.NoError(t, err)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, fmt.Sprintf("/slips/%d", f.slip.ID), nil)
	rec := httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, `id="slip-wager"`)
	assert.Contains(t, body, "My wager")
	assert.Contains(t, body, fmt.Sprintf(`hx-put="/slips/%d/wagers/me"`, f.slip.ID))
	assert.Contains(t, body, `value="150"`)
	assert.NotContains(t, body, fmt.Sprintf(`hx-delete="/slips/%d/wagers/me"`, f.slip.ID))
}

func TestPutAndDeleteMyWager(t *testing.T) {
	t.Parallel()

	f := newSlipFixture(t, 2)

	form := url.Values{
		"stake": {"25"},
		"odds":  {"350"},
		"boost": {"25"},
		"book":  {"DraftKings"},
	}
	putReq := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPut,
		fmt.Sprintf("/slips/%d/wagers/me", f.slip.ID),
		strings.NewReader(form.Encode()),
	)
	putReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	putRec := httptest.NewRecorder()
	f.handler.ServeHTTP(putRec, putReq)

	require.Equal(t, http.StatusOK, putRec.Code)
	putBody := putRec.Body.String()
	assert.Contains(t, putBody, "Saved")
	assert.Contains(t, putBody, "DraftKings")
	assert.Contains(t, putBody, fmt.Sprintf(`hx-delete="/slips/%d/wagers/me"`, f.slip.ID))

	wager, found, err := f.store.WagerForUser(t.Context(), f.slip.ID, f.user.ID)
	require.NoError(t, err)
	require.True(t, found)
	assert.InDelta(t, float64(25), wager.Stake, 0.001)
	assert.Equal(t, 350, wager.Odds)
	assert.InDelta(t, float64(25), wager.BoostPct, 0.001)
	require.NotNil(t, wager.Book)
	assert.Equal(t, "DraftKings", *wager.Book)

	// Upsert idempotently updates the existing wager.
	form.Set("stake", "40")
	form.Set("book", "FanDuel")
	putReq2 := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPut,
		fmt.Sprintf("/slips/%d/wagers/me", f.slip.ID),
		strings.NewReader(form.Encode()),
	)
	putReq2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	putRec2 := httptest.NewRecorder()
	f.handler.ServeHTTP(putRec2, putReq2)
	require.Equal(t, http.StatusOK, putRec2.Code)

	wager, found, err = f.store.WagerForUser(t.Context(), f.slip.ID, f.user.ID)
	require.NoError(t, err)
	require.True(t, found)
	assert.InDelta(t, float64(40), wager.Stake, 0.001)
	require.NotNil(t, wager.Book)
	assert.Equal(t, "FanDuel", *wager.Book)

	// Delete clears the saved wager.
	delReq := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodDelete,
		fmt.Sprintf("/slips/%d/wagers/me", f.slip.ID),
		nil,
	)
	delRec := httptest.NewRecorder()
	f.handler.ServeHTTP(delRec, delReq)

	require.Equal(t, http.StatusOK, delRec.Code)
	delBody := delRec.Body.String()
	assert.NotContains(t, delBody, ">Saved<")
	assert.NotContains(t, delBody, fmt.Sprintf(`hx-delete="/slips/%d/wagers/me"`, f.slip.ID))

	_, found, err = f.store.WagerForUser(t.Context(), f.slip.ID, f.user.ID)
	require.NoError(t, err)
	assert.False(t, found)
}

func TestMyWagerValidationAndGroupIsolation(t *testing.T) {
	t.Parallel()

	f := newSlipFixture(t, 2)

	invalidCases := []struct {
		name string
		form url.Values
	}{
		{name: "bad stake text", form: url.Values{"stake": {"abc"}, "odds": {"200"}}},
		{name: "zero stake", form: url.Values{"stake": {"0"}, "odds": {"200"}}},
		{name: "bad odds text", form: url.Values{"stake": {"10"}, "odds": {"abc"}}},
		{name: "odds in dead zone", form: url.Values{"stake": {"10"}, "odds": {"50"}}},
		{name: "negative boost", form: url.Values{"stake": {"10"}, "odds": {"200"}, "boost": {"-5"}}},
		{name: "book too long", form: url.Values{"stake": {"10"}, "odds": {"200"}, "book": {strings.Repeat("b", 41)}}},
	}

	for _, tc := range invalidCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequestWithContext(
				t.Context(),
				http.MethodPut,
				fmt.Sprintf("/slips/%d/wagers/me", f.slip.ID),
				strings.NewReader(tc.form.Encode()),
			)
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			rec := httptest.NewRecorder()
			f.handler.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
		})
	}

	// Outsider outside the slip's group is rejected.
	outsider, err := f.store.CreateUserWithIdentity(t.Context(), "Outsider", "", "", "")
	require.NoError(t, err)
	f.as(outsider)

	validForm := url.Values{"stake": {"10"}, "odds": {"200"}}
	putReq := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPut,
		fmt.Sprintf("/slips/%d/wagers/me", f.slip.ID),
		strings.NewReader(validForm.Encode()),
	)
	putReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	putRec := httptest.NewRecorder()
	f.handler.ServeHTTP(putRec, putReq)
	assert.Equal(t, http.StatusUnprocessableEntity, putRec.Code)

	delReq := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodDelete,
		fmt.Sprintf("/slips/%d/wagers/me", f.slip.ID),
		nil,
	)
	delRec := httptest.NewRecorder()
	f.handler.ServeHTTP(delRec, delReq)
	assert.Equal(t, http.StatusUnprocessableEntity, delRec.Code)
}
