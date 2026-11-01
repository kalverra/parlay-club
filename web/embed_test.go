package web_test

import (
	"io/fs"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/web"
)

func TestStaticIncludesGeneratedCSS(t *testing.T) {
	t.Parallel()

	css, err := fs.ReadFile(web.Static, "static/app.css")
	require.NoError(t, err)
	assert.NotEmpty(t, css)
}
