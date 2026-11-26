package web_test

import (
	"fmt"
	"html"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/db"
)

// helpBubbleTag matches the rendered ? anchor. The href comes first, so the
// helper reads it back out of the whole tag.
var (
	helpBubbleTag = regexp.MustCompile(`<a\s+[^>]*data-help-bubble[^>]*>`)
	hrefAttr      = regexp.MustCompile(`href="([^"]+)"`)
)

// helpBubbleHrefs returns every ? bubble target on a rendered page.
func helpBubbleHrefs(body string) []string {
	var hrefs []string
	for _, tag := range helpBubbleTag.FindAllString(body, -1) {
		match := hrefAttr.FindStringSubmatch(tag)
		if len(match) == 2 {
			hrefs = append(hrefs, html.UnescapeString(match[1]))
		}
	}
	return hrefs
}

// TestHelpBubblesTargetExistingDocsAnchors walks the screens that carry a ?
// bubble and checks every link lands on an anchor the docs pages render.
func TestHelpBubblesTargetExistingDocsAnchors(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 3)
	game := addGame(t, fixture, gameKickoff())
	addGameOdds(t, fixture.store, game.ID)

	now := time.Now().UTC()
	changed, err := fixture.store.UpsertNfeloGames(t.Context(), []db.NfeloGame{{
		GameID:      game.ID,
		HomeSpread:  -10,
		HomeWinProb: 0.76,
		HomeQB:      "Josh Allen",
		AwayQB:      "Tua Tagovailoa",
		HomeQBAdj:   5,
		AwayQBAdj:   -5,
		UpdatedAt:   now,
	}})
	require.NoError(t, err)
	require.True(t, changed)
	require.NoError(t, fixture.store.RecordSync(t.Context(), "nfelo:projections", now, nil))

	addStoredLeg(t, fixture, db.Leg{
		UserID:     fixture.user.ID,
		GameID:     new(game.ID),
		MarketType: "SPREAD",
		Selection:  "BUF -7.5",
		Team:       new("BUF"),
		Side:       new("TEAM"),
		Line:       new(-7.5),
		Odds:       -110,
	})
	addStoredLeg(t, fixture, db.Leg{
		UserID:     fixture.user.ID,
		GameID:     new(game.ID),
		MarketType: "PASS_YDS",
		Selection:  "Josh Allen Over 245.5 Pass Yds",
		Team:       new("BUF"),
		PlayerName: new("Josh Allen"),
		Side:       new("OVER"),
		Line:       new(245.5),
		Odds:       -110,
	})
	addStoredLeg(t, fixture, db.Leg{
		UserID:     fixture.user.ID,
		GameID:     new(game.ID),
		MarketType: "TOTAL",
		Selection:  "BUF@MIA Over 47.5",
		Side:       new("OVER"),
		Line:       new(47.5),
		Odds:       -110,
	})

	fetch := func(path string, htmx bool) string {
		t.Helper()
		recorder := httptest.NewRecorder()
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil)
		if htmx {
			request.Header.Set("Hx-Request", "true")
		}
		fixture.handler.ServeHTTP(recorder, request)
		require.Equalf(t, http.StatusOK, recorder.Code, "GET %s", path)
		return recorder.Body.String()
	}

	screens := map[string]string{
		"slip panel":  fetch(fmt.Sprintf("/slips/%d/panel", fixture.slip.ID), false),
		"pick tray":   fetch(fmt.Sprintf("/slips/%d/tray", fixture.slip.ID), false),
		"slip payout": fetch(fmt.Sprintf("/slips/%d/payout", fixture.slip.ID), false),
		"pick detail": fetch(fmt.Sprintf("/slips/%d/pick/detail?game=%s", fixture.slip.ID, game.ID), true),
	}

	// A live game adds the live hit chance bubble to the slip panel.
	game.Status = db.GameLive
	game.HomeScore = new(28)
	game.AwayScore = new(24)
	game.Period = new(4)
	game.Clock = new("2:10")
	_, err = fixture.store.UpsertGame(t.Context(), game)
	require.NoError(t, err)
	screens["live slip panel"] = fetch(fmt.Sprintf("/slips/%d/panel", fixture.slip.ID), false)

	docs := map[string]string{}
	docsIDs := map[string]map[string]struct{}{}
	for _, path := range []string{"/docs", "/docs/glossary", "/docs/grading"} {
		docs[path] = fetch(path, false)
		docsIDs[path] = renderedIDs(docs[path])
	}

	var hrefs []string
	for name, body := range screens {
		got := helpBubbleHrefs(body)
		assert.NotEmptyf(t, got, "%s should render a help bubble", name)
		hrefs = append(hrefs, got...)
	}
	require.NotEmpty(t, hrefs)

	fragments := make(map[string]struct{}, len(hrefs))
	for _, href := range hrefs {
		target, err := url.Parse(href)
		require.NoErrorf(t, err, "parse help bubble %q", href)

		_, inDocs := docs[target.Path]
		if !assert.Truef(t, inDocs, "help bubble %q points outside the docs pages", href) {
			continue
		}
		require.NotEmptyf(t, target.Fragment, "help bubble %q has no anchor", href)
		_, ok := docsIDs[target.Path][target.Fragment]
		assert.Truef(
			t,
			ok,
			"help bubble %q points at a missing anchor in %s",
			href,
			target.Path,
		)
		fragments[target.Fragment] = struct{}{}
	}

	for _, fragment := range []string{
		"leg-grades",
		"slip-grade",
		"term-nfelo",
		"term-implied-prob",
		"term-reference-odds",
		"sys-live",
		"suggestions",
	} {
		_, ok := fragments[fragment]
		assert.Truef(t, ok, "expected a help bubble for #%s", fragment)
	}

	hasRule := false
	for fragment := range fragments {
		if strings.HasPrefix(fragment, "rule-") {
			hasRule = true
		}
	}
	assert.True(t, hasRule, "expected a help bubble for a warning rule anchor")
}
