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
)

// sandboxOddsKey is the fake Parlay API key sandbox clients carry. The replay
// transport ignores credentials, and no real key ever reaches a client.
const sandboxOddsKey = "sandbox-offline-key"

// sandboxRuntime is the offline wiring the server adds only when
// SUNDAY_SANDBOX=1. It owns the replay transport, the shared scenario clock,
// and the persisted stage row.
type sandboxRuntime struct {
	refresher *refresh.Service
	scenario  *sandbox.Scenario
	transport *sandbox.Transport
	client    *http.Client
	state     *sandbox.StateStore
}

// newSandboxRuntime wires every ingest client to the week 9 replay, runs the
// morning bootstrap on a fresh database, and restores the saved stage and clock
// on later boots. Setup is synchronous so the server does not serve an empty
// database.
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

	saved, err := state.Load(ctx)
	switch {
	case errors.Is(err, sandbox.ErrStateNotFound):
		if err := refresher.RunOnce(ctx); err != nil {
			return nil, fmt.Errorf("sandbox morning bootstrap: %w", err)
		}
		if err := state.Save(ctx, scenario.State()); err != nil {
			return nil, err
		}
	case err != nil:
		return nil, err
	default:
		// An Air restart keeps the selected stage, the scenario clock, and every
		// manual edit. It does not rebuild the morning.
		scenario.Set(saved)
		transport.SetStage(saved.Stage)
	}

	return &sandboxRuntime{
		refresher: refresher,
		scenario:  scenario,
		transport: transport,
		client:    client,
		state:     state,
	}, nil
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
