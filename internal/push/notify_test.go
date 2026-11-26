package push_test

import (
	"net/http/httptest"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/parlay"
	"github.com/kalverra/parlay-club/internal/push"
	"github.com/kalverra/parlay-club/internal/slips"
)

// newSlip makes one group, week, and slip with the given members.
func newSlip(t *testing.T, store *db.Store, members ...db.User) db.Slip {
	t.Helper()

	group, err := store.Create(t.Context(), members[0].ID, "Sunday Parlay Club")
	require.NoError(t, err)
	week, err := store.UpsertWeek(t.Context(), 2026, 2, 9)
	require.NoError(t, err)

	memberIDs := make([]int64, 0, len(members))
	for _, member := range members {
		memberIDs = append(memberIDs, member.ID)
	}
	slip, err := store.CreateSlip(t.Context(), group.ID, week.ID, nil, "Sunday Slate",
		db.SlateScopeSundayMain, 2, memberIDs)
	require.NoError(t, err)
	return slip
}

// sentPaths returns the request paths in the order the stand-in service saw
// them.
func sentPaths(recorder *pushRecorder) []string {
	requests := recorder.snapshot()
	paths := make([]string, 0, len(requests))
	for _, request := range requests {
		paths = append(paths, request.Path)
	}
	return paths
}

func TestNotifyLegsAddedSkipsTheActorAndNonMembers(t *testing.T) {
	t.Parallel()

	store := newStore(t)
	recorder, handler := recordPushRequests(nil)
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	ada := newUser(t, store, "Ada")
	sam := newUser(t, store, "Sam")
	kim := newUser(t, store, "Kim")
	slip := newSlip(t, store, ada, sam)
	storeSubscriptions(t, store, ada, server.URL+"/ada")
	storeSubscriptions(t, store, sam, server.URL+"/sam")
	storeSubscriptions(t, store, kim, server.URL+"/kim")
	service := serviceForServer(t, store, 0)

	service.Notify(t.Context(), slips.Event{
		Kind:      slips.EventLegsAdded,
		SlipID:    slip.ID,
		ActorID:   ada.ID,
		Count:     1,
		Selection: "Bills -3.5",
	})

	assert.ElementsMatch(t, []string{"/sam"}, sentPaths(recorder),
		"the actor and anyone outside the slip hear nothing")
}

func TestNotifySlipLockedSkipsTheActor(t *testing.T) {
	t.Parallel()

	store := newStore(t)
	recorder, handler := recordPushRequests(nil)
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	ada := newUser(t, store, "Ada")
	sam := newUser(t, store, "Sam")
	slip := newSlip(t, store, ada, sam)
	storeSubscriptions(t, store, ada, server.URL+"/ada")
	storeSubscriptions(t, store, sam, server.URL+"/sam")
	service := serviceForServer(t, store, 0)

	service.Notify(t.Context(), slips.Event{
		Kind:    slips.EventSlipLocked,
		SlipID:  slip.ID,
		ActorID: sam.ID,
	})

	assert.ElementsMatch(t, []string{"/ada"}, sentPaths(recorder))
}

func TestNotifySlipGradedReachesEveryMemberOnce(t *testing.T) {
	t.Parallel()

	store := newStore(t)
	recorder, handler := recordPushRequests(nil)
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	ada := newUser(t, store, "Ada")
	sam := newUser(t, store, "Sam")
	kim := newUser(t, store, "Kim")
	slip := newSlip(t, store, ada, sam)
	storeSubscriptions(t, store, ada, server.URL+"/ada")
	storeSubscriptions(t, store, sam, server.URL+"/sam")
	storeSubscriptions(t, store, kim, server.URL+"/kim")
	require.NoError(t, store.SetSlipGrade(t.Context(), slip.ID, parlay.Won, db.SlipGraded))
	service := serviceForServer(t, store, 0)

	event := slips.Event{Kind: slips.EventSlipGraded, SlipID: slip.ID}
	service.Notify(t.Context(), event)
	service.Notify(t.Context(), event)

	assert.ElementsMatch(t, []string{"/ada", "/sam"}, sentPaths(recorder),
		"every member hears once; a later correction stays quiet")

	reserved, err := store.ReservePushLog(t.Context(), db.PushLogGraded, slip.ID, 0, "")
	require.NoError(t, err)
	assert.False(t, reserved, "the grade is already reserved")
}

func TestNotifySlipGradedSkipsInactiveMembers(t *testing.T) {
	t.Parallel()

	store := newStore(t)
	recorder, handler := recordPushRequests(nil)
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	ada := newUser(t, store, "Ada")
	sam := newUser(t, store, "Sam")
	slip := newSlip(t, store, ada, sam)
	storeSubscriptions(t, store, ada, server.URL+"/ada")
	storeSubscriptions(t, store, sam, server.URL+"/sam")
	require.NoError(t, store.SetSlipGrade(t.Context(), slip.ID, parlay.Lost, db.SlipGraded))
	_, err := store.SetUserActive(t.Context(), sam.ID, false)
	require.NoError(t, err)
	service := serviceForServer(t, store, 0)

	service.Notify(t.Context(), slips.Event{Kind: slips.EventSlipGraded, SlipID: slip.ID})

	assert.ElementsMatch(t, []string{"/ada"}, sentPaths(recorder),
		"an inactive person gets no pushes")
}

func TestNotifyDoesNothingOnADisabledService(t *testing.T) {
	t.Parallel()

	store := newStore(t)
	recorder, handler := recordPushRequests(nil)
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	ada := newUser(t, store, "Ada")
	slip := newSlip(t, store, ada)
	storeSubscriptions(t, store, ada, server.URL+"/ada")
	service := push.New(store, zerolog.Nop(), push.Keys{}, push.Options{})

	service.Notify(t.Context(), slips.Event{Kind: slips.EventLegsAdded, SlipID: slip.ID, ActorID: 999})

	assert.Empty(t, recorder.snapshot(), "no keys means no events")
}
