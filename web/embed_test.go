package web_test

import (
	"io/fs"
	"os"
	"path"
	"regexp"
	"strings"
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
		{name: "html2canvas-pro script", path: "static/html2canvas-pro.min.js"},
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

func TestBuildGradeThemeTokensAndClasses(t *testing.T) {
	t.Parallel()

	inputRaw, err := os.ReadFile("tailwind/input.css")
	require.NoError(t, err)
	inputCSS := string(inputRaw)

	compiledRaw, err := fs.ReadFile(web.Static, "static/app.css")
	require.NoError(t, err)
	compiledCSS := string(compiledRaw)

	themes := []string{"paper-turf", "redzone", "almanac"}
	requiredTokens := []string{
		"--grade-clean",
		"--grade-clean-soft",
		"--grade-clean-ink",
		"--grade-solid",
		"--grade-solid-soft",
		"--grade-solid-ink",
	}

	for _, theme := range themes {
		t.Run("theme_"+theme, func(t *testing.T) {
			t.Parallel()

			blockPattern := regexp.MustCompile(`(?s)\[data-theme="` + regexp.QuoteMeta(theme) + `"\]\s*\{([^}]*)\}`)
			match := blockPattern.FindStringSubmatch(inputCSS)
			require.Len(t, match, 2, "missing theme block for %s", theme)
			block := match[1]

			for _, token := range requiredTokens {
				valPattern := regexp.MustCompile(regexp.QuoteMeta(token) + `\s*:\s*([^;]+);`)
				valMatch := valPattern.FindStringSubmatch(block)
				require.Len(t, valMatch, 2, "missing %s in theme %s", token, theme)
				val := strings.TrimSpace(valMatch[1])
				assert.NotEmpty(t, val, "empty %s in theme %s", token, theme)
				assert.NotContains(t, val, "--positive", "%s in theme %s must not reference --positive", token, theme)
			}

			compiledBlockPattern := regexp.MustCompile(
				`\[data-theme=` + regexp.QuoteMeta(theme) + `\][^{]*\{[^}]*--grade-clean:[^}]*--grade-solid:`,
			)
			assert.Regexp(
				t,
				compiledBlockPattern,
				compiledCSS,
				"compiled app.css missing grade tokens for %s",
				theme,
			)
		})
	}

	gradeRulePattern := regexp.MustCompile(`(?s)(\.grade-[a-z]+(?:\s*,\s*\.grade-[a-z]+)*)\s*\{([^}]*)\}`)
	for _, source := range []struct {
		name string
		css  string
	}{
		{name: "input.css", css: inputCSS},
		{name: "app.css", css: compiledCSS},
	} {
		matches := gradeRulePattern.FindAllStringSubmatch(source.css, -1)
		require.NotEmpty(t, matches, "expected .grade-* rules in %s", source.name)

		seen := make(map[string]string)
		for _, m := range matches {
			selector, body := m[1], m[2]
			assert.NotContains(t, body, "--positive", "%s rule %q must not reference --positive", source.name, selector)
			seen[selector] = body
		}

		for _, check := range []struct {
			selector string
			token    string
		}{
			{".grade-a", "--grade-clean"},
			{".grade-a", "--grade-clean-soft"},
			{".grade-a", "--grade-clean-ink"},
			{".grade-b", "--grade-solid"},
			{".grade-b", "--grade-solid-soft"},
			{".grade-b", "--grade-solid-ink"},
		} {
			assert.Contains(
				t,
				seen[check.selector],
				check.token,
				"%s %s should use %s",
				source.name,
				check.selector,
				check.token,
			)
		}
	}
}
