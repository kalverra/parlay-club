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
		// A half-open range keeps the prefix comparison on the key index. A
		// function such as substr would force a scan of the whole archive.
		if upper, ok := prefixUpperBound(filter.KeyPrefix); ok {
			conditions = append(conditions, "key >= ? AND key < ?")
			args = append(args, filter.KeyPrefix, upper)
		} else {
			conditions = append(conditions, "key >= ?")
			args = append(args, filter.KeyPrefix)
		}
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

	rows, err := s.readConn().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list API responses: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var list []APIResponse
	for rows.Next() {
		response, err := scanAPIResponse(rows)
		if err != nil {
			return nil, fmt.Errorf("list API responses: %w", err)
		}
		list = append(list, response)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list API responses: %w", err)
	}
	return list, nil
}

// LatestAPIResponseBefore returns the newest 2xx archived response for key
// fetched at or before before. It reads only the metadata table, never a body.
func (s *Store) LatestAPIResponseBefore(ctx context.Context, key string, before time.Time) (APIResponse, bool, error) {
	row := s.readConn().QueryRowContext(ctx, `
		SELECT id, source, key, method, url, status, fetched_at, duration_ms,
		       body_sha256, credits_remaining, credits_used
		FROM api_responses
		WHERE key = ? AND fetched_at <= ? AND status >= 200 AND status < 300
		ORDER BY fetched_at DESC, id DESC
		LIMIT 1`, key, formatTimestamp(before))

	response, err := scanAPIResponse(row)
	if errors.Is(err, sql.ErrNoRows) {
		return APIResponse{}, false, nil
	}
	if err != nil {
		return APIResponse{}, false, fmt.Errorf("latest API response before: %w", err)
	}
	return response, true, nil
}

func scanAPIResponse(scanner interface{ Scan(dest ...any) error }) (APIResponse, error) {
	var (
		response  APIResponse
		fetchedAt string
		duration  int64
		credits   sql.NullInt64
		used      sql.NullInt64
	)
	if err := scanner.Scan(
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
		return APIResponse{}, err
	}

	var err error
	response.FetchedAt, err = scanTimestamp(fetchedAt)
	if err != nil {
		return APIResponse{}, err
	}
	response.Duration = time.Duration(duration) * time.Millisecond
	response.CreditsRemaining = nullableIntPtr(credits)
	response.CreditsUsed = nullableIntPtr(used)
	return response, nil
}

// prefixUpperBound returns the smallest string that sorts after every string
// starting with prefix. Comparing key against that half-open bound lets SQLite
// use the key index. It reports false when no bound exists, which happens only
// for a prefix made entirely of 0xff bytes.
func prefixUpperBound(prefix string) (string, bool) {
	bytes := []byte(prefix)
	for i := len(bytes) - 1; i >= 0; i-- {
		if bytes[i] < 0xff {
			bytes[i]++
			return string(bytes[:i+1]), true
		}
	}
	return "", false
}

// APIResponseBody returns one archived body, decompressed, by its SHA-256 hex
// hash. A hash with no stored body wraps ErrAPIBodyNotFound.
func (s *Store) APIResponseBody(ctx context.Context, sha string) ([]byte, error) {
	var (
		encoding   string
		size       int
		compressed []byte
	)
	err := s.readConn().QueryRowContext(ctx, `
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
