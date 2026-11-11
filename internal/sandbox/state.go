package sandbox

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrStateNotFound reports that the isolated sandbox database has no stage
// record yet, so this is the first boot.
var ErrStateNotFound = errors.New("sandbox state not found")

// State is the persisted scenario: which stage is active and what time it is.
type State struct {
	Stage Stage
	Now   time.Time
}

// StateStore keeps the sandbox's stage and clock in its isolated SQLite file.
// The table belongs to sandbox setup, not the production migrations, so a
// normal database never gains it.
type StateStore struct {
	db *sql.DB
}

// NewStateStore wraps the sandbox database.
func NewStateStore(database *sql.DB) *StateStore {
	return &StateStore{db: database}
}

// createStateTable makes the one-row stage record. The id check keeps a second
// scenario row out of the file.
const createStateTable = `
CREATE TABLE IF NOT EXISTS sandbox_state (
	id    INTEGER NOT NULL PRIMARY KEY CHECK (id = 1),
	stage TEXT NOT NULL,
	now   TEXT NOT NULL
)`

// Ensure creates the state table when it is missing.
func (s *StateStore) Ensure(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, createStateTable); err != nil {
		return fmt.Errorf("create sandbox state: %w", err)
	}
	return nil
}

// Load reads the saved stage and clock. A missing row reports ErrStateNotFound.
func (s *StateStore) Load(ctx context.Context) (State, error) {
	var (
		stage string
		now   string
	)
	err := s.db.QueryRowContext(ctx, "SELECT stage, now FROM sandbox_state WHERE id = 1").
		Scan(&stage, &now)
	if errors.Is(err, sql.ErrNoRows) {
		return State{}, ErrStateNotFound
	}
	if err != nil {
		return State{}, fmt.Errorf("read sandbox state: %w", err)
	}

	parsed, err := time.Parse(time.RFC3339Nano, now)
	if err != nil {
		return State{}, fmt.Errorf("parse sandbox time %q: %w", now, err)
	}
	if strings.TrimSpace(stage) == "" {
		return State{}, errors.New("sandbox state has no stage")
	}
	return State{Stage: Stage(stage), Now: parsed}, nil
}

// Save writes the current stage and clock. It replaces the one row.
func (s *StateStore) Save(ctx context.Context, state State) error {
	if strings.TrimSpace(string(state.Stage)) == "" {
		return errors.New("sandbox state needs a stage")
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO sandbox_state (id, stage, now) VALUES (1, ?, ?)
		ON CONFLICT (id) DO UPDATE SET stage = excluded.stage, now = excluded.now`,
		string(state.Stage), state.Now.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("write sandbox state: %w", err)
	}
	return nil
}
