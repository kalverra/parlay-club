package web_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	webui "github.com/kalverra/parlay-club/internal/web"
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

func TestStaticHandlerStaysInsideStaticDir(t *testing.T) {
	t.Parallel()

	handler := webui.StaticHandler()

	for _, path := range []string{"/static/../go.mod", "/static/..%2Fgo.mod", "/static/../../etc/passwd"} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil))

		assert.NotEqual(t, http.StatusOK, recorder.Code, "path %s escaped the static dir", path)
	}
}
