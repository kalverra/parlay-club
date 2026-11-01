// Package web renders the app's pages.
package web

import (
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

// SlipOption is one choice in the top bar's slip switcher.
type SlipOption struct {
	ID      int64
	Name    string
	Current bool
}

// User is the person shown in the top bar.
type User struct {
	Name        string
	Initials    string
	AvatarClass string
}

// Page is everything the layout needs from a page.
type Page struct {
	Title     string
	Tab       Tab
	WeekLabel string
	Slips     []SlipOption
	User      *User
}

// The four bottom-bar destinations.
const (
	TabSlip     Tab = "slip"
	TabGames    Tab = "games"
	TabResearch Tab = "research"
	TabRecord   Tab = "record"
)

// navTabs is the bottom bar in display order.
var navTabs = []NavTab{
	{Tab: TabSlip, Label: "Slip", Href: "/"},
	{Tab: TabGames, Label: "Games", Href: "/games"},
	{Tab: TabResearch, Label: "Research", Href: "/research"},
	{Tab: TabRecord, Label: "Record Book", Href: "/record"},
}

// Tabs returns the bottom bar with the page's own tab marked current.
func (p Page) Tabs() []NavTab {
	tabs := make([]NavTab, len(navTabs))
	for i, tab := range navTabs {
		tab.Current = tab.Tab == p.Tab
		tabs[i] = tab
	}
	return tabs
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
