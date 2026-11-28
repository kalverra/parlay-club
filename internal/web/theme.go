package web

import (
	"net/http"
	"strings"
)

// ThemeCookie stores the browser's selected UI theme ID.
const ThemeCookie = "pc_theme"

// ThemeID identifies one of the allowlisted site themes.
type ThemeID string

// The permanent site themes.
const (
	ThemePaperTurf ThemeID = "paper-turf"
	ThemeRedZone   ThemeID = "redzone"
	ThemeAlmanac   ThemeID = "almanac"
)

// Theme describes a selectable site style and its document-level metadata.
type Theme struct {
	ID          ThemeID
	Label       string
	ShortLabel  string
	ColorScheme string
	ThemeColor  string
}

var defaultTheme = Theme{
	ID:          ThemePaperTurf,
	Label:       "Paper & Turf",
	ShortLabel:  "Paper",
	ColorScheme: "dark",
	ThemeColor:  "#0f1e16",
}

var allowedThemes = map[ThemeID]Theme{
	ThemePaperTurf: defaultTheme,
	ThemeRedZone: {
		ID:          ThemeRedZone,
		Label:       "RedZone",
		ShortLabel:  "RedZone",
		ColorScheme: "dark",
		ThemeColor:  "#090a0c",
	},
	ThemeAlmanac: {
		ID:          ThemeAlmanac,
		Label:       "90s Almanac",
		ShortLabel:  "Almanac",
		ColorScheme: "light",
		ThemeColor:  "#fff8e7",
	},
}

// themeOrDefault returns the allowlisted theme for id, falling back to
// ThemePaperTurf for empty or unrecognized IDs.
func themeOrDefault(id ThemeID) Theme {
	if t, ok := allowedThemes[id]; ok {
		return t
	}
	return defaultTheme
}

// ThemeFromRequest reads the pc_theme cookie from r and returns the matching
// allowlisted Theme, falling back to ThemePaperTurf when absent or invalid.
func ThemeFromRequest(r *http.Request) Theme {
	id, ok := cookieThemeID(r)
	if !ok {
		return defaultTheme
	}
	return themeOrDefault(id)
}

// cookieThemeID reports the allowlisted theme ID stored in the pc_theme
// cookie. Invalid or empty values are ignored.
func cookieThemeID(r *http.Request) (ThemeID, bool) {
	if r == nil {
		return "", false
	}
	cookie, err := r.Cookie(ThemeCookie)
	if err != nil {
		return "", false
	}
	id := ThemeID(strings.TrimSpace(cookie.Value))
	if _, ok := allowedThemes[id]; !ok {
		return "", false
	}
	return id, true
}
