package db_test

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/ingest/archive"
)

func TestSaveAPIResponseStoresGzippedBody(t *testing.T) {
	t.Parallel()

	store, database := openTestStore(t)
	body := []byte(`{"events":[{"id":"401"}]}`)
	fetchedAt := time.Date(2026, time.November, 7, 13, 0, 0, 0, time.UTC)
	remaining, used := 412, 3

	err := store.SaveAPIResponse(t.Context(), archive.Record{
		Source:           "odds",
		Key:              "odds:lines",
		Method:           "GET",
		URL:              "https://api.the-odds-api.com/v4/odds?apiKey=REDACTED",
		Status:           200,
		FetchedAt:        fetchedAt,
		Duration:         25 * time.Millisecond,
		Body:             body,
		CreditsRemaining: &remaining,
		CreditsUsed:      &used,
	})
	require.NoError(t, err)

	var (
		source      string
		key         string
		storedURL   string
		fetchedText string
		durationMS  int64
		sha         string
		creditsLeft int
		creditsUsed int
		storedSize  int
		storedBody  []byte
	)
	require.NoError(t, database.QueryRowContext(t.Context(), `
		SELECT source, key, url, fetched_at, duration_ms, body_sha256,
		       credits_remaining, credits_used
		FROM api_responses`).Scan(
		&source, &key, &storedURL, &fetchedText, &durationMS, &sha, &creditsLeft, &creditsUsed,
	))
	require.NoError(t, database.QueryRowContext(t.Context(), `
		SELECT size, body
		FROM api_bodies
		WHERE sha256 = ?`, sha).Scan(&storedSize, &storedBody))

	sum := sha256.Sum256(body)
	assert.Equal(t, hex.EncodeToString(sum[:]), sha, "the hash covers the uncompressed body")
	assert.Equal(t, "odds", source)
	assert.Equal(t, "odds:lines", key)
	assert.Equal(t, "https://api.the-odds-api.com/v4/odds?apiKey=REDACTED", storedURL)
	assert.Equal(t, "2026-11-07T13:00:00Z", fetchedText)
	assert.Equal(t, int64(25), durationMS)
	assert.Equal(t, 412, creditsLeft)
	assert.Equal(t, 3, creditsUsed)
	assert.Equal(t, len(body), storedSize)

	reader, err := gzip.NewReader(bytes.NewReader(storedBody))
	require.NoError(t, err)
	defer func() { assert.NoError(t, reader.Close()) }()
	uncompressed, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.Equal(t, body, uncompressed)
}

func TestSaveAPIResponseDedupesBodies(t *testing.T) {
	t.Parallel()

	store, database := openTestStore(t)
	record := func(body string) archive.Record {
		return archive.Record{
			Source:    "espn",
			Key:       "espn:scoreboard",
			Method:    "GET",
			URL:       "https://site.api.espn.com/scoreboard",
			Status:    200,
			FetchedAt: time.Date(2026, time.November, 7, 13, 0, 0, 0, time.UTC),
			Body:      []byte(body),
		}
	}

	require.NoError(t, store.SaveAPIResponse(t.Context(), record(`{"id":"401"}`)))
	require.NoError(t, store.SaveAPIResponse(t.Context(), record(`{"id":"401"}`)))
	require.NoError(t, store.SaveAPIResponse(t.Context(), record(`{"id":"402"}`)))

	assert.Equal(t, 2, archiveRowCount(t, database, "SELECT count(*) FROM api_bodies"))
	assert.Equal(t, 3, archiveRowCount(t, database, "SELECT count(*) FROM api_responses"))
}

// archiveRowCount counts the rows a fixed count query returns.
func archiveRowCount(t *testing.T, database *sql.DB, query string) int {
	t.Helper()

	var count int
	require.NoError(t, database.QueryRowContext(t.Context(), query).Scan(&count))
	return count
}

func TestAPIResponseBodyRoundTripsBytes(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	// A full byte range proves the round trip keeps binary data intact, not
	// just the UTF-8 JSON the archive usually sees.
	body := bytes.Repeat([]byte{0, 1, 2, 127, 128, 254, 255}, 64)
	body = append(body, []byte(`{"events":[{"id":"401"}]}`)...)

	require.NoError(t, store.SaveAPIResponse(t.Context(), archive.Record{
		Source:    "espn",
		Key:       "espn:scoreboard",
		Method:    "GET",
		URL:       "https://site.api.espn.com/scoreboard",
		Status:    200,
		FetchedAt: time.Date(2026, time.November, 7, 13, 0, 0, 0, time.UTC),
		Body:      body,
	}))

	list, err := store.ListAPIResponses(t.Context(), db.APIResponseFilter{})
	require.NoError(t, err)
	require.Len(t, list, 1)

	got, err := store.APIResponseBody(t.Context(), list[0].BodySHA256)
	require.NoError(t, err)
	assert.Equal(t, body, got, "the body must come back byte-for-byte")
}

func TestListAPIResponsesMatchesPrefixLiterally(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	keys := []string{"odds", "odds:lines", "odds_extra", "odds%wild", "oddsy", "oddt"}
	for i, key := range keys {
		require.NoError(t, store.SaveAPIResponse(t.Context(), archive.Record{
			Source:    "odds",
			Key:       key,
			Method:    http.MethodGet,
			URL:       "https://api.the-odds-api.com/v4/odds",
			Status:    http.StatusOK,
			FetchedAt: time.Date(2026, time.November, 7, 13, i, 0, 0, time.UTC),
			Body:      []byte(`{"key":"` + key + `"}`),
		}))
	}

	tests := map[string]struct {
		prefix string
		want   []string
	}{
		"shared start excludes the next byte": {
			prefix: "odds",
			want:   []string{"odds", "odds:lines", "odds_extra", "odds%wild", "oddsy"},
		},
		"colon stays literal": {
			prefix: "odds:",
			want:   []string{"odds:lines"},
		},
		"underscore stays literal": {
			prefix: "odds_",
			want:   []string{"odds_extra"},
		},
		"percent stays literal": {
			prefix: "odds%",
			want:   []string{"odds%wild"},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			list, err := store.ListAPIResponses(t.Context(), db.APIResponseFilter{KeyPrefix: test.prefix})
			require.NoError(t, err)

			got := make([]string, len(list))
			for i, response := range list {
				got[i] = response.Key
			}
			assert.Equal(t, test.want, got)
		})
	}
}

func TestAPIResponseBodyReportsMissingHash(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)

	_, err := store.APIResponseBody(t.Context(), "not-a-stored-hash")
	require.Error(t, err)
	assert.ErrorIs(t, err, db.ErrAPIBodyNotFound)
}

func TestListAPIResponsesFiltersByKeyAndTime(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	remaining, used := 500, 2
	records := []archive.Record{
		{
			Source:    "espn",
			Key:       "espn:scoreboard",
			Method:    "GET",
			URL:       "https://site.api.espn.com/scoreboard",
			Status:    200,
			FetchedAt: time.Date(2026, time.November, 7, 13, 0, 0, 0, time.UTC),
			Body:      []byte(`{"espn":1}`),
		},
		{
			Source:           "odds",
			Key:              "odds:lines",
			Method:           "GET",
			URL:              "https://api.the-odds-api.com/v4/sports/nfl/odds?apiKey=REDACTED",
			Status:           200,
			FetchedAt:        time.Date(2026, time.November, 7, 14, 0, 0, 0, time.UTC),
			Body:             []byte(`{"lines":1}`),
			CreditsRemaining: &remaining,
			CreditsUsed:      &used,
		},
		{
			Source:    "odds",
			Key:       "odds:props:401",
			Method:    "GET",
			URL:       "https://api.the-odds-api.com/v4/events/401/odds?apiKey=REDACTED",
			Status:    200,
			FetchedAt: time.Date(2026, time.November, 7, 15, 0, 0, 0, time.UTC),
			Body:      []byte(`{"props":401}`),
		},
		{
			Source:    "odds",
			Key:       "odds:props:402",
			Method:    "GET",
			URL:       "https://api.the-odds-api.com/v4/events/402/odds?apiKey=REDACTED",
			Status:    200,
			FetchedAt: time.Date(2026, time.November, 7, 16, 0, 0, 0, time.UTC),
			Body:      []byte(`{"props":402}`),
		},
	}
	for _, record := range records {
		require.NoError(t, store.SaveAPIResponse(t.Context(), record))
	}

	tests := map[string]struct {
		filter db.APIResponseFilter
		want   []string
	}{
		"all": {
			filter: db.APIResponseFilter{},
			want:   []string{"espn:scoreboard", "odds:lines", "odds:props:401", "odds:props:402"},
		},
		"key prefix": {
			filter: db.APIResponseFilter{KeyPrefix: "odds:"},
			want:   []string{"odds:lines", "odds:props:401", "odds:props:402"},
		},
		"time range includes both ends": {
			filter: db.APIResponseFilter{
				Since: time.Date(2026, time.November, 7, 15, 0, 0, 0, time.UTC),
				Until: time.Date(2026, time.November, 7, 16, 0, 0, 0, time.UTC),
			},
			want: []string{"odds:props:401", "odds:props:402"},
		},
		"prefix and time": {
			filter: db.APIResponseFilter{
				KeyPrefix: "odds:props:",
				Since:     time.Date(2026, time.November, 7, 15, 30, 0, 0, time.UTC),
			},
			want: []string{"odds:props:402"},
		},
		"no matches": {
			filter: db.APIResponseFilter{KeyPrefix: "weather:"},
			want:   []string{},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			list, err := store.ListAPIResponses(t.Context(), test.filter)
			require.NoError(t, err)

			keys := make([]string, len(list))
			for i, response := range list {
				keys[i] = response.Key
			}
			assert.Equal(t, test.want, keys)
		})
	}

	list, err := store.ListAPIResponses(t.Context(), db.APIResponseFilter{KeyPrefix: "odds:lines"})
	require.NoError(t, err)
	require.Len(t, list, 1)
	line := list[0]
	assert.Equal(t, "odds", line.Source)
	assert.Equal(t, http.MethodGet, line.Method)
	assert.Equal(t, "https://api.the-odds-api.com/v4/sports/nfl/odds?apiKey=REDACTED", line.URL)
	assert.Equal(t, http.StatusOK, line.Status)
	assert.Equal(t, time.Date(2026, time.November, 7, 14, 0, 0, 0, time.UTC), line.FetchedAt)
	require.NotNil(t, line.CreditsRemaining)
	require.NotNil(t, line.CreditsUsed)
	assert.Equal(t, 500, *line.CreditsRemaining)
	assert.Equal(t, 2, *line.CreditsUsed)
}

func TestLatestAPIResponseBefore(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	base := time.Date(2026, time.November, 8, 12, 0, 0, 0, time.UTC)

	save := func(key string, status int, fetchedAt time.Time, body string) {
		t.Helper()
		require.NoError(t, store.SaveAPIResponse(t.Context(), archive.Record{
			Source:    "espn",
			Key:       key,
			Method:    http.MethodGet,
			URL:       "https://example.com/" + key,
			Status:    status,
			FetchedAt: fetchedAt,
			Body:      []byte(body),
		}))
	}

	key := db.ArchiveSummaryKey("401")
	save(key, http.StatusOK, base.Add(-2*time.Hour), `{"v":1}`)
	save(key, http.StatusCreated, base.Add(-1*time.Hour), `{"v":2}`)
	save(key, http.StatusOK, base.Add(-1*time.Hour), `{"v":3}`) // same timestamp, higher id
	save(key, http.StatusOK, base.Add(time.Minute), `{"v":4}`)  // after cutoff
	save(db.ArchiveSummaryKey("402"), http.StatusOK, base, `{"other":true}`)

	got, ok, err := store.LatestAPIResponseBefore(t.Context(), key, base)
	require.NoError(t, err)
	require.True(t, ok)
	body, err := store.APIResponseBody(t.Context(), got.BodySHA256)
	require.NoError(t, err)
	assert.Equal(t, `{"v":3}`, string(body))

	// Exact boundary match on a 2xx row.
	got, ok, err = store.LatestAPIResponseBefore(t.Context(), key, base.Add(-2*time.Hour))
	require.NoError(t, err)
	require.True(t, ok)
	body, err = store.APIResponseBody(t.Context(), got.BodySHA256)
	require.NoError(t, err)
	assert.Equal(t, `{"v":1}`, string(body))

	// Cutoff before any 2xx row returns ok=false.
	_, ok, err = store.LatestAPIResponseBefore(t.Context(), key, base.Add(-3*time.Hour))
	require.NoError(t, err)
	assert.False(t, ok)

	// Missing key returns ok=false.
	_, ok, err = store.LatestAPIResponseBefore(t.Context(), db.ArchiveSummaryKey("missing"), base)
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestLatestAPIResponse(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	base := time.Date(2026, time.November, 8, 12, 0, 0, 0, time.UTC)

	save := func(key string, status int, fetchedAt time.Time, body string) {
		t.Helper()
		require.NoError(t, store.SaveAPIResponse(t.Context(), archive.Record{
			Source:    "espn",
			Key:       key,
			Method:    http.MethodGet,
			URL:       "https://example.com/" + key,
			Status:    status,
			FetchedAt: fetchedAt,
			Body:      []byte(body),
		}))
	}

	key := db.ArchiveSummaryKey("401")
	save(key, http.StatusOK, base.Add(-2*time.Hour), `{"v":1}`)
	save(key, http.StatusCreated, base.Add(-1*time.Hour), `{"v":2}`)
	save(key, http.StatusOK, base.Add(-1*time.Hour), `{"v":3}`) // same timestamp, higher id

	got, ok, err := store.LatestAPIResponse(t.Context(), key)
	require.NoError(t, err)
	require.True(t, ok)
	body, err := store.APIResponseBody(t.Context(), got.BodySHA256)
	require.NoError(t, err)
	assert.Equal(t, `{"v":3}`, string(body))

	_, ok, err = store.LatestAPIResponse(t.Context(), db.ArchiveSummaryKey("missing"))
	require.NoError(t, err)
	assert.False(t, ok)
}
