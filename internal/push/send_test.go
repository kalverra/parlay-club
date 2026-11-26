package push_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/push"
)

// pushRecorder records what a stand-in push service sees.
type pushRecorder struct {
	mu       sync.Mutex
	requests []recordedRequest
}

type recordedRequest struct {
	Method string
	Path   string
	Header http.Header
	Body   []byte
}

// recordPushRequests serves every endpoint and answers with statuses by path,
// defaulting to 201. A nil status map accepts every send.
func recordPushRequests(statuses map[string]int) (*pushRecorder, http.Handler) {
	recorder := &pushRecorder{}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		recorder.mu.Lock()
		recorder.requests = append(recorder.requests, recordedRequest{
			Method: r.Method,
			Path:   r.URL.Path,
			Header: r.Header.Clone(),
			Body:   body,
		})
		recorder.mu.Unlock()

		status := http.StatusCreated
		if code, ok := statuses[r.URL.Path]; ok {
			status = code
		}
		w.WriteHeader(status)
	})
	return recorder, handler
}

// snapshot returns a copy of the recorded requests.
func (r *pushRecorder) snapshot() []recordedRequest {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]recordedRequest(nil), r.requests...)
}

// storeSubscriptions inserts endpoints straight into the store, so a test can
// point them at an httptest server rather than a real push host.
func storeSubscriptions(t *testing.T, store *db.Store, user db.User, endpoints ...string) {
	t.Helper()

	for _, endpoint := range endpoints {
		require.NoError(t, store.SavePushSubscription(t.Context(), db.PushSubscription{
			UserID:   user.ID,
			Endpoint: endpoint,
			P256dh:   subscription(endpoint).Keys.P256dh,
			Auth:     subscription(endpoint).Keys.Auth,
		}, push.MaxSubscriptionsPerUser))
	}
}

// serviceForServer returns an enabled service with a named send cap.
func serviceForServer(t *testing.T, store *db.Store, timeout time.Duration) *push.Service {
	t.Helper()

	return push.New(store, zerolog.Nop(), testKeys(t), push.Options{
		Subject: "https://parlay.example.com",
		Timeout: timeout,
	})
}

func TestSendToUsersSendsOneRequestPerDevice(t *testing.T) {
	t.Parallel()

	store := newStore(t)
	recorder, handler := recordPushRequests(nil)
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	alice := newUser(t, store, "Alice")
	bob := newUser(t, store, "Bob")
	storeSubscriptions(t, store, alice, server.URL+"/alice-1", server.URL+"/alice-2")
	storeSubscriptions(t, store, bob, server.URL+"/bob-1")
	service := serviceForServer(t, store, 0)

	service.SendToUsers(t.Context(), []int64{alice.ID, bob.ID}, push.Message{
		Title: "Sunday Slate",
		Body:  "Ben added 3 legs",
		URL:   "/slips/42",
		Tag:   "slip-42",
	})

	requests := recorder.snapshot()
	require.Len(t, requests, 3)
	for _, request := range requests {
		assert.Equal(t, http.MethodPost, request.Method)
		assert.Equal(t, "21600", request.Header.Get("TTL"), "an undelivered message lasts 6 hours")
		assert.Equal(t, "normal", request.Header.Get("Urgency"))
		assert.Equal(t, "slip-42", request.Header.Get("Topic"))
		assert.Equal(t, "aes128gcm", request.Header.Get("Content-Encoding"))
		assert.Contains(t, request.Header.Get("Authorization"), "vapid t=")
		assert.NotEmpty(t, request.Body)
	}
}

func TestSendToUsersPrunesGoneEndpoints(t *testing.T) {
	t.Parallel()

	store := newStore(t)
	recorder, handler := recordPushRequests(map[string]int{
		"/gone":    http.StatusGone,
		"/missing": http.StatusNotFound,
		"/broken":  http.StatusInternalServerError,
	})
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	user := newUser(t, store, "Ada")
	storeSubscriptions(
		t,
		store,
		user,
		server.URL+"/gone",
		server.URL+"/missing",
		server.URL+"/broken",
		server.URL+"/ok",
	)
	service := serviceForServer(t, store, 0)

	service.SendToUsers(t.Context(), []int64{user.ID}, push.Message{Title: "Sunday Slate", Tag: "slip-42"})

	assert.Len(t, recorder.snapshot(), 4)

	subs, err := store.ListPushSubscriptions(t.Context(), user.ID)
	require.NoError(t, err)
	endpoints := make([]string, 0, len(subs))
	for _, sub := range subs {
		endpoints = append(endpoints, sub.Endpoint)
	}
	assert.ElementsMatch(t,
		[]string{server.URL + "/broken", server.URL + "/ok"},
		endpoints,
		"404 and 410 retire an endpoint; a 500 keeps it for the next send")
}

func TestSendToUsersSkipsInactiveUsers(t *testing.T) {
	t.Parallel()

	store := newStore(t)
	recorder, handler := recordPushRequests(nil)
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	active := newUser(t, store, "Active")
	inactive := newUser(t, store, "Inactive")
	storeSubscriptions(t, store, active, server.URL+"/active")
	storeSubscriptions(t, store, inactive, server.URL+"/inactive")
	_, err := store.SetUserActive(t.Context(), inactive.ID, false)
	require.NoError(t, err)
	service := serviceForServer(t, store, 0)

	service.SendToUsers(t.Context(), []int64{active.ID, inactive.ID}, push.Message{Title: "Sunday Slate"})

	assert.Len(t, recorder.snapshot(), 1, "an inactive person gets no pushes")
}

func TestSendToUsersStopsAtTheSendCap(t *testing.T) {
	t.Parallel()

	store := newStore(t)
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	t.Cleanup(func() {
		close(release)
		server.Close()
	})
	user := newUser(t, store, "Ada")
	storeSubscriptions(t, store, user, server.URL+"/hang")
	service := serviceForServer(t, store, 50*time.Millisecond)

	start := time.Now()
	service.SendToUsers(t.Context(), []int64{user.ID}, push.Message{Title: "Sunday Slate"})
	elapsed := time.Since(start)

	assert.GreaterOrEqual(t, elapsed, 40*time.Millisecond, "the send waited for the cap")
	assert.Less(t, elapsed, time.Second, "a hanging push service does not block past the cap")
}

func TestSendToUsersDoesNothingWithoutUsersOrKeys(t *testing.T) {
	t.Parallel()

	store := newStore(t)
	recorder, handler := recordPushRequests(nil)
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	user := newUser(t, store, "Ada")
	storeSubscriptions(t, store, user, server.URL+"/device")

	disabled := push.New(store, zerolog.Nop(), push.Keys{}, push.Options{})
	disabled.SendToUsers(t.Context(), []int64{user.ID}, push.Message{Title: "Sunday Slate"})

	serviceForServer(t, store, 0).SendToUsers(t.Context(), nil, push.Message{Title: "Sunday Slate"})

	assert.Empty(t, recorder.snapshot(), "no keys or no recipients means no requests")
}
