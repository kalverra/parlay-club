package slips_test

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/parlay"
	"github.com/kalverra/parlay-club/internal/slips"
)

// testNow is the injected clock every rule test runs on.
var testNow = time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)

// recordingPublisher keeps the topics the rules publish.
type recordingPublisher struct {
	topics []string
}

func (p *recordingPublisher) Publish(topic string) {
	p.topics = append(p.topics, topic)
}

// fixedClock is the injected clock.
type fixedClock struct {
	at time.Time
}

func (c fixedClock) Now() time.Time {
	return c.at
}

// fixture is one group, two weeks, a slip, and the games the rules need.
type fixture struct {
	service   *slips.Service
	store     *db.Store
	database  *sql.DB
	published *recordingPublisher

	ada   db.User
	sam   db.User
	kim   db.User
	group db.Group
	week  db.Week
	slip  db.Slip

	upcoming string
	started  string
	atNow    string

	pastWeek db.Week
	pastSlip db.Slip
	pastGame string
}

// newFixture opens a fresh migrated database and builds the shared setup.
func newFixture(t *testing.T) *fixture {
	t.Helper()

	database, err := db.Open(filepath.Join(t.TempDir(), "parlay.db"))
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, database.Close()) })
	require.NoError(t, db.Migrate(t.Context(), database))
	store := db.NewStore(database)
	t.Cleanup(func() { assert.NoError(t, store.Close()) })

	f := &fixture{store: store, database: database, published: &recordingPublisher{}}
	f.ada = createUser(t, store, "Ada")
	f.sam = createUser(t, store, "Sam")
	f.kim = createUser(t, store, "Kim")

	f.group = createGroup(t, store, f.ada)
	f.week = createWeek(t, store, 9)
	f.slip = createSlip(t, store, f.group, f.week, 2, f.ada, f.sam)
	f.upcoming = createGame(t, database, f.week, "upcoming", "2026-11-08T18:00:00Z")
	f.started = createGame(t, database, f.week, "started", "2026-11-08T11:00:00Z")
	f.atNow = createGame(t, database, f.week, "at-now", "2026-11-08T12:00:00Z")

	f.pastWeek = createWeek(t, store, 8)
	f.pastGame = createGame(t, database, f.pastWeek, "past", "2026-11-02T18:00:00Z")
	f.pastSlip = createSlip(t, store, f.group, f.pastWeek, 2, f.ada)

	f.service = slips.New(store, f.published, fixedClock{at: testNow}.Now)
	return f
}

// clearHistory forgets the activity and publishes an arrangement made through
// the rules, so a test can assert what the next mutation writes.
func (f *fixture) clearHistory(t *testing.T) {
	t.Helper()

	f.published.topics = nil
	_, err := f.database.ExecContext(t.Context(), "DELETE FROM activity")
	require.NoError(t, err)
}

func createUser(t *testing.T, store *db.Store, name string) db.User {
	t.Helper()

	user, err := store.CreateUserWithIdentity(t.Context(), name, "", "", "")
	require.NoError(t, err)
	return user
}

func createGroup(t *testing.T, store *db.Store, owner db.User) db.Group {
	t.Helper()

	group, err := store.Create(t.Context(), owner.ID, "Sunday Parlay Club")
	require.NoError(t, err)
	return group
}

func createWeek(t *testing.T, store *db.Store, weekNum int) db.Week {
	t.Helper()

	week, err := store.UpsertWeek(t.Context(), 2026, 2, weekNum)
	require.NoError(t, err)
	return week
}

func createSlip(
	t *testing.T,
	store *db.Store,
	group db.Group,
	week db.Week,
	legsPerMember int,
	members ...db.User,
) db.Slip {
	t.Helper()

	ids := make([]int64, 0, len(members))
	for _, member := range members {
		ids = append(ids, member.ID)
	}
	slip, err := store.CreateSlip(t.Context(), group.ID, week.ID, nil, "Week "+strconv.Itoa(week.WeekNum),
		db.SlateScopeSundayMain, legsPerMember, ids)
	require.NoError(t, err)
	return slip
}

// createGame inserts a game row directly, the way slate-and-lines will.
func createGame(t *testing.T, database *sql.DB, week db.Week, id, kickoff string) string {
	t.Helper()

	_, err := database.ExecContext(t.Context(), `
		INSERT INTO games (
			id, week_id, home_team, away_team, home_name, away_name,
			home_espn_id, away_espn_id, kickoff_utc, status, is_indoor, updated_at
		) VALUES (?, ?, 'BUF', 'MIA', 'Buffalo Bills', 'Miami Dolphins', '2', '15',
			?, 'SCHEDULED', 1, '2026-11-07T00:00:00Z')`,
		id, week.ID, kickoff)
	require.NoError(t, err)
	return id
}

// addLeg writes a leg directly, bypassing the rules under test.
func addLeg(
	t *testing.T,
	store *db.Store,
	slip db.Slip,
	owner db.User,
	gameID *string,
	selection string,
) db.Leg {
	t.Helper()

	leg, err := store.CreateLeg(t.Context(), db.Leg{
		SlipID:     slip.ID,
		UserID:     owner.ID,
		GameID:     gameID,
		MarketType: "ML",
		Selection:  selection,
	})
	require.NoError(t, err)
	return leg
}

// addSpreadLeg writes a spread leg with a line, bypassing the rules under test.
func addSpreadLeg(
	t *testing.T,
	store *db.Store,
	slip db.Slip,
	owner db.User,
	selection string,
	line float64,
) db.Leg {
	t.Helper()

	leg, err := store.CreateLeg(t.Context(), db.Leg{
		SlipID:     slip.ID,
		UserID:     owner.ID,
		MarketType: "SPREAD",
		Selection:  selection,
		Team:       new("BUF"),
		Line:       &line,
	})
	require.NoError(t, err)
	return leg
}

// gradeLeg sets one leg's result through the service, then forgets the
// activity and publish it made so the next call's traces stand alone.
func gradeLeg(t *testing.T, f *fixture, actorID, legID int64, result parlay.Result) {
	t.Helper()

	_, err := f.service.SetLegResult(t.Context(), actorID, legID, result)
	require.NoError(t, err)
	f.clearHistory(t)
}

// activity returns a slip's feed, newest first.
func activity(t *testing.T, store *db.Store, slipID int64) []db.Activity {
	t.Helper()

	entries, err := store.ActivityForSlip(t.Context(), slipID)
	require.NoError(t, err)
	return entries
}

// reload returns the slip as stored.
func reload(t *testing.T, store *db.Store, slipID int64) db.Slip {
	t.Helper()

	slip, err := store.SlipByID(t.Context(), slipID)
	require.NoError(t, err)
	return slip
}

// assertRefused checks a rule error left the database and the publisher alone.
func assertRefused(t *testing.T, f *fixture, err, want error) {
	t.Helper()

	require.ErrorIs(t, err, want)
	assert.Empty(t, activity(t, f.store, f.slip.ID), "a refused mutation writes no activity")
	assert.Empty(t, f.published.topics, "a refused mutation publishes nothing")
}

// assertAccepted checks a mutation wrote exactly one activity entry and
// published the slip once.
func assertAccepted(t *testing.T, f *fixture, err error) {
	t.Helper()

	require.NoError(t, err)
	entries := activity(t, f.store, f.slip.ID)
	require.Len(t, entries, 1, "one mutation writes one activity entry")
	assert.Equal(t, []string{fmt.Sprintf("slip:%d", f.slip.ID)}, f.published.topics)
}

// TestRuleOneOnlyMembersTouchTheirOwnLegs covers rule 1.
func TestRuleOneOnlyMembersTouchTheirOwnLegs(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		act  func(t *testing.T, f *fixture) error
		want error
	}{
		"a non-member cannot add a leg": {
			act: func(t *testing.T, f *fixture) error {
				t.Helper()
				_, err := f.service.AddLeg(t.Context(), f.kim.ID, db.Leg{
					SlipID: f.slip.ID, MarketType: "ML", Selection: "BUF ML",
				})
				return err
			},
			want: slips.ErrNotMember,
		},
		"a member cannot edit another member's leg": {
			act: func(t *testing.T, f *fixture) error {
				t.Helper()
				leg := addLeg(t, f.store, f.slip, f.sam, nil, "MIA ML")
				_, err := f.service.UpdateLeg(t.Context(), f.ada.ID, db.Leg{
					ID: leg.ID, MarketType: "ML", Selection: "MIA +3.5",
				})
				return err
			},
			want: slips.ErrNotOwner,
		},
		"a member cannot delete another member's leg": {
			act: func(t *testing.T, f *fixture) error {
				t.Helper()
				leg := addLeg(t, f.store, f.slip, f.sam, nil, "MIA ML")
				return f.service.DeleteLeg(t.Context(), f.ada.ID, leg.ID)
			},
			want: slips.ErrNotOwner,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			f := newFixture(t)
			assertRefused(t, f, test.act(t, f), test.want)
		})
	}
}

// TestRuleTwoLegLimit covers rule 2.
func TestRuleTwoLegLimit(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		act  func(t *testing.T, f *fixture) error
		want error
	}{
		"a member at the limit cannot add": {
			act: func(t *testing.T, f *fixture) error {
				t.Helper()
				addLeg(t, f.store, f.slip, f.ada, nil, "BUF -3.5")
				addLeg(t, f.store, f.slip, f.ada, nil, "BUF ML")
				_, err := f.service.AddLeg(t.Context(), f.ada.ID, db.Leg{
					SlipID: f.slip.ID, MarketType: "ML", Selection: "MIA ML",
				})
				return err
			},
			want: slips.ErrLegLimit,
		},
		"one member's legs do not count against another": {
			act: func(t *testing.T, f *fixture) error {
				t.Helper()
				addLeg(t, f.store, f.slip, f.ada, nil, "BUF -3.5")
				addLeg(t, f.store, f.slip, f.ada, nil, "BUF ML")
				_, err := f.service.AddLeg(t.Context(), f.sam.ID, db.Leg{
					SlipID: f.slip.ID, MarketType: "ML", Selection: "MIA ML",
				})
				return err
			},
			want: nil,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			f := newFixture(t)
			err := test.act(t, f)
			if test.want != nil {
				assertRefused(t, f, err, test.want)
				return
			}
			assertAccepted(t, f, err)
		})
	}
}

// TestRuleThreeKickoffLocks covers rule 3.
func TestRuleThreeKickoffLocks(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		act  func(t *testing.T, f *fixture) error
		want error
	}{
		"a started game cannot take a new leg": {
			act: func(t *testing.T, f *fixture) error {
				t.Helper()
				_, err := f.service.AddLeg(t.Context(), f.ada.ID, db.Leg{
					SlipID: f.slip.ID, GameID: &f.started, MarketType: "ML", Selection: "BUF ML",
				})
				return err
			},
			want: slips.ErrKickoffLocked,
		},
		"a game kicking off exactly now counts as started": {
			act: func(t *testing.T, f *fixture) error {
				t.Helper()
				_, err := f.service.AddLeg(t.Context(), f.ada.ID, db.Leg{
					SlipID: f.slip.ID, GameID: &f.atNow, MarketType: "ML", Selection: "BUF ML",
				})
				return err
			},
			want: slips.ErrKickoffLocked,
		},
		"an upcoming game takes a leg": {
			act: func(t *testing.T, f *fixture) error {
				t.Helper()
				_, err := f.service.AddLeg(t.Context(), f.ada.ID, db.Leg{
					SlipID: f.slip.ID, GameID: &f.upcoming, MarketType: "ML", Selection: "BUF ML",
				})
				return err
			},
			want: nil,
		},
		"a kickoff-locked leg cannot be edited": {
			act: func(t *testing.T, f *fixture) error {
				t.Helper()
				leg := addLeg(t, f.store, f.slip, f.ada, &f.started, "BUF ML")
				_, err := f.service.UpdateLeg(t.Context(), f.ada.ID, db.Leg{
					ID: leg.ID, GameID: &f.started, MarketType: "ML", Selection: "BUF -3.5",
				})
				return err
			},
			want: slips.ErrKickoffLocked,
		},
		"a kickoff-locked leg cannot be deleted": {
			act: func(t *testing.T, f *fixture) error {
				t.Helper()
				leg := addLeg(t, f.store, f.slip, f.ada, &f.started, "BUF ML")
				return f.service.DeleteLeg(t.Context(), f.ada.ID, leg.ID)
			},
			want: slips.ErrKickoffLocked,
		},
		"an upcoming leg can be edited": {
			act: func(t *testing.T, f *fixture) error {
				t.Helper()
				leg := addLeg(t, f.store, f.slip, f.ada, &f.upcoming, "BUF ML")
				_, err := f.service.UpdateLeg(t.Context(), f.ada.ID, db.Leg{
					ID: leg.ID, GameID: &f.upcoming, MarketType: "ML", Selection: "BUF -3.5",
				})
				return err
			},
			want: nil,
		},
		"a leg without a game has no kickoff": {
			act: func(t *testing.T, f *fixture) error {
				t.Helper()
				_, err := f.service.AddLeg(t.Context(), f.ada.ID, db.Leg{
					SlipID: f.slip.ID, MarketType: "OTHER", Selection: "Coin toss",
				})
				return err
			},
			want: nil,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			f := newFixture(t)
			err := test.act(t, f)
			if test.want != nil {
				assertRefused(t, f, err, test.want)
				return
			}
			assertAccepted(t, f, err)
		})
	}
}

// TestRuleFourLockIn covers rule 4.
func TestRuleFourLockIn(t *testing.T) {
	t.Parallel()

	t.Run("lock in needs exactly legs_per_member legs", func(t *testing.T) {
		t.Parallel()

		f := newFixture(t)
		addLeg(t, f.store, f.slip, f.ada, nil, "BUF -3.5")

		err := f.service.LockIn(t.Context(), f.ada.ID, f.slip.ID)

		assertRefused(t, f, err, slips.ErrLegsNotReady)
	})

	t.Run("too many legs also blocks lock in", func(t *testing.T) {
		t.Parallel()

		f := newFixture(t)
		addLeg(t, f.store, f.slip, f.ada, nil, "BUF -3.5")
		addLeg(t, f.store, f.slip, f.ada, nil, "BUF ML")
		addLeg(t, f.store, f.slip, f.ada, nil, "BUF@MIA Over 47.5")

		err := f.service.LockIn(t.Context(), f.ada.ID, f.slip.ID)

		assertRefused(t, f, err, slips.ErrLegsNotReady)
	})

	t.Run("lock in sets locked_at and writes one entry", func(t *testing.T) {
		t.Parallel()

		f := newFixture(t)
		addLeg(t, f.store, f.slip, f.ada, nil, "BUF -3.5")
		addLeg(t, f.store, f.slip, f.ada, nil, "BUF ML")

		err := f.service.LockIn(t.Context(), f.ada.ID, f.slip.ID)

		assertAccepted(t, f, err)
		members, err := f.store.SlipMembers(t.Context(), f.slip.ID)
		require.NoError(t, err)
		require.NotNil(t, members[0].LockedAt)
		assert.True(t, testNow.Equal(*members[0].LockedAt))
		assert.Nil(t, members[1].LockedAt, "Sam is not locked in")
		assert.Equal(t, db.SlipOpen, reload(t, f.store, f.slip.ID).Status,
			"the slip stays open while a member is out")
		assert.Equal(t, "Ada locked in", activity(t, f.store, f.slip.ID)[0].Message)
	})

	t.Run("locking in twice changes nothing", func(t *testing.T) {
		t.Parallel()

		f := newFixture(t)
		addLeg(t, f.store, f.slip, f.ada, nil, "BUF -3.5")
		addLeg(t, f.store, f.slip, f.ada, nil, "BUF ML")
		require.NoError(t, f.service.LockIn(t.Context(), f.ada.ID, f.slip.ID))
		f.clearHistory(t)

		err := f.service.LockIn(t.Context(), f.ada.ID, f.slip.ID)

		require.NoError(t, err)
		assert.Empty(t, activity(t, f.store, f.slip.ID), "a second lock in writes nothing")
		assert.Empty(t, f.published.topics, "a second lock in publishes nothing")
	})

	t.Run("a locked-in member cannot add a leg", func(t *testing.T) {
		t.Parallel()

		f := newFixture(t)
		addLeg(t, f.store, f.slip, f.ada, nil, "BUF -3.5")
		addLeg(t, f.store, f.slip, f.ada, nil, "BUF ML")
		require.NoError(t, f.service.LockIn(t.Context(), f.ada.ID, f.slip.ID))
		f.clearHistory(t)

		_, err := f.service.AddLeg(t.Context(), f.ada.ID, db.Leg{
			SlipID: f.slip.ID, MarketType: "ML", Selection: "MIA ML",
		})

		assertRefused(t, f, err, slips.ErrLockedIn)
	})

	t.Run("a locked-in member cannot edit a leg", func(t *testing.T) {
		t.Parallel()

		f := newFixture(t)
		first := addLeg(t, f.store, f.slip, f.ada, nil, "BUF -3.5")
		addLeg(t, f.store, f.slip, f.ada, nil, "BUF ML")
		require.NoError(t, f.service.LockIn(t.Context(), f.ada.ID, f.slip.ID))
		f.clearHistory(t)

		_, err := f.service.UpdateLeg(t.Context(), f.ada.ID, db.Leg{
			ID: first.ID, MarketType: "ML", Selection: "BUF -7",
		})

		assertRefused(t, f, err, slips.ErrLockedIn)
	})

	t.Run("a locked-in member cannot delete a leg", func(t *testing.T) {
		t.Parallel()

		f := newFixture(t)
		first := addLeg(t, f.store, f.slip, f.ada, nil, "BUF -3.5")
		addLeg(t, f.store, f.slip, f.ada, nil, "BUF ML")
		require.NoError(t, f.service.LockIn(t.Context(), f.ada.ID, f.slip.ID))
		f.clearHistory(t)

		err := f.service.DeleteLeg(t.Context(), f.ada.ID, first.ID)

		assertRefused(t, f, err, slips.ErrLockedIn)
	})
}

// TestRuleFiveSlipLocksWhenEveryoneIsIn covers rule 5.
func TestRuleFiveSlipLocksWhenEveryoneIsIn(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	for _, member := range []db.User{f.ada, f.sam} {
		addLeg(t, f.store, f.slip, member, nil, member.Name+" leg one")
		addLeg(t, f.store, f.slip, member, nil, member.Name+" leg two")
	}

	require.NoError(t, f.service.LockIn(t.Context(), f.ada.ID, f.slip.ID))
	assert.Equal(t, db.SlipOpen, reload(t, f.store, f.slip.ID).Status)

	require.NoError(t, f.service.LockIn(t.Context(), f.sam.ID, f.slip.ID))

	assert.Equal(t, db.SlipLocked, reload(t, f.store, f.slip.ID).Status)
	members, err := f.store.SlipMembers(t.Context(), f.slip.ID)
	require.NoError(t, err)
	for _, member := range members {
		require.NotNil(t, member.LockedAt, member.User.Name)
	}
}

// TestRuleSixUnlock covers rule 6.
func TestRuleSixUnlock(t *testing.T) {
	t.Parallel()

	t.Run("unlock clears every lock and reopens the slip", func(t *testing.T) {
		t.Parallel()

		f := newFixture(t)
		for _, member := range []db.User{f.ada, f.sam} {
			require.NoError(t, f.store.SetSlipMemberLock(t.Context(), f.slip.ID, member.ID, &testNow))
		}
		require.NoError(t, f.store.SetSlipStatus(t.Context(), f.slip.ID, db.SlipLocked))

		err := f.service.Unlock(t.Context(), f.sam.ID, f.slip.ID)

		assertAccepted(t, f, err)
		members, err := f.store.SlipMembers(t.Context(), f.slip.ID)
		require.NoError(t, err)
		for _, member := range members {
			assert.Nil(t, member.LockedAt, member.User.Name)
		}
		assert.Equal(t, db.SlipOpen, reload(t, f.store, f.slip.ID).Status)
		assert.Equal(t, "unlocked by Sam", activity(t, f.store, f.slip.ID)[0].Message)
	})

	t.Run("one lock in unlocks an open slip", func(t *testing.T) {
		t.Parallel()

		f := newFixture(t)
		require.NoError(t, f.store.SetSlipMemberLock(t.Context(), f.slip.ID, f.ada.ID, &testNow))

		err := f.service.Unlock(t.Context(), f.sam.ID, f.slip.ID)

		assertAccepted(t, f, err)
		members, err := f.store.SlipMembers(t.Context(), f.slip.ID)
		require.NoError(t, err)
		assert.Nil(t, members[0].LockedAt)
		assert.Equal(t, db.SlipOpen, reload(t, f.store, f.slip.ID).Status)
	})

	t.Run("unlock with nobody locked in is refused", func(t *testing.T) {
		t.Parallel()

		f := newFixture(t)
		assertRefused(t, f,
			f.service.Unlock(t.Context(), f.sam.ID, f.slip.ID), slips.ErrNotLocked)
	})

	t.Run("a non-member cannot unlock", func(t *testing.T) {
		t.Parallel()

		f := newFixture(t)
		require.NoError(t, f.store.SetSlipMemberLock(t.Context(), f.slip.ID, f.ada.ID, &testNow))

		assertRefused(t, f,
			f.service.Unlock(t.Context(), f.kim.ID, f.slip.ID), slips.ErrNotMember)
	})
}

// TestRuleSevenSettingsOnlyWhileOpen covers rule 7.
func TestRuleSevenSettingsOnlyWhileOpen(t *testing.T) {
	t.Parallel()

	lockSlip := func(t *testing.T, f *fixture) {
		t.Helper()
		require.NoError(t, f.store.SetSlipStatus(t.Context(), f.slip.ID, db.SlipLocked))
	}

	tests := map[string]struct {
		arrange func(t *testing.T, f *fixture)
		act     func(t *testing.T, f *fixture) error
		want    error
	}{
		"a locked slip cannot add members": {
			arrange: lockSlip,
			act: func(t *testing.T, f *fixture) error {
				t.Helper()
				return f.service.AddMember(t.Context(), f.ada.ID, f.slip.ID, f.kim.ID)
			},
			want: slips.ErrSlipNotOpen,
		},
		"a locked slip cannot remove members": {
			arrange: lockSlip,
			act: func(t *testing.T, f *fixture) error {
				t.Helper()
				return f.service.RemoveMember(t.Context(), f.ada.ID, f.slip.ID, f.sam.ID)
			},
			want: slips.ErrSlipNotOpen,
		},
		"a locked slip cannot change legs per member": {
			arrange: lockSlip,
			act: func(t *testing.T, f *fixture) error {
				t.Helper()
				_, err := f.service.UpdateSlip(t.Context(), f.ada.ID, f.slip.ID, f.slip.Name, 3)
				return err
			},
			want: slips.ErrSlipNotOpen,
		},
		"a member with legs cannot be removed": {
			arrange: func(t *testing.T, f *fixture) {
				t.Helper()
				addLeg(t, f.store, f.slip, f.sam, nil, "MIA ML")
			},
			act: func(t *testing.T, f *fixture) error {
				t.Helper()
				return f.service.RemoveMember(t.Context(), f.ada.ID, f.slip.ID, f.sam.ID)
			},
			want: slips.ErrMemberHasLegs,
		},
		"the limit cannot drop below a member's legs": {
			arrange: func(t *testing.T, f *fixture) {
				t.Helper()
				addLeg(t, f.store, f.slip, f.sam, nil, "MIA ML")
				addLeg(t, f.store, f.slip, f.sam, nil, "MIA +3.5")
			},
			act: func(t *testing.T, f *fixture) error {
				t.Helper()
				_, err := f.service.UpdateSlip(t.Context(), f.ada.ID, f.slip.ID, f.slip.Name, 1)
				return err
			},
			want: slips.ErrLegsPerMemberTooLow,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			f := newFixture(t)
			if test.arrange != nil {
				test.arrange(t, f)
			}
			assertRefused(t, f, test.act(t, f), test.want)
		})
	}
}

// TestRuleEightPastWeeksSkipKickoffLocks covers rule 8.
func TestRuleEightPastWeeksSkipKickoffLocks(t *testing.T) {
	t.Parallel()

	t.Run("a kicked-off game takes a new leg on a past-week slip", func(t *testing.T) {
		t.Parallel()

		f := newFixture(t)
		_, err := f.service.AddLeg(t.Context(), f.ada.ID, db.Leg{
			SlipID: f.pastSlip.ID, GameID: &f.pastGame, MarketType: "ML", Selection: "BUF ML",
		})

		require.NoError(t, err)
		entries := activity(t, f.store, f.pastSlip.ID)
		require.Len(t, entries, 1)
		assert.Equal(t, []string{fmt.Sprintf("slip:%d", f.pastSlip.ID)}, f.published.topics)
	})

	t.Run("a kicked-off leg can be edited on a past-week slip", func(t *testing.T) {
		t.Parallel()

		f := newFixture(t)
		leg := addLeg(t, f.store, f.pastSlip, f.ada, &f.pastGame, "BUF ML")

		_, err := f.service.UpdateLeg(t.Context(), f.ada.ID, db.Leg{
			ID: leg.ID, GameID: &f.pastGame, MarketType: "ML", Selection: "BUF -3.5",
		})

		require.NoError(t, err)
		assert.Len(t, activity(t, f.store, f.pastSlip.ID), 1)
	})

	t.Run("a kicked-off leg can be deleted on a past-week slip", func(t *testing.T) {
		t.Parallel()

		f := newFixture(t)
		leg := addLeg(t, f.store, f.pastSlip, f.ada, &f.pastGame, "BUF ML")

		require.NoError(t, f.service.DeleteLeg(t.Context(), f.ada.ID, leg.ID))
		assert.Len(t, activity(t, f.store, f.pastSlip.ID), 1)
	})
}

// TestEveryMutationWritesOneActivityEntryAndPublishes covers rule 9.
func TestEveryMutationWritesOneActivityEntryAndPublishes(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		arrange func(t *testing.T, f *fixture)
		mutate  func(t *testing.T, f *fixture) error
		want    string
	}{
		"add member": {
			mutate: func(t *testing.T, f *fixture) error {
				t.Helper()
				return f.service.AddMember(t.Context(), f.ada.ID, f.slip.ID, f.kim.ID)
			},
			want: "Ada added Kim",
		},
		"remove member": {
			mutate: func(t *testing.T, f *fixture) error {
				t.Helper()
				return f.service.RemoveMember(t.Context(), f.ada.ID, f.slip.ID, f.sam.ID)
			},
			want: "Ada removed Sam",
		},
		"change legs per member": {
			mutate: func(t *testing.T, f *fixture) error {
				t.Helper()
				_, err := f.service.UpdateSlip(t.Context(), f.ada.ID, f.slip.ID, f.slip.Name, 3)
				return err
			},
			want: "Ada set 3 legs per member",
		},
		"rename slip": {
			mutate: func(t *testing.T, f *fixture) error {
				t.Helper()
				_, err := f.service.UpdateSlip(t.Context(), f.ada.ID, f.slip.ID, "Sunday Six", 2)
				return err
			},
			want: `Ada renamed the slip to "Sunday Six"`,
		},
		"add leg": {
			mutate: func(t *testing.T, f *fixture) error {
				t.Helper()
				_, err := f.service.AddLeg(t.Context(), f.ada.ID, db.Leg{
					SlipID: f.slip.ID, MarketType: "SPREAD", Selection: "BUF -3.5",
				})
				return err
			},
			want: "Ada added BUF -3.5",
		},
		"edit leg": {
			arrange: func(t *testing.T, f *fixture) {
				t.Helper()
				addLeg(t, f.store, f.slip, f.ada, nil, "BUF -3.5")
			},
			mutate: func(t *testing.T, f *fixture) error {
				t.Helper()
				legs, err := f.store.LegsForSlip(t.Context(), f.slip.ID)
				require.NoError(t, err)
				_, err = f.service.UpdateLeg(t.Context(), f.ada.ID, db.Leg{
					ID: legs[0].ID, MarketType: "SPREAD", Selection: "BUF -7",
				})
				return err
			},
			want: "Ada changed BUF -3.5 to BUF -7",
		},
		"delete leg": {
			arrange: func(t *testing.T, f *fixture) {
				t.Helper()
				addLeg(t, f.store, f.slip, f.ada, nil, "BUF -3.5")
			},
			mutate: func(t *testing.T, f *fixture) error {
				t.Helper()
				legs, err := f.store.LegsForSlip(t.Context(), f.slip.ID)
				require.NoError(t, err)
				return f.service.DeleteLeg(t.Context(), f.ada.ID, legs[0].ID)
			},
			want: "Ada deleted BUF -3.5",
		},
		"lock in": {
			arrange: func(t *testing.T, f *fixture) {
				t.Helper()
				addLeg(t, f.store, f.slip, f.ada, nil, "BUF -3.5")
				addLeg(t, f.store, f.slip, f.ada, nil, "BUF ML")
			},
			mutate: func(t *testing.T, f *fixture) error {
				t.Helper()
				return f.service.LockIn(t.Context(), f.ada.ID, f.slip.ID)
			},
			want: "Ada locked in",
		},
		"unlock": {
			arrange: func(t *testing.T, f *fixture) {
				t.Helper()
				require.NoError(t, f.store.SetSlipMemberLock(t.Context(), f.slip.ID, f.ada.ID, &testNow))
				require.NoError(t, f.store.SetSlipStatus(t.Context(), f.slip.ID, db.SlipLocked))
			},
			mutate: func(t *testing.T, f *fixture) error {
				t.Helper()
				return f.service.Unlock(t.Context(), f.ada.ID, f.slip.ID)
			},
			want: "unlocked by Ada",
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			f := newFixture(t)
			if test.arrange != nil {
				test.arrange(t, f)
			}
			err := test.mutate(t, f)

			assertAccepted(t, f, err)
			entries := activity(t, f.store, f.slip.ID)
			require.Len(t, entries, 1)
			assert.Equal(t, test.want, entries[0].Message)
			require.NotNil(t, entries[0].UserID)
			assert.Equal(t, f.ada.ID, *entries[0].UserID)
		})
	}
}

// TestRefusedMutationWritesNothing checks a rule violation leaves no trace.
func TestRefusedMutationWritesNothing(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	_, err := f.service.AddLeg(t.Context(), f.kim.ID, db.Leg{
		SlipID: f.slip.ID, MarketType: "ML", Selection: "BUF ML",
	})

	assertRefused(t, f, err, slips.ErrNotMember)
	assert.Empty(t, activity(t, f.store, f.slip.ID))
	assert.Empty(t, f.published.topics)
}

// TestSetLegResultRecomputesTheSlip covers every case in plan/product.md
// "Grading": a lost leg busts the slip, a waiting leg holds it, pushed and
// voided legs drop out, and a graded slip follows its legs back to pending.
func TestSetLegResultRecomputesTheSlip(t *testing.T) {
	t.Parallel()

	type testCase struct {
		// arrange leaves the other legs as the case needs and returns the leg
		// to grade. It clears the setup's activity before returning.
		arrange func(t *testing.T, f *fixture) int64
		result  parlay.Result
		want    parlay.Result
		status  db.SlipStatus
	}

	tests := map[string]testCase{
		"every leg won is a won slip": {
			arrange: func(t *testing.T, f *fixture) int64 {
				t.Helper()
				leg := addLeg(t, f.store, f.slip, f.ada, nil, "BUF -3.5")
				other := addLeg(t, f.store, f.slip, f.sam, nil, "MIA ML")
				gradeLeg(t, f, f.sam.ID, other.ID, parlay.Won)
				return leg.ID
			},
			result: parlay.Won,
			want:   parlay.Won,
			status: db.SlipGraded,
		},
		"a lost leg busts the slip before the others finish": {
			arrange: func(t *testing.T, f *fixture) int64 {
				t.Helper()
				leg := addLeg(t, f.store, f.slip, f.ada, nil, "BUF -3.5")
				addLeg(t, f.store, f.slip, f.sam, nil, "MIA ML")
				return leg.ID
			},
			result: parlay.Lost,
			want:   parlay.Lost,
			status: db.SlipGraded,
		},
		"a pending leg holds the slip pending": {
			arrange: func(t *testing.T, f *fixture) int64 {
				t.Helper()
				leg := addLeg(t, f.store, f.slip, f.ada, nil, "BUF -3.5")
				other := addLeg(t, f.store, f.slip, f.sam, nil, "MIA ML")
				gradeLeg(t, f, f.sam.ID, other.ID, parlay.Won)
				addLeg(t, f.store, f.slip, f.sam, nil, "BUF@MIA Over 47.5")
				return leg.ID
			},
			result: parlay.Won,
			want:   parlay.Pending,
			status: db.SlipOpen,
		},
		"a needs-grading leg holds the slip pending": {
			arrange: func(t *testing.T, f *fixture) int64 {
				t.Helper()
				leg := addLeg(t, f.store, f.slip, f.ada, nil, "BUF -3.5")
				other := addLeg(t, f.store, f.slip, f.sam, nil, "Josh Allen Anytime TD")
				gradeLeg(t, f, f.sam.ID, other.ID, parlay.NeedsGrading)
				return leg.ID
			},
			result: parlay.Won,
			want:   parlay.Pending,
			status: db.SlipOpen,
		},
		"a push on an exact line drops out": {
			arrange: func(t *testing.T, f *fixture) int64 {
				t.Helper()
				leg := addSpreadLeg(t, f.store, f.slip, f.ada, "BUF -3", -3)
				other := addLeg(t, f.store, f.slip, f.sam, nil, "MIA ML")
				gradeLeg(t, f, f.sam.ID, other.ID, parlay.Won)
				return leg.ID
			},
			result: parlay.Push,
			want:   parlay.Won,
			status: db.SlipGraded,
		},
		"a void leg drops out": {
			arrange: func(t *testing.T, f *fixture) int64 {
				t.Helper()
				leg := addLeg(t, f.store, f.slip, f.ada, nil, "BUF -3.5")
				other := addLeg(t, f.store, f.slip, f.sam, nil, "MIA ML")
				gradeLeg(t, f, f.sam.ID, other.ID, parlay.Won)
				return leg.ID
			},
			result: parlay.Void,
			want:   parlay.Won,
			status: db.SlipGraded,
		},
		"every leg push or void is a push": {
			arrange: func(t *testing.T, f *fixture) int64 {
				t.Helper()
				leg := addSpreadLeg(t, f.store, f.slip, f.ada, "BUF -3", -3)
				other := addLeg(t, f.store, f.slip, f.sam, nil, "MIA ML")
				gradeLeg(t, f, f.sam.ID, other.ID, parlay.Void)
				return leg.ID
			},
			result: parlay.Push,
			want:   parlay.Push,
			status: db.SlipGraded,
		},
		"a graded slip reopens when a leg goes back to pending": {
			arrange: func(t *testing.T, f *fixture) int64 {
				t.Helper()
				leg := addLeg(t, f.store, f.slip, f.ada, nil, "BUF -3.5")
				other := addLeg(t, f.store, f.slip, f.sam, nil, "MIA ML")
				gradeLeg(t, f, f.ada.ID, leg.ID, parlay.Won)
				gradeLeg(t, f, f.sam.ID, other.ID, parlay.Won)
				return leg.ID
			},
			result: parlay.Pending,
			want:   parlay.Pending,
			status: db.SlipOpen,
		},
		"a graded slip relocks when every member is locked in": {
			arrange: func(t *testing.T, f *fixture) int64 {
				t.Helper()
				require.NoError(t,
					f.store.SetSlipMemberLock(t.Context(), f.slip.ID, f.ada.ID, &testNow))
				require.NoError(t,
					f.store.SetSlipMemberLock(t.Context(), f.slip.ID, f.sam.ID, &testNow))
				leg := addLeg(t, f.store, f.slip, f.ada, nil, "BUF -3.5")
				other := addLeg(t, f.store, f.slip, f.sam, nil, "MIA ML")
				gradeLeg(t, f, f.ada.ID, leg.ID, parlay.Won)
				gradeLeg(t, f, f.sam.ID, other.ID, parlay.Won)
				return leg.ID
			},
			result: parlay.Pending,
			want:   parlay.Pending,
			status: db.SlipLocked,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			f := newFixture(t)
			legID := test.arrange(t, f)

			updated, err := f.service.SetLegResult(t.Context(), f.ada.ID, legID, test.result)

			assertAccepted(t, f, err)
			assert.Equal(t, test.want, updated.Result)
			assert.Equal(t, test.status, updated.Status)
			assert.Equal(t, updated, reload(t, f.store, f.slip.ID))

			leg, err := f.store.LegByID(t.Context(), legID)
			require.NoError(t, err)
			assert.Equal(t, test.result, leg.Result)
			require.NotNil(t, leg.GradedBy)
			assert.Equal(t, f.ada.Name, *leg.GradedBy)
			assert.Equal(t,
				fmt.Sprintf("%s set %s to %s", f.ada.Name, leg.Selection, test.result),
				activity(t, f.store, f.slip.ID)[0].Message)
		})
	}
}

// TestSetLegResultLetsAnyMemberGradeAnyLeg keeps grading open to the group.
func TestSetLegResultLetsAnyMemberGradeAnyLeg(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	leg := addLeg(t, f.store, f.slip, f.ada, nil, "BUF -3.5")

	_, err := f.service.SetLegResult(t.Context(), f.sam.ID, leg.ID, parlay.Won)

	assertAccepted(t, f, err)
	stored, err := f.store.LegByID(t.Context(), leg.ID)
	require.NoError(t, err)
	require.NotNil(t, stored.GradedBy)
	assert.Equal(t, f.sam.Name, *stored.GradedBy)
	assert.Equal(t, "Sam set BUF -3.5 to WON", activity(t, f.store, f.slip.ID)[0].Message)
}

// TestSetLegResultRefusesANonMember keeps grading inside the slip.
func TestSetLegResultRefusesANonMember(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	leg := addLeg(t, f.store, f.slip, f.ada, nil, "BUF -3.5")

	_, err := f.service.SetLegResult(t.Context(), f.kim.ID, leg.ID, parlay.Won)

	assertRefused(t, f, err, slips.ErrNotMember)
	stored, err := f.store.LegByID(t.Context(), leg.ID)
	require.NoError(t, err)
	assert.Equal(t, parlay.Pending, stored.Result)
	assert.Nil(t, stored.GradedBy)
}

// TestSetLegResultRefusesAnUnknownResult keeps a typo out of the table.
func TestSetLegResultRefusesAnUnknownResult(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	leg := addLeg(t, f.store, f.slip, f.ada, nil, "BUF -3.5")

	_, err := f.service.SetLegResult(t.Context(), f.ada.ID, leg.ID, parlay.Result("MAYBE"))

	assertRefused(t, f, err, db.ErrLegResultInvalid)
	stored, err := f.store.LegByID(t.Context(), leg.ID)
	require.NoError(t, err)
	assert.Equal(t, parlay.Pending, stored.Result)
}

// TestSetLegResultAgainChangesNothing keeps a repeated grade quiet.
func TestSetLegResultAgainChangesNothing(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	leg := addLeg(t, f.store, f.slip, f.ada, nil, "BUF -3.5")
	gradeLeg(t, f, f.ada.ID, leg.ID, parlay.Won)

	updated, err := f.service.SetLegResult(t.Context(), f.ada.ID, leg.ID, parlay.Won)

	require.NoError(t, err)
	assert.Equal(t, parlay.Won, updated.Result, "the slip comes back unchanged")
	assert.Empty(t, activity(t, f.store, f.slip.ID), "a repeated grade writes nothing")
	assert.Empty(t, f.published.topics, "a repeated grade publishes nothing")
}

// TestSetLegResultOverridesAnEarlierGrade proves any member can change a grade
// and the slip follows.
func TestSetLegResultOverridesAnEarlierGrade(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	leg := addLeg(t, f.store, f.slip, f.ada, nil, "BUF -3.5")
	other := addLeg(t, f.store, f.slip, f.sam, nil, "MIA ML")
	gradeLeg(t, f, f.sam.ID, other.ID, parlay.Won)
	gradeLeg(t, f, f.ada.ID, leg.ID, parlay.Lost)

	updated, err := f.service.SetLegResult(t.Context(), f.sam.ID, leg.ID, parlay.Won)

	assertAccepted(t, f, err)
	assert.Equal(t, parlay.Won, updated.Result)
	stored, err := f.store.LegByID(t.Context(), leg.ID)
	require.NoError(t, err)
	require.NotNil(t, stored.GradedBy)
	assert.Equal(t, f.sam.Name, *stored.GradedBy, "the latest grader owns the grade")
	assert.Equal(t, "Sam set BUF -3.5 to WON", activity(t, f.store, f.slip.ID)[0].Message)
}

// TestServiceErrorsKeepTheirIdentity proves handlers can map the sentinels even
// though the service adds context.
func TestServiceErrorsKeepTheirIdentity(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	_, err := f.service.AddLeg(t.Context(), f.kim.ID, db.Leg{
		SlipID: f.slip.ID, MarketType: "ML", Selection: "BUF ML",
	})

	require.ErrorIs(t, err, slips.ErrNotMember)
	assert.Contains(t, err.Error(), "slip")
}

// TestNewUsesTheWallClockWhenNoneIsGiven keeps the nil-clock default honest.
func TestNewUsesTheWallClockWhenNoneIsGiven(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	service := slips.New(f.store, nil, nil)
	addLeg(t, f.store, f.slip, f.ada, nil, "BUF -3.5")
	addLeg(t, f.store, f.slip, f.ada, nil, "BUF ML")

	require.NoError(t, service.LockIn(t.Context(), f.ada.ID, f.slip.ID))
}

func TestRulesEnforceSlateScopeOnPicks(t *testing.T) {
	t.Parallel()

	f := newFixture(t)

	// In UTC: Thursday night game 20:15 ET is 2026-11-06T01:15:00Z.
	// Sunday game 13:00 ET is 2026-11-08T18:00:00Z.
	thuGame := createGame(t, f.database, f.week, "thu-game", "2026-11-06T01:15:00Z")
	sunGame := createGame(t, f.database, f.week, "sun-game", "2026-11-08T18:00:00Z")

	// Service clocked before Thursday kickoff: Thursday noon ET (17:00 UTC).
	thuNoon := time.Date(2026, 11, 5, 17, 0, 0, 0, time.UTC)
	serviceBeforeThu := slips.New(f.store, f.published, fixedClock{at: thuNoon}.Now)

	// Slips with different scopes:
	thuSlip, err := f.store.CreateSlip(t.Context(), f.group.ID, f.week.ID, nil, "Thursday Slip",
		db.SlateScopeThursday, 2, []int64{f.ada.ID})
	require.NoError(t, err)

	sunSlip, err := f.store.CreateSlip(t.Context(), f.group.ID, f.week.ID, nil, "Sunday Slip",
		db.SlateScopeSundayMain, 2, []int64{f.ada.ID})
	require.NoError(t, err)

	fullSlip, err := f.store.CreateSlip(t.Context(), f.group.ID, f.week.ID, nil, "Full Week Slip",
		db.SlateScopeFullWeek, 2, []int64{f.ada.ID})
	require.NoError(t, err)

	// 1. AddLeg validation:
	// SundayMain slip refuses Thursday game.
	_, err = serviceBeforeThu.AddLeg(t.Context(), f.ada.ID, db.Leg{
		SlipID: sunSlip.ID, GameID: &thuGame, MarketType: "ML", Selection: "BUF ML",
	})
	require.ErrorIs(t, err, slips.ErrGameOutsideScope)

	// SundayMain slip accepts Sunday game.
	legSun, err := serviceBeforeThu.AddLeg(t.Context(), f.ada.ID, db.Leg{
		SlipID: sunSlip.ID, GameID: &sunGame, MarketType: "ML", Selection: "BUF ML",
	})
	require.NoError(t, err)

	// Thursday slip refuses Sunday game.
	_, err = serviceBeforeThu.AddLeg(t.Context(), f.ada.ID, db.Leg{
		SlipID: thuSlip.ID, GameID: &sunGame, MarketType: "ML", Selection: "MIA ML",
	})
	require.ErrorIs(t, err, slips.ErrGameOutsideScope)

	// Thursday slip accepts Thursday game.
	legThu, err := serviceBeforeThu.AddLeg(t.Context(), f.ada.ID, db.Leg{
		SlipID: thuSlip.ID, GameID: &thuGame, MarketType: "ML", Selection: "BUF -3.5",
	})
	require.NoError(t, err)

	// Full week slip accepts both Thursday and Sunday games.
	_, err = serviceBeforeThu.AddLeg(t.Context(), f.ada.ID, db.Leg{
		SlipID: fullSlip.ID, GameID: &thuGame, MarketType: "ML", Selection: "BUF -3.5",
	})
	require.NoError(t, err)
	_, err = serviceBeforeThu.AddLeg(t.Context(), f.ada.ID, db.Leg{
		SlipID: fullSlip.ID, GameID: &sunGame, MarketType: "ML", Selection: "MIA ML",
	})
	require.NoError(t, err)

	// 2. UpdateLeg validation:
	// SundayMain slip: updating existing Sunday leg to Thursday game is refused.
	_, err = serviceBeforeThu.UpdateLeg(t.Context(), f.ada.ID, db.Leg{
		ID: legSun.ID, GameID: &thuGame, MarketType: "ML", Selection: "BUF -3.5",
	})
	require.ErrorIs(t, err, slips.ErrGameOutsideScope)

	// Thursday slip: updating existing Thursday leg to Sunday game is refused.
	_, err = serviceBeforeThu.UpdateLeg(t.Context(), f.ada.ID, db.Leg{
		ID: legThu.ID, GameID: &sunGame, MarketType: "ML", Selection: "MIA -3.5",
	})
	require.ErrorIs(t, err, slips.ErrGameOutsideScope)

	// 3. Kickoff lock when game is in scope:
	// Once Thursday game kicks off (e.g. Friday noon), adding it to Thursday slip returns ErrKickoffLocked.
	friNoon := time.Date(2026, 11, 6, 17, 0, 0, 0, time.UTC)
	serviceAfterThu := slips.New(f.store, f.published, fixedClock{at: friNoon}.Now)

	anotherThuSlip, err := f.store.CreateSlip(t.Context(), f.group.ID, f.week.ID, nil, "Late Thursday Slip",
		db.SlateScopeThursday, 2, []int64{f.ada.ID})
	require.NoError(t, err)

	_, err = serviceAfterThu.AddLeg(t.Context(), f.ada.ID, db.Leg{
		SlipID: anotherThuSlip.ID, GameID: &thuGame, MarketType: "ML", Selection: "BUF ML",
	})
	require.ErrorIs(t, err, slips.ErrKickoffLocked)
}
