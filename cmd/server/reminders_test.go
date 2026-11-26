package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/push"
	"github.com/kalverra/parlay-club/internal/reminders"
)

// stubReminderRunner stands in for the reminder service.
type stubReminderRunner struct {
	fire func(ctx context.Context, slipID int64, start time.Time) (reminders.Report, error)
}

func (s stubReminderRunner) Fire(
	ctx context.Context,
	slipID int64,
	start time.Time,
) (reminders.Report, error) {
	return s.fire(ctx, slipID, start)
}

func TestReminderEndpoint(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 11, 8, 18, 0, 0, 0, time.UTC)

	t.Run("404 when REMINDER_TOKEN is empty", func(t *testing.T) {
		t.Parallel()

		called := false
		handler := newReminderHandler("", stubReminderRunner{
			fire: func(context.Context, int64, time.Time) (reminders.Report, error) {
				called = true
				return reminders.Report{}, nil
			},
		}, zerolog.Nop())

		rec := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost,
			"/internal/reminders?slip=1&start="+unixString(start), nil)
		req.Header.Set("Authorization", "Bearer anything")
		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
		assert.False(t, called)
	})

	t.Run("401 on missing or invalid bearer token", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name string
			auth string
		}{
			{name: "missing header"},
			{name: "wrong scheme", auth: "Basic dXNlcjpwYXNz"},
			{name: "empty bearer", auth: "Bearer "},
			{name: "wrong token", auth: "Bearer wrong-secret"},
			{name: "prefix match only", auth: "Bearer super-secret-extra"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				handler := newReminderHandler("super-secret", stubReminderRunner{
					fire: func(context.Context, int64, time.Time) (reminders.Report, error) {
						t.Fatal("the runner must not see an unauthorized request")
						return reminders.Report{}, nil
					},
				}, zerolog.Nop())

				rec := httptest.NewRecorder()
				req := httptest.NewRequestWithContext(t.Context(), http.MethodPost,
					"/internal/reminders?slip=1&start="+unixString(start), nil)
				if tc.auth != "" {
					req.Header.Set("Authorization", tc.auth)
				}
				handler.ServeHTTP(rec, req)

				assert.Equal(t, http.StatusUnauthorized, rec.Code)
				assert.Equal(t, "Bearer", rec.Header().Get("WWW-Authenticate"))
			})
		}
	})

	t.Run("400 on a bad slip or start", func(t *testing.T) {
		t.Parallel()

		handler := newReminderHandler("super-secret", stubReminderRunner{
			fire: func(context.Context, int64, time.Time) (reminders.Report, error) {
				t.Fatal("a bad query must not reach the runner")
				return reminders.Report{}, nil
			},
		}, zerolog.Nop())

		for _, query := range []string{
			"", "?slip=0&start=1", "?slip=bogus&start=1",
			"?slip=1&start=0", "?slip=1&start=later",
		} {
			rec := httptest.NewRecorder()
			req := httptest.NewRequestWithContext(t.Context(), http.MethodPost,
				"/internal/reminders"+query, nil)
			req.Header.Set("Authorization", "Bearer super-secret")
			handler.ServeHTTP(rec, req)
			assert.Equal(t, http.StatusBadRequest, rec.Code, query)
		}
	})

	t.Run("200 with a stale skip", func(t *testing.T) {
		t.Parallel()

		handler := newReminderHandler("super-secret", stubReminderRunner{
			fire: func(_ context.Context, slipID int64, got time.Time) (reminders.Report, error) {
				assert.Equal(t, int64(42), slipID)
				assert.True(t, start.Equal(got))
				return reminders.Report{Skipped: "stale"}, nil
			},
		}, zerolog.Nop())

		rec := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost,
			"/internal/reminders?slip=42&start="+unixString(start), nil)
		req.Header.Set("Authorization", "Bearer super-secret")
		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.JSONEq(t, `{"skipped":"stale"}`, rec.Body.String())
	})

	t.Run("500 on a database failure", func(t *testing.T) {
		t.Parallel()

		handler := newReminderHandler("super-secret", stubReminderRunner{
			fire: func(context.Context, int64, time.Time) (reminders.Report, error) {
				return reminders.Report{}, errors.New("database is locked")
			},
		}, zerolog.Nop())

		rec := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost,
			"/internal/reminders?slip=42&start="+unixString(start), nil)
		req.Header.Set("Authorization", "Bearer super-secret")
		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusInternalServerError, rec.Code)
		assert.Contains(t, rec.Body.String(), "database is locked")
	})
}

func TestReminderRouteSitsOutsideAuthAndCSRF(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 11, 8, 18, 0, 0, 0, time.UTC)
	called := false
	handler := newReminderHandler("super-secret", stubReminderRunner{
		fire: func(context.Context, int64, time.Time) (reminders.Report, error) {
			called = true
			return reminders.Report{Sent: 1}, nil
		},
	}, zerolog.Nop())
	mux := newMux(
		http.NotFoundHandler(),
		http.NotFoundHandler(),
		http.NotFoundHandler(),
		http.NotFoundHandler(),
		nil,
		handler,
		newTestHandlers(),
		nil,
		passthrough,
		nil,
		nil,
	)

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost,
		"/internal/reminders?slip=42&start="+unixString(start), nil)
	req.Header.Set("Authorization", "Bearer super-secret")
	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"sent":1}`, rec.Body.String())
	assert.True(t, called)
}

// reminderTestSender records reminder sends.
type reminderTestSender struct {
	calls    []int64
	messages []push.Message
}

func (s *reminderTestSender) Enabled() bool { return true }

func (s *reminderTestSender) SendToUsers(_ context.Context, userIDs []int64, m push.Message) {
	s.calls = append(s.calls, userIDs...)
	s.messages = append(s.messages, m)
}

// newReminderTestStore opens a migrated database for the reminder handler test.
func newReminderTestStore(t *testing.T) *db.Store {
	t.Helper()

	database, err := db.Open(filepath.Join(t.TempDir(), "parlay.db"))
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, database.Close()) })
	require.NoError(t, db.Migrate(t.Context(), database))

	store := db.NewStore(database)
	t.Cleanup(func() { assert.NoError(t, store.Close()) })
	return store
}

func TestReminderHandlerSendsOnceAcrossTwoFires(t *testing.T) {
	t.Parallel()

	store := newReminderTestStore(t)
	owner, err := store.CreateUserWithIdentity(t.Context(), "Ada", "", "", "")
	require.NoError(t, err)
	friend, err := store.CreateUserWithIdentity(t.Context(), "Ben", "", "", "")
	require.NoError(t, err)
	group, err := store.Create(t.Context(), owner.ID, "Sunday Parlay Club")
	require.NoError(t, err)
	week, err := store.UpsertWeek(t.Context(), 2026, 2, 9)
	require.NoError(t, err)

	start := time.Date(2026, 11, 8, 18, 0, 0, 0, time.UTC)
	_, err = store.UpsertGame(t.Context(), db.Game{
		ID:        "game-1",
		WeekID:    week.ID,
		HomeTeam:  "BUF",
		AwayTeam:  "MIA",
		HomeName:  "Buffalo Bills",
		AwayName:  "Miami Dolphins",
		Kickoff:   start,
		Status:    db.GameScheduled,
		UpdatedAt: start.Add(-24 * time.Hour),
	})
	require.NoError(t, err)
	slip, err := store.CreateSlip(
		t.Context(), group.ID, week.ID, nil, "Sunday Slate",
		db.SlateScopeFullWeek, 2, []int64{owner.ID, friend.ID},
	)
	require.NoError(t, err)

	sender := &reminderTestSender{}
	service := reminders.New(reminders.Deps{
		Store:  store,
		Sender: sender,
		Now:    func() time.Time { return start.Add(-20 * time.Minute) },
		Logger: zerolog.Nop(),
	})
	handler := newReminderHandler("super-secret", service, zerolog.Nop())

	post := func() *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost,
			"/internal/reminders?slip="+itoa(slip.ID)+"&start="+unixString(start), nil)
		req.Header.Set("Authorization", "Bearer super-secret")
		handler.ServeHTTP(rec, req)
		return rec
	}

	first := post()
	require.Equal(t, http.StatusOK, first.Code)
	assert.JSONEq(t, `{"sent":2}`, first.Body.String())
	assert.ElementsMatch(t, []int64{owner.ID, friend.ID}, sender.calls)

	second := post()
	require.Equal(t, http.StatusOK, second.Code)
	var report reminders.Report
	require.NoError(t, json.Unmarshal(second.Body.Bytes(), &report))
	assert.Zero(t, report.Sent)
	assert.Len(t, sender.calls, 2, "push_log stops the retry")
}

// itoa renders an int64 for a query string.
func itoa(value int64) string {
	return strconv.FormatInt(value, 10)
}

// unixString renders an instant's unix seconds.
func unixString(value time.Time) string {
	return strconv.FormatInt(value.Unix(), 10)
}
