package web_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/live"
	"github.com/kalverra/parlay-club/internal/push"
	"github.com/kalverra/parlay-club/internal/slips"
	webui "github.com/kalverra/parlay-club/internal/web"
	assets "github.com/kalverra/parlay-club/web"
)

// testPushKeys are valid-shaped browser keys. The push host rejects nothing in
// these tests because no send reaches it.
const (
	testP256dh = "BNcRdreALRFXTkOOUHK1EtK2wtaz5Ry4YfYCA_0QTpQtUbVlUls0VJXg7A8u-Ts1XbjhazAkj7I99e8QcYP7DkM"
	testAuth   = "tBHItJI5svbpez7KI4CCXg"
)

// pushFixture wires the push routes and the profile page the way the server
// does, with a switchable signed-in person.
type pushFixture struct {
	handler http.Handler
	store   *db.Store
	user    db.User
	other   db.User
	as      func(db.User)
}

func newPushFixture(t *testing.T, enabled bool) *pushFixture {
	t.Helper()

	database, err := db.Open(filepath.Join(t.TempDir(), "parlay.db"))
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, database.Close()) })
	require.NoError(t, db.Migrate(t.Context(), database))

	store := db.NewStore(database)
	t.Cleanup(func() { assert.NoError(t, store.Close()) })

	user, err := store.CreateUserWithIdentity(t.Context(), "Ada", "ada@example.com", "", "")
	require.NoError(t, err)
	other, err := store.CreateUserWithIdentity(t.Context(), "Ben", "ben@example.com", "", "")
	require.NoError(t, err)
	group, err := store.Create(t.Context(), user.ID, "Sunday Parlay Club")
	require.NoError(t, err)

	keys := push.Keys{}
	if enabled {
		private, public, err := webpush.GenerateVAPIDKeys()
		require.NoError(t, err)
		keys = push.Keys{Public: public, Private: private}
	}
	service := push.New(store, zerolog.Nop(), keys, push.Options{
		Subject: "https://parlay.example.com",
	})

	renderer, err := webui.NewRenderer(assets.Templates, false)
	require.NoError(t, err)

	signedIn := user
	handler := webui.NewWithDeps(webui.Dependencies{
		Logger:   zerolog.Nop(),
		Renderer: renderer,
		UserFrom: func(context.Context) (db.User, bool) {
			return signedIn, true
		},
		GroupsFrom: func(context.Context) []webui.GroupOption {
			return []webui.GroupOption{{ID: group.ID, Name: group.Name, Current: true}}
		},
		GroupFrom: func(context.Context) (db.Group, bool) {
			return group, true
		},
		Store: store,
		Slips: slips.New(store, &live.Hub{}, nil),
		Push:  service,
	})

	return &pushFixture{
		handler: handler,
		store:   store,
		user:    user,
		other:   other,
		as:      func(u db.User) { signedIn = u },
	}
}

// pushRequest sends an HTMX-shaped JSON request, which is what app.js does.
func (f *pushFixture) pushRequest(t *testing.T, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()

	var payload []byte
	switch value := body.(type) {
	case nil:
	case string:
		payload = []byte(value)
	default:
		var err error
		payload, err = json.Marshal(value)
		require.NoError(t, err)
	}

	request := httptest.NewRequestWithContext(t.Context(), method, path, strings.NewReader(string(payload)))
	request.Header.Set("Hx-Request", "true")
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	f.handler.ServeHTTP(recorder, request)
	return recorder
}

func TestPushSubscribeStoresThenReassignsTheEndpoint(t *testing.T) {
	t.Parallel()

	fixture := newPushFixture(t, true)
	endpoint := "https://fcm.googleapis.com/fcm/send/shared"

	recorder := fixture.pushRequest(t, http.MethodPost, "/push/subscription", push.Subscription{
		Endpoint: endpoint,
		Keys:     push.SubscriptionKeys{P256dh: testP256dh, Auth: testAuth},
	})
	require.Equal(t, http.StatusNoContent, recorder.Code)

	subs, err := fixture.store.ListPushSubscriptions(t.Context(), fixture.user.ID)
	require.NoError(t, err)
	require.Len(t, subs, 1)
	assert.Equal(t, endpoint, subs[0].Endpoint)

	fixture.as(fixture.other)
	recorder = fixture.pushRequest(t, http.MethodPost, "/push/subscription", push.Subscription{
		Endpoint: endpoint,
		Keys:     push.SubscriptionKeys{P256dh: testP256dh, Auth: testAuth},
	})
	require.Equal(t, http.StatusNoContent, recorder.Code)

	first, err := fixture.store.ListPushSubscriptions(t.Context(), fixture.user.ID)
	require.NoError(t, err)
	assert.Empty(t, first, "the last person to subscribe on the device owns it")

	second, err := fixture.store.ListPushSubscriptions(t.Context(), fixture.other.ID)
	require.NoError(t, err)
	require.Len(t, second, 1)
	assert.Equal(t, endpoint, second[0].Endpoint)
}

func TestPushSubscribeRejectsBadSubscriptions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body any
	}{
		{name: "not JSON", body: "not json"},
		{name: "plain http", body: push.Subscription{
			Endpoint: "http://web.push.apple.com/x",
			Keys:     push.SubscriptionKeys{P256dh: testP256dh, Auth: testAuth},
		}},
		{name: "unknown host", body: push.Subscription{
			Endpoint: "https://example.com/x",
			Keys:     push.SubscriptionKeys{P256dh: testP256dh, Auth: testAuth},
		}},
		{name: "look-alike host", body: push.Subscription{
			Endpoint: "https://web.push.apple.com.attacker.test/x",
			Keys:     push.SubscriptionKeys{P256dh: testP256dh, Auth: testAuth},
		}},
		{name: "missing keys", body: push.Subscription{
			Endpoint: "https://web.push.apple.com/x",
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			fixture := newPushFixture(t, true)
			recorder := fixture.pushRequest(t, http.MethodPost, "/push/subscription", test.body)

			assert.Equal(t, http.StatusBadRequest, recorder.Code)
			subs, err := fixture.store.ListPushSubscriptions(t.Context(), fixture.user.ID)
			require.NoError(t, err)
			assert.Empty(t, subs)
		})
	}
}

func TestPushSubscribeReplacesTheOldestPastTenDevices(t *testing.T) {
	t.Parallel()

	fixture := newPushFixture(t, true)

	for i := range push.MaxSubscriptionsPerUser + 1 {
		endpoint := "https://web.push.apple.com/device-" + strconv.Itoa(i)
		recorder := fixture.pushRequest(t, http.MethodPost, "/push/subscription", push.Subscription{
			Endpoint: endpoint,
			Keys:     push.SubscriptionKeys{P256dh: testP256dh, Auth: testAuth},
		})
		require.Equal(t, http.StatusNoContent, recorder.Code, endpoint)
	}

	subs, err := fixture.store.ListPushSubscriptions(t.Context(), fixture.user.ID)
	require.NoError(t, err)
	require.Len(t, subs, push.MaxSubscriptionsPerUser)
	assert.NotEqual(t, "https://web.push.apple.com/device-0", subs[0].Endpoint,
		"the oldest subscription is replaced")
}

func TestPushUnsubscribeRemovesOnlyTheCallersEndpoint(t *testing.T) {
	t.Parallel()

	fixture := newPushFixture(t, true)
	aliceEndpoint := "https://web.push.apple.com/alice"
	bobEndpoint := "https://web.push.apple.com/bob"

	require.NoError(t, fixture.store.SavePushSubscription(t.Context(), db.PushSubscription{
		UserID:   fixture.user.ID,
		Endpoint: aliceEndpoint,
		P256dh:   testP256dh,
		Auth:     testAuth,
	}, push.MaxSubscriptionsPerUser))
	require.NoError(t, fixture.store.SavePushSubscription(t.Context(), db.PushSubscription{
		UserID:   fixture.other.ID,
		Endpoint: bobEndpoint,
		P256dh:   testP256dh,
		Auth:     testAuth,
	}, push.MaxSubscriptionsPerUser))

	recorder := fixture.pushRequest(t, http.MethodDelete, "/push/subscription", map[string]string{
		"endpoint": bobEndpoint,
	})
	require.Equal(t, http.StatusNoContent, recorder.Code)
	bobSubs, err := fixture.store.ListPushSubscriptions(t.Context(), fixture.other.ID)
	require.NoError(t, err)
	assert.Len(t, bobSubs, 1, "another person's endpoint stays put")

	recorder = fixture.pushRequest(t, http.MethodDelete, "/push/subscription", map[string]string{
		"endpoint": aliceEndpoint,
	})
	require.Equal(t, http.StatusNoContent, recorder.Code)
	aliceSubs, err := fixture.store.ListPushSubscriptions(t.Context(), fixture.user.ID)
	require.NoError(t, err)
	assert.Empty(t, aliceSubs)

	recorder = fixture.pushRequest(t, http.MethodDelete, "/push/subscription", map[string]string{
		"endpoint": aliceEndpoint,
	})
	assert.Equal(t, http.StatusNoContent, recorder.Code, "turning it off twice is harmless")
}

func TestPushRoutesAnswer404WhenVAPIDIsOff(t *testing.T) {
	t.Parallel()

	fixture := newPushFixture(t, false)
	endpoint := map[string]string{"endpoint": "https://web.push.apple.com/x"}

	tests := []struct {
		name string
		path string
		body any
	}{
		{name: "subscribe", path: "/push/subscription", body: push.Subscription{
			Endpoint: "https://web.push.apple.com/x",
			Keys:     push.SubscriptionKeys{P256dh: testP256dh, Auth: testAuth},
		}},
		{name: "unsubscribe", path: "/push/subscription", body: endpoint},
		{name: "test", path: "/push/test", body: nil},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			method := http.MethodPost
			if test.name == "unsubscribe" {
				method = http.MethodDelete
			}
			recorder := fixture.pushRequest(t, method, test.path, test.body)
			assert.Equal(t, http.StatusNotFound, recorder.Code)
		})
	}
}

func TestPushTestSendsToTheCallersDevices(t *testing.T) {
	t.Parallel()

	fixture := newPushFixture(t, true)

	var sends atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		sends.Add(1)
		w.WriteHeader(http.StatusCreated)
	}))
	t.Cleanup(server.Close)

	for _, subscription := range []struct {
		user     db.User
		endpoint string
	}{
		{user: fixture.user, endpoint: server.URL + "/ada-1"},
		{user: fixture.user, endpoint: server.URL + "/ada-2"},
		{user: fixture.other, endpoint: server.URL + "/ben-1"},
	} {
		require.NoError(t, fixture.store.SavePushSubscription(t.Context(), db.PushSubscription{
			UserID:   subscription.user.ID,
			Endpoint: subscription.endpoint,
			P256dh:   testP256dh,
			Auth:     testAuth,
		}, push.MaxSubscriptionsPerUser))
	}

	recorder := fixture.pushRequest(t, http.MethodPost, "/push/test", nil)
	require.Equal(t, http.StatusNoContent, recorder.Code)
	assert.Equal(t, int32(2), sends.Load(), "only the caller's devices get the test")
}

func TestProfileRendersTheNotificationsCardOnlyWhenPushIsOn(t *testing.T) {
	t.Parallel()

	enabled := newPushFixture(t, true)
	recorder := httptest.NewRecorder()
	enabled.handler.ServeHTTP(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/me", nil))
	require.Equal(t, http.StatusOK, recorder.Code)
	body := recorder.Body.String()
	assert.Contains(t, body, "data-push-card")
	assert.Contains(t, body, "data-push-on")
	assert.Contains(t, body, "data-push-off")
	assert.Contains(t, body, "data-push-test")
	assert.Contains(t, body, "data-push-install-hint")
	assert.Contains(t, body, "data-push-denied-hint")
	assert.Contains(t, body, "data-logout-form")

	disabled := newPushFixture(t, false)
	recorder = httptest.NewRecorder()
	disabled.handler.ServeHTTP(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/me", nil))
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.NotContains(t, recorder.Body.String(), "data-push-card")
}
