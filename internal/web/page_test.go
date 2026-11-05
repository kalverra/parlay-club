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
		name string
		tab  webui.Tab
		href string
	}{
		{"slip", webui.TabSlip, "/"},
		{"games", webui.TabGames, "/games"},
		{"research", webui.TabResearch, "/research"},
		{"record", webui.TabRecord, "/record"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			tabs := webui.Page{Tab: test.tab}.Tabs()
			require.Len(t, tabs, 4)

			labels := make([]string, 0, len(tabs))
			for _, tab := range tabs {
				labels = append(labels, tab.Label)
				assert.Equal(t, tab.Href == test.href, tab.Current)
			}

			assert.Equal(t, []string{"Slip", "Games", "Research", "Record Book"}, labels)
		})
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
