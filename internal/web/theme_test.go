package web_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	webui "github.com/kalverra/parlay-club/internal/web"
)

func TestThemeFromRequest(t *testing.T) {
	t.Parallel()

	assert.Equal(t, webui.ThemePaperTurf, webui.ThemeFromRequest(nil).ID)

	tests := []struct {
		name        string
		cookieValue string
		hasCookie   bool
		wantID      webui.ThemeID
		wantLabel   string
		wantScheme  string
		wantColor   string
	}{
		{
			name:       "no cookie defaults to paper-turf",
			hasCookie:  false,
			wantID:     webui.ThemePaperTurf,
			wantLabel:  "Paper & Turf",
			wantScheme: "dark",
			wantColor:  "#0f1e16",
		},
		{
			name:        "paper-turf cookie",
			hasCookie:   true,
			cookieValue: "paper-turf",
			wantID:      webui.ThemePaperTurf,
			wantLabel:   "Paper & Turf",
			wantScheme:  "dark",
			wantColor:   "#0f1e16",
		},
		{
			name:        "redzone cookie",
			hasCookie:   true,
			cookieValue: "redzone",
			wantID:      webui.ThemeRedZone,
			wantLabel:   "RedZone",
			wantScheme:  "dark",
			wantColor:   "#090a0c",
		},
		{
			name:        "almanac cookie",
			hasCookie:   true,
			cookieValue: "almanac",
			wantID:      webui.ThemeAlmanac,
			wantLabel:   "90s Almanac",
			wantScheme:  "light",
			wantColor:   "#fff8e7",
		},
		{
			name:        "empty cookie falls back to default",
			hasCookie:   true,
			cookieValue: "",
			wantID:      webui.ThemePaperTurf,
			wantLabel:   "Paper & Turf",
			wantScheme:  "dark",
			wantColor:   "#0f1e16",
		},
		{
			name:        "invalid cookie falls back to default",
			hasCookie:   true,
			cookieValue: "evil",
			wantID:      webui.ThemePaperTurf,
			wantLabel:   "Paper & Turf",
			wantScheme:  "dark",
			wantColor:   "#0f1e16",
		},
		{
			name:        "uppercase cookie is rejected",
			hasCookie:   true,
			cookieValue: "REDZONE",
			wantID:      webui.ThemePaperTurf,
			wantLabel:   "Paper & Turf",
			wantScheme:  "dark",
			wantColor:   "#0f1e16",
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

			got := webui.ThemeFromRequest(req)
			assert.Equal(t, test.wantID, got.ID)
			assert.Equal(t, test.wantLabel, got.Label)
			assert.Equal(t, test.wantScheme, got.ColorScheme)
			assert.Equal(t, test.wantColor, got.ThemeColor)
		})
	}
}

func TestPageActiveTheme(t *testing.T) {
	t.Parallel()

	assert.Equal(t, webui.ThemePaperTurf, (webui.Page{}).ActiveTheme().ID)
	assert.Equal(t, webui.ThemeRedZone, (webui.Page{Theme: webui.ThemeRedZone}).ActiveTheme().ID)
	assert.Equal(t, webui.ThemeAlmanac, (webui.Page{Theme: webui.ThemeAlmanac}).ActiveTheme().ID)
	assert.Equal(t, webui.ThemePaperTurf, (webui.Page{Theme: webui.ThemeID("unknown")}).ActiveTheme().ID)
}
