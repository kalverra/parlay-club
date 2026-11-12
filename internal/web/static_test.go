package web_test

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	webui "github.com/kalverra/parlay-club/internal/web"
	assets "github.com/kalverra/parlay-club/web"
)

func TestAssetURL(t *testing.T) {
	t.Parallel()

	url := webui.AssetURL("app.css")
	assert.Regexp(t, `^/static/app\.css\?v=[0-9a-f]{10}$`, url)
	assert.NotEqual(t, url, webui.AssetURL("app.js"))
	assert.Equal(t, "/static/missing.css", webui.AssetURL("missing.css"))
}

func TestStaticHandlerServesForAYear(t *testing.T) {
	t.Parallel()

	handler := webui.StaticHandler()

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/static/app.css", nil))

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "public, max-age=31536000, immutable", recorder.Header().Get("Cache-Control"))
	assert.Equal(t, "text/css; charset=utf-8", recorder.Header().Get("Content-Type"))
	assert.NotEmpty(t, recorder.Body.String())
}

func TestStaticHandlerIgnoresVersionQuery(t *testing.T) {
	t.Parallel()

	handler := webui.StaticHandler()

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		webui.AssetURL("app.js"),
		nil,
	))

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "text/javascript; charset=utf-8", recorder.Header().Get("Content-Type"))
	assert.NotEmpty(t, recorder.Body.String())
}

func TestStaticHandlerMissingFile(t *testing.T) {
	t.Parallel()

	handler := webui.StaticHandler()

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/static/missing.css", nil))

	assert.Equal(t, http.StatusNotFound, recorder.Code)
}

func TestAppJSKeepsThePickSlateScroll(t *testing.T) {
	t.Parallel()

	source, err := fs.ReadFile(assets.Static, "static/app.js")
	require.NoError(t, err)
	script := string(source)

	assert.Contains(t, script, `querySelector("[data-keep-scroll]")`)
	assert.Contains(t, script, "htmx:beforeSwap", "the handler saves the slate's place")
	assert.Contains(t, script, "htmx:afterSwap", "the handler restores it after the swap")
}

func TestAppJSOpensAndClosesThePickDock(t *testing.T) {
	t.Parallel()

	source, err := fs.ReadFile(assets.Static, "static/app.js")
	require.NoError(t, err)
	script := string(source)

	assert.Contains(t, script, `"pick-tray-body"`, "the sheet opens once its tray lands")
	assert.Contains(t, script, `"pick-tray-sheet"`, "the sheet shell is wired up")
	assert.Contains(t, script, "data-close-dock", "the sheet has a close control")
}

func TestAppJSHandlesBugReportCaptureAndSubmission(t *testing.T) {
	t.Parallel()

	source, err := fs.ReadFile(assets.Static, "static/app.js")
	require.NoError(t, err)
	script := string(source)

	for _, want := range []string{
		`"bug-report-trigger"`,
		`"bug-report-dialog"`,
		`"bug-report-form"`,
		`"bug-report-error"`,
		`"bug-preview-slot"`,
		`"bug-preview"`,
		`"bug-input-screenshot"`,
		`"bug-input-page-url"`,
		`"bug-input-screen-size"`,
		`"bug-input-user-agent"`,
		"html2canvas",
		`"image/jpeg"`,
		"data-bug-dialog-close",
		"htmx:beforeSwap",
	} {
		assert.Contains(t, script, want)
	}
}

func TestStaticHandlerStaysInsideStaticDir(t *testing.T) {
	t.Parallel()

	handler := webui.StaticHandler()

	for _, path := range []string{"/static/../go.mod", "/static/..%2Fgo.mod", "/static/../../etc/passwd"} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil))

		assert.NotEqual(t, http.StatusOK, recorder.Code, "path %s escaped the static dir", path)
	}
}
