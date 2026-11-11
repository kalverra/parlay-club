package web

import (
	"context"
	"net/http"
	"time"

	"github.com/rs/zerolog"

	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/live"
	"github.com/kalverra/parlay-club/internal/slips"
)

// pageRoute is one full page the shell serves without slip data.
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

// GroupFrom returns the active group for a request. The server passes
// groups.From; tests pass a stub.
type GroupFrom func(context.Context) (db.Group, bool)

// Refresher describes on-demand sync for game props and research.
type Refresher interface {
	SyncGameProps(ctx context.Context, gameID string) error
	SyncGameResearch(ctx context.Context, gameID string) error
	SyncPlayerResearch(ctx context.Context, athleteID string) error
}

// Dependencies are the services the app's routes need. A nil store or slip
// service leaves the slip screen in its placeholder state, which keeps the
// shell's own tests small.
type Dependencies struct {
	Logger     zerolog.Logger
	Renderer   *Renderer
	UserFrom   UserFrom
	GroupsFrom GroupsFrom
	GroupFrom  GroupFrom
	Store      *db.Store
	Slips      *slips.Service
	Stream     *live.Stream
	Refresh    Refresher
	// Now supplies the app's clock. A nil Now uses time.Now; the sandbox passes
	// its scenario clock so pickers, exports, and freshness agree with refresh.
	Now func() time.Time
}

// now returns the app clock, defaulting to real time.
func (d Dependencies) now() time.Time {
	if d.Now != nil {
		return d.Now()
	}
	return time.Now()
}

// pageRoutes are the shell's pages without per-slip data. The slip screen owns
// "/" and the slip routes because it needs the store.
var pageRoutes = []pageRoute{
	{"GET /record", "record", TabRecord},
}

// New returns the shell's routes with no slip data. It is the small constructor
// the shell tests use; the server calls NewWithDeps.
func New(
	logger zerolog.Logger,
	renderer *Renderer,
	userFrom UserFrom,
	groupsFrom GroupsFrom,
) http.Handler {
	return NewWithDeps(Dependencies{
		Logger:     logger,
		Renderer:   renderer,
		UserFrom:   userFrom,
		GroupsFrom: groupsFrom,
	})
}

// NewWithDeps returns the app's routes: the tab pages, the slip screen, and
// /static. Handlers stay thin; rules live in internal/slips.
func NewWithDeps(deps Dependencies) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /static/{path...}", StaticHandler())

	slipRoutes := &slipHandlers{deps: deps}
	slipRoutes.Routes(mux)
	gameRoutes := &gameHandlers{deps: deps}
	gameRoutes.Routes(mux)
	researchRoutes := &researchHandlers{deps: deps}
	researchRoutes.Routes(mux)

	for _, route := range pageRoutes {
		mux.Handle(route.pattern, pageHandler(deps.Logger, deps.Renderer, route, deps.UserFrom, deps.GroupsFrom))
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
				view := NewUser(user)
				page.User = &view
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

// NewUser maps a stored user to the identity the top bar draws.
func NewUser(user db.User) User {
	return User{
		Name:        user.Name,
		Initials:    Initials(user.Name),
		AvatarClass: user.AvatarColor.Class(),
		PictureURL:  user.PictureURL,
	}
}
