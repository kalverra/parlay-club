package web

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGlossaryEntriesAreUniqueAndComplete keeps every #term-<slug> anchor
// addressable and every card readable.
func TestGlossaryEntriesAreUniqueAndComplete(t *testing.T) {
	t.Parallel()

	seen := make(map[string]struct{}, len(glossaryEntries))
	for _, entry := range glossaryEntries {
		require.NotEmpty(t, entry.Slug, "%q has no slug", entry.Term)
		assert.NotContains(t, entry.Slug, " ", "%q slug must be anchor-safe", entry.Term)
		assert.Equal(t, strings.ToLower(entry.Slug), entry.Slug, "%q slug must be lowercase", entry.Term)

		_, duplicate := seen[entry.Slug]
		assert.False(t, duplicate, "duplicate glossary slug %q", entry.Slug)
		seen[entry.Slug] = struct{}{}

		assert.NotEmpty(t, entry.Term, "entry %q has no term", entry.Slug)
		assert.NotEmpty(t, entry.Category, "entry %q has no category", entry.Slug)
		assert.NotEmpty(t, entry.Summary, "entry %q has no summary", entry.Slug)
		assert.NotEmpty(t, entry.Example, "entry %q has no example", entry.Slug)
	}

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
		assert.Contains(t, seen, slug, "the docs deep-link to #term-%s", slug)
	}
}

// TestDocsHubSectionsAreUniqueAndComplete keeps every #sys-<id> anchor stable.
func TestDocsHubSectionsAreUniqueAndComplete(t *testing.T) {
	t.Parallel()

	hub := docsHubView()
	seen := make(map[string]struct{}, len(hub.Sections))
	for _, section := range hub.Sections {
		require.NotEmpty(t, section.ID)
		assert.NotContains(t, section.ID, " ", "section id %q must be anchor-safe", section.ID)

		_, duplicate := seen[section.ID]
		assert.False(t, duplicate, "duplicate system section id %q", section.ID)
		seen[section.ID] = struct{}{}

		assert.NotEmpty(t, section.Title, "section %q has no title", section.ID)
		assert.NotEmpty(t, section.Summary, "section %q has no summary", section.ID)
		assert.NotEmpty(t, section.Points, "section %q has no points", section.ID)
	}

	for _, id := range []string{
		"grading",
		"odds",
		"badges",
		"live",
		"autograde",
		"sources",
		"export",
	} {
		assert.Contains(t, seen, id, "the docs deep-link to #sys-%s", id)
	}

	assert.NotEmpty(t, hub.QuickJumps)
	assert.NotEmpty(t, hub.FAQ)
	for _, item := range hub.FAQ {
		assert.NotEmpty(t, item.Question)
		assert.NotEmpty(t, item.Answer)
	}
}

// TestDocsHubAnchorsResolveInData walks the hub's own links and checks each
// bare fragment against the data that draws it: #sys-<id> to a section and
// #term-<slug> to a glossary entry. The rendered-page walk in
// docs_handlers_test.go covers cross-page grading and glossary targets.
func TestDocsHubAnchorsResolveInData(t *testing.T) {
	t.Parallel()

	hub := docsHubView()
	sections := make(map[string]struct{}, len(hub.Sections))
	for _, section := range hub.Sections {
		sections[section.ID] = struct{}{}
	}
	terms := make(map[string]struct{}, len(glossaryEntries))
	for _, entry := range glossaryEntries {
		terms[entry.Slug] = struct{}{}
	}

	links := make([]string, 0, len(hub.QuickJumps)*2+len(hub.Sections)*2+len(hub.FAQ))
	for _, jump := range hub.QuickJumps {
		links = append(links, jump.Href)
	}
	for _, section := range hub.Sections {
		for _, link := range section.Links {
			links = append(links, link.Href)
		}
	}
	for _, item := range hub.FAQ {
		if item.LinkHref != "" {
			links = append(links, item.LinkHref)
		}
	}

	for _, href := range links {
		fragment, ok := strings.CutPrefix(href, "#")
		if !ok {
			continue
		}
		switch {
		case strings.HasPrefix(fragment, "sys-"):
			_, ok := sections[strings.TrimPrefix(fragment, "sys-")]
			assert.Truef(t, ok, "hub link %q names no system section", href)
		case strings.HasPrefix(fragment, "term-"):
			_, ok := terms[strings.TrimPrefix(fragment, "term-")]
			assert.Truef(t, ok, "hub link %q names no glossary term", href)
		default:
			assert.Failf(t, "unknown hub anchor", "hub link %q uses #%s", href, fragment)
		}
	}
}

func TestFilterGlossary(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		query     string
		wantSlugs []string
	}{
		{name: "empty query keeps every entry", query: ""},
		{name: "whitespace keeps every entry", query: "   "},
		{name: "term match", query: "epa", wantSlugs: []string{"epa"}},
		{name: "match ignores case", query: "EPA", wantSlugs: []string{"epa"}},
		{name: "trims the query", query: "  de-vig  ", wantSlugs: []string{"de-vig"}},
		{
			name:      "matches a category",
			query:     "usage & efficiency",
			wantSlugs: []string{"epa", "wopr", "target-share", "snap-share"},
		},
		{name: "matches a summary word", query: "favorite", wantSlugs: []string{"chalk"}},
		{name: "no match", query: "touchdown dance", wantSlugs: []string{}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := filterGlossary(glossaryEntries, test.query)
			if test.query == "" || strings.TrimSpace(test.query) == "" {
				assert.Len(t, got, len(glossaryEntries))
				return
			}

			slugs := make([]string, 0, len(got))
			for _, entry := range got {
				slugs = append(slugs, entry.Slug)
			}
			assert.Equal(t, test.wantSlugs, slugs)
		})
	}
}
