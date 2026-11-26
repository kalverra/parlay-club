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
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"

	"github.com/kalverra/parlay-club/internal/auth"
	"github.com/kalverra/parlay-club/internal/config"
	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/eval"
	"github.com/kalverra/parlay-club/internal/groups"
	"github.com/kalverra/parlay-club/internal/ingest/espn"
	"github.com/kalverra/parlay-club/internal/ingest/nfelo"
	"github.com/kalverra/parlay-club/internal/ingest/nflverse"
	"github.com/kalverra/parlay-club/internal/ingest/odds"
	"github.com/kalverra/parlay-club/internal/ingest/weather"
	"github.com/kalverra/parlay-club/internal/live"
	"github.com/kalverra/parlay-club/internal/refresh"
	"github.com/kalverra/parlay-club/internal/reminders"
	"github.com/kalverra/parlay-club/internal/sandbox"
	"github.com/kalverra/parlay-club/internal/slips"
	webui "github.com/kalverra/parlay-club/internal/web"
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
	cookies *auth.Cookies
	// renderer draws the standalone controls page. The composition root sets it
	// after construction, like cookies.
	renderer *webui.Renderer
	// evalCache is set by the composition root after construction; stage
	// transitions and resets invalidate it.
	evalCache *eval.Cache
	logger    zerolog.Logger
	refresher *refresh.Service
	slips     *slips.Service
	// reminders delivers the sandbox button's reminder. The composition root
	// sets it after construction because it needs the push service.
	reminders *reminders.Service
	scenario  *sandbox.Scenario
	transport *sandbox.Transport
	client    *http.Client
	state     *sandbox.StateStore

	// mu serializes resets and page-load kicks. A pass that starts before a
	// reset finishes first, so its writes cannot land after the fresh morning.
	mu          sync.Mutex
	kickPending atomic.Bool
	seed        sandbox.Seed

	// stopCtx cancels a page-load pass when the server stops. stopMu keeps the
	// "not stopping" check and the goroutine start one step, and kickWg lets
	// Shutdown wait for that pass before the database closes.
	stopCtx    context.Context
	stopCancel context.CancelFunc
	stopMu     sync.Mutex
	kickWg     sync.WaitGroup
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
	slipService := slips.New(store, hub, scenario.Now)
	refresher := refresh.New(refresh.Deps{
		Store:       store,
		Scoreboard:  espn.NewClient("", client),
		Forecast:    weather.NewClient("", client),
		Lines:       odds.NewClient("", sandboxOddsKey, client),
		Projections: nfelo.NewClient("", client),
		NFLverse:    nflverse.NewClient("", client),
		Slips:       slipService,
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
		slips:     slipService,
		scenario:  scenario,
		transport: transport,
		client:    client,
		state:     state,
	}

	stopCtx, stopCancel := context.WithCancel(ctx)
	runtime.stopCtx = stopCtx
	runtime.stopCancel = stopCancel

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
	return r.Stage(ctx, sandbox.Morning)
}

// Stage selects one scripted situation. Every stage rebuilds the morning first,
// so each control works from any previous state, then runs the real refresh and
// the manifest's locking and manual grades. One stage or reset at a time, and
// page-load kicks wait their turn so an older snapshot cannot land after the
// chosen stage.
func (r *sandboxRuntime) Stage(ctx context.Context, stage sandbox.Stage) (sandbox.Seed, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	plan, ok := sandbox.PlanFor(stage)
	if !ok {
		return sandbox.Seed{}, fmt.Errorf("sandbox: unknown stage %q", stage)
	}
	if stage == sandbox.Morning {
		seed, err := r.rebuildMorning(ctx, true)
		if err == nil && r.evalCache != nil {
			r.evalCache.InvalidateAll()
		}
		return seed, err
	}

	seed, err := r.rebuildMorning(ctx, true)
	if err != nil {
		return seed, err
	}

	// Set the clock and replay stage before saving and refreshing, so the
	// scoreboard, weather, lines, pickers, and slip all read the same instant.
	state := sandbox.State{Stage: stage, Now: plan.Now}
	r.scenario.Set(state)
	r.transport.SetStage(stage)
	if err := r.state.Save(ctx, state); err != nil {
		return seed, err
	}
	if err := r.refresher.RunOnce(ctx); err != nil {
		return seed, fmt.Errorf("sandbox %s refresh: %w", stage, err)
	}
	if err := r.applyPlan(ctx, seed, plan); err != nil {
		return seed, err
	}

	r.publishSeed(seed)
	if r.evalCache != nil {
		r.evalCache.InvalidateAll()
	}
	return seed, nil
}

// applyPlan locks the shared slip and records the manifest's scripted manual
// grades through the ordinary slip service. It never writes result columns or
// infers an outcome from a score.
func (r *sandboxRuntime) applyPlan(ctx context.Context, seed sandbox.Seed, plan sandbox.StagePlan) error {
	if plan.Lock {
		for _, person := range seed.People {
			if !person.Persona.Shared {
				continue
			}
			if err := r.slips.LockIn(ctx, person.User.ID, seed.Slip.ID); err != nil {
				return fmt.Errorf("sandbox %s: lock in %s: %w", plan.Stage, person.Persona.ID, err)
			}
		}
	}

	for _, grade := range plan.Grades {
		legID, ok := seed.Legs[grade.Leg]
		if !ok {
			return fmt.Errorf("sandbox %s: no seeded leg %q", plan.Stage, grade.Leg)
		}
		actor, ok := seed.Person(grade.Actor)
		if !ok {
			return fmt.Errorf("sandbox %s: no seeded persona %q", plan.Stage, grade.Actor)
		}
		if _, err := r.slips.SetLegResult(ctx, actor.User.ID, legID, grade.Result); err != nil {
			return fmt.Errorf("sandbox %s: grade %s: %w", plan.Stage, grade.Leg, err)
		}
	}
	return nil
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
	if r.evalCache != nil {
		r.evalCache.InvalidateAll()
	}
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
// reset. Once the server is shutting down, a kick does nothing.
func (r *sandboxRuntime) Kick() {
	r.stopMu.Lock()
	defer r.stopMu.Unlock()

	if r.stopCtx.Err() != nil {
		return
	}
	if !r.kickPending.CompareAndSwap(false, true) {
		return
	}
	r.kickWg.Go(func() {
		defer r.kickPending.Store(false)
		defer r.mu.Unlock()

		r.mu.Lock()
		if err := r.refresher.RunOnce(r.stopCtx); err != nil && r.stopCtx.Err() == nil {
			r.logger.Error().Err(err).Msg("sandbox page-load refresh")
		}
	})
}

// Shutdown cancels a page-load pass in flight and waits for it, then drains the
// refresher. It returns ctx's error when the wait outlives the caller's
// deadline. Calling it twice is safe.
func (r *sandboxRuntime) Shutdown(ctx context.Context) error {
	r.stopMu.Lock()
	r.stopCancel()
	r.stopMu.Unlock()

	return errors.Join(waitForGroup(ctx, &r.kickWg), r.refresher.Shutdown(ctx))
}

// sandboxPersonaView is one switch control on the sandbox page.
type sandboxPersonaView struct {
	ID        sandbox.PersonaID
	Name      string
	Email     string
	Shared    bool
	Current   bool
	GroupName string
}

// sandboxStageView is one situation control on the sandbox page.
type sandboxStageView struct {
	ID          sandbox.Stage
	Name        string
	Description string
	Current     bool
}

// sandboxStageViews names the four scripted situations in baseline-first order.
func sandboxStageViews(current sandbox.Stage) []sandboxStageView {
	return []sandboxStageView{
		{
			ID:          sandbox.Morning,
			Name:        "Sunday morning",
			Description: "Full scheduled week, open slip, every pick pending.",
			Current:     current == sandbox.Morning,
		},
		{
			ID:          sandbox.EarlyLoss,
			Name:        "Early loss",
			Description: "Live Madrid game, locked slip, one scripted losing pick.",
			Current:     current == sandbox.EarlyLoss,
		},
		{
			ID:          sandbox.AllClear,
			Name:        "All clear",
			Description: "Every Sunday game final, every scripted pick won, Monday still upcoming.",
			Current:     current == sandbox.AllClear,
		},
		{
			ID:          sandbox.SundayComplete,
			Name:        "Sunday complete",
			Description: "Every Sunday and Monday game final with scores.",
			Current:     current == sandbox.SundayComplete,
		},
	}
}

// sandboxControlsPage is the standalone sandbox screen. It names the live
// scenario and every seeded person so a tester can move between them without
// editing cookies by hand.
type sandboxControlsPage struct {
	webui.Page
	Stage        string
	WhenISO      string
	WhenLabel    string
	CurrentName  string
	CurrentEmail string
	GroupName    string
	Stages       []sandboxStageView
	Personas     []sandboxPersonaView
	InviteToken  string
}

// MountRoutes adds the sandbox-only controls. The caller mounts it inside the
// normal signed-in and group-scoped middleware, so CSRF and auth behave exactly
// as they do for the rest of the app.
func (r *sandboxRuntime) MountRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /dev/sandbox", r.controls)
	mux.HandleFunc("POST /dev/sandbox/reset", r.reset)
	mux.HandleFunc("POST /dev/sandbox/stage", r.selectStage)
	mux.HandleFunc("POST /dev/sandbox/persona", r.switchPersona)
	mux.HandleFunc("POST /dev/sandbox/reminders", r.sendReminders)
}

// controls renders the current persona, group, stage, scenario time, and the
// switch and reset forms.
func (r *sandboxRuntime) controls(w http.ResponseWriter, req *http.Request) {
	if r.renderer == nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	user, ok := auth.UserFrom(req.Context())
	if !ok {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	state := r.scenario.State()
	view := webui.NewUser(user)
	//nolint:modernize // naming the embedded layout field keeps the data readable.
	page := sandboxControlsPage{
		Page: webui.Page{
			Title:  "Sandbox",
			User:   &view,
			Groups: groups.Options(req.Context()),
			Theme:  webui.ThemeForRequest(req, user.Name).ID,
		},
		Stage:        string(state.Stage),
		WhenISO:      state.Now.UTC().Format(time.RFC3339),
		WhenLabel:    state.Now.UTC().Format("Mon Jan 2, 3:04 PM MST"),
		CurrentName:  user.Name,
		CurrentEmail: user.Email,
		Stages:       sandboxStageViews(state.Stage),
	}
	if group, ok := groups.From(req.Context()); ok {
		page.GroupName = group.Name
	}

	for _, persona := range sandbox.Personas() {
		personaView := sandboxPersonaView{
			ID:     persona.ID,
			Name:   persona.Name,
			Email:  persona.Email,
			Shared: persona.Shared,
		}
		personaView.Current = strings.EqualFold(persona.Email, user.Email)
		if seeded, err := r.store.UserByEmail(req.Context(), persona.Email); err == nil {
			group, groupErr := sandbox.PersonaGroup(req.Context(), r.store, persona, seeded.ID)
			if groupErr == nil {
				personaView.GroupName = group.Name
				if persona.Shared && page.InviteToken == "" {
					page.InviteToken = group.InviteToken
				}
			}
		}
		page.Personas = append(page.Personas, personaView)
	}

	html, err := r.renderer.Render("sandbox", page)
	if err != nil {
		r.logger.Error().Err(err).Msg("render sandbox controls")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(html)
}

// switchPersona signs in as a fixed sandbox person and sets the real session
// and active-group cookies. It does not inject a user or skip group scope, so
// activity, grading, invites, and 404s behave as they do for any signed-in
// person.
func (r *sandboxRuntime) switchPersona(w http.ResponseWriter, req *http.Request) {
	if err := req.ParseForm(); err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	persona, ok := sandbox.PersonaByID(
		sandbox.PersonaID(strings.TrimSpace(req.PostForm.Get("persona"))),
	)
	if !ok {
		http.Error(w, "Unknown sandbox persona.", http.StatusUnprocessableEntity)
		return
	}

	// A reset clears the users between its clear and its seed, so a switch waits
	// for the rebuild rather than racing it.
	r.mu.Lock()
	defer r.mu.Unlock()

	user, err := auth.SignIn(req.Context(), r.store, auth.Identity{
		Sub:   "dev:" + persona.Email,
		Email: persona.Email,
		Name:  persona.Name,
	})
	if err != nil {
		r.logger.Error().Err(err).Str("persona", string(persona.ID)).Msg("sandbox persona sign-in")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	group, err := sandbox.PersonaGroup(req.Context(), r.store, persona, user.ID)
	if err != nil {
		r.logger.Error().Err(err).Str("persona", string(persona.ID)).Msg("sandbox persona group")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	r.cookies.SetSession(w, user.ID)
	r.cookies.SetGroup(w, group.ID)
	w.Header().Set("Hx-Refresh", "true")
	w.WriteHeader(http.StatusOK)
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
	r.finishRebuild(w, user, seed)
}

// selectStage rebuilds the morning and applies one scripted situation. An
// unknown stage changes nothing and answers 422.
func (r *sandboxRuntime) selectStage(w http.ResponseWriter, req *http.Request) {
	user, ok := auth.UserFrom(req.Context())
	if !ok {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}
	if err := req.ParseForm(); err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	stage, ok := sandbox.StageByName(req.PostForm.Get("stage"))
	if !ok {
		http.Error(w, "Unknown sandbox stage.", http.StatusUnprocessableEntity)
		return
	}

	seed, err := r.Stage(req.Context(), stage)
	if err != nil {
		r.logger.Error().Err(err).Str("stage", string(stage)).Msg("sandbox stage")
		http.Error(w, "Sandbox stage failed. Check the server log and try again.", http.StatusInternalServerError)
		return
	}
	r.finishRebuild(w, user, seed)
}

// sendReminders stands in for the scheduler in sandbox. The scenario clock is
// frozen, so a wall-clock timer would never fire; this uses the service clock,
// skips the fire window, and still checks that the slip is OPEN and its start
// has not moved.
func (r *sandboxRuntime) sendReminders(w http.ResponseWriter, req *http.Request) {
	if r.reminders == nil {
		http.Error(w, "reminders are not configured", http.StatusServiceUnavailable)
		return
	}
	if _, ok := auth.UserFrom(req.Context()); !ok {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}
	group, ok := groups.From(req.Context())
	if !ok {
		http.Error(w, "No active group.", http.StatusUnprocessableEntity)
		return
	}
	slip, ok := r.openSlipForGroup(req.Context(), group.ID)
	if !ok {
		http.Error(w, "No open slip in this group to remind.", http.StatusUnprocessableEntity)
		return
	}

	report, err := r.reminders.SendNow(req.Context(), slip.ID)
	if err != nil {
		r.logger.Error().Err(err).Int64("slip", slip.ID).Msg("sandbox reminders")
		http.Error(w, "Reminder send failed. Check the server log and try again.", http.StatusInternalServerError)
		return
	}
	r.logger.Info().
		Int64("slip", slip.ID).
		Int("sent", report.Sent).
		Str("skipped", report.Skipped).
		Msg("sandbox reminders")
	w.Header().Set("Hx-Refresh", "true")
	w.WriteHeader(http.StatusOK)
}

// openSlipForGroup returns the group's newest OPEN slip, if it has one.
func (r *sandboxRuntime) openSlipForGroup(ctx context.Context, groupID int64) (db.Slip, bool) {
	open, err := r.store.OpenSlips(ctx)
	if err != nil {
		r.logger.Error().Err(err).Msg("sandbox reminders: list open slips")
		return db.Slip{}, false
	}
	for _, o := range slices.Backward(open) {
		if o.GroupID == groupID {
			return o, true
		}
	}
	return db.Slip{}, false
}

// finishRebuild reissues the acting browser's session and group after a rebuild
// that removed and recreated the seeded people. A signed-in person who was not
// part of the seed is cleared and sent back to sign-in.
func (r *sandboxRuntime) finishRebuild(w http.ResponseWriter, user db.User, seed sandbox.Seed) {
	if person, found := seed.PersonByEmail(user.Email); found {
		r.cookies.SetSession(w, person.User.ID)
		r.cookies.SetGroup(w, person.GroupID)
		w.Header().Set("Hx-Refresh", "true")
		w.WriteHeader(http.StatusOK)
		return
	}

	// The signed-in person was not one of the seeded personas, so the rebuild
	// removed them. Clear the stale cookies and let the browser sign in again.
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
