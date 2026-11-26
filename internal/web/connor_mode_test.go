package web_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	webui "github.com/kalverra/parlay-club/internal/web"
)

func TestConnorModeIsRegistered(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{
		Name:     webui.ThemeCookie,
		Value:    "connor-mode",
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	got := webui.ThemeFromRequest(req)
	assert.Equal(t, webui.ThemeConnorMode, got.ID)
	assert.Equal(t, "Connor Mode", got.Label)
	assert.Equal(t, "Connor", got.ShortLabel)
	assert.Equal(t, "light", got.ColorScheme)
	assert.Equal(t, "#e40303", got.ThemeColor)

	assert.Equal(t, webui.ThemeConnorMode, (webui.Page{Theme: webui.ThemeConnorMode}).ActiveTheme().ID)
}

func TestConnorPersonalTheme(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		userName    string
		cookieValue string
		hasCookie   bool
		chosen      bool
		wantID      webui.ThemeID
	}{
		{
			name:     "connor without a cookie gets connor mode",
			userName: "Connor",
			wantID:   webui.ThemeConnorMode,
		},
		{
			name:     "connor hite gets connor mode",
			userName: "Connor Hite",
			wantID:   webui.ThemeConnorMode,
		},
		{
			name:     "connor full name is matched case-insensitively",
			userName: "connor smith",
			wantID:   webui.ThemeConnorMode,
		},
		{
			name:        "a chosen cookie wins over connor mode",
			userName:    "Connor",
			hasCookie:   true,
			cookieValue: "almanac",
			chosen:      true,
			wantID:      webui.ThemeAlmanac,
		},
		{
			name:        "connor hite ignores a stale cookie without the chosen marker",
			userName:    "Connor Hite",
			hasCookie:   true,
			cookieValue: "almanac",
			wantID:      webui.ThemeConnorMode,
		},
		{
			name:        "connor with an invalid cookie falls back to connor mode",
			userName:    "Connor",
			hasCookie:   true,
			cookieValue: "evil",
			wantID:      webui.ThemeConnorMode,
		},
		{
			name:     "a name that merely starts with connor is not connor",
			userName: "Connoria",
			wantID:   webui.ThemePaperTurf,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
			if test.hasCookie {
				req.AddCookie(&http.Cookie{
					Name:     webui.ThemeCookie,
					Value:    test.cookieValue,
					Secure:   true,
					HttpOnly: true,
					SameSite: http.SameSiteLaxMode,
				})
			}
			if test.chosen {
				req.AddCookie(&http.Cookie{
					Name:     webui.ThemeChosenCookie,
					Value:    "1",
					Secure:   true,
					HttpOnly: true,
					SameSite: http.SameSiteLaxMode,
				})
			}

			assert.Equal(t, test.wantID, webui.ThemeForRequest(req, test.userName).ID)
		})
	}
}

func TestConnorModeLayoutSSR(t *testing.T) {
	t.Parallel()

	out := renderPage(t, "slip", webui.Page{
		Title: "Slip",
		Tab:   webui.TabWeek,
		Theme: webui.ThemeConnorMode,
	})

	assert.Contains(t, out, `data-theme="connor-mode"`)
	assert.Contains(t, out, `<meta name="color-scheme" content="light">`)
	assert.Contains(t, out, `<meta name="theme-color" content="#e40303">`)
}

func TestConnorModeMarqueeOnlyInConnorMode(t *testing.T) {
	t.Parallel()

	base := webui.Page{
		Title:     "Slip",
		Tab:       webui.TabWeek,
		WeekLabel: "Week 9",
	}

	connor := base
	connor.Theme = webui.ThemeConnorMode
	out := renderPage(t, "slip", connor)
	assert.Contains(t, out, "connor-marquee")
	assert.Contains(t, out, "GAY RIGHTS ARE HUMAN RIGHTS")

	paper := base
	paper.Theme = webui.ThemePaperTurf
	out = renderPage(t, "slip", paper)
	assert.NotContains(t, out, "connor-marquee")
}
