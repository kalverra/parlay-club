package web_test

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/db"
	webui "github.com/kalverra/parlay-club/internal/web"
)

func TestScreenshotHandlerServesJPEG(t *testing.T) {
	t.Parallel()

	store := openScreenshotTestStore(t)
	user, err := store.CreateUserWithIdentity(t.Context(), "Ada", "ada@example.com", "google-ada", "")
	require.NoError(t, err)

	rawJPEG := []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 'J', 'F', 'I', 'F'}
	report, err := store.CreateBugReport(t.Context(), db.BugReport{
		UserID:      user.ID,
		PageURL:     "https://parlay.club/slips/1",
		Description: "Broken layout",
		Screenshot:  rawJPEG,
	})
	require.NoError(t, err)

	handler := newScreenshotTestRouter(t, store)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		"/screenshots/"+report.Token+".jpg",
		nil,
	)
	handler.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "image/jpeg", recorder.Header().Get("Content-Type"))
	assert.Equal(t, "public, max-age=31536000, immutable", recorder.Header().Get("Cache-Control"))
	assert.Equal(t, rawJPEG, recorder.Body.Bytes())
}

func TestScreenshotHandlerNotFoundCases(t *testing.T) {
	t.Parallel()

	store := openScreenshotTestStore(t)
	user, err := store.CreateUserWithIdentity(t.Context(), "Ada", "ada@example.com", "google-ada", "")
	require.NoError(t, err)

	emptyReport, err := store.CreateBugReport(t.Context(), db.BugReport{
		UserID:      user.ID,
		PageURL:     "https://parlay.club/slips/1",
		Description: "No screenshot attached",
	})
	require.NoError(t, err)

	handler := newScreenshotTestRouter(t, store)

	tests := []struct {
		name string
		path string
	}{
		{name: "unknown token", path: "/screenshots/0123456789abcdef0123456789abcdef.jpg"},
		{name: "report without image", path: "/screenshots/" + emptyReport.Token + ".jpg"},
		{name: "wrong extension", path: "/screenshots/" + emptyReport.Token + ".png"},
		{name: "empty token", path: "/screenshots/.jpg"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			recorder := httptest.NewRecorder()
			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, test.path, nil)
			handler.ServeHTTP(recorder, request)

			assert.Equal(t, http.StatusNotFound, recorder.Code)
		})
	}
}

func TestScreenshotHandlerNilStoreReturnsNotFound(t *testing.T) {
	t.Parallel()

	handler := webui.ScreenshotHandler(nil, zerolog.Nop())
	recorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		"/screenshots/0123456789abcdef0123456789abcdef.jpg",
		nil,
	)
	handler.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusNotFound, recorder.Code)
}

func openScreenshotTestStore(t *testing.T) *db.Store {
	t.Helper()

	database, err := db.Open(filepath.Join(t.TempDir(), "parlay.db"))
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, database.Close()) })
	require.NoError(t, db.Migrate(t.Context(), database))
	store := db.NewStore(database)
	t.Cleanup(func() { assert.NoError(t, store.Close()) })
	return store
}

func newScreenshotTestRouter(t *testing.T, store *db.Store) http.Handler {
	t.Helper()

	mux := http.NewServeMux()
	mux.Handle("GET /screenshots/{file}", webui.ScreenshotHandler(store, zerolog.Nop()))
	return mux
}
