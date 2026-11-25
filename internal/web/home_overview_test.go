package web

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/parlay"
	"github.com/kalverra/parlay-club/internal/slips"
)

func TestIsSpotlightSlip(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC) // Sunday morning ET
	upcomingSun := now.Add(6 * time.Hour)
	pastThu := now.Add(-60 * time.Hour) // Thursday night

	tests := []struct {
		name  string
		slip  db.Slip
		next  slips.Next
		games []db.Game
		want  bool
	}{
		{
			name: "open slip needing picks",
			slip: db.Slip{ID: 1, Status: db.SlipOpen, SlateScope: db.SlateScopeSundayMain},
			next: slips.Next{Step: slips.StepPick, Need: 2},
			want: true,
		},
		{
			name: "open slip ready to lock in",
			slip: db.Slip{ID: 2, Status: db.SlipOpen, SlateScope: db.SlateScopeSundayMain},
			next: slips.Next{Step: slips.StepLockIn},
			want: true,
		},
		{
			name: "open slip viewer can join",
			slip: db.Slip{ID: 3, Status: db.SlipOpen, SlateScope: db.SlateScopeSundayMain},
			next: slips.Next{Step: slips.StepJoin},
			want: true,
		},
		{
			name: "open slip waiting on others",
			slip: db.Slip{ID: 4, Status: db.SlipOpen, SlateScope: db.SlateScopeSundayMain},
			next: slips.Next{Step: slips.StepWaiting},
			want: true,
		},
		{
			name: "locked slip ready to export",
			slip: db.Slip{ID: 5, Status: db.SlipLocked, SlateScope: db.SlateScopeSundayMain},
			next: slips.Next{Step: slips.StepExport},
			want: true,
		},
		{
			name: "locked slip with live eligible game",
			slip: db.Slip{ID: 6, Status: db.SlipLocked, SlateScope: db.SlateScopeSundayMain},
			next: slips.Next{Step: slips.StepClosed},
			games: []db.Game{
				{ID: "sun-live", Kickoff: now.Add(-30 * time.Minute), Status: db.GameLive},
			},
			want: true,
		},
		{
			name: "locked slip with upcoming eligible game",
			slip: db.Slip{ID: 7, Status: db.SlipLocked, SlateScope: db.SlateScopeSundayMain},
			next: slips.Next{Step: slips.StepClosed},
			games: []db.Game{
				{ID: "sun-upcoming", Kickoff: upcomingSun, Status: db.GameScheduled},
			},
			want: true,
		},
		{
			name: "locked slip with only final past eligible games",
			slip: db.Slip{ID: 8, Status: db.SlipLocked, SlateScope: db.SlateScopeThursday},
			next: slips.Next{Step: slips.StepClosed},
			games: []db.Game{
				{ID: "thu-final", Kickoff: pastThu, Status: db.GameFinal},
				{ID: "sun-live", Kickoff: now.Add(-30 * time.Minute), Status: db.GameLive},
			},
			want: false,
		},
		{
			name: "closed open slip",
			slip: db.Slip{ID: 9, Status: db.SlipOpen, SlateScope: db.SlateScopeSundayMain},
			next: slips.Next{Step: slips.StepClosed},
			want: false,
		},
		{
			name: "graded slip is never spotlight",
			slip: db.Slip{ID: 10, Status: db.SlipGraded, Result: parlay.Won, SlateScope: db.SlateScopeSundayMain},
			next: slips.Next{Step: slips.StepDone, Result: parlay.Won},
			games: []db.Game{
				{ID: "sun-live", Kickoff: now.Add(-30 * time.Minute), Status: db.GameLive},
			},
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, isSpotlightSlip(tc.slip, tc.next, tc.games, now))
		})
	}
}

func TestBuildGroupOverviewPageStates(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC) // Sunday morning ET
	sunKick := now.Add(6 * time.Hour)
	thuPast := now.Add(-60 * time.Hour)

	group := db.Group{ID: 1, Name: "Sunday Parlay Club"}
	week9 := db.Week{ID: 9, Season: 2026, SeasonType: 2, WeekNum: 9}
	week8 := db.Week{ID: 8, Season: 2026, SeasonType: 2, WeekNum: 8}
	week10 := db.Week{ID: 10, Season: 2026, SeasonType: 2, WeekNum: 10}

	ada := db.User{ID: 1, Name: "Ada", AvatarColor: db.AvatarColor("amber")}
	ben := db.User{ID: 2, Name: "Ben", AvatarColor: db.AvatarColor("sage")}
	lockedAt := now.Add(-time.Hour)

	schedules := []db.GroupSlipSchedule{
		{
			ID:            10,
			GroupID:       1,
			Name:          "Sunday Main",
			SlateScope:    db.SlateScopeSundayMain,
			LegsPerMember: 2,
			IsActive:      true,
		},
	}

	t.Run("open and multi-slip spotlight ordered by priority and excludes from recent", func(t *testing.T) {
		t.Parallel()
		exportSlip := db.Slip{
			ID: 101, GroupID: 1, WeekID: 9, Name: "Early Export",
			SlateScope: db.SlateScopeSundayMain, LegsPerMember: 1, Status: db.SlipLocked,
		}
		pickSlip := db.Slip{
			ID: 102, GroupID: 1, WeekID: 9, Name: "Sunday Pick",
			SlateScope: db.SlateScopeSundayMain, LegsPerMember: 2, Status: db.SlipOpen,
		}
		gradedSlip := db.Slip{
			ID: 99, GroupID: 1, WeekID: 8, Name: "Week 8",
			SlateScope: db.SlateScopeSundayMain, LegsPerMember: 2, Status: db.SlipGraded, Result: parlay.Won,
		}

		page := buildGroupOverviewPage(groupOverviewInput{
			Page:       Page{Title: "This week"},
			Group:      group,
			ActiveWeek: &week9,
			WeekGames: []db.Game{
				{ID: "g1", Kickoff: sunKick, Status: db.GameScheduled},
			},
			WeekSlips: []groupOverviewSlip{
				{
					Slip: exportSlip,
					Week: week9,
					Members: []db.SlipMember{
						{User: ada, LockedAt: &lockedAt},
						{User: ben, LockedAt: &lockedAt},
					},
					Legs: []db.Leg{{ID: 1, SlipID: 101, UserID: ada.ID}, {ID: 2, SlipID: 101, UserID: ben.ID}},
				},
				{
					Slip: pickSlip,
					Week: week9,
					Members: []db.SlipMember{
						{User: ada},
						{User: ben},
					},
				},
			},
			RecentSlips: []db.RecentSlipRow{
				{Slip: pickSlip, Week: week9},
				{Slip: exportSlip, Week: week9},
				{Slip: gradedSlip, Week: week8},
			},
			RecordSeason: 2026,
			GradedSlips: []db.GradedSlip{
				{Slip: gradedSlip, Week: week8},
			},
			Schedules:    schedules,
			GroupMembers: []db.User{ada, ben},
			ViewerID:     ada.ID,
			Now:          now,
		})

		require.Len(t, page.Spotlight, 2)
		assert.Equal(t, pickSlip.ID, page.Spotlight[0].ID)
		assert.Equal(t, "Pick 2 more", page.Spotlight[0].ActionLabel)
		assert.Equal(t, "/slips/102/pick", page.Spotlight[0].ActionHref)
		assert.Equal(t, "2 members · 0/4 picks", page.Spotlight[0].Progress)
		assert.False(t, page.Spotlight[0].Live.Show)

		assert.Equal(t, exportSlip.ID, page.Spotlight[1].ID)
		assert.Equal(t, "Open slip", page.Spotlight[1].ActionLabel)
		assert.Equal(t, "/slips/101", page.Spotlight[1].ActionHref)

		require.Len(t, page.RecentSlips, 1)
		assert.Equal(t, gradedSlip.ID, page.RecentSlips[0].ID)
		assert.Equal(t, "Won", page.RecentSlips[0].ResultLabel)
		assert.Equal(t, "won", page.RecentSlips[0].ResultTone)

		assert.True(t, page.Record.HasSlips)
		assert.Equal(t, "1-0", page.Record.Record)
		assert.Equal(t, "W1", page.Record.Streak)
		assert.Equal(t, 1, page.Record.TotalSlips)
		require.Len(t, page.Schedules, 1)
		require.Len(t, page.Members, 2)
	})

	t.Run("locked-live slip stays in spotlight", func(t *testing.T) {
		t.Parallel()
		liveSlip := db.Slip{
			ID: 201, GroupID: 1, WeekID: 9, Name: "Live Sunday",
			SlateScope: db.SlateScopeSundayMain, LegsPerMember: 1, Status: db.SlipLocked,
		}

		page := buildGroupOverviewPage(groupOverviewInput{
			Group:      group,
			ActiveWeek: &week9,
			WeekGames: []db.Game{
				{
					ID:        "g-live",
					HomeTeam:  "BUF",
					AwayTeam:  "KC",
					Kickoff:   now.Add(-time.Hour),
					Status:    db.GameLive,
					HomeScore: new(21),
					AwayScore: new(17),
					Period:    new(3),
					Clock:     new("8:42"),
				},
			},
			WeekSlips: []groupOverviewSlip{
				{
					Slip:    liveSlip,
					Week:    week9,
					Members: []db.SlipMember{{User: ada, LockedAt: &lockedAt}},
					Legs: []db.Leg{
						{
							ID:         1,
							SlipID:     201,
							UserID:     ada.ID,
							GameID:     new("g-live"),
							MarketType: "SPREAD",
							Team:       new("BUF"),
							Side:       new("TEAM"),
							Line:       new(-3.5),
							Odds:       -110,
							Result:     parlay.Pending,
						},
					},
				},
			},
			RecentSlips: []db.RecentSlipRow{{Slip: liveSlip, Week: week9}},
			ViewerID:    ada.ID,
			Now:         now,
		})

		require.Len(t, page.Spotlight, 1)
		assert.Equal(t, liveSlip.ID, page.Spotlight[0].ID)
		assert.Equal(t, "Open slip", page.Spotlight[0].ActionLabel)
		assert.True(t, page.Spotlight[0].Live.Show)
		assert.Equal(t, "LIVE", page.Spotlight[0].Live.State)
		assert.Equal(t, "0/1 hit · 1 live", page.Spotlight[0].Live.Summary)
		assert.NotEmpty(t, page.Spotlight[0].Live.LiveChance)
		assert.Empty(t, page.RecentSlips)
	})

	t.Run("closed and graded slips move out of spotlight into recent slips and record", func(t *testing.T) {
		t.Parallel()
		closedSlip := db.Slip{
			ID: 301, GroupID: 1, WeekID: 9, Name: "Missed Thursday",
			SlateScope: db.SlateScopeThursday, LegsPerMember: 2, Status: db.SlipOpen,
		}
		graded1 := db.Slip{
			ID: 302, GroupID: 1, WeekID: 8, Name: "Week 8",
			SlateScope: db.SlateScopeSundayMain, LegsPerMember: 2, Status: db.SlipGraded, Result: parlay.Won,
			CreatedAt: now.Add(-14 * 24 * time.Hour),
		}
		graded2 := db.Slip{
			ID: 303, GroupID: 1, WeekID: 9, Name: "Week 9 Push",
			SlateScope: db.SlateScopeSundayMain, LegsPerMember: 2, Status: db.SlipGraded, Result: parlay.Push,
			CreatedAt: now.Add(-7 * 24 * time.Hour),
		}
		graded3 := db.Slip{
			ID: 304, GroupID: 1, WeekID: 9, Name: "Week 9 Loss",
			SlateScope: db.SlateScopeSundayMain, LegsPerMember: 2, Status: db.SlipGraded, Result: parlay.Lost,
			CreatedAt: now.Add(-6 * 24 * time.Hour),
		}

		page := buildGroupOverviewPage(groupOverviewInput{
			Group:      group,
			ActiveWeek: &week9,
			NextWeek:   &week10,
			WeekGames: []db.Game{
				{ID: "g-thu", Kickoff: thuPast, Status: db.GameFinal},
			},
			WeekSlips: []groupOverviewSlip{
				{Slip: closedSlip, Week: week9, Members: []db.SlipMember{{User: ada}}},
				{Slip: graded3, Week: week9, Members: []db.SlipMember{{User: ada, LockedAt: &lockedAt}}},
			},
			RecentSlips: []db.RecentSlipRow{
				{Slip: graded3, Week: week9},
				{Slip: closedSlip, Week: week9},
				{Slip: graded2, Week: week9},
				{Slip: graded1, Week: week8},
			},
			RecordSeason: 2026,
			GradedSlips: []db.GradedSlip{
				{Slip: graded1, Week: week8},
				{Slip: graded2, Week: week9},
				{Slip: graded3, Week: week9},
			},
			ViewerID: ada.ID,
			Now:      now,
		})

		assert.Empty(t, page.Spotlight)
		assert.Equal(t, "Week 10", page.NextWeekLabel)

		require.Len(t, page.RecentSlips, 4)
		assert.Equal(t, "Lost", page.RecentSlips[0].ResultLabel)
		assert.Equal(t, "lost", page.RecentSlips[0].ResultTone)
		assert.Equal(t, "Closed", page.RecentSlips[1].ResultLabel)
		assert.Equal(t, "closed", page.RecentSlips[1].ResultTone)
		assert.Equal(t, "Push", page.RecentSlips[2].ResultLabel)
		assert.Equal(t, "push", page.RecentSlips[2].ResultTone)
		assert.Equal(t, "Won", page.RecentSlips[3].ResultLabel)
		assert.Equal(t, "won", page.RecentSlips[3].ResultTone)

		assert.Equal(t, "1-1-1", page.Record.Record)
		assert.Equal(t, "L1", page.Record.Streak)
		assert.Equal(t, 3, page.Record.TotalSlips)
	})

	t.Run("empty state with upcoming week and schedules", func(t *testing.T) {
		t.Parallel()
		page := buildGroupOverviewPage(groupOverviewInput{
			Group:        group,
			NextWeek:     &week10,
			Schedules:    schedules,
			GroupMembers: []db.User{ada},
			ViewerID:     ada.ID,
			Now:          now,
		})

		assert.Empty(t, page.Spotlight)
		assert.Empty(t, page.RecentSlips)
		assert.Equal(t, "Week 10", page.NextWeekLabel)
		assert.False(t, page.Record.HasSlips)
		assert.Equal(t, "0-0", page.Record.Record)
		assert.Equal(t, "–", page.Record.Streak)
		require.Len(t, page.Schedules, 1)
		require.Len(t, page.Members, 1)
	})
}
