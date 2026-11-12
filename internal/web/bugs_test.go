package web_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/bugs"
	"github.com/kalverra/parlay-club/internal/db"
	webui "github.com/kalverra/parlay-club/internal/web"
	assets "github.com/kalverra/parlay-club/web"
)

type fakeGitHubClient struct {
	result *bugs.IssueResult
	err    error
	calls  int
	last   bugs.CreateIssueRequest
}

func (f *fakeGitHubClient) CreateIssue(
	_ context.Context,
	_ string,
	req bugs.CreateIssueRequest,
) (*bugs.IssueResult, error) {
	f.calls++
	f.last = req
	if f.err != nil {
		return nil, f.err
	}
	return f.result, nil
}

func TestBugReportSubmitSuccessWithGitHubIssue(t *testing.T) {
	t.Parallel()

	store := openBugTestStore(t)
	user, err := store.CreateUserWithIdentity(t.Context(), "Ada", "ada@example.com", "google-ada", "")
	require.NoError(t, err)

	gh := &fakeGitHubClient{
		result: &bugs.IssueResult{
			Number:  42,
			HTMLURL: "https://github.com/kalverra/parlay-club/issues/42",
		},
	}
	handler := newBugTestRouter(t, store, gh, func(context.Context) (db.User, bool) {
		return user, true
	})

	rawJPEG := []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 'J', 'F', 'I', 'F'}
	form := url.Values{
		"description": {"Odds badge overlaps team name"},
		"page_url":    {"https://parlay.club/slips/1"},
		"screen_size": {"390x844"},
		"user_agent":  {"MobileSafari/17.0"},
		"screenshot":  {"data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(rawJPEG)},
	}

	req := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"/bug-reports",
		strings.NewReader(form.Encode()),
	)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Hx-Request", "true")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "text/html; charset=utf-8", rec.Header().Get("Content-Type"))
	body := rec.Body.String()
	assert.Contains(t, body, "Bug report submitted.")
	assert.Contains(t, body, `href="https://github.com/kalverra/parlay-club/issues/42"`)
	assert.Contains(t, body, "Issue #42")
	assert.Equal(t, 1, gh.calls)
	assert.Contains(t, gh.last.Body, "/screenshots/")
}

func TestBugReportSubmitSuccessWithoutScreenshotOrGitHub(t *testing.T) {
	t.Parallel()

	store := openBugTestStore(t)
	user, err := store.CreateUserWithIdentity(t.Context(), "Ada", "ada@example.com", "google-ada", "")
	require.NoError(t, err)

	handler := newBugTestRouter(t, store, nil, func(context.Context) (db.User, bool) {
		return user, true
	})

	form := url.Values{
		"description": {"Submit button stays disabled"},
		"page_url":    {"https://parlay.club/pick"},
	}

	req := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"/bug-reports",
		strings.NewReader(form.Encode()),
	)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Hx-Request", "true")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "Bug report submitted.")
	assert.Contains(t, body, "Saved locally for review.")
}

func TestBugReportSubmitRequiresUser(t *testing.T) {
	t.Parallel()

	store := openBugTestStore(t)
	handler := newBugTestRouter(t, store, nil, func(context.Context) (db.User, bool) {
		return db.User{}, false
	})

	form := url.Values{"description": {"Broken"}}
	req := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"/bug-reports",
		strings.NewReader(form.Encode()),
	)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Hx-Request", "true")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestBugReportSubmitValidationErrors(t *testing.T) {
	t.Parallel()

	store := openBugTestStore(t)
	user, err := store.CreateUserWithIdentity(t.Context(), "Ada", "ada@example.com", "google-ada", "")
	require.NoError(t, err)

	gh := &fakeGitHubClient{}
	handler := newBugTestRouter(t, store, gh, func(context.Context) (db.User, bool) {
		return user, true
	})

	oversizedImage := "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{'a'}, 2<<20))
	oversized := url.Values{
		"description": {"Too big"},
		"screenshot":  {oversizedImage},
	}.Encode()

	oversizedDecodedImage := "data:image/jpeg;base64," +
		base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{'b'}, (1<<20)+1))
	oversizedScreenshot := url.Values{
		"description": {"Image over 1 MiB"},
		"screenshot":  {oversizedDecodedImage},
	}.Encode()

	tests := []struct {
		name        string
		body        string
		wantMessage string
	}{
		{
			name:        "missing description",
			body:        url.Values{"description": {"   "}}.Encode(),
			wantMessage: "Please describe what broke or looks wrong.",
		},
		{
			name:        "description too long",
			body:        url.Values{"description": {strings.Repeat("x", 4001)}}.Encode(),
			wantMessage: "Description is too long (max 4000 characters).",
		},
		{
			name:        "invalid screenshot prefix",
			body:        url.Values{"description": {"Broken"}, "screenshot": {"not-a-data-url"}}.Encode(),
			wantMessage: "screenshot must be a base64 data URL",
		},
		{
			name: "invalid screenshot mime",
			body: url.Values{
				"description": {"Broken"},
				"screenshot":  {"data:text/plain;base64,SGVsbG8="},
			}.Encode(),
			wantMessage: "screenshot must be a base64 JPEG, PNG, or WebP image",
		},
		{
			name: "invalid screenshot base64",
			body: url.Values{
				"description": {"Broken"},
				"screenshot":  {"data:image/jpeg;base64,%%%invalid%%%"},
			}.Encode(),
			wantMessage: "screenshot image data is invalid",
		},
		{
			name:        "screenshot exceeds 1 MiB limit",
			body:        oversizedScreenshot,
			wantMessage: "screenshot exceeds maximum size",
		},
		{
			name:        "oversized payload",
			body:        oversized,
			wantMessage: "Bug report is too large (max 2 MB).",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequestWithContext(
				t.Context(),
				http.MethodPost,
				"/bug-reports",
				strings.NewReader(tt.body),
			)
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.Header.Set("Hx-Request", "true")

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusBadRequest, rec.Code)
			assert.Equal(t, "text/html; charset=utf-8", rec.Header().Get("Content-Type"))
			assert.Contains(t, rec.Body.String(), `role="alert"`)
			assert.Contains(t, rec.Body.String(), tt.wantMessage)
		})
	}

	assert.Zero(t, gh.calls)
}

func TestBugReportSubmitServesScreenshotRoundTrip(t *testing.T) {
	t.Parallel()

	store := openBugTestStore(t)
	user, err := store.CreateUserWithIdentity(t.Context(), "Ada", "ada@example.com", "google-ada", "")
	require.NoError(t, err)

	gh := &fakeGitHubClient{
		result: &bugs.IssueResult{
			Number:  77,
			HTMLURL: "https://github.com/kalverra/parlay-club/issues/77",
		},
	}
	handler := newBugTestRouter(t, store, gh, func(context.Context) (db.User, bool) {
		return user, true
	})

	rawJPEG := []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0xff, 0xd9}
	form := url.Values{
		"description": {"Dialog clips on mobile"},
		"page_url":    {"https://parlay.club/games"},
		"screen_size": {"390x844"},
		"user_agent":  {"MobileSafari/17.0"},
		"screenshot":  {"data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(rawJPEG)},
	}

	postReq := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"/bug-reports",
		strings.NewReader(form.Encode()),
	)
	postReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	postReq.Header.Set("Hx-Request", "true")

	postRec := httptest.NewRecorder()
	handler.ServeHTTP(postRec, postReq)
	require.Equal(t, http.StatusOK, postRec.Code)

	screenshotPath := regexp.MustCompile(`/screenshots/[0-9a-f]+\.jpg`).FindString(gh.last.Body)
	require.NotEmpty(t, screenshotPath, "GitHub issue body should embed the screenshot capability URL")

	getReq := httptest.NewRequestWithContext(t.Context(), http.MethodGet, screenshotPath, nil)
	getReq.SetPathValue("file", strings.TrimPrefix(screenshotPath, "/screenshots/"))
	getRec := httptest.NewRecorder()
	webui.ScreenshotHandler(store, zerolog.Nop()).ServeHTTP(getRec, getReq)

	assert.Equal(t, http.StatusOK, getRec.Code)
	assert.Equal(t, "image/jpeg", getRec.Header().Get("Content-Type"))
	assert.Equal(t, "public, max-age=31536000, immutable", getRec.Header().Get("Cache-Control"))
	assert.Equal(t, rawJPEG, getRec.Body.Bytes())
}

func TestBugReportSubmitGitHubFailureStillSavesLocally(t *testing.T) {
	t.Parallel()

	store := openBugTestStore(t)
	user, err := store.CreateUserWithIdentity(t.Context(), "Ada", "ada@example.com", "google-ada", "")
	require.NoError(t, err)

	gh := &fakeGitHubClient{err: errors.New("github unreachable")}
	handler := newBugTestRouter(t, store, gh, func(context.Context) (db.User, bool) {
		return user, true
	})

	rawPNG := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}
	form := url.Values{
		"description": {"Live updates stalled"},
		"page_url":    {"https://parlay.club/slips/1"},
		"screenshot":  {"data:image/png;base64," + base64.StdEncoding.EncodeToString(rawPNG)},
	}

	req := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"/bug-reports",
		strings.NewReader(form.Encode()),
	)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Hx-Request", "true")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Bug report submitted.")
	assert.Contains(t, rec.Body.String(), "Saved locally for review.")
	assert.Equal(t, 1, gh.calls)
}

func openBugTestStore(t *testing.T) *db.Store {
	t.Helper()

	database, err := db.Open(filepath.Join(t.TempDir(), "parlay.db"))
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, database.Close()) })
	require.NoError(t, db.Migrate(t.Context(), database))
	return db.NewStore(database)
}

func newBugTestRouter(t *testing.T, store *db.Store, gh bugs.GitHubClient, userFrom webui.UserFrom) http.Handler {
	t.Helper()

	renderer, err := webui.NewRenderer(assets.Templates, false)
	require.NoError(t, err)

	svc := bugs.NewService(bugs.Deps{
		Store:   store,
		GitHub:  gh,
		Repo:    "kalverra/parlay-club",
		BaseURL: "https://parlay.club",
		Logger:  zerolog.Nop(),
	})

	return webui.NewWithDeps(webui.Dependencies{
		Logger:   zerolog.Nop(),
		Renderer: renderer,
		UserFrom: userFrom,
		Store:    store,
		Bugs:     svc,
	})
}
