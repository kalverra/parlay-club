package web

import (
	"context"
	"net/http"

	"github.com/rs/zerolog"

	"github.com/kalverra/parlay-club/internal/db"
)

// pageRoute is one full page the shell serves.
type pageRoute struct {
	pattern string
	page    string
	tab     Tab
}

// UserFrom returns the signed-in user for a request. The server passes
// auth.UserFrom; tests pass a stub.
type UserFrom func(context.Context) (db.User, bool)

// GroupsFrom returns the groups to offer in the top bar's switcher, with the
// active one marked. The server passes groups.Options, which reads what the
// group scope put on the context; tests may pass nil.
type GroupsFrom func(context.Context) []GroupOption

// pageRoutes are the shell's pages, in tab order. Later changes replace the
// placeholders with real screens.
var pageRoutes = []pageRoute{
	{"GET /{$}", "slip", TabSlip},
	{"GET /games", "games", TabGames},
	{"GET /research", "research", TabResearch},
	{"GET /record", "record", TabRecord},
}

// New returns the shell's routes: the four tab pages and /static.
func New(
	logger zerolog.Logger,
	renderer *Renderer,
	userFrom UserFrom,
	groupsFrom GroupsFrom,
) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /static/{path...}", StaticHandler())

	for _, route := range pageRoutes {
		mux.Handle(route.pattern, pageHandler(logger, renderer, route, userFrom, groupsFrom))
	}

	return mux
}

// pageHandler renders one page of the shell.
func pageHandler(
	logger zerolog.Logger,
	renderer *Renderer,
	route pageRoute,
	userFrom UserFrom,
	groupsFrom GroupsFrom,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page := Page{Title: route.tab.Label(), Tab: route.tab}
		if userFrom != nil {
			if user, ok := userFrom(r.Context()); ok {
				page.User = pageUser(user)
			}
		}
		if groupsFrom != nil {
			page.Groups = groupsFrom(r.Context())
		}

		html, err := renderer.Render(route.page, page)
		if err != nil {
			logger.Error().Err(err).Str("page", route.page).Msg("render page")
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(html)
	})
}

// pageUser maps a stored user to the identity the top bar draws.
func pageUser(user db.User) *User {
	return &User{
		Name:        user.Name,
		Initials:    Initials(user.Name),
		AvatarClass: user.AvatarColor.Class(),
		PictureURL:  user.PictureURL,
	}
}
