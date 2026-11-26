package db_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/db"
)

// pushSub is a stored subscription for one user.
func pushSub(user db.User, endpoint string) db.PushSubscription {
	return db.PushSubscription{
		UserID:   user.ID,
		Endpoint: endpoint,
		P256dh:   "p256dh-" + endpoint,
		Auth:     "auth-" + endpoint,
	}
}

func TestSavePushSubscriptionReassignsAnExistingEndpoint(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	alice := createUser(t, store, "Alice")
	bob := createUser(t, store, "Bob")
	endpoint := "https://fcm.googleapis.com/fcm/send/device"

	require.NoError(t, store.SavePushSubscription(t.Context(), pushSub(alice, endpoint), 10))
	require.NoError(t, store.SavePushSubscription(t.Context(), pushSub(bob, endpoint), 10))

	aliceSubs, err := store.ListPushSubscriptions(t.Context(), alice.ID)
	require.NoError(t, err)
	assert.Empty(t, aliceSubs)

	bobSubs, err := store.ListPushSubscriptions(t.Context(), bob.ID)
	require.NoError(t, err)
	require.Len(t, bobSubs, 1)
	assert.Equal(t, bob.ID, bobSubs[0].UserID)
	assert.Equal(t, "p256dh-"+endpoint, bobSubs[0].P256dh, "the keys are refreshed")
	assert.WithinDuration(t, time.Now(), bobSubs[0].CreatedAt, time.Minute)
}

func TestSavePushSubscriptionDropsTheOldestPastTheLimit(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	user := createUser(t, store, "Ada")

	for i := range 4 {
		endpoint := "https://web.push.apple.com/device-" + string(rune('a'+i))
		require.NoError(t, store.SavePushSubscription(t.Context(), pushSub(user, endpoint), 2))
	}

	subs, err := store.ListPushSubscriptions(t.Context(), user.ID)
	require.NoError(t, err)
	require.Len(t, subs, 2)
	assert.Equal(t, "https://web.push.apple.com/device-c", subs[0].Endpoint)
	assert.Equal(t, "https://web.push.apple.com/device-d", subs[1].Endpoint)
}

func TestDeletePushSubscriptionLeavesAnotherUsersEndpoint(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	alice := createUser(t, store, "Alice")
	bob := createUser(t, store, "Bob")
	endpoint := "https://web.push.apple.com/shared"
	require.NoError(t, store.SavePushSubscription(t.Context(), pushSub(alice, endpoint), 10))

	removed, err := store.DeletePushSubscription(t.Context(), bob.ID, endpoint)
	require.NoError(t, err)
	assert.False(t, removed)

	removed, err = store.DeletePushSubscription(t.Context(), alice.ID, endpoint)
	require.NoError(t, err)
	assert.True(t, removed)

	subs, err := store.ListPushSubscriptions(t.Context(), alice.ID)
	require.NoError(t, err)
	assert.Empty(t, subs)
}

func TestDeletePushSubscriptionByID(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	user := createUser(t, store, "Ada")
	require.NoError(t, store.SavePushSubscription(t.Context(), pushSub(user, "https://web.push.apple.com/a"), 10))
	require.NoError(t, store.SavePushSubscription(t.Context(), pushSub(user, "https://web.push.apple.com/b"), 10))
	subs, err := store.ListPushSubscriptions(t.Context(), user.ID)
	require.NoError(t, err)
	require.Len(t, subs, 2)

	require.NoError(t, store.DeletePushSubscriptionByID(t.Context(), subs[0].ID))

	remaining, err := store.ListPushSubscriptions(t.Context(), user.ID)
	require.NoError(t, err)
	require.Len(t, remaining, 1)
	assert.Equal(t, subs[1].ID, remaining[0].ID)
}

func TestPushSubscriptionsForUsersSkipsInactiveUsers(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	active := createUser(t, store, "Active")
	inactive := createUser(t, store, "Inactive")
	require.NoError(t, store.SavePushSubscription(t.Context(), pushSub(active, "https://web.push.apple.com/a"), 10))
	require.NoError(t, store.SavePushSubscription(t.Context(), pushSub(inactive, "https://web.push.apple.com/b"), 10))
	_, err := store.SetUserActive(t.Context(), inactive.ID, false)
	require.NoError(t, err)

	subs, err := store.PushSubscriptionsForUsers(t.Context(), []int64{active.ID, inactive.ID})
	require.NoError(t, err)
	require.Len(t, subs, 1)
	assert.Equal(t, active.ID, subs[0].UserID)
}

func TestPushSubscriptionsForUsersWithNoIDs(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)

	subs, err := store.PushSubscriptionsForUsers(t.Context(), nil)
	require.NoError(t, err)
	assert.Empty(t, subs)
}

func TestPushSubscriptionsCascadeWithTheirUser(t *testing.T) {
	t.Parallel()

	store, database := openTestStore(t)
	user := createUser(t, store, "Ada")
	require.NoError(t, store.SavePushSubscription(t.Context(), pushSub(user, "https://web.push.apple.com/a"), 10))

	_, err := database.ExecContext(t.Context(), "DELETE FROM users WHERE id = ?", user.ID)
	require.NoError(t, err)

	subs, err := store.ListPushSubscriptions(t.Context(), user.ID)
	require.NoError(t, err)
	assert.Empty(t, subs)
}
