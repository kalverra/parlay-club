package web_test

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	webui "github.com/kalverra/parlay-club/internal/web"
	assets "github.com/kalverra/parlay-club/web"
)

func TestShellPages(t *testing.T) {
	t.Parallel()

	handler := newShell(t)

	tests := []struct {
		path    string
		heading string
	}{
		{"/", "Slip"},
		{"/games", "Games"},
		{"/research", "Research"},
		{"/record", "Record Book"},
	}

	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			t.Parallel()

			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodGet, test.path, nil))

			assert.Equal(t, http.StatusOK, recorder.Code)
			assert.Equal(t, "text/html; charset=utf-8", recorder.Header().Get("Content-Type"))
			assert.Contains(t, recorder.Body.String(), `<h1 class="text-xl font-semibold">`+test.heading+"</h1>")

			current := regexp.MustCompile(
				`<a\s+href="` + regexp.QuoteMeta(test.path) + `"[^>]*aria-current="page"`,
			)
			assert.Regexp(t, current, recorder.Body.String())
		})
	}
}

func TestShellServesStatic(t *testing.T) {
	t.Parallel()

	handler := newShell(t)

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		webui.AssetURL("app.css"),
		nil,
	))

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "public, max-age=31536000, immutable", recorder.Header().Get("Cache-Control"))
	assert.NotEmpty(t, recorder.Body.String())
}

func TestShellUnknownPath(t *testing.T) {
	t.Parallel()

	handler := newShell(t)

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/nope", nil))

	assert.Equal(t, http.StatusNotFound, recorder.Code)
}

// newShell builds the web routes the way the server does.
func newShell(t *testing.T) http.Handler {
	t.Helper()

	renderer, err := webui.NewRenderer(assets.Templates, false)
	require.NoError(t, err)

	return webui.New(zerolog.Nop(), renderer)
}
