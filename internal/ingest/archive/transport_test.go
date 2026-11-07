package archive_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/ingest/archive"
)

// testClock is the fixed time every test records.
var testClock = time.Date(2026, time.November, 7, 13, 0, 0, 0, time.UTC)

// openArchiveStore opens a migrated database and wraps it in a Store.
func openArchiveStore(t *testing.T) (*db.Store, *sql.DB) {
	t.Helper()

	database, err := db.Open(filepath.Join(t.TempDir(), "parlay.db"))
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, database.Close()) })
	require.NoError(t, db.Migrate(t.Context(), database))

	return db.NewStore(database), database
}

// archivedClient builds an HTTP client whose transport archives every call.
func archivedClient(sink archive.Sink, now func() time.Time) *http.Client {
	return &http.Client{Transport: archive.NewTransport(http.DefaultTransport, sink, now)}
}

// get reads one full response and returns its status and body as text.
func get(ctx context.Context, t *testing.T, client *http.Client, target string) (int, string) {
	t.Helper()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	require.NoError(t, err)
	resp, err := client.Do(req)
	require.NoError(t, err)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	return resp.StatusCode, string(body)
}

// responseRow is one api_responses row.
type responseRow struct {
	Source           string
	Key              string
	Method           string
	URL              string
	Status           int
	FetchedAt        string
	DurationMS       int64
	BodySHA256       string
	CreditsRemaining sql.NullInt64
	CreditsUsed      sql.NullInt64
}

// responses lists api_responses in insertion order.
func responses(t *testing.T, database *sql.DB) []responseRow {
	t.Helper()

	rows, err := database.QueryContext(t.Context(), `
		SELECT source, key, method, url, status, fetched_at, duration_ms,
		       body_sha256, credits_remaining, credits_used
		FROM api_responses
		ORDER BY id`)
	require.NoError(t, err)
	defer func() { assert.NoError(t, rows.Close()) }()

	var list []responseRow
	for rows.Next() {
		var row responseRow
		require.NoError(t, rows.Scan(
			&row.Source, &row.Key, &row.Method, &row.URL, &row.Status,
			&row.FetchedAt, &row.DurationMS, &row.BodySHA256,
			&row.CreditsRemaining, &row.CreditsUsed,
		))
		list = append(list, row)
	}
	require.NoError(t, rows.Err())
	return list
}

// storedBody returns one api_bodies row and its compression metadata.
func storedBody(t *testing.T, database *sql.DB, sha string) (string, int, []byte) {
	t.Helper()

	var (
		encoding string
		size     int
		body     []byte
	)
	require.NoError(t, database.QueryRowContext(t.Context(), `
		SELECT encoding, size, body
		FROM api_bodies
		WHERE sha256 = ?`, sha).Scan(&encoding, &size, &body))
	return encoding, size, body
}

// bodyCount returns the number of stored bodies.
func bodyCount(t *testing.T, database *sql.DB) int {
	t.Helper()

	var count int
	require.NoError(t, database.QueryRowContext(t.Context(), "SELECT count(*) FROM api_bodies").Scan(&count))
	return count
}

func TestTransportArchivesSuccessfulResponse(t *testing.T) {
	t.Parallel()

	store, database := openArchiveStore(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"events":[{"id":"401"}]}`))
	}))
	t.Cleanup(server.Close)

	client := archivedClient(store, func() time.Time { return testClock })
	target := server.URL + "/scoreboard?week=9&seasontype=2"
	status, body := get(archive.WithKey(t.Context(), "espn:scoreboard"), t, client, target)

	require.Equal(t, http.StatusOK, status)
	assert.JSONEq(t, `{"events":[{"id":"401"}]}`, body, "the client sees the full body")

	list := responses(t, database)
	require.Len(t, list, 1)
	row := list[0]
	assert.Equal(t, "espn", row.Source)
	assert.Equal(t, "espn:scoreboard", row.Key)
	assert.Equal(t, http.MethodGet, row.Method)
	assert.Equal(t, target, row.URL)
	assert.Equal(t, http.StatusOK, row.Status)
	assert.Equal(t, testClock, parseTimestamp(t, row.FetchedAt))
	assert.Equal(t, int64(0), row.DurationMS)
	assert.False(t, row.CreditsRemaining.Valid)
	assert.False(t, row.CreditsUsed.Valid)

	encoding, size, compressed := storedBody(t, database, row.BodySHA256)
	assert.Equal(t, "gzip", encoding)
	assert.Equal(t, len(body), size)
	assert.Equal(t, body, gunzip(t, compressed))
}

func TestTransportPassesThroughNon2xxUnrecorded(t *testing.T) {
	t.Parallel()

	store, database := openArchiveStore(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("server on fire"))
	}))
	t.Cleanup(server.Close)

	client := archivedClient(store, func() time.Time { return testClock })
	status, body := get(t.Context(), t, client, server.URL+"/scoreboard")

	assert.Equal(t, http.StatusInternalServerError, status)
	assert.Equal(t, "server on fire", body)
	assert.Empty(t, responses(t, database))
	assert.Equal(t, 0, bodyCount(t, database))
}

func TestTransportPassesThroughTransportErrorUnrecorded(t *testing.T) {
	t.Parallel()

	store, database := openArchiveStore(t)
	client := &http.Client{Transport: archive.NewTransport(
		roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, context.DeadlineExceeded
		}),
		store,
		func() time.Time { return testClock },
	)}

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://example.test/scoreboard", nil)
	require.NoError(t, err)
	resp, err := client.Do(req)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	if resp != nil {
		require.NoError(t, resp.Body.Close())
	}
	assert.Empty(t, responses(t, database))
	assert.Equal(t, 0, bodyCount(t, database))
}

func TestTransportDropsSinkErrors(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"events":[]}`))
	}))
	t.Cleanup(server.Close)

	sink := sinkFunc(func(context.Context, archive.Record) error {
		return errors.New("disk is full")
	})
	client := archivedClient(sink, func() time.Time { return testClock })
	status, body := get(t.Context(), t, client, server.URL+"/scoreboard")

	assert.Equal(t, http.StatusOK, status)
	assert.JSONEq(t, `{"events":[]}`, body)
}

func TestTransportSkipsOversizeBody(t *testing.T) {
	t.Parallel()

	store, database := openArchiveStore(t)
	oversize := bytes.Repeat([]byte("a"), archive.MaxBodySize+1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(oversize)
	}))
	t.Cleanup(server.Close)

	client := archivedClient(store, func() time.Time { return testClock })
	status, body := get(t.Context(), t, client, server.URL+"/huge")

	assert.Equal(t, http.StatusOK, status)
	assert.Len(t, body, len(oversize), "the client still reads the whole body")
	assert.Empty(t, responses(t, database))
	assert.Equal(t, 0, bodyCount(t, database))
}

func TestTransportDedupesBodiesAndKeepsResponses(t *testing.T) {
	t.Parallel()

	store, database := openArchiveStore(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("version") == "2" {
			_, _ = w.Write([]byte(`{"events":[{"id":"402"}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"events":[{"id":"401"}]}`))
	}))
	t.Cleanup(server.Close)

	client := archivedClient(store, func() time.Time { return testClock })
	get(archive.WithKey(t.Context(), "espn:scoreboard"), t, client, server.URL+"/scoreboard")
	get(archive.WithKey(t.Context(), "espn:scoreboard"), t, client, server.URL+"/scoreboard")
	get(archive.WithKey(t.Context(), "espn:scoreboard"), t, client, server.URL+"/scoreboard?version=2")

	assert.Equal(t, 2, bodyCount(t, database))
	assert.Len(t, responses(t, database), 3)

	list := responses(t, database)
	assert.Equal(t, list[0].BodySHA256, list[1].BodySHA256, "the same body reuses one row")
	assert.NotEqual(t, list[1].BodySHA256, list[2].BodySHA256)
}

func TestTransportRedactsAPIKeyAndKeepsOtherParams(t *testing.T) {
	t.Parallel()

	store, database := openArchiveStore(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[]`))
	}))
	t.Cleanup(server.Close)

	client := archivedClient(store, func() time.Time { return testClock })
	target := server.URL + "/odds?apiKey=super-secret&markets=h2h&regions=us&copy=super-secret"
	get(archive.WithKey(t.Context(), "odds:lines"), t, client, target)

	list := responses(t, database)
	require.Len(t, list, 1)
	assert.NotContains(t, list[0].URL, "super-secret")
	assert.Contains(t, list[0].URL, "apiKey=REDACTED")
	assert.Contains(t, list[0].URL, "markets=h2h")
	assert.Contains(t, list[0].URL, "regions=us")
}

func TestTransportRedactsAPIKeyAfterRedirect(t *testing.T) {
	t.Parallel()

	store, database := openArchiveStore(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/start" {
			http.Redirect(w, r, "/final?apiKey=super-secret&markets=spreads", http.StatusFound)
			return
		}
		_, _ = w.Write([]byte(`[]`))
	}))
	t.Cleanup(server.Close)

	client := archivedClient(store, func() time.Time { return testClock })
	status, _ := get(archive.WithKey(t.Context(), "odds:lines"), t, client, server.URL+"/start?apiKey=super-secret")

	assert.Equal(t, http.StatusOK, status)
	list := responses(t, database)
	require.Len(t, list, 1, "the redirect itself is not archived")
	assert.Equal(t, server.URL+"/final?apiKey=REDACTED&markets=spreads", list[0].URL)
}

func TestTransportRecordsOddsCreditsOnly(t *testing.T) {
	t.Parallel()

	store, database := openArchiveStore(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Requests-Remaining", "412")
		w.Header().Set("X-Requests-Last", "3")
		_, _ = w.Write([]byte(`[]`))
	}))
	t.Cleanup(server.Close)

	client := archivedClient(store, func() time.Time { return testClock })
	get(archive.WithKey(t.Context(), "odds:lines"), t, client, server.URL+"/odds")
	get(archive.WithKey(t.Context(), "espn:scoreboard"), t, client, server.URL+"/scoreboard")

	list := responses(t, database)
	require.Len(t, list, 2)
	require.True(t, list[0].CreditsRemaining.Valid)
	assert.Equal(t, int64(412), list[0].CreditsRemaining.Int64)
	require.True(t, list[0].CreditsUsed.Valid)
	assert.Equal(t, int64(3), list[0].CreditsUsed.Int64)
	assert.False(t, list[1].CreditsRemaining.Valid, "non-odds sources keep NULL credits")
	assert.False(t, list[1].CreditsUsed.Valid)
}

func TestTransportFallsBackToHostAndPath(t *testing.T) {
	t.Parallel()

	store, database := openArchiveStore(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[]`))
	}))
	t.Cleanup(server.Close)

	client := archivedClient(store, func() time.Time { return testClock })
	get(t.Context(), t, client, server.URL+"/unlabelled?week=9")

	list := responses(t, database)
	require.Len(t, list, 1)
	assert.Equal(t, strings.TrimPrefix(server.URL, "http://")+"/unlabelled", list[0].Key)
	assert.Equal(t, "127.0.0.1", list[0].Source)
}

func TestTransportFallsBackToKnownHostSource(t *testing.T) {
	t.Parallel()

	store, database := openArchiveStore(t)
	client := &http.Client{Transport: archive.NewTransport(
		roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Body:       io.NopCloser(strings.NewReader(`[]`)),
				Header:     http.Header{},
				Request:    req,
			}, nil
		}),
		store,
		func() time.Time { return testClock },
	)}

	get(t.Context(), t, client, "https://site.api.espn.com/apis/site/v2/sports/football/nfl/scoreboard")

	list := responses(t, database)
	require.Len(t, list, 1)
	assert.Equal(t, "espn", list[0].Source)
	assert.Equal(t, "site.api.espn.com/apis/site/v2/sports/football/nfl/scoreboard", list[0].Key)
}

// roundTripFunc adapts a function to http.RoundTripper.
type roundTripFunc func(*http.Request) (*http.Response, error)

// RoundTrip calls the function.
func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

// sinkFunc adapts a function to archive.Sink.
type sinkFunc func(context.Context, archive.Record) error

// SaveAPIResponse calls the function.
func (f sinkFunc) SaveAPIResponse(ctx context.Context, record archive.Record) error {
	return f(ctx, record)
}

// parseTimestamp reads the RFC 3339 text the store writes.
func parseTimestamp(t *testing.T, value string) time.Time {
	t.Helper()

	parsed, err := time.Parse(time.RFC3339, value)
	require.NoError(t, err)
	return parsed
}

// gunzip decompresses a stored body.
func gunzip(t *testing.T, compressed []byte) string {
	t.Helper()

	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	require.NoError(t, err)
	defer func() { assert.NoError(t, reader.Close()) }()

	body, err := io.ReadAll(reader)
	require.NoError(t, err)
	return string(body)
}
