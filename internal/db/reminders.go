package db

import (
	"context"
	"fmt"
	"time"
)

// ReminderTask is the slip start a reminder was scheduled for. One row per
// slip keeps a reconcile pass from enqueuing the same task again.
type ReminderTask struct {
	SlipID      int64
	Start       time.Time
	ScheduledAt time.Time
}

// OpenSlips returns every OPEN slip across all groups, oldest first. A custom
// slip carries its eligible game IDs, so the caller can find its first
// kickoff without another read.
func (s *Store) OpenSlips(ctx context.Context) ([]Slip, error) {
	rows, err := s.readConn().QueryContext(ctx,
		"SELECT"+slipColumns+`
		FROM slips
		WHERE status = ?
		ORDER BY id`, string(SlipOpen))
	if err != nil {
		return nil, fmt.Errorf("list open slips: %w", err)
	}
	defer func() { _ = rows.Close() }()

	slips := make([]Slip, 0)
	for rows.Next() {
		slip, err := scanSlip(rows)
		if err != nil {
			return nil, fmt.Errorf("list open slips: %w", err)
		}
		slips = append(slips, slip)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list open slips: %w", err)
	}
	if err := s.hydrateSlipsGameIDs(ctx, slips); err != nil {
		return nil, fmt.Errorf("list open slips: %w", err)
	}
	return slips, nil
}

// ReminderTasks returns the stored reminder start for every slip that has one.
func (s *Store) ReminderTasks(ctx context.Context) (map[int64]ReminderTask, error) {
	rows, err := s.readConn().QueryContext(ctx, `
		SELECT slip_id, start_utc, scheduled_at
		FROM reminder_tasks`)
	if err != nil {
		return nil, fmt.Errorf("list reminder tasks: %w", err)
	}
	defer func() { _ = rows.Close() }()

	tasks := make(map[int64]ReminderTask)
	for rows.Next() {
		var (
			task      ReminderTask
			start     string
			scheduled string
		)
		if err := rows.Scan(&task.SlipID, &start, &scheduled); err != nil {
			return nil, fmt.Errorf("list reminder tasks: %w", err)
		}
		task.Start, err = scanTimestamp(start)
		if err != nil {
			return nil, fmt.Errorf("list reminder tasks: %w", err)
		}
		task.ScheduledAt, err = scanTimestamp(scheduled)
		if err != nil {
			return nil, fmt.Errorf("list reminder tasks: %w", err)
		}
		tasks[task.SlipID] = task
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list reminder tasks: %w", err)
	}
	return tasks, nil
}

// UpsertReminderTask records the start a slip's reminder was scheduled for.
// Writing the same slip again replaces the old start, which is what a moved
// kickoff produces.
func (s *Store) UpsertReminderTask(ctx context.Context, task ReminderTask) error {
	if _, err := s.conn().ExecContext(ctx, `
		INSERT INTO reminder_tasks (slip_id, start_utc, scheduled_at)
		VALUES (?, ?, ?)
		ON CONFLICT(slip_id) DO UPDATE SET
			start_utc    = excluded.start_utc,
			scheduled_at = excluded.scheduled_at,
			updated_at   = strftime('%Y-%m-%dT%H:%M:%SZ','now')`,
		task.SlipID,
		task.Start.UTC().Format(time.RFC3339),
		task.ScheduledAt.UTC().Format(time.RFC3339),
	); err != nil {
		return fmt.Errorf("save reminder task for slip %d: %w", task.SlipID, err)
	}
	return nil
}
