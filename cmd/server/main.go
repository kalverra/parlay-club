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
	"syscall"
	"time"

	"github.com/rs/zerolog"

	"github.com/kalverra/parlay-club/internal/config"
	"github.com/kalverra/parlay-club/internal/db"
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

	server := &http.Server{
		Addr:              net.JoinHostPort("", strconv.Itoa(cfg.Port)),
		Handler:           newMux(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	return listenAndServe(ctx, logger, server)
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

// newMux builds the routes. Later phases add auth and the web handlers here.
func newMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthz)
	return mux
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
