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
	"github.com/kalverra/parlay-club/internal/web"
	assets "github.com/kalverra/parlay-club/web"
)

// shutdownTimeout leaves two seconds of Cloud Run's ten before the instance is
// killed, so in-flight requests finish and Litestream can sync.
const shutdownTimeout = 8 * time.Second

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
	logger.Info().Str("env", cfg.AppEnv).Int("port", cfg.Port).Msg("starting")

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

	provider, err := newProvider(ctx, cfg)
	if err != nil {
		return err
	}

	devLogin := !cfg.IsProd() && cfg.GoogleClientID == ""
	handlers := auth.NewHandlers(logger, store, cookies, renderer, provider, devLogin)
	scope := groups.Scope(logger, store, auth.UserFrom, cookies)
	groupHandlers := groups.NewHandlers(logger, store, auth.UserFrom, cookies, renderer)

	server := &http.Server{
		Addr: net.JoinHostPort("", strconv.Itoa(cfg.Port)),
		Handler: newMux(
			web.New(logger, renderer, auth.UserFrom),
			web.StaticHandler(),
			handlers,
			groupHandlers,
			scope,
		),
		ReadHeaderTimeout: 5 * time.Second,
	}
	return listenAndServe(ctx, logger, server)
}

// newProvider builds the Google sign-in provider, or returns nil when Google is
// not configured in dev.
func newProvider(ctx context.Context, cfg config.Config) (auth.Provider, error) {
	if cfg.GoogleClientID == "" {
		return nil, nil //nolint:nilnil // no provider is the dev-only case.
	}

	provider, err := auth.NewGoogleProvider(
		ctx,
		cfg.GoogleClientID,
		cfg.GoogleClientSecret,
		strings.TrimRight(cfg.BaseURL, "/")+"/auth/google/callback",
		"",
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
// plain forms.
func newMux(
	shell, static http.Handler,
	handlers *auth.Handlers,
	groupHandlers *groups.Handlers,
	scope func(http.Handler) http.Handler,
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
	mux.Handle("/", handlers.RequireUser(scope(app)))

	return handlers.CSRF(mux)
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
