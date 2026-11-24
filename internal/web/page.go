// Package web renders the app's pages.
package web

import (
	"fmt"
	"strings"
	"unicode"
)

// Tab identifies a bottom-bar destination.
type Tab string

// NavTab is one entry in the bottom bar.
type NavTab struct {
	Tab     Tab
	Label   string
	Href    string
	Current bool
}

// GroupOption is one choice in the top bar's group switcher.
type GroupOption struct {
	ID      int64
	Name    string
	Current bool
}

// User is the person shown in the top bar.
type User struct {
	Name        string
	Initials    string
	AvatarClass string
	PictureURL  string
}

// Page is everything the layout needs from a page.
type Page struct {
	Title     string
	Tab       Tab
	WeekLabel string
	Groups    []GroupOption
	User      *User
	// Wide widens the header and main column for a two-pane screen. Pages
	// without it keep the one-column max-w-3xl layout.
	Wide bool
	// Theme selects the active site style; empty defaults to ThemePaperTurf.
	Theme ThemeID
}

// ActiveTheme returns the allowlisted Theme for the page, defaulting to
// ThemePaperTurf when Theme is empty or unrecognized.
func (p Page) ActiveTheme() Theme {
	return themeOrDefault(p.Theme)
}

// CurrentGroup returns the name of the active group, or "" when the page has
// no group choices. The layout shows it in the top bar's group switcher.
func (p Page) CurrentGroup() string {
	for _, group := range p.Groups {
		if group.Current {
			return group.Name
		}
	}
	return ""
}

// The three bottom-bar destinations.
const (
	TabGroups Tab = "groups"
	TabWeek   Tab = "week"
	TabRecord Tab = "record"
)

// navTabs is the bottom bar in display order. The This week link is filled in
// per page because it names the active group.
var navTabs = []NavTab{
	{Tab: TabGroups, Label: "Groups", Href: "/"},
	{Tab: TabWeek, Label: "This week", Href: "/"},
	{Tab: TabRecord, Label: "Record Book", Href: "/record"},
}

// Tabs returns the bottom bar with the page's own tab marked current.
func (p Page) Tabs() []NavTab {
	tabs := make([]NavTab, len(navTabs))
	for i, tab := range navTabs {
		tab.Current = tab.Tab == p.Tab
		if tab.Tab == TabWeek {
			tab.Href = p.weekHref()
		}
		tabs[i] = tab
	}
	return tabs
}

// weekHref points This week at the active group, falling back to the group
// list when the page has no group choices.
func (p Page) weekHref() string {
	for _, group := range p.Groups {
		if group.Current {
			return fmt.Sprintf("/groups/%d/week", group.ID)
		}
	}
	return "/"
}

// Label returns the tab's display name.
func (t Tab) Label() string {
	for _, tab := range navTabs {
		if tab.Tab == t {
			return tab.Label
		}
	}
	return string(t)
}

// Initials returns up to two uppercase letters from a name for its avatar.
func Initials(name string) string {
	initials := make([]rune, 0, 2)
	for word := range strings.FieldsSeq(name) {
		initials = append(initials, unicode.ToUpper([]rune(word)[0]))
		if len(initials) == 2 {
			break
		}
	}
	return string(initials)
}
