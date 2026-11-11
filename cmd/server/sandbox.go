package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/rs/zerolog"

	"github.com/kalverra/parlay-club/internal/auth"
	"github.com/kalverra/parlay-club/internal/config"
	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/ingest/espn"
	"github.com/kalverra/parlay-club/internal/ingest/nfelo"
	"github.com/kalverra/parlay-club/internal/ingest/odds"
	"github.com/kalverra/parlay-club/internal/ingest/weather"
	"github.com/kalverra/parlay-club/internal/live"
	"github.com/kalverra/parlay-club/internal/refresh"
	"github.com/kalverra/parlay-club/internal/sandbox"
	"github.com/kalverra/parlay-club/internal/slips"
)

// sandboxOddsKey is the fake Parlay API key sandbox clients carry. The replay
// transport ignores credentials, and no real key ever reaches a client.
const sandboxOddsKey = "sandbox-offline-key"

// sandboxRuntime is the offline wiring the server adds only when
// SUNDAY_SANDBOX=1. It owns the replay transport, the shared scenario clock,
// the persisted stage row, and the serialized reset and page-load work.
type sandboxRuntime struct {
	database *sql.DB
	store    *db.Store
	hub      *live.Hub
	// cookies is set by the composition root after construction; reset uses
	// it to reissue the browser's real session and group.
	cookies   *auth.Cookies
	logger    zerolog.Logger
	refresher *refresh.Service
	slips     *slips.Service
	scenario  *sandbox.Scenario
	transport *sandbox.Transport
	client    *http.Client
	state     *sandbox.StateStore

	// mu serializes resets and page-load kicks. A pass that starts before a
	// reset finishes first, so its writes cannot land after the fresh morning.
	mu          sync.Mutex
	kickPending atomic.Bool
	seed        sandbox.Seed
}

// newSandboxRuntime wires every ingest client to the week 9 replay. A fresh
// database builds the morning baseline and seeds its people and slip; a later
// boot restores the saved stage and clock without touching manual edits.
func newSandboxRuntime(
	ctx context.Context,
	database *sql.DB,
	store *db.Store,
	hub *live.Hub,
	fixtures fs.FS,
	logger zerolog.Logger,
) (*sandboxRuntime, error) {
	registry, err := sandbox.Week9(fixtures)
	if err != nil {
		return nil, fmt.Errorf("sandbox fixtures: %w", err)
	}
	transport := sandbox.NewTransport(registry, sandbox.Morning)
	client := &http.Client{Timeout: ingestTimeout, Transport: transport}

	scenario := sandbox.NewScenario(sandbox.Morning, sandbox.MorningTime())
	refresher := refresh.New(refresh.Deps{
		Store:       store,
		Scoreboard:  espn.NewClient("", client),
		Forecast:    weather.NewClient("", client),
		Lines:       odds.NewClient("", sandboxOddsKey, client),
		Projections: nfelo.NewClient("", client),
		Publisher:   hub,
		Now:         scenario.Now,
		Logger:      logger,
	})

	state := sandbox.NewStateStore(database)
	if err := state.Ensure(ctx); err != nil {
		return nil, err
	}

	runtime := &sandboxRuntime{
		database:  database,
		store:     store,
		hub:       hub,
		logger:    logger,
		refresher: refresher,
		slips:     slips.New(store, hub, scenario.Now),
		scenario:  scenario,
		transport: transport,
		client:    client,
		state:     state,
	}

	saved, err := state.Load(ctx)
	switch {
	case errors.Is(err, sandbox.ErrStateNotFound):
		if err := runtime.bootstrap(ctx); err != nil {
			return nil, fmt.Errorf("sandbox morning bootstrap: %w", err)
		}
	case err != nil:
		return nil, err
	default:
		// An Air restart keeps the selected stage, the scenario clock, and every
		// manual edit. It does not rebuild the morning.
		scenario.Set(saved)
		transport.SetStage(saved.Stage)
	}

	return runtime, nil
}

// bootstrap builds the first morning. It writes the stage row only after the
// whole build succeeds, so a failed first boot retries instead of restoring a
// half-built sandbox.
func (r *sandboxRuntime) bootstrap(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	_, err := r.rebuildMorning(ctx, false)
	return err
}

// Reset returns the sandbox to the morning baseline and returns the symbolic
// people and legs the browser can use. One reset at a time, and page-load kicks
// wait their turn behind it.
func (r *sandboxRuntime) Reset(ctx context.Context) (sandbox.Seed, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.rebuildMorning(ctx, true)
}

// Seed returns the people and legs the last build created. It is empty after a
// restart that restored a saved stage without rebuilding.
func (r *sandboxRuntime) Seed() sandbox.Seed {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.seed
}

// rebuildMorning sets the scenario clock, clears every disposable row, runs the
// real ingest synchronously, seeds the people and slip through the normal
// services, and publishes the final signals after every write commits. A reset
// saves the stage first so a failed rebuild still leaves the app on a morning
// clock it can retry from.
func (r *sandboxRuntime) rebuildMorning(
	ctx context.Context,
	persistFirst bool,
) (sandbox.Seed, error) {
	state := sandbox.State{Stage: sandbox.Morning, Now: sandbox.MorningTime()}
	r.scenario.Set(state)
	r.transport.SetStage(state.Stage)
	if persistFirst {
		if err := r.state.Save(ctx, state); err != nil {
			return sandbox.Seed{}, err
		}
	}

	if err := sandbox.Clear(ctx, r.database); err != nil {
		return sandbox.Seed{}, err
	}
	r.refresher.ClearBackoff()
	if err := r.refresher.RunOnce(ctx); err != nil {
		return sandbox.Seed{}, fmt.Errorf("sandbox morning ingest: %w", err)
	}

	week, err := r.store.LatestWeek(ctx)
	if err != nil {
		return sandbox.Seed{}, fmt.Errorf("sandbox morning week: %w", err)
	}
	seed, err := sandbox.SeedScenario(ctx, r.store, r.slips, week)
	if err != nil {
		return sandbox.Seed{}, err
	}

	if err := r.refresher.SyncGameProps(ctx, sandbox.FeaturedGameID); err != nil {
		return sandbox.Seed{}, fmt.Errorf("sandbox featured props: %w", err)
	}

	if !persistFirst {
		if err := r.state.Save(ctx, state); err != nil {
			return sandbox.Seed{}, err
		}
	}

	r.seed = seed
	r.publishSeed(seed)
	return seed, nil
}

// publishSeed tells every listener that the rebuilt week and slip are ready.
// It runs after the reset's writes, so a subscriber only refetches committed
// state.
func (r *sandboxRuntime) publishSeed(seed sandbox.Seed) {
	if r.hub == nil {
		return
	}
	r.hub.Publish(fmt.Sprintf("week:%d", seed.Week.ID))
	r.hub.Publish(fmt.Sprintf("slip:%d", seed.Slip.ID))
}

// Kick starts one serialized background refresh pass for a full-page load. A
// burst of page loads queues at most one pass, and a pass never overlaps a
// reset.
func (r *sandboxRuntime) Kick() {
	if !r.kickPending.CompareAndSwap(false, true) {
		return
	}
	go func() {
		defer r.kickPending.Store(false)

		r.mu.Lock()
		defer r.mu.Unlock()

		if err := r.refresher.RunOnce(context.Background()); err != nil {
			r.logger.Error().Err(err).Msg("sandbox page-load refresh")
		}
	}()
}

// MountRoutes adds the sandbox-only controls. The caller mounts it inside the
// normal signed-in and group-scoped middleware, so CSRF and auth behave exactly
// as they do for the rest of the app.
func (r *sandboxRuntime) MountRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /dev/sandbox/reset", r.reset)
}

// reset rebuilds the morning and then reissues the browser's real session and
// group cookies for the same persona when the reset removed and recreated it.
func (r *sandboxRuntime) reset(w http.ResponseWriter, req *http.Request) {
	user, ok := auth.UserFrom(req.Context())
	if !ok {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	seed, err := r.Reset(req.Context())
	if err != nil {
		r.logger.Error().Err(err).Msg("sandbox reset")
		http.Error(w, "Sandbox reset failed. Check the server log and try again.", http.StatusInternalServerError)
		return
	}

	if person, found := seed.PersonByEmail(user.Email); found {
		r.cookies.SetSession(w, person.User.ID)
		r.cookies.SetGroup(w, person.GroupID)
		w.Header().Set("Hx-Refresh", "true")
		w.WriteHeader(http.StatusOK)
		return
	}

	// The signed-in person was not one of the seeded personas, so reset removed
	// them. Clear the stale cookies and let the browser sign in again.
	r.cookies.ClearSession(w)
	r.cookies.ClearGroup(w)
	w.Header().Set("Hx-Redirect", "/login")
	w.WriteHeader(http.StatusOK)
}

// authSettings picks the sign-in provider and whether the dev form is mounted.
// Sandbox mode forces the dev form and never discovers Google, even when .env
// carries real client values.
func authSettings(ctx context.Context, cfg config.Config) (auth.Provider, bool, error) {
	if cfg.Sandbox {
		return nil, true, nil //nolint:nilnil // no provider is the sandbox case.
	}
	provider, err := newProvider(ctx, cfg)
	if err != nil {
		return nil, false, err
	}
	return provider, !cfg.IsProd() && cfg.GoogleClientID == "", nil
}

// repoRoot finds the checkout that holds the sandbox fixtures. A built binary
// still needs the repository's testdata, which is why the sandbox is a
// development-only mode.
func repoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("sandbox: working directory: %w", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		} else if !errors.Is(err, fs.ErrNotExist) {
			return "", fmt.Errorf("sandbox: read %s: %w", dir, err)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("sandbox: no go.mod above the working directory; run from the repository")
		}
		dir = parent
	}
}
