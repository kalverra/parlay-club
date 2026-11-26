package web

import (
	"net/http"
	"net/url"
	"strings"
)

// DocsPage is the shell data for the Docs hub and the glossary. Exactly one of
// Hub or Glossary is set, so web/templates/docs.html can render either screen
// from one template file.
type DocsPage struct {
	Page
	BackURL  string
	Nav      []DocsNavView
	Hub      *DocsHubView
	Glossary *DocsGlossaryView
}

// DocsNavView is one entry in the docs section switcher. The switcher forwards
// the sanitized back link so a person can return to the screen they came from.
type DocsNavView struct {
	Label   string
	Href    string
	Current bool
}

// DocsHubView is the Docs overview: quick jumps, system explainers, and FAQ.
type DocsHubView struct {
	QuickJumps []DocsJumpView
	Sections   []DocsSectionView
	FAQ        []DocsFAQView
}

// DocsJumpView is one quick-jump card at the top of the hub.
type DocsJumpView struct {
	Label   string
	Href    string
	Summary string
}

// DocsSectionView is one "How systems work" section. Its ID is the stable
// anchor suffix: the section renders as id="sys-<ID>".
type DocsSectionView struct {
	ID      string
	Title   string
	Summary string
	Points  []string
	Links   []DocsLinkView
}

// DocsLinkView is one inline link under a section or FAQ answer.
type DocsLinkView struct {
	Label string
	Href  string
}

// DocsFAQView is one common question and its plain-language answer.
type DocsFAQView struct {
	Question  string
	Answer    string
	LinkLabel string
	LinkHref  string
}

// DocsGlossaryView is the searchable glossary. Entries is the filtered slice
// and Total is the unfiltered count.
type DocsGlossaryView struct {
	Query    string
	Entries  []GlossaryEntry
	Total    int
	Filtered bool
}

// GlossaryEntry is one user-facing betting or app term. Slug is the stable
// anchor suffix: the entry renders as id="term-<Slug>".
type GlossaryEntry struct {
	Slug     string
	Term     string
	Category string
	Summary  string
	Example  string
}

// docsHub serves GET /docs: plain-language explainers and common questions.
func (h *slipHandlers) docsHub(w http.ResponseWriter, r *http.Request) {
	backURL := safeReturnPath(r.URL.Query().Get("from"))

	h.renderPage(w, "docs", DocsPage{
		Page:    h.pageWithTab(r, "Docs", TabDocs),
		BackURL: backURL,
		Nav:     docsNav("overview", backURL),
		Hub:     docsHubView(),
	})
}

// docsGlossary serves GET /docs/glossary: the searchable term list. Filtering
// runs over the static slice, so it reads no database.
func (h *slipHandlers) docsGlossary(w http.ResponseWriter, r *http.Request) {
	backURL := safeReturnPath(r.URL.Query().Get("from"))
	query := strings.Join(strings.Fields(r.URL.Query().Get("q")), " ")
	entries := filterGlossary(glossaryEntries, query)

	h.renderPage(w, "docs", DocsPage{
		Page:    h.pageWithTab(r, "Glossary", TabDocs),
		BackURL: backURL,
		Nav:     docsNav("glossary", backURL),
		Glossary: &DocsGlossaryView{
			Query:    query,
			Entries:  entries,
			Total:    len(glossaryEntries),
			Filtered: query != "",
		},
	})
}

// docsNav builds the docs section switcher with the current section marked and
// the back link forwarded on every destination.
func docsNav(current, backURL string) []DocsNavView {
	suffix := ""
	if backURL != "" {
		suffix = "?from=" + url.QueryEscape(backURL)
	}

	return []DocsNavView{
		{Label: "Overview", Href: "/docs" + suffix, Current: current == "overview"},
		{Label: "Glossary", Href: "/docs/glossary" + suffix, Current: current == "glossary"},
		{Label: "Grading & Rules", Href: "/analyzer/guide" + suffix},
		{Label: "Eval Results", Href: "/analyzer/results" + suffix},
	}
}

// filterGlossary returns the entries whose term, category, summary, or example
// contains query, ignoring case. An empty query returns every entry.
func filterGlossary(entries []GlossaryEntry, query string) []GlossaryEntry {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return entries
	}

	var matches []GlossaryEntry
	for _, entry := range entries {
		haystack := strings.ToLower(strings.Join(
			[]string{entry.Term, entry.Category, entry.Summary, entry.Example}, " "))
		if strings.Contains(haystack, q) {
			matches = append(matches, entry)
		}
	}
	return matches
}

// docsHubView builds the hub's static content. The anchors are stable because
// help bubbles elsewhere in the app deep-link to them.
func docsHubView() *DocsHubView {
	return &DocsHubView{
		QuickJumps: []DocsJumpView{
			{Label: "Grading", Href: "#sys-grading", Summary: "What the A–F grade checks."},
			{Label: "Odds", Href: "#sys-odds", Summary: "Implied chance, vig, and price edge."},
			{Label: "Live", Href: "#sys-live", Summary: "How progress and hit chance move."},
			{Label: "Glossary", Href: "/docs/glossary", Summary: "Every term in plain words."},
		},
		Sections: []DocsSectionView{
			{
				ID:      "grading",
				Title:   "How grades work",
				Summary: "Every leg and slip gets an A–F grade from four 0–100 pillars. The grade is a heads-up, never a gate.",
				Points: []string{
					"Price & Vig checks what you paid against sharp prices for the same market and line.",
					"Script Fit looks at whether the legs in one game rise or fall together.",
					"Stability & Tail Risk measures how repeatable each stat has been.",
					"Context & Traps covers injuries, weather, key numbers, and role changes.",
				},
				Links: []DocsLinkView{
					{Label: "Full grading rubric", Href: "/analyzer/guide#leg-grades"},
					{Label: "What price edge means", Href: "/docs/glossary#term-price-edge"},
				},
			},
			{
				ID:      "odds",
				Title:   "Odds, implied chance, and edge",
				Summary: "American odds turn into a chance. Removing the book's margin gives a fair chance to compare against.",
				Points: []string{
					"-110 means risking $110 to win $100. +150 means $100 wins $150.",
					"Implied chance is the probability a single price suggests.",
					"De-vigging splits a two-sided market back into fair chances that add up to 100%.",
					"Price edge is the gap between your price and the reference price from other books.",
				},
				Links: []DocsLinkView{
					{Label: "Implied probability", Href: "/docs/glossary#term-implied-prob"},
					{Label: "De-vig", Href: "/docs/glossary#term-de-vig"},
					{Label: "Reference odds", Href: "/docs/glossary#term-reference-odds"},
				},
			},
			{
				ID:      "badges",
				Title:   "Pick-board badges",
				Summary: "Short tags on the pick board flag a matchup or role signal worth a second look.",
				Points: []string{
					"Smash matchup: a high-volume player against a bottom-tier defense.",
					"High volume: strong snap share, target share, or WOPR even when recent yardage is quiet.",
					"Role bump: a teammate is out and this player's role grew.",
					"Fits slip: the pick pairs well with a leg already on your slip.",
				},
				Links: []DocsLinkView{
					{Label: "Badge definitions", Href: "/analyzer/guide#suggestions"},
					{Label: "EPA", Href: "/docs/glossary#term-epa"},
					{Label: "WOPR", Href: "/docs/glossary#term-wopr"},
				},
			},
			{
				ID:      "live",
				Title:   "Live progress and hit chance",
				Summary: "During a game the slip shows how each leg is tracking from the clock, score, and box score.",
				Points: []string{
					"Live progress moves as plays land. It does not change the leg's grade.",
					"Live hit chance estimates the odds the leg still cashes from here.",
					"A leg can look ahead early and still lose, or look behind and still win.",
				},
				Links: []DocsLinkView{
					{Label: "Push and void", Href: "/docs/glossary#term-push-void"},
				},
			},
			{
				ID:      "autograde",
				Title:   "Automatic grading",
				Summary: "When a game goes final, legs grade themselves from the box score. You can still correct a call by hand.",
				Points: []string{
					"Won and lost come from the final stat line against your number.",
					"A push ties the number and returns the stake. A void leg drops out of the slip.",
					"Needs grading means the app could not settle the leg and wants your call.",
					"Custom legs the data does not cover wait for a manual result.",
				},
				Links: []DocsLinkView{
					{Label: "Push and void", Href: "/docs/glossary#term-push-void"},
				},
			},
			{
				ID:      "sources",
				Title:   "Where the numbers come from",
				Summary: "Lines, usage, ratings, and scores come from public feeds and cache in the app's database.",
				Points: []string{
					"Game lines come from the Odds API. Player props come from The Parlay API.",
					"ESPN supplies scores, injuries, and box scores. nflverse supplies usage and efficiency.",
					"nfelo supplies team ratings and win probabilities shown beside a leg.",
					"Lines refresh on a game-day morning pre-fill and again when you open a pick screen or a game.",
					"Every rule is plain Go. The app never calls an LLM.",
				},
				Links: []DocsLinkView{
					{Label: "nfelo", Href: "/docs/glossary#term-nfelo"},
					{Label: "Target share", Href: "/docs/glossary#term-target-share"},
				},
			},
			{
				ID:      "export",
				Title:   "Export to your sportsbook",
				Summary: "Copy a clean list of legs and odds into your sportsbook's bet slip. The app never places the bet or moves money.",
				Points: []string{
					"An export lists each leg's market, selection, and price.",
					"Odds, boosts, and stake stay in your hands at the book.",
					"The app tracks slips. People place them.",
				},
			},
		},
		FAQ: []DocsFAQView{
			{
				Question: "Why is every pick still allowed?",
				Answer:   "The analyzer is a sharp friend, not a gatekeeper. Grades and warnings point at risk, and a person makes the call.",
			},
			{
				Question:  "Why did my grade change?",
				Answer:    "Grades read the latest lines, injuries, weather, and usage. When those move, the grade moves with them.",
				LinkLabel: "How grades work",
				LinkHref:  "#sys-grading",
			},
			{
				Question:  "What does the high uncertainty badge mean?",
				Answer:    "Injury news, a role change, or a wide model-vs-market gap makes the number shakier. The grader then leans harder on the live market price.",
				LinkLabel: "Full grading rubric",
				LinkHref:  "/analyzer/guide#pillars",
			},
			{
				Question: "Do you place bets or hold money?",
				Answer:   "No. The app tracks slips and shows numbers. You place the bet at your own sportsbook.",
			},
			{
				Question:  "How fresh are the lines?",
				Answer:    "Game lines refresh on a game-day morning pre-fill, and again when you open a pick screen. Props load for the game you open, then cache.",
				LinkLabel: "Where the numbers come from",
				LinkHref:  "#sys-sources",
			},
			{
				Question: "Why does a leg show no grade?",
				Answer:   "Custom or unmatched legs stay ungraded when the feeds do not carry that market. They still count toward the slip.",
			},
			{
				Question: "Who can see my slip?",
				Answer:   "Only the members of your group. Sign-in is Google, and the app has no separate accounts or permissions.",
			},
		},
	}
}

// glossaryEntries is the static term list. Slugs must stay unique because each
// one is an anchor other screens deep-link to.
var glossaryEntries = []GlossaryEntry{
	{
		Slug:     "implied-prob",
		Term:     "Implied probability",
		Category: "Odds",
		Summary:  "The chance a single price suggests. A -110 price implies about 52.4%.",
		Example:  "A -110 leg wins about 52 times in 100, before the book's margin is removed.",
	},
	{
		Slug:     "de-vig",
		Term:     "De-vig",
		Category: "Odds",
		Summary:  "Removing the sportsbook's built-in margin from both sides of a market to estimate a fair chance.",
		Example:  "-110 and -110 imply 104.8% together. De-vigging splits that back into 50/50.",
	},
	{
		Slug:     "price-edge",
		Term:     "Price edge",
		Category: "Odds",
		Summary:  "The gap between the price you took and the reference price from other books. Positive edge means you beat the market.",
		Example:  "You take +105 while the reference price sits at -108.",
	},
	{
		Slug:     "reference-odds",
		Term:     "Reference odds",
		Category: "Odds",
		Summary:  "The median price across the books your group uses for the same market, player, side, and line.",
		Example:  "Three books show -105, -110, and -115, so the reference is -110.",
	},
	{
		Slug:     "epa",
		Term:     "EPA",
		Category: "Usage & efficiency",
		Summary:  "Expected points added. A per-play measure of how much a play helped the offense.",
		Example:  "0.15 EPA per dropback is a strong passing day.",
	},
	{
		Slug:     "wopr",
		Term:     "WOPR",
		Category: "Usage & efficiency",
		Summary:  "A weighted blend of target share and air yards share that estimates how much of the passing offense runs through a player.",
		Example:  "A 0.72 WOPR receiver sees a big slice of the targets and the downfield work.",
	},
	{
		Slug:     "target-share",
		Term:     "Target share",
		Category: "Usage & efficiency",
		Summary:  "The share of a team's pass targets that goes to one player.",
		Example:  "Eight of the team's 32 targets is a 25% target share.",
	},
	{
		Slug:     "snap-share",
		Term:     "Snap share",
		Category: "Usage & efficiency",
		Summary:  "The share of the team's offensive snaps a player is on the field for.",
		Example:  "A back at 65% snap share usually sees most of the backfield work.",
	},
	{
		Slug:     "nfelo",
		Term:     "nfelo",
		Category: "Model",
		Summary:  "A public NFL rating and win-probability model. The app shows its spread or win chance beside a leg.",
		Example:  "nfelo BUF -3.2 next to your BUF -2.5 spread means the model likes your side.",
	},
	{
		Slug:     "key-numbers",
		Term:     "Key numbers",
		Category: "Betting",
		Summary:  "The margins that decide the most games, such as 3, 7, 10, and 14. A line sitting on one is harder to beat.",
		Example:  "Laying -3 pushes on a 3-point win. Laying -2.5 does not.",
	},
	{
		Slug:     "line-drift",
		Term:     "Line drift",
		Category: "Betting",
		Summary:  "How far a line or price has moved since it opened.",
		Example:  "A spread that opened -2.5 and now sits -3.5 drifted a full point.",
	},
	{
		Slug:     "chalk",
		Term:     "Chalk",
		Category: "Betting",
		Summary:  "A heavy favorite with a short price. Stacking chalk legs makes a parlay pay less for the risk.",
		Example:  "A -400 moneyline leg adds little to a parlay's payout.",
	},
	{
		Slug:     "correlation",
		Term:     "Correlation",
		Category: "Betting",
		Summary:  "When two legs rise or fall together. Same-game legs are often correlated in ways a parlay price may not reflect.",
		Example:  "A quarterback's passing yards and his top receiver's catches tend to rise together.",
	},
	{
		Slug:     "push-void",
		Term:     "Push and void",
		Category: "Betting",
		Summary:  "A push ties the number and returns the stake. A void leg does not count at all.",
		Example:  "BUF -3 with a 3-point win pushes. A canceled game voids the leg.",
	},
	{
		Slug:     "slate-scope",
		Term:     "Slate scope",
		Category: "App",
		Summary:  "Which games a slip can pick from: the full week or just one game day's slate.",
		Example:  "A full-week slip can use Sunday and Monday games. A slate slip stays on one date.",
	},
}
