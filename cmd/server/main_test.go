package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/config"
)

func TestHealthz(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	newMux(http.NotFoundHandler(), func(*http.ServeMux) {}).
		ServeHTTP(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/healthz", nil))

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "ok", recorder.Body.String())
	assert.Equal(t, "text/plain; charset=utf-8", recorder.Header().Get("Content-Type"))
}

func TestMuxMountsAuthRoutes(t *testing.T) {
	t.Parallel()

	mux := newMux(http.NotFoundHandler(), func(mux *http.ServeMux) {
		mux.HandleFunc("GET /login", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		})
	})

	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/login", nil))

	assert.Equal(t, http.StatusNoContent, recorder.Code, "auth routes are registered before the shell")
}

func TestLoggerWritesJSONInProd(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	logger := newLogger(config.Config{AppEnv: config.EnvProd, LogLevel: zerolog.InfoLevel}, &out)
	logger.Info().Msg("hello")

	assert.True(t, json.Valid(out.Bytes()), "prod logs must be JSON: %q", out.String())
	assert.Contains(t, out.String(), `"message":"hello"`)
}

func TestLoggerWritesConsoleInDev(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	logger := newLogger(config.Config{AppEnv: config.EnvDev, LogLevel: zerolog.DebugLevel}, &out)
	logger.Info().Msg("hello")

	assert.Contains(t, out.String(), "hello")
	assert.False(t, json.Valid(out.Bytes()), "dev logs should be readable, not JSON: %q", out.String())
}

func TestServeDrainsInflightRequestOnShutdown(t *testing.T) {
	t.Parallel()

	started := make(chan struct{})
	release := make(chan struct{})
	mux := http.NewServeMux()
	mux.HandleFunc("GET /slow", func(w http.ResponseWriter, _ *http.Request) {
		close(started)
		<-release
		w.WriteHeader(http.StatusNoContent)
	})

	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	server := &http.Server{Handler: mux, ReadHeaderTimeout: time.Second}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- serve(ctx, zerolog.Nop(), server, listener) }()

	url := fmt.Sprintf("http://%s/slow", listener.Addr())
	response := make(chan error, 1)
	go func() {
		request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
		if err != nil {
			response <- err
			return
		}
		resp, err := http.DefaultClient.Do(request)
		if err != nil {
			response <- err
			return
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusNoContent {
			response <- fmt.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNoContent)
			return
		}
		response <- nil
	}()

	<-started
	cancel()

	select {
	case err := <-done:
		t.Fatalf("serve returned before the in-flight request finished: %v", err)
	case <-time.After(100 * time.Millisecond):
		// Shutdown is still waiting on the handler, as it should.
	}

	close(release)
	require.NoError(t, <-response)
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("serve did not return after the in-flight request finished")
	}
}
