package bugs_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/bugs"
)

func TestRESTClientCreateIssue(t *testing.T) {
	t.Parallel()

	var (
		gotMethod      string
		gotPath        string
		gotAuth        string
		gotAccept      string
		gotContentType string
		gotPayload     bugs.CreateIssueRequest
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotAccept = r.Header.Get("Accept")
		gotContentType = r.Header.Get("Content-Type")

		err := json.NewDecoder(r.Body).Decode(&gotPayload)
		assert.NoError(t, err)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"number":42,"html_url":"https://github.com/kalverra/parlay-club/issues/42"}`))
	}))
	t.Cleanup(srv.Close)

	client := bugs.NewRESTClient("ghp_test_token", bugs.WithBaseURL(srv.URL))
	result, err := client.CreateIssue(t.Context(), "kalverra/parlay-club", bugs.CreateIssueRequest{
		Title:  "Bug: Slip button overlaps header",
		Body:   "### Description\nOverlaps on iPhone SE",
		Labels: []string{"bug"},
	})
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, http.MethodPost, gotMethod)
	assert.Equal(t, "/repos/kalverra/parlay-club/issues", gotPath)
	assert.Equal(t, "Bearer ghp_test_token", gotAuth)
	assert.Equal(t, "application/vnd.github+json", gotAccept)
	assert.Equal(t, "application/json", gotContentType)
	assert.Equal(t, "Bug: Slip button overlaps header", gotPayload.Title)
	assert.Equal(t, "### Description\nOverlaps on iPhone SE", gotPayload.Body)
	assert.Equal(t, []string{"bug"}, gotPayload.Labels)
	assert.Equal(t, 42, result.Number)
	assert.Equal(t, "https://github.com/kalverra/parlay-club/issues/42", result.HTMLURL)
}

func TestRESTClientCreateIssueErrors(t *testing.T) {
	t.Parallel()

	t.Run("missing token", func(t *testing.T) {
		t.Parallel()

		client := bugs.NewRESTClient("")
		_, err := client.CreateIssue(t.Context(), "kalverra/parlay-club", bugs.CreateIssueRequest{Title: "Bug"})
		require.ErrorIs(t, err, bugs.ErrMissingGitHubToken)
	})

	t.Run("invalid repo", func(t *testing.T) {
		t.Parallel()

		client := bugs.NewRESTClient("ghp_token")
		_, err := client.CreateIssue(t.Context(), "not-a-repo", bugs.CreateIssueRequest{Title: "Bug"})
		require.ErrorIs(t, err, bugs.ErrInvalidGitHubRepo)
	})

	t.Run("non-2xx status", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte(`{"message":"Validation Failed"}`))
		}))
		t.Cleanup(srv.Close)

		client := bugs.NewRESTClient("ghp_token", bugs.WithBaseURL(srv.URL))
		_, err := client.CreateIssue(t.Context(), "kalverra/parlay-club", bugs.CreateIssueRequest{Title: "Bug"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "422")
		assert.Contains(t, err.Error(), "Validation Failed")
	})

	t.Run("unreachable server", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
		srv.Close()

		client := bugs.NewRESTClient("ghp_token", bugs.WithBaseURL(srv.URL))
		_, err := client.CreateIssue(t.Context(), "kalverra/parlay-club", bugs.CreateIssueRequest{Title: "Bug"})
		require.Error(t, err)
	})

	t.Run("canceled context", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusCreated)
		}))
		t.Cleanup(srv.Close)

		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		client := bugs.NewRESTClient("ghp_token", bugs.WithBaseURL(srv.URL))
		_, err := client.CreateIssue(ctx, "kalverra/parlay-club", bugs.CreateIssueRequest{Title: "Bug"})
		require.Error(t, err)
	})
}
