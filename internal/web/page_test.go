package web_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	webui "github.com/kalverra/parlay-club/internal/web"
)

func TestPageTabs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		tab   webui.Tab
		href  string
		group []webui.GroupOption
	}{
		{"groups", webui.TabGroups, "/", nil},
		{
			"week",
			webui.TabWeek,
			"/groups/7/week",
			[]webui.GroupOption{{ID: 4, Name: "Just me"}, {ID: 7, Name: "Sunday Parlay Club", Current: true}},
		},
		{"research", webui.TabResearch, "/research", nil},
		{"record", webui.TabRecord, "/record", nil},
		{"docs", webui.TabDocs, "/docs", nil},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			tabs := webui.Page{Tab: test.tab, Groups: test.group}.Tabs()
			require.Len(t, tabs, 5)

			labels := make([]string, 0, len(tabs))
			for _, tab := range tabs {
				labels = append(labels, tab.Label)
				if tab.Tab == test.tab {
					assert.True(t, tab.Current)
					assert.Equal(t, test.href, tab.Href)
				} else {
					assert.False(t, tab.Current)
				}
			}

			assert.Equal(t, []string{"Groups", "This week", "Research", "Record Book", "Docs"}, labels)
		})
	}
}

func TestWeekTabFallsBackToTheGroupList(t *testing.T) {
	t.Parallel()

	for _, tab := range (webui.Page{Tab: webui.TabWeek}).Tabs() {
		if tab.Tab == webui.TabWeek {
			assert.Equal(t, "/", tab.Href)
		}
	}
}

func TestInitials(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"one word", "adam", "A"},
		{"two words", "Adam Hamrick", "AH"},
		{"extra spaces", "  jane   doe ", "JD"},
		{"hyphenated", "Jean-Luc Picard", "JP"},
		{"three words", "Mary Jane Watson", "MJ"},
		{"empty", "", ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.want, webui.Initials(test.input))
		})
	}
}
