package web_test

import (
	"io/fs"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	webui "github.com/kalverra/parlay-club/internal/web"
	assets "github.com/kalverra/parlay-club/web"
)

func TestLayoutRendersBottomTabs(t *testing.T) {
	t.Parallel()

	out := renderPage(t, "record", webui.Page{
		Title:  "Record Book",
		Tab:    webui.TabRecord,
		Groups: []webui.GroupOption{{ID: 7, Name: "Sunday Parlay Club", Current: true}},
	})

	for _, label := range []string{"Groups", "This week", "Record Book"} {
		assert.Contains(t, out, ">"+label+"</a")
	}

	assert.Equal(t, 1, strings.Count(out, `aria-current="page"`))

	current := regexp.MustCompile(`<a\s+href="([^"]+)"[^>]*aria-current="page"`)
	match := current.FindStringSubmatch(out)
	require.Len(t, match, 2)
	assert.Equal(t, "/record", match[1])
	assert.Contains(t, out, `href="/groups/7/week"`, "This week names the active group")
}

func TestLayoutLinksAssets(t *testing.T) {
	t.Parallel()

	out := renderPage(t, "slip", webui.Page{Title: "Slip", Tab: webui.TabWeek})

	assert.Contains(t, out, "/static/app.css?v=")
	assert.Contains(t, out, "/static/app.js?v=")

	urls := regexp.MustCompile(`/static/[^"]+`).FindAllString(out, -1)
	require.NotEmpty(t, urls)

	for _, url := range urls {
		name, query, found := strings.Cut(strings.TrimPrefix(url, "/static/"), "?")
		assert.True(t, found, "asset URL %s has no cache-busting query", url)
		assert.NotEmpty(t, query)

		// The template names the vendored file, so check it was really vendored.
		_, err := fs.Stat(assets.Static, "static/"+name)
		require.NoError(t, err, "template links missing asset %s", url)
	}
}

func TestLayoutTopBar(t *testing.T) {
	t.Parallel()

	out := renderPage(t, "slip", webui.Page{
		Title:     "Slip",
		Tab:       webui.TabWeek,
		WeekLabel: "Week 9",
		Slips: []webui.SlipOption{
			{ID: 1, Name: "Week 9"},
			{ID: 2, Name: "Week 9 #2", Current: true},
		},
		User: &webui.User{Name: "Adam Hamrick", Initials: "AH", AvatarClass: "bg-sky-500"},
	})

	assert.Contains(t, out, ">Week 9</span>")
	assert.Contains(t, out, `id="slip-switcher"`)
	assert.Contains(t, out, `<option value="2" selected>Week 9 #2</option>`)
	assert.Equal(t, 1, strings.Count(out, "selected"))
	assert.Contains(t, out, `href="/me"`)
	assert.Contains(t, out, `title="Adam Hamrick"`)
	assert.Contains(t, out, ">AH</span")
	assert.Contains(t, out, "bg-sky-500")
}

func TestAvatarShowsPicture(t *testing.T) {
	t.Parallel()

	out := renderPage(t, "slip", webui.Page{
		Title: "Slip",
		Tab:   webui.TabWeek,
		User: &webui.User{
			Name:        "Adam Hamrick",
			Initials:    "AH",
			AvatarClass: "bg-sky-500",
			PictureURL:  "https://example.com/ada.png",
		},
	})

	assert.Contains(t, out, `src="https://example.com/ada.png"`)
	assert.Contains(t, out, `referrerpolicy="no-referrer"`)
	assert.NotContains(t, out, ">AH</span")
}

func TestAvatarFallsBackToInitials(t *testing.T) {
	t.Parallel()

	out := renderPage(t, "slip", webui.Page{
		Title: "Slip",
		Tab:   webui.TabWeek,
		User:  &webui.User{Name: "Adam Hamrick", Initials: "AH", AvatarClass: "bg-sky-500"},
	})

	assert.Contains(t, out, ">AH</span")
	assert.Contains(t, out, "bg-sky-500")
	assert.NotContains(t, out, "<img")
}

func TestLayoutHidesSwitcherForOneSlip(t *testing.T) {
	t.Parallel()

	out := renderPage(t, "slip", webui.Page{
		Title: "Slip",
		Tab:   webui.TabWeek,
		Slips: []webui.SlipOption{{ID: 1, Name: "Week 9", Current: true}},
	})

	assert.NotContains(t, out, `id="slip-switcher"`)
}

func TestLayoutGroupSwitcher(t *testing.T) {
	t.Parallel()

	out := renderPage(t, "slip", webui.Page{
		Title: "Slip",
		Tab:   webui.TabWeek,
		User:  &webui.User{Name: "Adam Hamrick", Initials: "AH", AvatarClass: "bg-sky-500"},
		Groups: []webui.GroupOption{
			{ID: 1, Name: "Just me"},
			{ID: 2, Name: "Sunday Parlay Club", Current: true},
		},
	})

	assert.Contains(t, out, `data-group-switcher`)
	assert.Contains(t, out, ">Sunday Parlay Club</span>")
	assert.Contains(t, out, "Your groups")
	assert.Contains(t, out, `hx-post="/groups/switch"`)
	assert.Contains(t, out, `name="group_id" value="1"`)
	assert.Contains(t, out, `name="group_id" value="2"`)
	assert.Contains(t, out, `href="/groups"`)
}

func TestLayoutHidesGroupSwitcherForOneGroup(t *testing.T) {
	t.Parallel()

	out := renderPage(t, "slip", webui.Page{
		Title:  "Slip",
		Tab:    webui.TabWeek,
		User:   &webui.User{Name: "Adam Hamrick", Initials: "AH", AvatarClass: "bg-sky-500"},
		Groups: []webui.GroupOption{{ID: 1, Name: "Just me", Current: true}},
	})

	assert.NotContains(t, out, `data-group-switcher`)
	assert.NotContains(t, out, "Your groups")
	assert.Contains(t, out, `href="/groups"`, "a solo player still reaches the groups page")
	assert.Contains(t, out, `title="Just me"`)
	assert.Contains(t, out, ">Just me</a")
}

func TestLayoutWithoutUser(t *testing.T) {
	t.Parallel()

	out := renderPage(t, "slip", webui.Page{Title: "Slip", Tab: webui.TabWeek})

	assert.NotContains(t, out, `href="/me"`)
}

func TestPlaceholderPages(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		file    string
		tab     webui.Tab
		heading string
		message string
	}{
		{"slip", "slip", webui.TabWeek, "Slip", "Slip contents show up here once the season starts."},
		{"research", "research", "", "Research", "Player and game research shows up here."},
		{"record", "record", webui.TabRecord, "Record Book", "Season results show up here."},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			out := renderPage(t, test.file, webui.Page{Title: test.heading, Tab: test.tab})

			assert.Contains(t, out, "<title>"+test.heading+" · Sunday Parlay Club</title>")
			assert.Contains(t, out, `<h1 class="text-xl font-semibold">`+test.heading+"</h1>")
			assert.Contains(t, out, test.message)
		})
	}
}

// renderPage parses the layout and one page file the way the server does, then
// returns the HTML.
func renderPage(t *testing.T, page string, data webui.Page) string {
	t.Helper()

	renderer, err := webui.NewRenderer(assets.Templates, false)
	require.NoError(t, err)

	var value any = data
	switch page {
	case "home":
		value = webui.HomePage{Page: data}
	case "slip":
		value = webui.SlipPage{Page: data}
	case "research":
		value = webui.ResearchPage{Page: data}
	}

	html, err := renderer.Render(page, value)
	require.NoError(t, err)

	return string(html)
}
