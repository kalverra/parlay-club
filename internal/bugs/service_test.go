package bugs_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/bugs"
	"github.com/kalverra/parlay-club/internal/db"
)

type fakeReportStore struct {
	created      []db.BugReport
	createErr    error
	setIssueID   int64
	setIssueURL  string
	setIssueNum  int
	setIssueErr  error
	returnReport db.BugReport
}

func (f *fakeReportStore) CreateBugReport(_ context.Context, report db.BugReport) (db.BugReport, error) {
	if f.createErr != nil {
		return db.BugReport{}, f.createErr
	}
	report.ID = 7
	report.Token = "abcdef0123456789abcdef0123456789"
	report.CreatedAt = time.Date(2026, 11, 12, 18, 30, 0, 0, time.UTC)
	if report.ScreenshotMIME == "" {
		report.ScreenshotMIME = "image/jpeg"
	}
	f.created = append(f.created, report)
	f.returnReport = report
	return report, nil
}

func (f *fakeReportStore) SetGitHubIssue(_ context.Context, id int64, issueURL string, issueNum int) error {
	f.setIssueID = id
	f.setIssueURL = issueURL
	f.setIssueNum = issueNum
	return f.setIssueErr
}

func TestServiceSubmitReport(t *testing.T) {
	t.Parallel()

	reporter := db.User{ID: 3, Name: "Alex", Email: "alex@example.com"}
	input := bugs.SubmitInput{
		PageURL:     "https://parlay.example.com/slips",
		Description: "Odds badge overlaps team name",
		UserAgent:   "Mozilla/5.0",
		ScreenSize:  "390x844",
		Screenshot:  []byte{0xff, 0xd8, 0xff},
	}

	t.Run("creates report and github issue", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"number":99,"html_url":"https://github.com/kalverra/parlay-club/issues/99"}`))
		}))
		t.Cleanup(srv.Close)

		store := &fakeReportStore{}
		gh := bugs.NewRESTClient("ghp_token", bugs.WithBaseURL(srv.URL))
		svc := bugs.NewService(bugs.Deps{
			Store:   store,
			GitHub:  gh,
			Repo:    "kalverra/parlay-club",
			BaseURL: "https://parlay.example.com",
			Logger:  zerolog.Nop(),
		})

		result, err := svc.SubmitReport(t.Context(), reporter, input)
		require.NoError(t, err)
		require.Len(t, store.created, 1)
		assert.Equal(t, int64(7), store.setIssueID)
		assert.Equal(t, 99, store.setIssueNum)
		assert.Equal(t, "https://github.com/kalverra/parlay-club/issues/99", store.setIssueURL)
		assert.Equal(t, 99, result.IssueNum)
		assert.Equal(t, "https://github.com/kalverra/parlay-club/issues/99", result.IssueURL)
		require.NotNil(t, result.Report.GitHubIssueURL)
		assert.Equal(t, "https://github.com/kalverra/parlay-club/issues/99", *result.Report.GitHubIssueURL)
	})

	t.Run("degrades gracefully when github fails", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"message":"server error"}`))
		}))
		t.Cleanup(srv.Close)

		store := &fakeReportStore{}
		gh := bugs.NewRESTClient("ghp_token", bugs.WithBaseURL(srv.URL))
		svc := bugs.NewService(bugs.Deps{
			Store:   store,
			GitHub:  gh,
			Repo:    "kalverra/parlay-club",
			BaseURL: "https://parlay.example.com",
			Logger:  zerolog.Nop(),
		})

		result, err := svc.SubmitReport(t.Context(), reporter, input)
		require.NoError(t, err)
		require.Len(t, store.created, 1)
		assert.Empty(t, result.IssueURL)
		assert.Zero(t, result.IssueNum)
		assert.Zero(t, store.setIssueID)
	})

	t.Run("degrades gracefully when github token is unset", func(t *testing.T) {
		t.Parallel()

		store := &fakeReportStore{}
		svc := bugs.NewService(bugs.Deps{
			Store:   store,
			Repo:    "kalverra/parlay-club",
			BaseURL: "https://parlay.example.com",
			Logger:  zerolog.Nop(),
		})

		result, err := svc.SubmitReport(t.Context(), reporter, input)
		require.NoError(t, err)
		require.Len(t, store.created, 1)
		assert.Empty(t, result.IssueURL)
		assert.Zero(t, result.IssueNum)
	})

	t.Run("returns store error without calling github", func(t *testing.T) {
		t.Parallel()

		called := false
		srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			called = true
		}))
		t.Cleanup(srv.Close)

		store := &fakeReportStore{createErr: errors.New("db write failed")}
		gh := bugs.NewRESTClient("ghp_token", bugs.WithBaseURL(srv.URL))
		svc := bugs.NewService(bugs.Deps{
			Store:  store,
			GitHub: gh,
			Repo:   "kalverra/parlay-club",
			Logger: zerolog.Nop(),
		})

		_, err := svc.SubmitReport(t.Context(), reporter, input)
		require.Error(t, err)
		assert.False(t, called)
	})
}
