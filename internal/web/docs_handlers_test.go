package web_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	renderedID     = regexp.MustCompile(`\sid="([^"]+)"`)
	renderedAnchor = regexp.MustCompile(`href="#([^"]+)"`)
)

// assertUniqueIDs fails when the page draws the same id twice, which would make
// every deep link ambiguous.
func assertUniqueIDs(t *testing.T, body string) {
	t.Helper()

	counts := make(map[string]int)
	for _, match := range renderedID.FindAllStringSubmatch(body, -1) {
		counts[match[1]]++
	}
	for id, count := range counts {
		assert.Equalf(t, 1, count, "id %q appears %d times", id, count)
	}
}

// assertAnchorsResolve checks every in-page #link against the page's ids.
func assertAnchorsResolve(t *testing.T, body string) {
	t.Helper()

	ids := make(map[string]struct{})
	for _, match := range renderedID.FindAllStringSubmatch(body, -1) {
		ids[match[1]] = struct{}{}
	}
	for _, match := range renderedAnchor.FindAllStringSubmatch(body, -1) {
		_, ok := ids[match[1]]
		assert.Truef(t, ok, "anchor #%s has no matching id", match[1])
	}
}

func TestDocsHubRendersSystemsFAQAndCurrentTab(t *testing.T) {
	t.Parallel()

	handler := newShell(t)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/docs", nil)
	handler.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "text/html; charset=utf-8", recorder.Header().Get("Content-Type"))
	body := recorder.Body.String()

	assert.Contains(t, body, "<title>Docs · Sunday Parlay Club</title>")
	assert.Regexp(t, `<a\s+href="/docs"\s+class="flex flex-col[^>]*aria-current="page"`, body)

	for _, id := range []string{
		"sys-grading",
		"sys-odds",
		"sys-badges",
		"sys-live",
		"sys-autograde",
		"sys-sources",
		"sys-export",
	} {
		assert.Containsf(t, body, `id="`+id+`"`, "the hub keeps the %s anchor", id)
	}

	assert.Contains(t, body, "How systems work")
	assert.Contains(t, body, "Common questions")
	assert.Contains(t, body, "Why is every pick still allowed?")
	assert.Contains(t, body, `href="/docs/glossary"`)
	assert.Contains(t, body, `href="/docs/glossary#term-implied-prob"`)
	assert.Contains(t, body, `aria-current="page"`+">"+"Overview</a")

	assertUniqueIDs(t, body)
	assertAnchorsResolve(t, body)
}

func TestDocsGlossaryRendersEveryAnchorAndFilters(t *testing.T) {
	t.Parallel()

	handler := newShell(t)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/docs/glossary", nil)
	handler.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	body := recorder.Body.String()

	assert.Contains(t, body, "<title>Glossary · Sunday Parlay Club</title>")
	assert.Regexp(t, `<a\s+href="/docs"\s+class="flex flex-col[^>]*aria-current="page"`, body)
	assert.Contains(t, body, `aria-current="page"`+">"+"Glossary</a")
	assert.Contains(t, body, "15 terms.")

	for _, slug := range []string{
		"implied-prob",
		"de-vig",
		"price-edge",
		"epa",
		"wopr",
		"target-share",
		"snap-share",
		"nfelo",
		"key-numbers",
		"line-drift",
		"chalk",
		"correlation",
		"push-void",
		"reference-odds",
		"slate-scope",
	} {
		assert.Containsf(t, body, `id="term-`+slug+`"`, "glossary keeps the #term-%s anchor", slug)
	}

	assertUniqueIDs(t, body)
	assertAnchorsResolve(t, body)

	filtered := httptest.NewRecorder()
	filterRequest := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/docs/glossary?q=EPA", nil)
	handler.ServeHTTP(filtered, filterRequest)

	require.Equal(t, http.StatusOK, filtered.Code)
	filteredBody := filtered.Body.String()
	assert.Contains(t, filteredBody, `id="term-epa"`)
	assert.NotContains(t, filteredBody, `id="term-nfelo"`)
	assert.Contains(t, filteredBody, "1 of 15 terms match")
	assert.Contains(t, filteredBody, `value="EPA"`)
	assert.Contains(t, filteredBody, `href="/docs/glossary"`)
	assertUniqueIDs(t, filteredBody)

	empty := httptest.NewRecorder()
	emptyRequest := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/docs/glossary?q=touchdown+dance", nil)
	handler.ServeHTTP(empty, emptyRequest)

	require.Equal(t, http.StatusOK, empty.Code)
	emptyBody := empty.Body.String()
	assert.Contains(t, emptyBody, "0 of 15 terms match")
	assert.Contains(t, emptyBody, `data-glossary-empty`)
	assert.NotContains(t, emptyBody, `data-glossary-list`)
}

func TestDocsPagesForwardSafeBackLink(t *testing.T) {
	t.Parallel()

	handler := newShell(t)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(
		t.Context(), http.MethodGet, "/docs/glossary?from=/slips/42", nil)
	handler.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	body := recorder.Body.String()
	assert.Contains(t, body, `href="/slips/42"`)
	assert.Contains(t, body, `href="/docs?from=%2Fslips%2F42"`)
	assert.Contains(t, body, `href="/docs/glossary?from=%2Fslips%2F42"`)
	assert.Contains(t, body, `href="/analyzer/guide?from=%2Fslips%2F42"`)
	assert.Contains(t, body, `<input type="hidden" name="from" value="/slips/42">`)

	for _, raw := range []string{"https://evil.example/phish", "//evil.example/phish", `/\evil.example`} {
		t.Run(raw, func(t *testing.T) {
			t.Parallel()

			unsafe := httptest.NewRecorder()
			unsafeRequest := httptest.NewRequestWithContext(
				t.Context(), http.MethodGet, "/docs?from="+raw, nil)
			handler.ServeHTTP(unsafe, unsafeRequest)

			require.Equal(t, http.StatusOK, unsafe.Code)
			unsafeBody := unsafe.Body.String()
			assert.NotContains(t, unsafeBody, "evil.example")
			assert.NotContains(t, unsafeBody, "&larr; Back")
			assertUniqueIDs(t, unsafeBody)
		})
	}
}

func TestDocsGlossaryQueryIsEscaped(t *testing.T) {
	t.Parallel()

	handler := newShell(t)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		"/docs/glossary?q="+url.QueryEscape(`"><script>alert(1)</script>`),
		nil,
	)
	handler.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	body := recorder.Body.String()
	assert.NotContains(t, body, "<script>alert(1)</script>")
	assert.Contains(t, body, "&lt;script&gt;")
	assert.Contains(t, body, "0 of 15 terms match")
}
