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

	page := webui.Page{Title: "Slip", Tab: webui.TabSlip}

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

	out, err := renderer.Render("slip", webui.Page{Title: "Slip", Tab: webui.TabSlip})
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

// writeTemplates writes a minimal template set and returns its directory.
func writeTemplates(t *testing.T, content string) string {
	t.Helper()

	dir := t.TempDir()

	writeTemplate(t, dir, "layout.html", `{{define "layout"}}<main>{{template "content" .}}</main>{{end}}`)
	writeTemplate(t, dir, "avatar.html", `{{define "avatar"}}{{end}}`)
	for _, page := range []string{"slip", "games", "research", "record", "login", "dev-login", "join"} {
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
