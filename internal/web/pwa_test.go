package web_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	webui "github.com/kalverra/parlay-club/internal/web"
)

func TestManifestHandlerServesAnInstallableManifest(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	webui.ManifestHandler().ServeHTTP(recorder, httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		"/manifest.webmanifest",
		nil,
	))

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "application/manifest+json; charset=utf-8", recorder.Header().Get("Content-Type"))
	assert.Equal(t, "no-cache", recorder.Header().Get("Cache-Control"))

	var manifest struct {
		Name            string `json:"name"`
		ShortName       string `json:"short_name"`
		StartURL        string `json:"start_url"`
		Scope           string `json:"scope"`
		Display         string `json:"display"`
		BackgroundColor string `json:"background_color"`
		ThemeColor      string `json:"theme_color"`
		Icons           []struct {
			Src     string `json:"src"`
			Sizes   string `json:"sizes"`
			Type    string `json:"type"`
			Purpose string `json:"purpose"`
		} `json:"icons"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &manifest))

	assert.Equal(t, "Sunday Parlay Club", manifest.Name)
	assert.Equal(t, "Parlay Club", manifest.ShortName)
	assert.Equal(t, "/", manifest.StartURL)
	assert.Equal(t, "/", manifest.Scope)
	assert.Equal(t, "standalone", manifest.Display)
	assert.NotEmpty(t, manifest.BackgroundColor)
	assert.Regexp(t, `^#[0-9a-f]{6}$`, manifest.ThemeColor)

	hashedIcon := regexp.MustCompile(`^/static/icon-[a-z0-9-]+\.png\?v=[0-9a-f]{10}$`)
	sizes := make(map[string]bool)
	hasMaskable := false
	for _, icon := range manifest.Icons {
		assert.Regexp(t, hashedIcon, icon.Src, "icon URLs carry the static content hash")
		assert.Equal(t, "image/png", icon.Type)
		sizes[icon.Sizes] = true
		if icon.Purpose == "maskable" {
			hasMaskable = true
			assert.Equal(t, "512x512", icon.Sizes)
		}
	}

	assert.True(t, sizes["192x192"], "the manifest lists a 192 icon")
	assert.True(t, sizes["512x512"], "the manifest lists a 512 icon")
	assert.True(t, hasMaskable, "the manifest lists a maskable 512 icon")
}

func TestServiceWorkerHandlerHasOnlyInstallAndPushListeners(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	webui.ServiceWorkerHandler().ServeHTTP(recorder, httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		"/sw.js",
		nil,
	))

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "text/javascript; charset=utf-8", recorder.Header().Get("Content-Type"))
	assert.Equal(t, "no-cache", recorder.Header().Get("Cache-Control"))
	assert.Equal(t, "nosniff", recorder.Header().Get("X-Content-Type-Options"))

	script := recorder.Body.String()
	for _, listener := range []string{`addEventListener("install"`, `addEventListener("push"`, `addEventListener("notificationclick"`} {
		assert.Contains(t, script, listener)
	}
	assert.NotContains(t, script, `addEventListener("fetch"`, "the worker must never intercept requests")
	assert.NotContains(t, script, "caches", "the worker keeps no offline cache")
}
