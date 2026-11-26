package web

import (
	"net/http"
	"strings"
)

// ThemeConnorMode is a temporary joke theme. Everything it needs on the server
// lives in this file. Its styles and markup sit in blocks marked "BEGIN Connor
// Mode" / "END Connor Mode" in web/tailwind/input.css, web/static/app.js, and
// web/templates/layout.html. Run scripts/remove-connor-mode.sh to remove it.
const ThemeConnorMode ThemeID = "connor-mode"

// ThemeChosenCookie marks that pc_theme is an explicit choice made with the
// theme switcher. Without it, a pc_theme cookie predates Connor Mode and must
// not hide Connor's default.
const ThemeChosenCookie = "pc_theme_chosen"

func init() {
	allowedThemes[ThemeConnorMode] = Theme{
		ID:          ThemeConnorMode,
		Label:       "Connor Mode",
		ShortLabel:  "Connor",
		ColorScheme: "light",
		ThemeColor:  "#e40303",
	}
	personalThemeFor = connorPersonalTheme
}

// connorPersonalTheme gives Connor connor-mode unless he explicitly picked
// another theme with the switcher. Ignoring a legacy cookie is what makes the
// default keep showing up on every visit.
func connorPersonalTheme(r *http.Request, name string) (Theme, bool) {
	if !isConnor(name) {
		return Theme{}, false
	}
	if _, ok := cookieThemeID(r); ok && hasChosenTheme(r) {
		return Theme{}, false
	}
	return themeOrDefault(ThemeConnorMode), true
}

// isConnor matches the first word of a display name case-insensitively, so
// "Connor Hite" and "connor" both count.
func isConnor(name string) bool {
	first, _, _ := strings.Cut(strings.TrimSpace(name), " ")
	return strings.EqualFold(first, "connor")
}

// hasChosenTheme reports whether the browser has an explicit theme choice from
// the switcher.
func hasChosenTheme(r *http.Request) bool {
	if r == nil {
		return false
	}
	_, err := r.Cookie(ThemeChosenCookie)
	return err == nil
}
