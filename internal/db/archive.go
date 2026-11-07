package db

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"

	"github.com/kalverra/parlay-club/internal/ingest/archive"
)

// SaveAPIResponse stores one archived response. The body is hashed and
// gzipped, then inserted only when that hash is new; the response row always
// goes in. Both inserts share one transaction, so a body is never recorded
// without the response that points at it. It satisfies archive.Sink.
func (s *Store) SaveAPIResponse(ctx context.Context, record archive.Record) error {
	sum := sha256.Sum256(record.Body)
	sha := hex.EncodeToString(sum[:])

	compressed, err := gzipBody(record.Body)
	if err != nil {
		return fmt.Errorf("save API response %s: %w", record.Key, err)
	}

	err = s.Tx(ctx, func(tx *Store) error {
		if _, err := tx.conn().ExecContext(ctx, `
			INSERT OR IGNORE INTO api_bodies (sha256, encoding, size, body)
			VALUES (?, 'gzip', ?, ?)`,
			sha,
			len(record.Body),
			compressed,
		); err != nil {
			return fmt.Errorf("insert body: %w", err)
		}

		if _, err := tx.conn().ExecContext(ctx, `
			INSERT INTO api_responses (
				source, key, method, url, status, fetched_at, duration_ms,
				body_sha256, credits_remaining, credits_used
			)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			record.Source,
			record.Key,
			record.Method,
			record.URL,
			record.Status,
			formatTimestamp(record.FetchedAt),
			record.Duration.Milliseconds(),
			sha,
			nullableInt(record.CreditsRemaining),
			nullableInt(record.CreditsUsed),
		); err != nil {
			return fmt.Errorf("insert response: %w", err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("save API response %s: %w", record.Key, err)
	}
	return nil
}

// gzipBody compresses body for storage. JSON compresses roughly tenfold, and
// the archive keeps the uncompressed size alongside the compressed bytes.
func gzipBody(body []byte) ([]byte, error) {
	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)
	if _, err := writer.Write(body); err != nil {
		return nil, fmt.Errorf("gzip body: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("gzip body: %w", err)
	}
	return buf.Bytes(), nil
}

// decodeBody reverses the storage encoding named in api_bodies.encoding. Only
// gzip exists today, but the column lets a later change add another one.
func decodeBody(encoding string, compressed []byte) ([]byte, error) {
	if encoding != "gzip" {
		return nil, fmt.Errorf("unknown body encoding %q", encoding)
	}

	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return nil, fmt.Errorf("open gzip body: %w", err)
	}
	defer func() { _ = reader.Close() }()

	body, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("read gzip body: %w", err)
	}
	return body, nil
}
