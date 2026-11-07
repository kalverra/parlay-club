package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrAPIBodyNotFound is returned when no archived body has the requested hash.
var ErrAPIBodyNotFound = errors.New("API body not found")

// APIResponse is one archived response row, without its body. Fetch the body
// with BodySHA256.
type APIResponse struct {
	ID               int64
	Source           string
	Key              string
	Method           string
	URL              string
	Status           int
	FetchedAt        time.Time
	Duration         time.Duration
	BodySHA256       string
	CreditsRemaining *int
	CreditsUsed      *int
}

// APIResponseFilter narrows ListAPIResponses. A zero value keeps every
// response. KeyPrefix matches on the start of the key; Since and Until are
// inclusive bounds on FetchedAt.
type APIResponseFilter struct {
	KeyPrefix string
	Since     time.Time
	Until     time.Time
}

// ListAPIResponses returns archived responses matching filter, oldest first.
// It reads only the metadata table, never a body.
func (s *Store) ListAPIResponses(ctx context.Context, filter APIResponseFilter) ([]APIResponse, error) {
	query := `
		SELECT id, source, key, method, url, status, fetched_at, duration_ms,
		       body_sha256, credits_remaining, credits_used
		FROM api_responses`

	conditions := make([]string, 0, 3)
	args := make([]any, 0, 4)
	if filter.KeyPrefix != "" {
		// substr compares the prefix literally, so '%' and '_' in a key stay
		// literal instead of acting as LIKE wildcards.
		conditions = append(conditions, "substr(key, 1, length(?)) = ?")
		args = append(args, filter.KeyPrefix, filter.KeyPrefix)
	}
	if !filter.Since.IsZero() {
		conditions = append(conditions, "fetched_at >= ?")
		args = append(args, formatTimestamp(filter.Since))
	}
	if !filter.Until.IsZero() {
		conditions = append(conditions, "fetched_at <= ?")
		args = append(args, formatTimestamp(filter.Until))
	}
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY fetched_at, id"

	rows, err := s.conn().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list API responses: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var list []APIResponse
	for rows.Next() {
		var (
			response  APIResponse
			fetchedAt string
			duration  int64
			credits   sql.NullInt64
			used      sql.NullInt64
		)
		if err := rows.Scan(
			&response.ID,
			&response.Source,
			&response.Key,
			&response.Method,
			&response.URL,
			&response.Status,
			&fetchedAt,
			&duration,
			&response.BodySHA256,
			&credits,
			&used,
		); err != nil {
			return nil, fmt.Errorf("scan API response: %w", err)
		}

		response.FetchedAt, err = scanTimestamp(fetchedAt)
		if err != nil {
			return nil, fmt.Errorf("list API responses: %w", err)
		}
		response.Duration = time.Duration(duration) * time.Millisecond
		response.CreditsRemaining = nullableIntPtr(credits)
		response.CreditsUsed = nullableIntPtr(used)
		list = append(list, response)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list API responses: %w", err)
	}
	return list, nil
}

// APIResponseBody returns one archived body, decompressed, by its SHA-256 hex
// hash. A hash with no stored body wraps ErrAPIBodyNotFound.
func (s *Store) APIResponseBody(ctx context.Context, sha string) ([]byte, error) {
	var (
		encoding   string
		size       int
		compressed []byte
	)
	err := s.conn().QueryRowContext(ctx, `
		SELECT encoding, size, body
		FROM api_bodies
		WHERE sha256 = ?`, sha).Scan(&encoding, &size, &compressed)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("API body %s: %w", sha, ErrAPIBodyNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("read API body %s: %w", sha, err)
	}

	body, err := decodeBody(encoding, compressed)
	if err != nil {
		return nil, fmt.Errorf("read API body %s: %w", sha, err)
	}
	if len(body) != size {
		return nil, fmt.Errorf(
			"read API body %s: size is %d, stored size is %d",
			sha, len(body), size,
		)
	}
	return body, nil
}
