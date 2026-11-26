package reminders

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testQueue is the queue path the Cloud Tasks tests enqueue against.
const testQueue = "projects/parlay/locations/us-central1/queues/parlay-club-reminders"

// tasksRequest mirrors the tasks.create body for assertions.
type tasksRequest struct {
	Task struct {
		Name         string `json:"name"`
		ScheduleTime string `json:"scheduleTime"`
		HTTPRequest  struct {
			URL        string            `json:"url"`
			HTTPMethod string            `json:"httpMethod"`
			Headers    map[string]string `json:"headers"`
		} `json:"httpRequest"`
	} `json:"task"`
}

// tasksServer stands in for the Cloud Tasks API and records one request.
type tasksServer struct {
	*httptest.Server
	status int
	auth   atomic.Value
	path   atomic.Value
	body   atomic.Value
}

func newTasksServer(t *testing.T, status int) *tasksServer {
	t.Helper()

	server := &tasksServer{status: status}
	server.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.path.Store(r.URL.Path)
		server.auth.Store(r.Header.Get("Authorization"))
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		server.body.Store(string(raw))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(server.Close)
	return server
}

// metadataServer stands in for the metadata token endpoint and counts calls.
type metadataServer struct {
	*httptest.Server
	calls     atomic.Int64
	status    int
	expiresIn int
	flavor    atomic.Value
}

func newMetadataServer(t *testing.T, status, expiresIn int) *metadataServer {
	t.Helper()

	server := &metadataServer{status: status, expiresIn: expiresIn}
	server.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.calls.Add(1)
		server.flavor.Store(r.Header.Get("Metadata-Flavor"))
		if status != http.StatusOK {
			http.Error(w, "metadata is down", status)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "meta-token",
			"expires_in":   expiresIn,
			"token_type":   "Bearer",
		})
	}))
	t.Cleanup(server.Close)
	return server
}

func TestCloudTasksEnqueuesANamedTask(t *testing.T) {
	t.Parallel()

	metadata := newMetadataServer(t, http.StatusOK, 3599)
	tasks := newTasksServer(t, http.StatusOK)
	scheduler, err := NewCloudTasks(
		CloudTasksConfig{
			Queue:   testQueue,
			BaseURL: "https://parlay.example.com",
			Token:   "reminder-secret",
		},
		WithTasksEndpoint(tasks.URL+"/v2"),
		WithMetadataURL(metadata.URL),
	)
	require.NoError(t, err)

	start := time.Date(2026, 11, 8, 18, 0, 0, 0, time.UTC)
	at := start.Add(-LeadTime)
	require.NoError(t, scheduler.Enqueue(t.Context(), 42, start, at))

	assert.Equal(t, "/v2/"+testQueue+"/tasks", tasks.path.Load())
	assert.Equal(t, "Bearer meta-token", tasks.auth.Load())
	assert.Equal(t, "Google", metadata.flavor.Load())
	assert.Equal(t, int64(1), metadata.calls.Load())

	var sent tasksRequest
	require.NoError(t, json.Unmarshal([]byte(tasks.body.Load().(string)), &sent))
	assert.Regexp(
		t,
		`^`+testQueue+`/tasks/[0-9a-f]{4}-slip-42-`+strconv.FormatInt(start.Unix(), 10)+`$`,
		sent.Task.Name,
	)
	assert.Equal(t, at.UTC().Format(time.RFC3339), sent.Task.ScheduleTime)
	assert.Equal(
		t,
		"https://parlay.example.com/internal/reminders?slip=42&start="+strconv.FormatInt(start.Unix(), 10),
		sent.Task.HTTPRequest.URL,
	)
	assert.Equal(t, http.MethodPost, sent.Task.HTTPRequest.HTTPMethod)
	assert.Equal(t, "Bearer reminder-secret", sent.Task.HTTPRequest.Headers["Authorization"])

	require.NoError(t, scheduler.Enqueue(t.Context(), 42, start, at))
	assert.Equal(t, int64(1), metadata.calls.Load(), "the cached token serves the second task")
}

func TestCloudTasksTreatsConflictAsAlreadyExists(t *testing.T) {
	t.Parallel()

	metadata := newMetadataServer(t, http.StatusOK, 3599)
	tasks := newTasksServer(t, http.StatusConflict)
	scheduler, err := NewCloudTasks(
		CloudTasksConfig{Queue: testQueue, BaseURL: "https://parlay.example.com", Token: "secret"},
		WithTasksEndpoint(tasks.URL+"/v2"),
		WithMetadataURL(metadata.URL),
	)
	require.NoError(t, err)

	start := time.Date(2026, 11, 8, 18, 0, 0, 0, time.UTC)
	err = scheduler.Enqueue(t.Context(), 42, start, start.Add(-LeadTime))
	require.ErrorIs(t, err, ErrAlreadyExists)
}

func TestCloudTasksReportsATaskFailure(t *testing.T) {
	t.Parallel()

	metadata := newMetadataServer(t, http.StatusOK, 3599)
	tasks := newTasksServer(t, http.StatusServiceUnavailable)
	scheduler, err := NewCloudTasks(
		CloudTasksConfig{Queue: testQueue, BaseURL: "https://parlay.example.com", Token: "secret"},
		WithTasksEndpoint(tasks.URL+"/v2"),
		WithMetadataURL(metadata.URL),
	)
	require.NoError(t, err)

	start := time.Date(2026, 11, 8, 18, 0, 0, 0, time.UTC)
	err = scheduler.Enqueue(t.Context(), 42, start, start.Add(-LeadTime))
	require.Error(t, err)
	require.NotErrorIs(t, err, ErrAlreadyExists)
	require.Contains(t, err.Error(), "status 503")
}

func TestCloudTasksFailsWhenTheMetadataServerDoes(t *testing.T) {
	t.Parallel()

	metadata := newMetadataServer(t, http.StatusInternalServerError, 0)
	scheduler, err := NewCloudTasks(
		CloudTasksConfig{Queue: testQueue, BaseURL: "https://parlay.example.com", Token: "secret"},
		WithTasksEndpoint("https://tasks.invalid"),
		WithMetadataURL(metadata.URL),
	)
	require.NoError(t, err)

	start := time.Date(2026, 11, 8, 18, 0, 0, 0, time.UTC)
	err = scheduler.Enqueue(t.Context(), 42, start, start.Add(-LeadTime))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "metadata token")
}

func TestCloudTasksRefreshesAnExpiredToken(t *testing.T) {
	t.Parallel()

	metadata := newMetadataServer(t, http.StatusOK, 3599)
	tasks := newTasksServer(t, http.StatusOK)
	scheduler, err := NewCloudTasks(
		CloudTasksConfig{Queue: testQueue, BaseURL: "https://parlay.example.com", Token: "secret"},
		WithTasksEndpoint(tasks.URL+"/v2"),
		WithMetadataURL(metadata.URL),
	)
	require.NoError(t, err)

	base := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)
	scheduler.tokens.now = func() time.Time { return base }
	start := time.Date(2026, 11, 8, 18, 0, 0, 0, time.UTC)

	require.NoError(t, scheduler.Enqueue(t.Context(), 42, start, start.Add(-LeadTime)))
	assert.Equal(t, int64(1), metadata.calls.Load())

	scheduler.tokens.now = func() time.Time { return base.Add(2 * time.Hour) }
	require.NoError(t, scheduler.Enqueue(t.Context(), 42, start, start.Add(-LeadTime)))
	assert.Equal(t, int64(2), metadata.calls.Load(), "an expired token is fetched again")
}

func TestCloudTasksRejectsBadConfig(t *testing.T) {
	t.Parallel()

	valid := CloudTasksConfig{Queue: testQueue, BaseURL: "https://parlay.example.com", Token: "secret"}

	tests := map[string]struct {
		cfg  CloudTasksConfig
		want string
	}{
		"short queue path": {
			cfg:  CloudTasksConfig{Queue: "parlay-club-reminders", BaseURL: valid.BaseURL, Token: valid.Token},
			want: "REMINDER_QUEUE",
		},
		"queue with a blank part": {
			cfg: CloudTasksConfig{
				Queue:   "projects//locations/us-central1/queues/q",
				BaseURL: valid.BaseURL,
				Token:   valid.Token,
			},
			want: "REMINDER_QUEUE",
		},
		"bare base url": {
			cfg:  CloudTasksConfig{Queue: valid.Queue, BaseURL: "parlay.example.com", Token: valid.Token},
			want: "BASE_URL",
		},
		"empty token": {
			cfg:  CloudTasksConfig{Queue: valid.Queue, BaseURL: valid.BaseURL, Token: "  "},
			want: "REMINDER_TOKEN",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := NewCloudTasks(test.cfg)
			require.Error(t, err)
			assert.Contains(t, err.Error(), test.want)
		})
	}
}

func TestCloudTasksTaskNamesAreStable(t *testing.T) {
	t.Parallel()

	scheduler, err := NewCloudTasks(CloudTasksConfig{
		Queue:   testQueue,
		BaseURL: "https://parlay.example.com",
		Token:   "secret",
	})
	require.NoError(t, err)

	start := time.Date(2026, 11, 8, 18, 0, 0, 0, time.UTC).Unix()
	first := scheduler.taskName(42, start)
	assert.Equal(t, first, scheduler.taskName(42, start))
	assert.NotEqual(t, first, scheduler.taskName(43, start))
	assert.NotEqual(t, first, scheduler.taskName(42, start+3600))
	name := strings.TrimPrefix(first, testQueue+"/tasks/")
	assert.Len(t, name, len("0000-slip-42-")+len(strconv.FormatInt(start, 10)))
}
