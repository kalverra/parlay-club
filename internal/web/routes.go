package web

import (
	"net/http"

	"github.com/rs/zerolog"
)

// pageRoute is one full page the shell serves.
type pageRoute struct {
	pattern string
	page    string
	tab     Tab
}

// pageRoutes are the shell's pages, in tab order. Later changes replace the
// placeholders with real screens.
var pageRoutes = []pageRoute{
	{"GET /{$}", "slip", TabSlip},
	{"GET /games", "games", TabGames},
	{"GET /research", "research", TabResearch},
	{"GET /record", "record", TabRecord},
}

// New returns the shell's routes: the four tab pages and /static.
func New(logger zerolog.Logger, renderer *Renderer) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /static/{path...}", StaticHandler())

	for _, route := range pageRoutes {
		mux.Handle(route.pattern, pageHandler(logger, renderer, route))
	}

	return mux
}

// pageHandler renders one page of the shell.
func pageHandler(logger zerolog.Logger, renderer *Renderer, route pageRoute) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		html, err := renderer.Render(route.page, Page{Title: route.tab.Label(), Tab: route.tab})
		if err != nil {
			logger.Error().Err(err).Str("page", route.page).Msg("render page")
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(html)
	})
}
