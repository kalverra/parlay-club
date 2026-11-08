// Package archive records every successful outside HTTP response in the app's
// SQLite file. A Transport wraps the ingest *http.Client's transport, so a
// client cannot forget to save a call and the client itself still never touches
// the database.
package archive

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/rs/zerolog"
)

const (
	// MaxBodySize is the largest uncompressed body the archive keeps. A bigger
	// body passes through unrecorded, because it is probably not market data.
	MaxBodySize = 8 << 20
	// redacted replaces the API key's value in a stored URL.
	redacted = "REDACTED"
)

// Record is one successful outside response, ready for the sink. Body holds the
// uncompressed bytes; the sink hashes and compresses them.
type Record struct {
	Source           string
	Key              string
	Method           string
	URL              string
	Status           int
	FetchedAt        time.Time
	Duration         time.Duration
	Body             []byte
	CreditsRemaining *int
	CreditsUsed      *int
}

// Sink stores one Record. *db.Store satisfies it.
type Sink interface {
	SaveAPIResponse(ctx context.Context, record Record) error
}

// Transport is an http.RoundTripper that archives successful responses. It must
// wrap the base transport, not sit beside it, so every client call goes
// through it.
type Transport struct {
	base http.RoundTripper
	sink Sink
	now  func() time.Time
}

// NewTransport wraps base with an archive sink. A nil base means
// http.DefaultTransport, and a nil clock means time.Now. A nil sink is a
// wiring mistake: the transport exists to save responses, so reject it at
// construction rather than panicking on the first successful call.
func NewTransport(base http.RoundTripper, sink Sink, now func() time.Time) *Transport {
	if sink == nil {
		panic("archive: nil Sink")
	}
	if base == nil {
		base = http.DefaultTransport
	}
	if now == nil {
		now = time.Now
	}
	return &Transport{base: base, sink: sink, now: now}
}

// RoundTrip sends the request, records a 2xx response, and returns the response
// unchanged. A failed save is logged and dropped, so the archive never changes
// what the client sees. Non-2xx responses and transport errors pass through
// unrecorded.
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	started := t.now()

	resp, err := t.base.RoundTrip(req)
	if err != nil {
		return resp, err
	}
	if resp.Body == nil || resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return resp, nil
	}

	body, over, readErr := readCapped(resp.Body, MaxBodySize)
	if readErr != nil {
		// Keep the bytes already read in front of the untouched remainder so the
		// client still gets as much of the body as the source sent.
		resp.Body = &multiReadCloser{
			Reader: io.MultiReader(bytes.NewReader(body), resp.Body),
			Closer: resp.Body,
		}
		zerolog.Ctx(req.Context()).Warn().
			Err(readErr).
			Msg("read API response body")
		return resp, nil
	}
	if over {
		// The extra byte that proved the body is too big is already read; put it
		// back in front of the rest.
		resp.Body = &multiReadCloser{
			Reader: io.MultiReader(bytes.NewReader(body), resp.Body),
			Closer: resp.Body,
		}
		zerolog.Ctx(req.Context()).Warn().
			Int("max", MaxBodySize).
			Msg("API response over archive cap")
		return resp, nil
	}

	_ = resp.Body.Close()
	resp.Body = io.NopCloser(bytes.NewReader(body))

	key, ok := KeyFrom(req.Context())
	if !ok {
		key = req.URL.Host + req.URL.Path
	}
	record := Record{
		Source:    sourceForKey(key, req.URL.Hostname()),
		Key:       key,
		Method:    req.Method,
		URL:       redactURL(req.URL),
		Status:    resp.StatusCode,
		FetchedAt: t.now(),
		Body:      body,
	}
	record.Duration = record.FetchedAt.Sub(started)
	if record.Source == SourceOdds {
		record.CreditsRemaining = headerInt(resp.Header, "X-Requests-Remaining")
		record.CreditsUsed = headerInt(resp.Header, "X-Requests-Last")
	}

	// A failed save must not change what the client sees, so log and drop it.
	sinkStarted := t.now()
	if err := t.sink.SaveAPIResponse(req.Context(), record); err != nil {
		zerolog.Ctx(req.Context()).Warn().
			Err(err).
			Str("key", record.Key).
			Str("url", record.URL).
			Msg("save archived API response")
	}
	zerolog.Ctx(req.Context()).Debug().
		Dur("duration", t.now().Sub(sinkStarted)).
		Str("key", record.Key).
		Msg("archived API response")
	return resp, nil
}

// readCapped reads at most limit+1 bytes and reports whether the body was
// larger than limit. It reads the extra byte only to tell "exactly limit" from
// "over", leaving the rest of the body untouched.
func readCapped(body io.Reader, limit int) ([]byte, bool, error) {
	data, err := io.ReadAll(io.LimitReader(body, int64(limit)+1))
	if err != nil {
		return data, false, err
	}
	return data, len(data) > limit, nil
}

// multiReadCloser presents already-read bytes followed by the original body,
// and still closes the original.
type multiReadCloser struct {
	io.Reader
	io.Closer
}

// headerInt reads an integer header, returning nil when it is missing or not a
// number. A nil leaves the database column NULL.
func headerInt(header http.Header, key string) *int {
	raw := header.Get(key)
	if raw == "" {
		return nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return nil
	}
	return &value
}
