package web_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/live"
	"github.com/kalverra/parlay-club/internal/slips"
	webui "github.com/kalverra/parlay-club/internal/web"
	assets "github.com/kalverra/parlay-club/web"
)

type scheduleTestFixture struct {
	handler  http.Handler
	store    *db.Store
	user     db.User
	other    db.User
	group    db.Group
	schedule db.GroupSlipSchedule
	as       func(db.User)
	signedIn *db.User
}

func newScheduleTestFixture(t *testing.T) *scheduleTestFixture {
	t.Helper()

	database, err := db.Open(filepath.Join(t.TempDir(), "schedules.db"))
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, database.Close()) })
	require.NoError(t, db.Migrate(t.Context(), database))

	store := db.NewStore(database)
	t.Cleanup(func() { assert.NoError(t, store.Close()) })
	user, err := store.CreateUserWithIdentity(t.Context(), "Ada", "ada@example.com", "google-ada", "")
	require.NoError(t, err)
	other, err := store.CreateUserWithIdentity(t.Context(), "Ben", "ben@example.com", "google-ben", "")
	require.NoError(t, err)

	group, err := store.Create(t.Context(), user.ID, "Sunday Parlay Club")
	require.NoError(t, err)

	sched, err := store.CreateGroupSlipSchedule(
		t.Context(),
		group.ID,
		"Thursday Night Slip",
		db.SlateScopeThursday,
		2,
	)
	require.NoError(t, err)

	renderer, err := webui.NewRenderer(assets.Templates, false)
	require.NoError(t, err)

	hub := &live.Hub{}
	service := slips.New(store, hub, nil)
	stream := live.NewStream(hub, func(int64) bool { return false })

	fixture := &scheduleTestFixture{
		store:    store,
		user:     user,
		other:    other,
		group:    group,
		schedule: sched,
		signedIn: &user,
	}
	fixture.as = func(u db.User) {
		fixture.signedIn = &u
	}

	handler := webui.NewWithDeps(webui.Dependencies{
		Logger:   zerolog.Nop(),
		Renderer: renderer,
		UserFrom: func(context.Context) (db.User, bool) {
			if fixture.signedIn == nil {
				return db.User{}, false
			}
			return *fixture.signedIn, true
		},
		GroupsFrom: func(context.Context) []webui.GroupOption {
			return []webui.GroupOption{{ID: group.ID, Name: group.Name, Current: true}}
		},
		GroupFrom: func(context.Context) (db.Group, bool) {
			return group, true
		},
		Store:  store,
		Slips:  service,
		Stream: stream,
	})
	fixture.handler = handler

	return fixture
}

func TestCreateGroupSlipSchedule(t *testing.T) {
	t.Parallel()

	t.Run("unauthenticated request redirects to login", func(t *testing.T) {
		t.Parallel()
		f := newScheduleTestFixture(t)
		f.signedIn = nil

		form := url.Values{
			"name":        {"Sunday Main"},
			"slate_scope": {"sunday_main"},
		}
		// Plain POST returns 303 to /login
		req := httptest.NewRequestWithContext(
			t.Context(),
			http.MethodPost,
			fmt.Sprintf("/groups/%d/schedules", f.group.ID),
			strings.NewReader(form.Encode()),
		)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		f.handler.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusSeeOther, rec.Code)
		assert.Equal(t, "/login", rec.Header().Get("Location"))

		// HTMX POST returns 200 with Hx-Redirect
		recHTMX := postForm(t, f.handler, fmt.Sprintf("/groups/%d/schedules", f.group.ID), form)
		assert.Equal(t, http.StatusOK, recHTMX.Code)
		assert.Equal(t, "/login", recHTMX.Header().Get("Hx-Redirect"))
	})

	t.Run("non-member caller gets 404", func(t *testing.T) {
		t.Parallel()
		f := newScheduleTestFixture(t)
		f.as(f.other)

		form := url.Values{
			"name":        {"Sunday Main"},
			"slate_scope": {"sunday_main"},
		}
		rec := postForm(t, f.handler, fmt.Sprintf("/groups/%d/schedules", f.group.ID), form)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("invalid group id gets 404", func(t *testing.T) {
		t.Parallel()
		f := newScheduleTestFixture(t)

		form := url.Values{
			"name": {"Sunday Main"},
		}
		rec := postForm(t, f.handler, "/groups/invalid-id/schedules", form)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("creates schedule successfully with defaults via form", func(t *testing.T) {
		t.Parallel()
		f := newScheduleTestFixture(t)

		form := url.Values{
			"name": {"New Sunday Slate"},
		}
		rec := postForm(t, f.handler, fmt.Sprintf("/groups/%d/schedules", f.group.ID), form)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "true", rec.Header().Get("Hx-Refresh"))

		schedules, err := f.store.GroupSlipSchedules(t.Context(), f.group.ID)
		require.NoError(t, err)
		var created *db.GroupSlipSchedule
		for _, s := range schedules {
			if s.Name == "New Sunday Slate" {
				created = &s
				break
			}
		}
		require.NotNil(t, created)
		assert.Equal(t, db.SlateScopeSundayMain, created.SlateScope)
		assert.Equal(t, 2, created.LegsPerMember)
		assert.True(t, created.IsActive)
	})

	t.Run("creates schedule successfully via JSON", func(t *testing.T) {
		t.Parallel()
		f := newScheduleTestFixture(t)

		payload := `{"name":"Weekend Fun","slate_scope":"full_week","legs_per_member":3}`
		req := httptest.NewRequestWithContext(
			t.Context(),
			http.MethodPost,
			fmt.Sprintf("/groups/%d/schedules", f.group.ID),
			strings.NewReader(payload),
		)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		rec := httptest.NewRecorder()
		f.handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusCreated, rec.Code)
		var sched db.GroupSlipSchedule
		err := json.Unmarshal(rec.Body.Bytes(), &sched)
		require.NoError(t, err)
		assert.Equal(t, "Weekend Fun", sched.Name)
		assert.Equal(t, db.SlateScopeFullWeek, sched.SlateScope)
		assert.Equal(t, 3, sched.LegsPerMember)
		assert.True(t, sched.IsActive)
	})

	t.Run("empty name returns 422", func(t *testing.T) {
		t.Parallel()
		f := newScheduleTestFixture(t)

		form := url.Values{
			"name": {"   "},
		}
		rec := postForm(t, f.handler, fmt.Sprintf("/groups/%d/schedules", f.group.ID), form)
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	})

	t.Run("duplicate name in group returns 422", func(t *testing.T) {
		t.Parallel()
		f := newScheduleTestFixture(t)

		form := url.Values{
			"name": {f.schedule.Name},
		}
		rec := postForm(t, f.handler, fmt.Sprintf("/groups/%d/schedules", f.group.ID), form)
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	})

	t.Run("invalid scope returns 422", func(t *testing.T) {
		t.Parallel()
		f := newScheduleTestFixture(t)

		form := url.Values{
			"name":        {"Bad Scope Schedule"},
			"slate_scope": {"tuesday_night"},
		}
		rec := postForm(t, f.handler, fmt.Sprintf("/groups/%d/schedules", f.group.ID), form)
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	})

	t.Run("invalid legs per member range returns 422", func(t *testing.T) {
		t.Parallel()
		f := newScheduleTestFixture(t)

		form := url.Values{
			"name":            {"Six Legs"},
			"legs_per_member": {"6"},
		}
		rec := postForm(t, f.handler, fmt.Sprintf("/groups/%d/schedules", f.group.ID), form)
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	})

	t.Run("non-integer legs per member returns 422", func(t *testing.T) {
		t.Parallel()
		f := newScheduleTestFixture(t)

		form := url.Values{
			"name":            {"Invalid Legs"},
			"legs_per_member": {"abc"},
		}
		rec := postForm(t, f.handler, fmt.Sprintf("/groups/%d/schedules", f.group.ID), form)
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	})
}

func TestToggleGroupSlipScheduleActive(t *testing.T) {
	t.Parallel()

	t.Run("unauthenticated request redirects to login", func(t *testing.T) {
		t.Parallel()
		f := newScheduleTestFixture(t)
		f.signedIn = nil

		req := httptest.NewRequestWithContext(
			t.Context(),
			http.MethodPost,
			fmt.Sprintf("/groups/%d/schedules/%d/active", f.group.ID, f.schedule.ID),
			nil,
		)
		rec := httptest.NewRecorder()
		f.handler.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusSeeOther, rec.Code)
		assert.Equal(t, "/login", rec.Header().Get("Location"))

		recHTMX := postForm(
			t,
			f.handler,
			fmt.Sprintf("/groups/%d/schedules/%d/active", f.group.ID, f.schedule.ID),
			url.Values{},
		)
		assert.Equal(t, http.StatusOK, recHTMX.Code)
		assert.Equal(t, "/login", recHTMX.Header().Get("Hx-Redirect"))
	})

	t.Run("schedule not found returns 404", func(t *testing.T) {
		t.Parallel()
		f := newScheduleTestFixture(t)

		rec := postForm(t, f.handler, fmt.Sprintf("/groups/%d/schedules/99999/active", f.group.ID), url.Values{})
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("non-member of group returns 404", func(t *testing.T) {
		t.Parallel()
		f := newScheduleTestFixture(t)
		f.as(f.other)

		rec := postForm(
			t,
			f.handler,
			fmt.Sprintf("/groups/%d/schedules/%d/active", f.group.ID, f.schedule.ID),
			url.Values{},
		)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("group mismatch in path returns 404", func(t *testing.T) {
		t.Parallel()
		f := newScheduleTestFixture(t)

		rec := postForm(t, f.handler, fmt.Sprintf("/groups/9999/schedules/%d/active", f.schedule.ID), url.Values{})
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("empty active form value toggles status", func(t *testing.T) {
		t.Parallel()
		f := newScheduleTestFixture(t)
		require.True(t, f.schedule.IsActive)

		// Toggle to inactive via /groups/{id}/schedules/{scheduleID}/active
		rec := postForm(
			t,
			f.handler,
			fmt.Sprintf("/groups/%d/schedules/%d/active", f.group.ID, f.schedule.ID),
			url.Values{},
		)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "true", rec.Header().Get("Hx-Refresh"))

		sched, err := f.store.GroupSlipScheduleByID(t.Context(), f.schedule.ID)
		require.NoError(t, err)
		assert.False(t, sched.IsActive)

		// Toggle back to active via /groups/{id}/schedules/{scheduleID}/active
		rec2 := postForm(
			t,
			f.handler,
			fmt.Sprintf("/groups/%d/schedules/%d/active", f.group.ID, f.schedule.ID),
			url.Values{},
		)
		assert.Equal(t, http.StatusOK, rec2.Code)

		sched2, err := f.store.GroupSlipScheduleByID(t.Context(), f.schedule.ID)
		require.NoError(t, err)
		assert.True(t, sched2.IsActive)
	})

	t.Run("explicit active value turns on or off", func(t *testing.T) {
		t.Parallel()
		f := newScheduleTestFixture(t)

		// Explicit off
		formOff := url.Values{"active": {"false"}}
		rec := postForm(t, f.handler, fmt.Sprintf("/groups/%d/schedules/%d/active", f.group.ID, f.schedule.ID), formOff)
		assert.Equal(t, http.StatusOK, rec.Code)

		sched, err := f.store.GroupSlipScheduleByID(t.Context(), f.schedule.ID)
		require.NoError(t, err)
		assert.False(t, sched.IsActive)

		// Explicit on
		formOn := url.Values{"active": {"true"}}
		rec2 := postForm(t, f.handler, fmt.Sprintf("/groups/%d/schedules/%d/active", f.group.ID, f.schedule.ID), formOn)
		assert.Equal(t, http.StatusOK, rec2.Code)

		sched2, err := f.store.GroupSlipScheduleByID(t.Context(), f.schedule.ID)
		require.NoError(t, err)
		assert.True(t, sched2.IsActive)
	})

	t.Run("invalid active form value returns 422", func(t *testing.T) {
		t.Parallel()
		f := newScheduleTestFixture(t)

		form := url.Values{"active": {"maybe"}}
		rec := postForm(t, f.handler, fmt.Sprintf("/groups/%d/schedules/%d/active", f.group.ID, f.schedule.ID), form)
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	})

	t.Run("toggle via JSON returns updated object", func(t *testing.T) {
		t.Parallel()
		f := newScheduleTestFixture(t)

		payload := `{"active":false}`
		req := httptest.NewRequestWithContext(
			t.Context(),
			http.MethodPost,
			fmt.Sprintf("/groups/%d/schedules/%d/active", f.group.ID, f.schedule.ID),
			strings.NewReader(payload),
		)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		rec := httptest.NewRecorder()
		f.handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var sched db.GroupSlipSchedule
		err := json.Unmarshal(rec.Body.Bytes(), &sched)
		require.NoError(t, err)
		assert.False(t, sched.IsActive)
	})
}

func TestListGroupSlipSchedules(t *testing.T) {
	t.Parallel()

	f := newScheduleTestFixture(t)

	req := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		fmt.Sprintf("/groups/%d/schedules", f.group.ID),
		nil,
	)
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var list []db.GroupSlipSchedule
	err := json.Unmarshal(rec.Body.Bytes(), &list)
	require.NoError(t, err)
	// group was created with 1 backfilled schedule from migration, plus 1 in fixture
	assert.NotEmpty(t, list)
}

func TestUpdateAndDeleteGroupSlipSchedule(t *testing.T) {
	t.Parallel()

	f := newScheduleTestFixture(t)

	// Update schedule
	payload := `{"name":"Updated Name","slate_scope":"full_week","legs_per_member":4}`
	req := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPatch,
		fmt.Sprintf("/groups/%d/schedules/%d", f.group.ID, f.schedule.ID),
		strings.NewReader(payload),
	)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var updated db.GroupSlipSchedule
	err := json.Unmarshal(rec.Body.Bytes(), &updated)
	require.NoError(t, err)
	assert.Equal(t, "Updated Name", updated.Name)
	assert.Equal(t, db.SlateScopeFullWeek, updated.SlateScope)
	assert.Equal(t, 4, updated.LegsPerMember)

	// Delete schedule
	delReq := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodDelete,
		fmt.Sprintf("/groups/%d/schedules/%d", f.group.ID, f.schedule.ID),
		nil,
	)
	delReq.Header.Set("Accept", "application/json")
	delRec := httptest.NewRecorder()
	f.handler.ServeHTTP(delRec, delReq)

	assert.Equal(t, http.StatusNoContent, delRec.Code)

	// Verify it is gone
	_, err = f.store.GroupSlipScheduleByID(t.Context(), f.schedule.ID)
	assert.ErrorIs(t, err, db.ErrScheduleNotFound)
}
