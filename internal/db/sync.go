package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ErrSyncStateNotFound is returned when a source has never been fetched.
var ErrSyncStateNotFound = errors.New("sync state not found")

// SyncState is one source's refresh bookkeeping.
type SyncState struct {
	Source string
	// FetchedAt is when the source last synced successfully. A zero time means
	// it has never run.
	FetchedAt time.Time
	// OddsCreditsRemaining is the last count Parlay API reported for this
	// source. It is nil for sources that spend no credits.
	OddsCreditsRemaining *int
}

// SyncState returns a source's bookkeeping. A source that has never run wraps
// ErrSyncStateNotFound.
func (s *Store) SyncState(ctx context.Context, source string) (SyncState, error) {
	var (
		state     SyncState
		fetchedAt string
		credits   sql.NullInt64
	)
	err := s.conn().QueryRowContext(ctx, `
		SELECT source, fetched_at, odds_credits_remaining
		FROM sync_state
		WHERE source = ?`, source).
		Scan(&state.Source, &fetchedAt, &credits)
	if errors.Is(err, sql.ErrNoRows) {
		return SyncState{}, fmt.Errorf("sync state %s: %w", source, ErrSyncStateNotFound)
	}
	if err != nil {
		return SyncState{}, fmt.Errorf("sync state %s: %w", source, err)
	}

	parsed, err := scanTimestamp(fetchedAt)
	if err != nil {
		return SyncState{}, fmt.Errorf("sync state %s: %w", source, err)
	}
	state.FetchedAt = parsed
	if credits.Valid {
		remaining := int(credits.Int64)
		state.OddsCreditsRemaining = &remaining
	}
	return state, nil
}

// RecordSync stores a successful fetch time and, for an odds source, the
// remaining credits the response reported. A nil credits pointer clears the
// stored count.
func (s *Store) RecordSync(ctx context.Context, source string, fetchedAt time.Time, credits *int) error {
	_, err := s.conn().ExecContext(ctx, `
		INSERT INTO sync_state (source, fetched_at, odds_credits_remaining)
		VALUES (?, ?, ?)
		ON CONFLICT (source) DO UPDATE SET
			fetched_at = excluded.fetched_at,
			odds_credits_remaining = excluded.odds_credits_remaining`,
		source,
		formatTimestamp(fetchedAt),
		nullableInt(credits),
	)
	if err != nil {
		return fmt.Errorf("record sync %s: %w", source, err)
	}
	return nil
}

// SetSyncCredits updates the stored remaining count for an existing source
// without moving its fetched_at. It is used when an odds call succeeds but the
// full sync then fails, so the balance stays current without pretending the
// source is fresh. A source with no row is left alone.
func (s *Store) SetSyncCredits(ctx context.Context, source string, remaining int) error {
	if _, err := s.conn().ExecContext(ctx, `
		UPDATE sync_state
		SET odds_credits_remaining = ?
		WHERE source = ?`, remaining, source); err != nil {
		return fmt.Errorf("set sync credits %s: %w", source, err)
	}
	return nil
}
