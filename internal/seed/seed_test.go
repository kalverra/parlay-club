package seed_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/parlay"
	"github.com/kalverra/parlay-club/internal/seed"
)

// validFile is one valid slip the validation tests bend one field at a time.
const validFile = `slips:
  - season: 2026
    season_type: 2
    week: 7
    name: Week 7
    legs_per_member: 1
    legs:
      - user: Ada
        selection: BUF -3.5
        result: WON
`

// newStore opens a fresh migrated database.
func newStore(t *testing.T) *db.Store {
	t.Helper()

	database, err := db.Open(filepath.Join(t.TempDir(), "parlay.db"))
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, database.Close()) })
	require.NoError(t, db.Migrate(t.Context(), database))
	return db.NewStore(database)
}

// newGroup creates a group owned by a new user.
func newGroup(t *testing.T, store *db.Store) db.Group {
	t.Helper()

	owner, err := store.CreateUserWithIdentity(t.Context(), "Owner", "", "", "")
	require.NoError(t, err)
	group, err := store.Create(t.Context(), owner.ID, "Sunday Parlay Club")
	require.NoError(t, err)
	return group
}

// writeHistory saves body as a history file and returns its path.
func writeHistory(t *testing.T, body string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "history.yaml")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
	return path
}

// slipNamed returns the group's slip with name in the week.
func slipNamed(t *testing.T, store *db.Store, groupID, weekID int64, name string) db.Slip {
	t.Helper()

	slips, err := store.SlipsForWeek(t.Context(), groupID, weekID)
	require.NoError(t, err)
	for _, slip := range slips {
		if slip.Name == name {
			return slip
		}
	}
	require.FailNowf(t, "slip not found", "no slip %q in week %d", name, weekID)
	return db.Slip{}
}

func TestLoadCreatesSlipsUsersAndResults(t *testing.T) {
	t.Parallel()

	store := newStore(t)
	group := newGroup(t, store)
	ada, err := store.CreateUserWithIdentity(t.Context(), "Ada", "", "", "")
	require.NoError(t, err)

	path := writeHistory(t, `
slips:
  - season: 2026
    season_type: 2
    week: 7
    name: Week 7
    legs_per_member: 2
    legs:
      - user: ada
        selection: BUF -3.5
        result: WON
      - user: Adam
        selection: MIA +3.5
        market: SPREAD
        team: MIA
        line: 3.5
        result: LOST
  - season: 2026
    season_type: 2
    week: 6
    name: Week 6
    legs_per_member: 1
    legs:
      - user: Sam
        selection: KC ML
        result: LOST
`)

	report, err := seed.Load(t.Context(), store, group.ID, path)
	require.NoError(t, err)

	assert.Equal(t, seed.Report{Slips: 2, Skipped: 0, Users: 2}, report)

	users, err := store.ListActiveUsers(t.Context())
	require.NoError(t, err)
	assert.Len(t, users, 4, "Owner and Ada existed; Adam and Sam are new")

	week7 := slipNamed(t, store, group.ID, db.WeekID(2026, 2, 7), "Week 7")
	assert.Equal(t, parlay.Lost, week7.Result, "one LOST leg busts the slip")
	assert.Equal(t, db.SlipGraded, week7.Status)

	legs, err := store.LegsForSlip(t.Context(), week7.ID)
	require.NoError(t, err)
	require.Len(t, legs, 2)
	assert.Equal(t, ada.ID, legs[0].UserID, "a name matches a user ignoring case")
	assert.Equal(t, "OTHER", legs[0].MarketType, "the market defaults to OTHER")
	assert.Equal(t, db.DefaultLegOdds, legs[0].Odds, "odds default to -110")
	assert.Equal(t, parlay.Won, legs[0].Result)
	require.NotNil(t, legs[0].GradedBy)
	assert.Equal(t, "Ada", *legs[0].GradedBy)
	assert.Equal(t, "SPREAD", legs[1].MarketType)
	require.NotNil(t, legs[1].Team)
	assert.Equal(t, "MIA", *legs[1].Team)
	require.NotNil(t, legs[1].Line)
	assert.InDelta(t, 3.5, *legs[1].Line, 0)
	assert.Equal(t, parlay.Lost, legs[1].Result)

	week6 := slipNamed(t, store, group.ID, db.WeekID(2026, 2, 6), "Week 6")
	assert.Equal(t, parlay.Lost, week6.Result)
	assert.Equal(t, db.SlipGraded, week6.Status)
}

func TestLoadComputesAWonSlip(t *testing.T) {
	t.Parallel()

	store := newStore(t)
	group := newGroup(t, store)
	path := writeHistory(t, `
slips:
  - season: 2026
    season_type: 2
    week: 8
    name: Week 8
    legs:
      - user: Ada
        selection: BUF ML
        result: WON
      - user: Sam
        selection: KC ML
        result: WON
`)

	report, err := seed.Load(t.Context(), store, group.ID, path)
	require.NoError(t, err)
	assert.Equal(t, 1, report.Slips)

	week := slipNamed(t, store, group.ID, db.WeekID(2026, 2, 8), "Week 8")
	assert.Equal(t, parlay.Won, week.Result)
	assert.Equal(t, db.SlipGraded, week.Status)
}

func TestLoadTwiceCreatesEachSlipOnce(t *testing.T) {
	t.Parallel()

	store := newStore(t)
	group := newGroup(t, store)
	path := writeHistory(t, `
slips:
  - season: 2026
    season_type: 2
    week: 7
    name: Week 7
    legs:
      - user: Ada
        selection: BUF -3.5
        result: WON
      - user: Sam
        selection: KC -2.5
        result: PUSH
`)

	first, err := seed.Load(t.Context(), store, group.ID, path)
	require.NoError(t, err)
	assert.Equal(t, seed.Report{Slips: 1, Users: 2}, first)

	week := slipNamed(t, store, group.ID, db.WeekID(2026, 2, 7), "Week 7")
	before, err := store.ActivityForSlip(t.Context(), week.ID)
	require.NoError(t, err)

	second, err := seed.Load(t.Context(), store, group.ID, path)
	require.NoError(t, err)
	assert.Equal(t, seed.Report{Skipped: 1}, second, "the second load creates no users either")

	slips, err := store.SlipsForWeek(t.Context(), group.ID, db.WeekID(2026, 2, 7))
	require.NoError(t, err)
	assert.Len(t, slips, 1)

	legs, err := store.LegsForSlip(t.Context(), week.ID)
	require.NoError(t, err)
	assert.Len(t, legs, 2, "a skipped slip gains no duplicate legs")

	after, err := store.ActivityForSlip(t.Context(), week.ID)
	require.NoError(t, err)
	assert.Len(t, after, len(before), "a skipped slip writes no activity")
	assert.Equal(t, parlay.Won, week.Result, "a PUSH leg still leaves the other winner to win the slip")
	assert.Equal(t, db.SlipGraded, week.Status)
}

func TestLoadEmptyHistoryFileIsANoOp(t *testing.T) {
	t.Parallel()

	store := newStore(t)
	group := newGroup(t, store)

	path := writeHistory(t, "# comments only\n")
	report, err := seed.Load(t.Context(), store, group.ID, path)
	require.NoError(t, err)
	assert.Equal(t, seed.Report{}, report)

	slips, err := store.SlipsForWeek(t.Context(), group.ID, db.WeekID(2026, 2, 7))
	require.NoError(t, err)
	assert.Empty(t, slips)
}

func TestLoadRepoHistoryFile(t *testing.T) {
	t.Parallel()

	store := newStore(t)
	group := newGroup(t, store)

	path := filepath.Join("..", "..", seed.DefaultPath)
	report, err := seed.Load(t.Context(), store, group.ID, path)
	require.NoError(t, err)
	assert.Equal(t, seed.Report{Slips: 3, Skipped: 0, Users: 3}, report)

	for _, week := range []int{7, 8, 9} {
		slips, err := store.SlipsForWeek(t.Context(), group.ID, db.WeekID(2026, 2, week))
		require.NoError(t, err)
		require.Len(t, slips, 1)
		assert.Equal(t, db.SlipGraded, slips[0].Status)
		assert.Equal(t, parlay.Lost, slips[0].Result)
	}
}

func TestLoadReportsAMissingFile(t *testing.T) {
	t.Parallel()

	store := newStore(t)
	group := newGroup(t, store)

	_, err := seed.Load(t.Context(), store, group.ID, filepath.Join(t.TempDir(), "missing.yaml"))
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestLoadValidatesTheFileBeforeWriting(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "unknown result",
			body: strings.Replace(validFile, "        result: WON", "        result: MAYBE", 1),
			want: "is not a leg result",
		},
		{
			name: "unknown market",
			body: strings.Replace(validFile, "        result: WON", "        market: LOTTERY\n        result: WON", 1),
			want: "is not a market the app knows",
		},
		{
			name: "missing selection",
			body: strings.Replace(validFile, "selection: BUF -3.5", `selection: ""`, 1),
			want: "selection is required",
		},
		{
			name: "missing user",
			body: strings.Replace(validFile, "user: Ada", `user: ""`, 1),
			want: "user is required",
		},
		{
			name: "too many legs for the limit",
			body: strings.Replace(validFile, "        result: WON",
				"        result: WON\n      - user: Ada\n        selection: KC ML\n        result: WON", 1),
			want: "more than legs_per_member 1",
		},
		{
			name: "missing name",
			body: strings.Replace(validFile, "name: Week 7", `name: ""`, 1),
			want: "name is required",
		},
		{
			name: "week out of range",
			body: strings.Replace(validFile, "week: 7", "week: 100", 1),
			want: "week must be from 1 to 99",
		},
		{
			name: "bad season type",
			body: strings.Replace(validFile, "season_type: 2", "season_type: 4", 1),
			want: "season_type must be 1, 2, or 3",
		},
		{
			name: "unknown field",
			body: strings.Replace(validFile, "name: Week 7", "name: Week 7\n    group: Club", 1),
			want: "field group not found",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			store := newStore(t)
			group := newGroup(t, store)

			_, err := seed.Load(t.Context(), store, group.ID, writeHistory(t, test.body))
			require.ErrorContains(t, err, test.want)

			slips, err := store.SlipsForWeek(t.Context(), group.ID, db.WeekID(2026, 2, 7))
			require.NoError(t, err)
			assert.Empty(t, slips, "validation writes nothing")
		})
	}
}
