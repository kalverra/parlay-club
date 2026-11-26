package reminders_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/push"
	"github.com/kalverra/parlay-club/internal/reminders"
	"github.com/kalverra/parlay-club/internal/slips"
)

// sundayKickoff is the start of a Sunday-main slip in the tests.
var sundayKickoff = time.Date(2026, 11, 8, 18, 0, 0, 0, time.UTC)

// dueMember is one active or inactive member, locked when locked is true.
func dueMember(id int64, name string, active, locked bool) db.SlipMember {
	member := db.SlipMember{User: db.User{ID: id, Name: name, Active: active}}
	if locked {
		at := sundayKickoff.Add(-2 * time.Hour)
		member.LockedAt = &at
	}
	return member
}

// dueGame is one scheduled game.
func dueGame(id string, kickoff time.Time) db.Game {
	return db.Game{ID: id, Kickoff: kickoff, Status: db.GameScheduled}
}

// dueLeg is one pick owned by userID.
func dueLeg(userID int64) db.Leg {
	return db.Leg{UserID: userID, Selection: "BUF ML"}
}

func TestDueRemindsPeopleWhoStillHaveWork(t *testing.T) {
	t.Parallel()

	slip := db.Slip{
		ID:            7,
		Name:          "Sunday Slate",
		LegsPerMember: 2,
		Status:        db.SlipOpen,
		SlateScope:    db.SlateScopeSundayMain,
	}
	games := []db.Game{dueGame("sun", sundayKickoff)}
	members := []db.SlipMember{
		dueMember(1, "Ada", true, false),
		dueMember(2, "Ben", true, false),
		dueMember(3, "Cleo", true, true),
		dueMember(4, "Dex", false, false),
	}
	legs := []db.Leg{dueLeg(2), dueLeg(2)}

	got := reminders.Due(slip, members, legs, games, sundayKickoff, sundayKickoff.Add(-20*time.Minute))

	assert.Equal(t, []reminders.Reminder{
		{UserID: 1, Step: slips.StepPick, Need: 2},
		{UserID: 2, Step: slips.StepLockIn},
	}, got, "a lock_in member hears, a locked member waits, and an inactive member is skipped")
}

func TestDueLeavesOutEveryoneElse(t *testing.T) {
	t.Parallel()

	slip := db.Slip{
		ID:            7,
		Name:          "Sunday Slate",
		LegsPerMember: 2,
		Status:        db.SlipOpen,
		SlateScope:    db.SlateScopeSundayMain,
	}
	games := []db.Game{dueGame("sun", sundayKickoff)}
	allLocked := []db.SlipMember{
		dueMember(1, "Ada", true, true),
		dueMember(2, "Ben", true, true),
	}
	legs := []db.Leg{dueLeg(1), dueLeg(1), dueLeg(2), dueLeg(2)}

	got := reminders.Due(slip, allLocked, legs, games, sundayKickoff, sundayKickoff.Add(-20*time.Minute))
	assert.Empty(t, got, "export members have nothing left to do")

	locked := slip
	locked.Status = db.SlipLocked
	got = reminders.Due(locked, allLocked, legs, games, sundayKickoff, sundayKickoff.Add(-20*time.Minute))
	assert.Empty(t, got, "a locked slip is stale")
}

func TestDueHonorsTheFireWindow(t *testing.T) {
	t.Parallel()

	slip := db.Slip{
		ID:            7,
		LegsPerMember: 2,
		Status:        db.SlipOpen,
		SlateScope:    db.SlateScopeSundayMain,
	}
	games := []db.Game{dueGame("sun", sundayKickoff)}
	members := []db.SlipMember{dueMember(1, "Ada", true, false)}

	for _, now := range []time.Time{
		sundayKickoff.Add(-35 * time.Minute),
		sundayKickoff.Add(-31 * time.Minute),
		sundayKickoff.Add(-9 * time.Minute),
		sundayKickoff.Add(-5 * time.Minute),
	} {
		assert.Empty(t, reminders.Due(slip, members, nil, games, sundayKickoff, now), now)
	}

	for _, now := range []time.Time{
		sundayKickoff.Add(-30 * time.Minute),
		sundayKickoff.Add(-20 * time.Minute),
		sundayKickoff.Add(-10 * time.Minute),
	} {
		assert.Len(t, reminders.Due(slip, members, nil, games, sundayKickoff, now), 1, now)
	}
}

func TestDueIgnoresAMovedStartAndLaterKickoffs(t *testing.T) {
	t.Parallel()

	later := sundayKickoff.Add(3 * time.Hour)
	slip := db.Slip{
		ID:            7,
		LegsPerMember: 2,
		Status:        db.SlipOpen,
		SlateScope:    db.SlateScopeSundayMain,
	}
	games := []db.Game{dueGame("early", sundayKickoff), dueGame("late", later)}
	members := []db.SlipMember{dueMember(1, "Ada", true, false)}

	assert.Empty(t, reminders.Due(
		slip, members, nil, games, later, later.Add(-20*time.Minute),
	), "a later kickoff is not the slip start")

	past := sundayKickoff.Add(30 * time.Minute)
	assert.Empty(t, reminders.Due(
		slip, members, nil, games, sundayKickoff, past,
	), "a start that already passed cannot send")
}

func TestDueUsesOnlyTheSlipsScopedGames(t *testing.T) {
	t.Parallel()

	thursday := time.Date(2026, 11, 6, 1, 20, 0, 0, time.UTC)
	games := []db.Game{dueGame("thu", thursday), dueGame("sun", sundayKickoff)}
	slip := db.Slip{
		ID:            7,
		LegsPerMember: 2,
		Status:        db.SlipOpen,
		SlateScope:    db.SlateScopeCustom,
		GameIDs:       []string{"sun"},
	}
	members := []db.SlipMember{dueMember(1, "Ada", true, false)}

	assert.Empty(t, reminders.Due(
		slip, members, nil, games, thursday, thursday.Add(-20*time.Minute),
	), "Thursday is outside the custom slip")

	assert.Len(t, reminders.Due(
		slip, members, nil, games, sundayKickoff, sundayKickoff.Add(-20*time.Minute),
	), 1)
}

// recordingSender records every send.
type recordingSender struct {
	enabled  bool
	calls    []int64
	messages []push.Message
}

func (s *recordingSender) Enabled() bool { return s.enabled }

func (s *recordingSender) SendToUsers(_ context.Context, userIDs []int64, m push.Message) {
	s.calls = append(s.calls, userIDs...)
	s.messages = append(s.messages, m)
}

func TestFireSendsOncePerPersonPerSlip(t *testing.T) {
	t.Parallel()

	store, slip, users := seedReminderSlip(t, []time.Time{sundayKickoff})
	sender := &recordingSender{enabled: true}
	now := sundayKickoff.Add(-20 * time.Minute)
	service := reminders.New(reminders.Deps{
		Store:  store,
		Sender: sender,
		Now:    func() time.Time { return now },
		Logger: zerolog.Nop(),
	})

	base, err := store.GamesForWeek(t.Context(), slip.WeekID)
	require.NoError(t, err)
	start, ok := slips.FirstKickoff(slip, base)
	require.True(t, ok)

	report, err := service.Fire(t.Context(), slip.ID, start)
	require.NoError(t, err)
	assert.Equal(t, len(users), report.Sent)
	assert.ElementsMatch(t, []int64{users[0].ID, users[1].ID}, sender.calls)

	report, err = service.Fire(t.Context(), slip.ID, start)
	require.NoError(t, err)
	assert.Zero(t, report.Sent, "the push_log rows stop a retry")
	assert.Len(t, sender.calls, len(users), "no second send")

	require.NotEmpty(t, sender.messages)
	assert.Equal(t, slip.Name, sender.messages[0].Title)
	assert.Equal(t, "/slips/"+strconv.FormatInt(slip.ID, 10), sender.messages[0].URL)
	assert.Equal(t, "slip-"+strconv.FormatInt(slip.ID, 10)+"-reminder", sender.messages[0].Tag)
	assert.Equal(t, "Kickoff in 20 minutes. You still need 2 legs.", sender.messages[0].Body)
}

func TestFireSkipsStaleAndUnconfiguredFires(t *testing.T) {
	t.Parallel()

	store, slip, _ := seedReminderSlip(t, []time.Time{sundayKickoff})
	now := sundayKickoff.Add(-20 * time.Minute)
	sender := &recordingSender{enabled: true}
	service := reminders.New(reminders.Deps{
		Store:  store,
		Sender: sender,
		Now:    func() time.Time { return now },
		Logger: zerolog.Nop(),
	})

	report, err := service.Fire(t.Context(), slip.ID, sundayKickoff.Add(time.Hour))
	require.NoError(t, err)
	assert.Equal(t, "stale", report.Skipped)

	early := reminders.New(reminders.Deps{
		Store:  store,
		Sender: sender,
		Now:    func() time.Time { return sundayKickoff.Add(-35 * time.Minute) },
		Logger: zerolog.Nop(),
	})
	report, err = early.Fire(t.Context(), slip.ID, sundayKickoff)
	require.NoError(t, err)
	assert.Equal(t, "stale", report.Skipped)

	off := reminders.New(reminders.Deps{
		Store:  store,
		Sender: &recordingSender{enabled: false},
		Now:    func() time.Time { return now },
		Logger: zerolog.Nop(),
	})
	report, err = off.Fire(t.Context(), slip.ID, sundayKickoff)
	require.NoError(t, err)
	assert.Equal(t, "disabled", report.Skipped)
	assert.Empty(t, sender.calls)
}

func TestSendNowSkipsTheWindow(t *testing.T) {
	t.Parallel()

	store, slip, users := seedReminderSlip(t, []time.Time{sundayKickoff})
	sender := &recordingSender{enabled: true}
	service := reminders.New(reminders.Deps{
		Store:  store,
		Sender: sender,
		Now:    func() time.Time { return sundayKickoff.Add(-6 * time.Hour) },
		Logger: zerolog.Nop(),
	})

	report, err := service.SendNow(t.Context(), slip.ID)
	require.NoError(t, err)
	assert.Equal(t, len(users), report.Sent)
}
