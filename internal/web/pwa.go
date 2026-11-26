package web

import (
	"encoding/json"
	"net/http"

	assets "github.com/kalverra/parlay-club/web"
)

// pwaBackgroundColor is the paper-turf theme's page color, which the splash
// screen shows while the installed app boots.
const pwaBackgroundColor = "#14261c"

type manifestIcon struct {
	Src     string `json:"src"`
	Sizes   string `json:"sizes"`
	Type    string `json:"type"`
	Purpose string `json:"purpose,omitempty"`
}

type webAppManifest struct {
	Name            string         `json:"name"`
	ShortName       string         `json:"short_name"`
	Description     string         `json:"description"`
	StartURL        string         `json:"start_url"`
	Scope           string         `json:"scope"`
	Display         string         `json:"display"`
	BackgroundColor string         `json:"background_color"`
	ThemeColor      string         `json:"theme_color"`
	Icons           []manifestIcon `json:"icons"`
}

// ManifestHandler serves the web app manifest. It sits outside auth, so an
// installed app can read it before sign-in. Icon URLs carry the static content
// hash, so the one-year /static cache stays safe when the art changes.
func ManifestHandler() http.Handler {
	body, err := json.Marshal(webAppManifest{
		Name:            "Sunday Parlay Club",
		ShortName:       "Parlay Club",
		Description:     "Build parlays with friends.",
		StartURL:        "/",
		Scope:           "/",
		Display:         "standalone",
		BackgroundColor: pwaBackgroundColor,
		ThemeColor:      defaultTheme.ThemeColor,
		Icons: []manifestIcon{
			{Src: AssetURL("icon-192.png"), Sizes: "192x192", Type: "image/png", Purpose: "any"},
			{Src: AssetURL("icon-512.png"), Sizes: "512x512", Type: "image/png", Purpose: "any"},
			{Src: AssetURL("icon-maskable-512.png"), Sizes: "512x512", Type: "image/png", Purpose: "maskable"},
		},
	})
	if err != nil {
		panic("web: build manifest: " + err.Error())
	}

	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/manifest+json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(body)
	})
}

// ServiceWorkerHandler serves the worker that carries push notifications. It
// sends no-cache so a deploy's new worker is picked up on the next load.
//
// The worker has no fetch handler, so it never stands between a page and the
// server.
func ServiceWorkerHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		_, _ = w.Write(assets.ServiceWorker)
	})
}
