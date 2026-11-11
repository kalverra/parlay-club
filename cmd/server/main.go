// Command server runs the Sunday Parlay Club web app.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/rs/zerolog"

	"github.com/kalverra/parlay-club/internal/auth"
	"github.com/kalverra/parlay-club/internal/config"
	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/groups"
	"github.com/kalverra/parlay-club/internal/ingest/archive"
	"github.com/kalverra/parlay-club/internal/ingest/espn"
	"github.com/kalverra/parlay-club/internal/ingest/nfelo"
	"github.com/kalverra/parlay-club/internal/ingest/odds"
	"github.com/kalverra/parlay-club/internal/ingest/weather"
	"github.com/kalverra/parlay-club/internal/live"
	"github.com/kalverra/parlay-club/internal/refresh"
	"github.com/kalverra/parlay-club/internal/seed"
	"github.com/kalverra/parlay-club/internal/slips"
	"github.com/kalverra/parlay-club/internal/web"
	assets "github.com/kalverra/parlay-club/web"
)

// shutdownTimeout leaves two seconds of Cloud Run's ten before the instance is
// killed, so in-flight requests finish and Litestream can sync.
const shutdownTimeout = 8 * time.Second

// ingestTimeout bounds one outside API call. It matches the ingest clients'
// own defaults, which the shared client replaces.
const ingestTimeout = 10 * time.Second

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "parlay-club: %v\n", err)
		os.Exit(1)
	}
}

// run wires config, logging, the database, and the HTTP server, then serves
// until SIGINT or SIGTERM.
func run() error {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return err
	}

	logger := newLogger(cfg, os.Stdout)
	logger.Info().
		Str("env", cfg.AppEnv).
		Int("port", cfg.Port).
		Bool("sandbox", cfg.Sandbox).
		Msg("starting")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	database, err := db.Open(cfg.DBPath)
	if err != nil {
		return err
	}
	defer func() {
		if err := database.Close(); err != nil {
			logger.Error().Err(err).Msg("close database")
		}
	}()

	if err := db.Migrate(ctx, database); err != nil {
		return err
	}

	renderer, err := newRenderer(cfg)
	if err != nil {
		return err
	}

	store := db.NewStore(database)
	cookies := auth.NewCookies(cfg.SessionKey, cfg.IsProd())
	hub := &live.Hub{}

	provider, devLogin, err := authSettings(ctx, cfg)
	if err != nil {
		return err
	}

	var (
		refresher   *refresh.Service
		slipService *slips.Service
		appNow      func() time.Time
		kick        func()
		sandboxApp  func(*http.ServeMux)
	)
	if cfg.Sandbox {
		root, err := repoRoot()
		if err != nil {
			return err
		}
		runtime, err := newSandboxRuntime(ctx, database, store, hub, os.DirFS(root), logger)
		if err != nil {
			return err
		}
		runtime.cookies = cookies
		runtime.renderer = renderer
		refresher = runtime.refresher
		slipService = runtime.slips
		appNow = runtime.scenario.Now
		kick = runtime.Kick
		sandboxApp = runtime.MountRoutes
	} else {
		loadHistory(ctx, logger, store)
		ingestClient := newIngestClient(store)
		refresher = refresh.New(refresh.Deps{
			Store:         store,
			Scoreboard:    espn.NewClient("", ingestClient),
			Forecast:      weather.NewClient("", ingestClient),
			Lines:         odds.NewClient("", cfg.OddsAPIKey, ingestClient),
			FallbackLines: odds.NewClient(odds.TheOddsAPIBaseURL, cfg.TheOddsAPIKey, ingestClient),
			Projections:   nfelo.NewClient("", ingestClient),
			Publisher:     hub,
			Logger:        logger,
		})
		slipService = slips.New(store, hub, nil)
		kick = refresher.Kick
	}

	handlers := auth.NewHandlers(logger, store, cookies, renderer, provider, devLogin)
	scope := groups.Scope(logger, store, auth.UserFrom, cookies)
	groupHandlers := groups.NewHandlers(logger, store, auth.UserFrom, cookies, renderer)

	stream := live.NewStream(hub, refresher.LiveWindow)
	shell := web.NewWithDeps(web.Dependencies{
		Logger:     logger,
		Renderer:   renderer,
		UserFrom:   auth.UserFrom,
		GroupsFrom: groups.Options,
		GroupFrom:  groups.From,
		SetGroup:   cookies.SetGroup,
		Store:      store,
		Slips:      slipService,
		Stream:     stream,
		Refresh:    refresher,
		Now:        appNow,
	})

	server := &http.Server{
		Addr: net.JoinHostPort("", strconv.Itoa(cfg.Port)),
		Handler: newMux(
			shell,
			web.StaticHandler(),
			handlers,
			groupHandlers,
			scope,
			kick,
			sandboxApp,
		),
		ReadHeaderTimeout: 5 * time.Second,
	}
	return listenAndServe(ctx, logger, server)
}

// loadHistory imports seed/history.yaml after migrations. A bad file logs and
// lets the app serve: history is nice to have, and the rest of the app must
// still come up. With no group yet there is nowhere to put the slips, so it
// waits for a later boot.
func loadHistory(ctx context.Context, logger zerolog.Logger, store *db.Store) {
	group, err := store.FirstGroup(ctx)
	if errors.Is(err, db.ErrGroupNotFound) {
		logger.Info().Msg("history: no group yet, skipping " + seed.DefaultPath)
		return
	}
	if err != nil {
		logger.Error().Err(err).Msg("history: read first group")
		return
	}

	report, err := seed.Load(ctx, store, group.ID, seed.DefaultPath)
	if err != nil {
		logger.Error().Err(err).Str("path", seed.DefaultPath).Msg("history: load")
		return
	}
	if report.Slips == 0 && report.Users == 0 {
		return
	}
	logger.Info().
		Int64("group", group.ID).
		Int("slips", report.Slips).
		Int("skipped", report.Skipped).
		Int("users", report.Users).
		Msg("history loaded")
}

// newIngestClient builds the HTTP client every ingest source shares. Its
// transport archives each successful response in the app's database, so the
// clients stay database-free and cannot forget to record a call. The auth
// package keeps its own client and never passes through this one.
func newIngestClient(store *db.Store) *http.Client {
	return &http.Client{
		Timeout:   ingestTimeout,
		Transport: archive.NewTransport(http.DefaultTransport, store, nil),
	}
}

// newProvider builds the Google sign-in provider, or returns nil when Google is
// not configured in dev.
func newProvider(ctx context.Context, cfg config.Config) (auth.Provider, error) {
	return newProviderForIssuer(ctx, cfg, "")
}

// newProviderForIssuer is newProvider with a testable issuer. An empty issuer
// means Google.
func newProviderForIssuer(
	ctx context.Context,
	cfg config.Config,
	issuer string,
) (auth.Provider, error) {
	if cfg.GoogleClientID == "" {
		return nil, nil //nolint:nilnil // no provider is the dev-only case.
	}

	provider, err := auth.NewGoogleProvider(
		ctx,
		cfg.GoogleClientID,
		cfg.GoogleClientSecret,
		strings.TrimRight(cfg.BaseURL, "/")+"/auth/google/callback",
		issuer,
	)
	if err != nil {
		return nil, fmt.Errorf("google sign-in: %w", err)
	}

	return provider, nil
}

// newRenderer reads templates from disk on every request in dev, so an edit
// shows up on reload, and from the binary in prod.
func newRenderer(cfg config.Config) (*web.Renderer, error) {
	if cfg.IsProd() {
		renderer, err := web.NewRenderer(assets.Templates, false)
		if err != nil {
			return nil, fmt.Errorf("load templates: %w", err)
		}

		return renderer, nil
	}

	renderer, err := web.NewRenderer(os.DirFS("web"), true)
	if err != nil {
		return nil, fmt.Errorf("load templates from web/ (run from the repo root): %w", err)
	}

	return renderer, nil
}

// newLogger writes JSON in prod for Cloud Run's log parser and a readable
// console format in dev.
func newLogger(cfg config.Config, out io.Writer) zerolog.Logger {
	writer := out
	if !cfg.IsProd() {
		writer = zerolog.ConsoleWriter{Out: out, TimeFormat: time.TimeOnly}
	}
	return zerolog.New(writer).Level(cfg.LogLevel).With().Timestamp().Logger()
}

// newMux builds the routes. Health, static files, and the sign-in screens sit
// outside the shell's checks. Everything else needs a signed-in user and an
// active group, and every non-GET form must come from HTMX or be one of the
// plain forms. mountExtra adds dev-only routes behind those same checks; it may
// be nil outside sandbox mode.
func newMux(
	shell, static http.Handler,
	handlers *auth.Handlers,
	groupHandlers *groups.Handlers,
	scope func(http.Handler) http.Handler,
	kick func(),
	mountExtra func(*http.ServeMux),
) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthz)
	mux.Handle("GET /static/{path...}", static)
	handlers.Routes(mux)

	app := http.NewServeMux()
	app.Handle("/", shell)
	if groupHandlers != nil {
		groupHandlers.Routes(app)
	}
	if mountExtra != nil {
		mountExtra(app)
	}
	mux.Handle("/", handlers.RequireUser(scope(kickOnPage(kick)(app))))

	return handlers.CSRF(mux)
}

// kickOnPage starts a refresh pass for full-page GETs. HTMX partials and SSE
// streams skip it: the page load that mounted them already kicked, and an open
// stream must not trigger a sync on every reconnect.
func kickOnPage(kick func()) func(http.Handler) http.Handler {
	if kick == nil {
		return func(next http.Handler) http.Handler { return next }
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isFullPageGet(r) {
				kick()
			}
			next.ServeHTTP(w, r)
		})
	}
}

// isFullPageGet reports whether a request is a browser page load rather than an
// HTMX partial or an EventSource stream.
func isFullPageGet(r *http.Request) bool {
	if r.Method != http.MethodGet {
		return false
	}
	if r.Header.Get("Hx-Request") == "true" {
		return false
	}
	if strings.Contains(r.Header.Get("Accept"), "text/event-stream") {
		return false
	}
	return !strings.HasSuffix(r.URL.Path, "/events")
}

// healthz answers Cloud Run's startup and liveness probes.
func healthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("ok"))
}

// listenAndServe serves server.Addr until ctx is cancelled.
func listenAndServe(ctx context.Context, logger zerolog.Logger, server *http.Server) error {
	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", server.Addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", server.Addr, err)
	}
	return serve(ctx, logger, server, listener)
}

// serve serves until ctx is cancelled, then drains for up to shutdownTimeout.
func serve(ctx context.Context, logger zerolog.Logger, server *http.Server, listener net.Listener) error {
	logger.Info().Str("addr", listener.Addr().String()).Msg("listening")

	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
	}

	logger.Info().Msg("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shut down: %w", err)
	}
	if err := <-done; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
