package web_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	webui "github.com/kalverra/parlay-club/internal/web"
	assets "github.com/kalverra/parlay-club/web"
)

func TestRendererReloadsFromDisk(t *testing.T) {
	t.Parallel()

	dir := writeTemplates(t, "first")

	renderer, err := webui.NewRenderer(os.DirFS(dir), true)
	require.NoError(t, err)

	page := webui.Page{Title: "Slip", Tab: webui.TabWeek}

	first, err := renderer.Render("slip", page)
	require.NoError(t, err)
	assert.Contains(t, string(first), "first")

	writeTemplate(t, dir, "slip.html", `{{define "content"}}second{{end}}`)

	second, err := renderer.Render("slip", page)
	require.NoError(t, err)
	assert.Contains(t, string(second), "second")
}

func TestRendererCachesWhenNotReloading(t *testing.T) {
	t.Parallel()

	dir := writeTemplates(t, "first")

	renderer, err := webui.NewRenderer(os.DirFS(dir), false)
	require.NoError(t, err)

	writeTemplate(t, dir, "slip.html", `{{define "content"}}second{{end}}`)

	out, err := renderer.Render("slip", webui.Page{Title: "Slip", Tab: webui.TabWeek})
	require.NoError(t, err)
	assert.Contains(t, string(out), "first")
	assert.NotContains(t, string(out), "second")
}

func TestRendererRejectsBrokenTemplates(t *testing.T) {
	t.Parallel()

	dir := writeTemplates(t, "first")
	writeTemplate(t, dir, "layout.html", `{{define "layout"}}{{template "content" .}}`)

	_, err := webui.NewRenderer(os.DirFS(dir), false)
	require.Error(t, err)
}

func TestRendererRejectsUnknownPage(t *testing.T) {
	t.Parallel()

	renderer, err := webui.NewRenderer(assets.Templates, false)
	require.NoError(t, err)

	_, err = renderer.Render("nope", webui.Page{})
	require.Error(t, err)
}

func TestMediaHelpersRenderAndFallback(t *testing.T) {
	t.Parallel()

	dir := writeTemplates(t, "")
	mediaBytes, err := assets.Templates.ReadFile("templates/media.html")
	require.NoError(t, err)
	writeTemplate(t, dir, "media.html", string(mediaBytes))
	writeTemplate(t, dir, "partials.html", `
		{{define "pick-slate"}}
			<div id="logo-with-url">{{template "team-logo" (dict "URL" "https://a.espncdn.com/buf.png")}}</div>
			<div id="logo-with-fallback">{{template "team-logo" (dict "URL" "" "Fallback" "BUF" "Size" "h-6 w-6")}}</div>
			<div id="logo-empty">{{template "team-logo" (dict "URL" "")}}</div>
			<div id="portrait-with-url">{{template "player-portrait" (dict "URL" "https://a.espncdn.com/ja.png" "Initials" "JA")}}</div>
			<div id="portrait-empty">{{template "player-portrait" (dict "URL" "" "Initials" "JA")}}</div>
			<div id="matchup">{{template "matchup-logos" (dict "AwayURL" "https://a.espncdn.com/buf.png" "HomeURL" "https://a.espncdn.com/kc.png")}}</div>
		{{end}}
	`)
	writeTemplate(t, dir, "slip.html", `{{define "content"}}{{template "pick-slate" .}}{{end}}`)

	renderer, err := webui.NewRenderer(os.DirFS(dir), false)
	require.NoError(t, err)

	full, err := renderer.Render("slip", webui.Page{Title: "Slip", Tab: webui.TabWeek})
	require.NoError(t, err)

	partial, err := renderer.RenderPartial("pick-slate", struct{}{})
	require.NoError(t, err)

	for _, html := range []string{string(full), string(partial)} {
		assert.Contains(t, html, `src="https://a.espncdn.com/buf.png"`)
		assert.Contains(t, html, `data-fallback-img`)
		assert.Contains(t, html, `referrerpolicy="no-referrer"`)
		assert.Contains(t, html, `loading="lazy"`)
		assert.Contains(t, html, `data-fallback-badge`)
		assert.Contains(t, html, `>BUF</span>`)
		assert.Contains(t, html, `>JA</span>`)
		assert.Contains(t, html, `src="https://a.espncdn.com/kc.png"`)
		assert.Contains(t, html, `<div id="logo-empty"></div>`)
	}
}

// writeTemplates writes a minimal template set and returns its directory.
func writeTemplates(t *testing.T, content string) string {
	t.Helper()

	dir := t.TempDir()

	writeTemplate(t, dir, "layout.html", `{{define "layout"}}<main>{{template "content" .}}</main>{{end}}`)
	writeTemplate(t, dir, "avatar.html", `{{define "avatar"}}{{end}}`)
	writeTemplate(
		t,
		dir,
		"media.html",
		`{{define "team-logo"}}{{end}}{{define "player-portrait"}}{{end}}{{define "matchup-logos"}}{{end}}`,
	)
	writeTemplate(t, dir, "partials.html", "")
	for _, page := range []string{"home", "week", "slip", "pick", "research", "record", "login", "dev-login", "join", "groups", "profile", "sandbox"} {
		writeTemplate(t, dir, page+".html", `{{define "content"}}`+content+`{{end}}`)
	}

	return dir
}

func writeTemplate(t *testing.T, dir, name, content string) {
	t.Helper()

	file := filepath.Join(dir, "templates", name)
	require.NoError(t, os.MkdirAll(filepath.Dir(file), 0o750))
	require.NoError(t, os.WriteFile(file, []byte(content), 0o600))
}
