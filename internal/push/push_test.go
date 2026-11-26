package push_test

import (
	"testing"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/push"
)

// newStore opens a migrated database and a store for one test.
func newStore(t *testing.T) *db.Store {
	t.Helper()

	database, err := db.Open(t.TempDir() + "/parlay.db")
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, database.Close()) })
	require.NoError(t, db.Migrate(t.Context(), database))

	store := db.NewStore(database)
	t.Cleanup(func() { assert.NoError(t, store.Close()) })
	return store
}

// newUser adds one active person.
func newUser(t *testing.T, store *db.Store, name string) db.User {
	t.Helper()

	user, err := store.CreateUserWithIdentity(t.Context(), name, name+"@example.com", "", "")
	require.NoError(t, err)
	return user
}

// testKeys returns a fresh VAPID key pair.
func testKeys(t *testing.T) push.Keys {
	t.Helper()

	private, public, err := webpush.GenerateVAPIDKeys()
	require.NoError(t, err)
	return push.Keys{Public: public, Private: private}
}

// newService returns an enabled service with no HTTP overrides.
func newService(t *testing.T, store *db.Store) *push.Service {
	t.Helper()

	return push.New(store, zerolog.Nop(), testKeys(t), push.Options{
		Subject: "https://parlay.example.com",
	})
}

// subscription is a valid subscription on a known push host.
func subscription(endpoint string) push.Subscription {
	return push.Subscription{
		Endpoint: endpoint,
		Keys: push.SubscriptionKeys{
			P256dh: "BNcRdreALRFXTkOOUHK1EtK2wtaz5Ry4YfYCA_0QTpQtUbVlUls0VJXg7A8u-Ts1XbjhazAkj7I99e8QcYP7DkM",
			Auth:   "tBHItJI5svbpez7KI4CCXg",
		},
	}
}

func TestSubscribeStoresTheEndpoint(t *testing.T) {
	t.Parallel()

	store := newStore(t)
	service := newService(t, store)
	user := newUser(t, store, "Ada")

	require.NoError(t, service.Subscribe(t.Context(), user.ID, subscription("https://web.push.apple.com/sub-1")))

	subs, err := store.ListPushSubscriptions(t.Context(), user.ID)
	require.NoError(t, err)
	require.Len(t, subs, 1)
	assert.Equal(t, "https://web.push.apple.com/sub-1", subs[0].Endpoint)
	assert.Equal(t, user.ID, subs[0].UserID)
	assert.NotEmpty(t, subs[0].P256dh)
	assert.NotEmpty(t, subs[0].Auth)
	assert.WithinDuration(t, time.Now(), subs[0].CreatedAt, time.Minute)
}

func TestSubscribeReassignsASharedEndpoint(t *testing.T) {
	t.Parallel()

	store := newStore(t)
	service := newService(t, store)
	alice := newUser(t, store, "Alice")
	bob := newUser(t, store, "Bob")
	endpoint := "https://fcm.googleapis.com/fcm/send/shared"

	require.NoError(t, service.Subscribe(t.Context(), alice.ID, subscription(endpoint)))
	require.NoError(t, service.Subscribe(t.Context(), bob.ID, subscription(endpoint)))

	aliceSubs, err := store.ListPushSubscriptions(t.Context(), alice.ID)
	require.NoError(t, err)
	assert.Empty(t, aliceSubs, "the last person to subscribe on the device owns it")

	bobSubs, err := store.ListPushSubscriptions(t.Context(), bob.ID)
	require.NoError(t, err)
	require.Len(t, bobSubs, 1)
	assert.Equal(t, endpoint, bobSubs[0].Endpoint)
}

func TestSubscribeRejectsBadSubscriptions(t *testing.T) {
	t.Parallel()

	tests := map[string]push.Subscription{
		"missing endpoint": {Keys: subscription("https://web.push.apple.com/x").Keys},
		"plain http":       subscription("http://web.push.apple.com/x"),
		"unknown host":     subscription("https://example.com/x"),
		"look-alike host":  subscription("https://web.push.apple.com.attacker.test/x"),
		"missing keys":     {Endpoint: "https://web.push.apple.com/x"},
		"partial keys": {
			Endpoint: "https://web.push.apple.com/x",
			Keys:     push.SubscriptionKeys{Auth: "auth"},
		},
	}

	for name, sub := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			store := newStore(t)
			service := newService(t, store)
			user := newUser(t, store, "Ada")

			err := service.Subscribe(t.Context(), user.ID, sub)
			require.ErrorIs(t, err, push.ErrInvalidSubscription)

			subs, listErr := store.ListPushSubscriptions(t.Context(), user.ID)
			require.NoError(t, listErr)
			assert.Empty(t, subs)
		})
	}
}

func TestSubscribeAcceptsEveryKnownPushService(t *testing.T) {
	t.Parallel()

	endpoints := []string{
		"https://fcm.googleapis.com/fcm/send/abc",
		"https://web.push.apple.com/abc",
		"https://updates.push.services.mozilla.com/wpush/v2/abc",
		"https://wns2-db5p.notify.windows.com/w/?token=abc",
	}

	store := newStore(t)
	service := newService(t, store)
	user := newUser(t, store, "Ada")

	for i, endpoint := range endpoints {
		require.NoError(t, service.Subscribe(t.Context(), user.ID, subscription(endpoint)), endpoint)
		subs, err := store.ListPushSubscriptions(t.Context(), user.ID)
		require.NoError(t, err)
		assert.Len(t, subs, i+1)
	}
}

func TestSubscribeKeepsOnlyTheNewestDevices(t *testing.T) {
	t.Parallel()

	store := newStore(t)
	service := newService(t, store)
	user := newUser(t, store, "Ada")

	for i := range push.MaxSubscriptionsPerUser + 1 {
		endpoint := "https://web.push.apple.com/sub-" + string(rune('a'+i))
		require.NoError(t, service.Subscribe(t.Context(), user.ID, subscription(endpoint)))
	}

	subs, err := store.ListPushSubscriptions(t.Context(), user.ID)
	require.NoError(t, err)
	require.Len(t, subs, push.MaxSubscriptionsPerUser)
	assert.NotEqual(t, "https://web.push.apple.com/sub-a", subs[0].Endpoint, "the oldest device is replaced")
}

func TestUnsubscribeRemovesOnlyTheCallersEndpoint(t *testing.T) {
	t.Parallel()

	store := newStore(t)
	service := newService(t, store)
	alice := newUser(t, store, "Alice")
	bob := newUser(t, store, "Bob")
	aliceEndpoint := "https://web.push.apple.com/alice"
	bobEndpoint := "https://web.push.apple.com/bob"
	require.NoError(t, service.Subscribe(t.Context(), alice.ID, subscription(aliceEndpoint)))
	require.NoError(t, service.Subscribe(t.Context(), bob.ID, subscription(bobEndpoint)))

	require.NoError(t, service.Unsubscribe(t.Context(), alice.ID, bobEndpoint))
	bobSubs, err := store.ListPushSubscriptions(t.Context(), bob.ID)
	require.NoError(t, err)
	assert.Len(t, bobSubs, 1, "another person's endpoint stays put")

	require.NoError(t, service.Unsubscribe(t.Context(), alice.ID, aliceEndpoint))
	aliceSubs, err := store.ListPushSubscriptions(t.Context(), alice.ID)
	require.NoError(t, err)
	assert.Empty(t, aliceSubs)

	require.NoError(t, service.Unsubscribe(t.Context(), alice.ID, aliceEndpoint),
		"a second unsubscribe is harmless")
}

func TestDisabledServiceRefusesSubscriptions(t *testing.T) {
	t.Parallel()

	store := newStore(t)
	service := push.New(store, zerolog.Nop(), push.Keys{}, push.Options{})
	user := newUser(t, store, "Ada")

	assert.False(t, service.Enabled())
	assert.Empty(t, service.PublicKey())
	require.ErrorIs(t,
		service.Subscribe(t.Context(), user.ID, subscription("https://web.push.apple.com/x")),
		push.ErrDisabled)
	require.ErrorIs(t,
		service.Unsubscribe(t.Context(), user.ID, "https://web.push.apple.com/x"),
		push.ErrDisabled)
}
