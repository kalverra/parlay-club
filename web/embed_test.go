package web_test

import (
	"io/fs"
	"path"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/web"
)

func TestStaticAssetsEmbedded(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
	}{
		{name: "generated CSS", path: "static/app.css"},
		{name: "app script", path: "static/app.js"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			content, err := fs.ReadFile(web.Static, test.path)
			require.NoError(t, err)
			assert.NotEmpty(t, content)
		})
	}
}

// TestVendoredHTMX keeps the vendored library named with its version, which is
// also how the templates reference it.
func TestVendoredHTMX(t *testing.T) {
	t.Parallel()

	name := regexp.MustCompile(`^htmx-2\.\d+\.\d+\.min\.js$`)

	matches, err := fs.Glob(web.Static, "static/htmx-*.min.js")
	require.NoError(t, err)
	require.Len(t, matches, 1)

	assert.Regexp(t, name, path.Base(matches[0]))

	content, err := fs.ReadFile(web.Static, matches[0])
	require.NoError(t, err)
	assert.NotEmpty(t, content)
}
