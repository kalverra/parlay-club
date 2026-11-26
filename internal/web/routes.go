package web

import (
	"context"
	"net/http"
	"time"

	"github.com/rs/zerolog"

	"github.com/kalverra/parlay-club/internal/analyzer"
	"github.com/kalverra/parlay-club/internal/bugs"
	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/eval"
	"github.com/kalverra/parlay-club/internal/live"
	"github.com/kalverra/parlay-club/internal/slips"
)

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

// SetGroup makes a group the browser's active one. The server passes
// cookies.SetGroup; tests record the choice.
type SetGroup func(http.ResponseWriter, int64)

// Refresher describes on-demand sync for game props, research, and pick-time odds.
type Refresher interface {
	SyncGameProps(ctx context.Context, gameID string) error
	SyncGameResearch(ctx context.Context, gameID string) error
	SyncPlayerResearch(ctx context.Context, athleteID string) error
	KickPickLines(ctx context.Context, weekID int64)
	KickPickGame(ctx context.Context, weekID int64, gameID string)
	KickSlipExportEnrich(ctx context.Context, slipID int64)
	ForcePickRefresh(ctx context.Context, weekID int64, gameID string) error
}

// Dependencies are the services the app's routes need. A nil store or slip
// service leaves the slip screen in its placeholder state, which keeps the
// shell's own tests small.
type Dependencies struct {
	Logger        zerolog.Logger
	Renderer      *Renderer
	UserFrom      UserFrom
	GroupsFrom    GroupsFrom
	GroupFrom     GroupFrom
	SetGroup      SetGroup
	Store         *db.Store
	Slips         *slips.Service
	Bugs          *bugs.Service
	Stream        *live.Stream
	Refresh       Refresher
	EvalCache     *eval.Cache
	GradingEngine analyzer.GradingEngine
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
	if deps.EvalCache == nil {
		deps.EvalCache = eval.NewCache()
	}

	mux := http.NewServeMux()
	mux.Handle("GET /static/{path...}", StaticHandler())

	slipRoutes := &slipHandlers{deps: deps}
	slipRoutes.Routes(mux)
	gameRoutes := &gameHandlers{deps: deps}
	gameRoutes.Routes(mux)
	researchRoutes := &researchHandlers{deps: deps}
	researchRoutes.Routes(mux)
	bugRoutes := &bugHandlers{deps: deps}
	bugRoutes.Routes(mux)

	return mux
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
