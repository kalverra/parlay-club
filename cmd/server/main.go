// Command server runs the Sunday Parlay Club web app.
package main

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/rs/zerolog"

	"github.com/kalverra/parlay-club/internal/auth"
	"github.com/kalverra/parlay-club/internal/bugs"
	"github.com/kalverra/parlay-club/internal/config"
	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/eval"
	"github.com/kalverra/parlay-club/internal/groups"
	"github.com/kalverra/parlay-club/internal/ingest/archive"
	"github.com/kalverra/parlay-club/internal/ingest/espn"
	"github.com/kalverra/parlay-club/internal/ingest/nfelo"
	"github.com/kalverra/parlay-club/internal/ingest/nflverse"
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

// healthzPingTimeout bounds a probe's database check, so a busy SQLite cannot
// hold Cloud Run's probe open.
const healthzPingTimeout = 2 * time.Second

// prefillTimeout caps one synchronous POST /internal/prefill sweep under Cloud
// Scheduler's 3-minute default deadline.
const prefillTimeout = 2 * time.Minute

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

	store := db.NewStore(database).WithLogger(logger)
	defer func() {
		if err := store.Close(); err != nil {
			logger.Error().Err(err).Msg("close store read pool")
		}
	}()
	cookies := auth.NewCookies(cfg.SessionKey, cfg.IsProd())
	hub := &live.Hub{}
	evalCache := eval.NewCache()

	provider, devLogin, err := authSettings(ctx, cfg)
	if err != nil {
		return err
	}

	var (
		refresher   *refresh.Service
		slipService *slips.Service
		appNow      func() time.Time
		kick        func()
		drain       func(context.Context) error
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
		runtime.evalCache = evalCache
		refresher = runtime.refresher
		slipService = runtime.slips
		slipService.SetOnGradeChange(evalCache.InvalidateGroup)
		refresher.SetOnMarketDataChange(evalCache.InvalidateMarket)
		appNow = runtime.scenario.Now
		kick = runtime.Kick
		drain = runtime.Shutdown
		sandboxApp = runtime.MountRoutes
	} else {
		loadHistory(ctx, logger, store)
		refresher, slipService = newProdServices(cfg, store, hub, evalCache, newIngestClient(store), logger)
		kick = refresher.Kick
		drain = refresher.Shutdown
	}

	handlers := auth.NewHandlers(logger, store, cookies, renderer, provider, devLogin)
	scope := groups.Scope(logger, store, auth.UserFrom, cookies)
	groupHandlers := groups.NewHandlers(logger, store, auth.UserFrom, cookies, renderer, cfg.BaseURL)

	stream := live.NewStream(hub, refresher.LiveWindow)
	bugService := bugs.NewService(bugs.Deps{
		Store:       store,
		Repo:        cfg.GitHubRepo,
		BaseURL:     cfg.BaseURL,
		GitHubToken: cfg.GitHubToken,
		Logger:      logger,
	})
	shell := web.NewWithDeps(web.Dependencies{
		Logger:     logger,
		Renderer:   renderer,
		UserFrom:   auth.UserFrom,
		GroupsFrom: groups.Options,
		GroupFrom:  groups.From,
		SetGroup:   cookies.SetGroup,
		Store:      store,
		Slips:      slipService,
		Bugs:       bugService,
		Stream:     stream,
		Refresh:    refresher,
		EvalCache:  evalCache,
		Now:        appNow,
	})

	server := &http.Server{
		Addr: net.JoinHostPort("", strconv.Itoa(cfg.Port)),
		Handler: withPprof(
			withRequestTiming(logger, !cfg.IsProd())(newMux(
				shell,
				web.StaticHandler(),
				web.ScreenshotHandler(store, logger),
				newHealthzHandler(database, logger),
				newPrefillHandler(ctx, cfg.PrefillToken, cfg.Sandbox, refresher, logger),
				handlers,
				groupHandlers,
				scope,
				kick,
				sandboxApp,
			)),
			cfg.IsProd(),
		),
		ReadHeaderTimeout: 5 * time.Second,
	}
	return listenAndServe(ctx, logger, server, shutdownHooks{
		beforeHTTP: stream.Shutdown,
		afterHTTP:  drain,
	})
}

// loadHistory imports seed/history.yaml after migrations. A bad file logs and
// lets the app serve: history is nice to have, and the rest of the app must
// still come up. With no group yet there is nowhere to put the slips, so it
// waits for a later boot.
func loadHistory(ctx context.Context, logger zerolog.Logger, store *db.Store) {
	group, err := seed.TargetGroup(ctx, store, seed.DefaultPath)
	if errors.Is(err, db.ErrGroupNotFound) {
		logger.Info().Msg("history: target group not found yet, skipping " + seed.DefaultPath)
		return
	}
	if err != nil {
		logger.Error().Err(err).Msg("history: resolve target group")
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

// newProdServices constructs the shared slip service before the refresh service
// so auto-grading and HTTP handlers share one instance and invalidate evalCache.
func newProdServices(
	cfg config.Config,
	store *db.Store,
	hub *live.Hub,
	evalCache *eval.Cache,
	ingestClient *http.Client,
	logger zerolog.Logger,
) (*refresh.Service, *slips.Service) {
	slipService := slips.New(store, hub, nil)
	if evalCache != nil {
		slipService.SetOnGradeChange(evalCache.InvalidateGroup)
	}
	refresher := refresh.New(refresh.Deps{
		Store:         store,
		Scoreboard:    espn.NewClient("", ingestClient),
		Forecast:      weather.NewClient("", ingestClient),
		Lines:         odds.NewClient("", cfg.OddsAPIKey, ingestClient),
		FallbackLines: odds.NewClient(odds.TheOddsAPIBaseURL, cfg.TheOddsAPIKey, ingestClient),
		Projections:   nfelo.NewClient("", ingestClient),
		NFLverse:      nflverse.NewClient("", ingestClient),
		Slips:         slipService,
		Publisher:     hub,
		Logger:        logger,
	})
	if evalCache != nil {
		refresher.SetOnMarketDataChange(evalCache.InvalidateMarket)
	}
	return refresher, slipService
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
// outside the shell's checks. POST /internal/prefill sits outside cookie/CSRF
// middleware and uses bearer auth instead. Everything else needs a signed-in
// user and an active group, and every non-GET form must come from HTMX or be
// one of the plain forms. mountExtra adds dev-only routes behind those same
// checks; it may be nil outside sandbox mode.
func newMux(
	shell, static, screenshots, healthz, prefill http.Handler,
	handlers *auth.Handlers,
	groupHandlers *groups.Handlers,
	scope func(http.Handler) http.Handler,
	kick func(),
	mountExtra func(*http.ServeMux),
) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /healthz", healthz)
	mux.Handle("GET /readyz", healthz)
	mux.Handle("GET /static/{path...}", static)
	mux.Handle("GET /manifest.webmanifest", web.ManifestHandler())
	mux.Handle("GET /sw.js", web.ServiceWorkerHandler())
	mux.Handle("GET /screenshots/{file}", screenshots)
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

	csrfProtected := handlers.CSRF(mux)
	if prefill == nil {
		return csrfProtected
	}
	outer := http.NewServeMux()
	outer.Handle("POST /internal/prefill", prefill)
	outer.Handle("/", csrfProtected)
	return outer
}

type prefillRunner interface {
	RunPrefill(ctx context.Context) (refresh.PrefillReport, error)
}

// newPrefillHandler serves POST /internal/prefill synchronously with fail-closed
// constant-time bearer auth.
func newPrefillHandler(
	stopCtx context.Context,
	token string,
	sandbox bool,
	runner prefillRunner,
	logger zerolog.Logger,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token == "" {
			http.NotFound(w, r)
			return
		}
		if !validBearerToken(r.Header.Get("Authorization"), token) {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}
		if sandbox || runner == nil {
			http.Error(w, "prefill is disabled in sandbox mode", http.StatusServiceUnavailable)
			return
		}

		prefillCtx, cancel := context.WithTimeout(r.Context(), prefillTimeout)
		defer cancel()
		if stopCtx != nil {
			unregister := context.AfterFunc(stopCtx, cancel)
			defer unregister()
		}

		report, err := runner.RunPrefill(prefillCtx)
		if err != nil {
			logger.Error().Err(err).Msg("prefill: run")
			writeJSON(w, http.StatusInternalServerError, map[string]any{
				"error":  err.Error(),
				"report": report,
			})
			return
		}
		writeJSON(w, http.StatusOK, report)
	})
}

func validBearerToken(header, want string) bool {
	if want == "" {
		return false
	}
	const prefix = "Bearer "
	if len(header) <= len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return false
	}
	got := header[len(prefix):]
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
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

// newHealthzHandler answers Cloud Run's startup and liveness probes. It pings
// SQLite, so a revision whose database is closed or unreachable is replaced
// instead of serving errors. The body is a fixed word: the SQLite error goes to
// the log, never to the probe.
func newHealthzHandler(database *sql.DB, logger zerolog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), healthzPingTimeout)
		defer cancel()

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		if err := database.PingContext(ctx); err != nil {
			logger.Error().Err(err).Msg("healthz: ping database")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("unavailable"))
			return
		}
		_, _ = w.Write([]byte("ok"))
	})
}

// shutdownHooks are the drains serve runs around http.Server.Shutdown. Both use
// the same shutdown context, so the whole stop fits Cloud Run's kill window.
type shutdownHooks struct {
	// beforeHTTP ends handlers that would otherwise hold the drain open, such
	// as open SSE streams. It may be nil.
	beforeHTTP func()
	// afterHTTP waits for background work that writes to SQLite, so it cannot
	// outlive the database. It may be nil.
	afterHTTP func(context.Context) error
}

// listenAndServe serves server.Addr until ctx is cancelled. The hooks drain the
// server's long-lived handlers and its background work; either may be nil.
func listenAndServe(
	ctx context.Context,
	logger zerolog.Logger,
	server *http.Server,
	hooks shutdownHooks,
) error {
	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", server.Addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", server.Addr, err)
	}
	return serve(ctx, logger, server, listener, hooks)
}

// serve serves until ctx is cancelled, then drains for up to shutdownTimeout.
// The hooks end long-lived handlers and background writers in that order.
func serve(
	ctx context.Context,
	logger zerolog.Logger,
	server *http.Server,
	listener net.Listener,
	hooks shutdownHooks,
) error {
	logger.Info().Str("addr", listener.Addr().String()).Msg("listening")

	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
	}

	logger.Info().Msg("shutting down")
	if hooks.beforeHTTP != nil {
		hooks.beforeHTTP()
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	shutdownErr := server.Shutdown(shutdownCtx)
	if shutdownErr != nil {
		shutdownErr = fmt.Errorf("shut down: %w", shutdownErr)
	}
	// The background drain runs even when the HTTP drain expired: a sync still
	// writing is the worst thing to leave open while the database closes.
	if hooks.afterHTTP != nil {
		if err := hooks.afterHTTP(shutdownCtx); err != nil {
			return errors.Join(shutdownErr, fmt.Errorf("drain refresh: %w", err))
		}
	}
	if shutdownErr != nil {
		return shutdownErr
	}
	if err := <-done; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// waitForGroup blocks until wg is done or ctx is cancelled. An expired ctx
// leaves one goroutine behind until the work it waits on finishes.
func waitForGroup(ctx context.Context, wg *sync.WaitGroup) error {
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

const slowRequestThreshold = 250 * time.Millisecond

func durationMS(d time.Duration) float64 {
	return float64(d.Microseconds()) / 1000.0
}

// withRequestTiming logs HTTP request durations with per-phase breakdowns
// (db_ms, board_ms, render_ms) and emits Server-Timing headers when
// emitServerTiming is true. Long-lived SSE streams are skipped.
func withRequestTiming(logger zerolog.Logger, emitServerTiming bool) func(http.Handler) http.Handler {
	return withRequestTimingThreshold(logger, emitServerTiming, slowRequestThreshold)
}

func withRequestTimingThreshold(
	logger zerolog.Logger,
	emitServerTiming bool,
	slowThreshold time.Duration,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isSSEStream(r) {
				next.ServeHTTP(w, r)
				return
			}
			ctx, timings := web.WithPhaseTimings(r.Context())
			start := time.Now()
			rw := &timingResponseWriter{
				ResponseWriter:   w,
				status:           http.StatusOK,
				start:            start,
				timings:          timings,
				emitServerTiming: emitServerTiming,
			}
			next.ServeHTTP(rw, r.WithContext(ctx))

			duration := time.Since(start)
			level := zerolog.DebugLevel
			if duration >= slowThreshold {
				level = zerolog.WarnLevel
			}
			logger.WithLevel(level).
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Int("status", rw.status).
				Float64("duration_ms", durationMS(duration)).
				Float64("db_ms", durationMS(timings.DB())).
				Float64("board_ms", durationMS(timings.Board())).
				Float64("render_ms", durationMS(timings.Render())).
				Msg("http request")
		})
	}
}

type timingResponseWriter struct {
	http.ResponseWriter
	status           int
	wroteHeader      bool
	start            time.Time
	timings          *web.PhaseTimings
	emitServerTiming bool
}

func (w *timingResponseWriter) WriteHeader(statusCode int) {
	if w.wroteHeader {
		return
	}
	w.status = statusCode
	w.wroteHeader = true
	if w.emitServerTiming {
		w.Header().Set("Server-Timing", formatServerTiming(w.timings, time.Since(w.start)))
	}
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *timingResponseWriter) Write(p []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(p)
}

func (w *timingResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func formatServerTiming(timings *web.PhaseTimings, total time.Duration) string {
	return fmt.Sprintf(
		"db;dur=%.2f, board;dur=%.2f, render;dur=%.2f, total;dur=%.2f",
		durationMS(timings.DB()),
		durationMS(timings.Board()),
		durationMS(timings.Render()),
		durationMS(total),
	)
}

func isSSEStream(r *http.Request) bool {
	if strings.Contains(r.Header.Get("Accept"), "text/event-stream") {
		return true
	}
	return strings.HasSuffix(r.URL.Path, "/events")
}

// withPprof exposes net/http/pprof under /debug/pprof/ in non-prod
// environments and returns 404 in prod.
func withPprof(next http.Handler, isProd bool) http.Handler {
	mux := http.NewServeMux()
	if !isProd {
		mux.HandleFunc("GET /debug/pprof/", pprof.Index)
		mux.HandleFunc("GET /debug/pprof/cmdline", pprof.Cmdline)
		mux.HandleFunc("GET /debug/pprof/profile", pprof.Profile)
		mux.HandleFunc("GET /debug/pprof/symbol", pprof.Symbol)
		mux.HandleFunc("GET /debug/pprof/trace", pprof.Trace)
	} else {
		mux.Handle("/debug/pprof/", http.NotFoundHandler())
	}
	mux.Handle("/", next)
	return mux
}
