package web

import (
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/parlay"
	"github.com/kalverra/parlay-club/internal/slips"
)

// HomePage is the group list a signed-in person lands on.
type HomePage struct {
	Page
	Cards []HomeCard
}

// HomeCard is one group on the landing list.
type HomeCard struct {
	ID      int64
	Name    string
	Summary string
}

// ScheduleView holds schedule data formatted for templates.
type ScheduleView struct {
	ID            int64
	Name          string
	SlateScope    db.SlateScope
	ScopeBadge    string
	LegsPerMember int
	IsActive      bool
}

const recentSlipsLimit = 5

// GroupOverviewPage is the per-group landing page showing active/upcoming
// spotlight slips, a group record snapshot, recent slips, recurring schedules,
// and members.
type GroupOverviewPage struct {
	Page
	Group         db.Group
	Week          *db.Week
	NextWeekLabel string
	Spotlight     []SpotlightSlipCard
	Record        GroupRecordSnapshot
	RecentSlips   []RecentSlipCard
	Schedules     []ScheduleView
	Members       []User
}

// SpotlightSlipCard is an active or upcoming slip featured in the group
// overview hero.
type SpotlightSlipCard struct {
	ID          int64
	Name        string
	WeekLabel   string
	ScopeBadge  string
	NextStep    string
	Step        slips.Step
	Deadline    *DeadlineView
	MemberCount int
	PicksCount  int
	TotalPicks  int
	LockedCount int
	Progress    string
	ActionLabel string
	ActionHref  string
}

// GroupRecordSnapshot is the compact 3-metric record summary on the group
// overview page.
type GroupRecordSnapshot struct {
	HasSlips   bool
	Season     int
	Record     string
	Wins       int
	Losses     int
	Pushes     int
	TotalSlips int
	Streak     string
}

// RecentSlipCard is one completed or closed slip in the group overview's
// recent slips list.
type RecentSlipCard struct {
	ID          int64
	Name        string
	WeekLabel   string
	ScopeBadge  string
	ResultLabel string
	ResultTone  string
	Href        string
}

// isSpotlightSlip reports whether a slip in the current or upcoming week
// should appear in the group overview hero rather than the recent slips list.
func isSpotlightSlip(slip db.Slip, next slips.Next, games []db.Game, now time.Time) bool {
	if slip.Status == db.SlipGraded {
		return false
	}
	switch next.Step {
	case slips.StepPick, slips.StepLockIn, slips.StepJoin, slips.StepWaiting, slips.StepExport:
		return true
	}
	if slip.Status == db.SlipLocked {
		for _, g := range slips.FilterGamesForSlip(games, slip) {
			if g.Status == db.GameLive || now.Before(g.Kickoff) {
				return true
			}
		}
	}
	return false
}

type groupOverviewSlip struct {
	Slip    db.Slip
	Week    db.Week
	Members []db.SlipMember
	Legs    []db.Leg
}

type groupOverviewInput struct {
	Page         Page
	Group        db.Group
	ActiveWeek   *db.Week
	NextWeek     *db.Week
	WeekSlips    []groupOverviewSlip
	WeekGames    []db.Game
	RecentSlips  []db.RecentSlipRow
	RecordSeason int
	GradedSlips  []db.GradedSlip
	Schedules    []db.GroupSlipSchedule
	GroupMembers []db.User
	ViewerID     int64
	Now          time.Time
}

func buildGroupOverviewPage(in groupOverviewInput) GroupOverviewPage {
	type spotlightItem struct {
		card SpotlightSlipCard
		next slips.Next
	}

	spotlightItems := make([]spotlightItem, 0, len(in.WeekSlips))
	spotlightIDs := make(map[int64]bool, len(in.WeekSlips))
	for _, ws := range in.WeekSlips {
		next := slips.NextStep(ws.Slip, ws.Members, ws.Legs, in.WeekGames, in.ViewerID, in.Now)
		if !isSpotlightSlip(ws.Slip, next, in.WeekGames, in.Now) {
			continue
		}
		var deadline *DeadlineView
		if next.Deadline != nil {
			deadline = newDeadlineView(in.WeekGames, *next.Deadline)
		}
		lockedCount, progress := slipProgressSummary(ws.Members, ws.Legs, ws.Slip.LegsPerMember)
		actionLabel := "Open slip"
		actionHref := fmt.Sprintf("/slips/%d", ws.Slip.ID)
		if next.Step == slips.StepPick {
			actionLabel = next.Label()
			actionHref = fmt.Sprintf("/slips/%d/pick", ws.Slip.ID)
		}

		spotlightIDs[ws.Slip.ID] = true
		spotlightItems = append(spotlightItems, spotlightItem{
			card: SpotlightSlipCard{
				ID:          ws.Slip.ID,
				Name:        ws.Slip.Name,
				WeekLabel:   weekLabel(ws.Week),
				ScopeBadge:  scopeBadge(ws.Slip),
				NextStep:    next.Label(),
				Step:        next.Step,
				Deadline:    deadline,
				MemberCount: len(ws.Members),
				PicksCount:  len(ws.Legs),
				TotalPicks:  len(ws.Members) * ws.Slip.LegsPerMember,
				LockedCount: lockedCount,
				Progress:    progress,
				ActionLabel: actionLabel,
				ActionHref:  actionHref,
			},
			next: next,
		})
	}

	slices.SortStableFunc(spotlightItems, func(a, b spotlightItem) int {
		if betterStep(a.next, b.next) {
			return -1
		}
		if betterStep(b.next, a.next) {
			return 1
		}
		return 0
	})

	spotlight := make([]SpotlightSlipCard, len(spotlightItems))
	for i, item := range spotlightItems {
		spotlight[i] = item.card
	}

	recent := make([]RecentSlipCard, 0, min(recentSlipsLimit, len(in.RecentSlips)))
	for _, row := range in.RecentSlips {
		if spotlightIDs[row.Slip.ID] {
			continue
		}
		label, tone := recentSlipResult(row.Slip)
		recent = append(recent, RecentSlipCard{
			ID:          row.Slip.ID,
			Name:        row.Slip.Name,
			WeekLabel:   weekLabel(row.Week),
			ScopeBadge:  scopeBadge(row.Slip),
			ResultLabel: label,
			ResultTone:  tone,
			Href:        fmt.Sprintf("/slips/%d", row.Slip.ID),
		})
		if len(recent) >= recentSlipsLimit {
			break
		}
	}

	members := make([]User, 0, len(in.GroupMembers))
	for _, m := range in.GroupMembers {
		members = append(members, NewUser(m))
	}

	var nextWeekLabel string
	if in.NextWeek != nil {
		nextWeekLabel = weekLabel(*in.NextWeek)
	}

	return GroupOverviewPage{
		Page:          in.Page,
		Group:         in.Group,
		Week:          in.ActiveWeek,
		NextWeekLabel: nextWeekLabel,
		Spotlight:     spotlight,
		Record:        buildGroupRecordSnapshot(in.RecordSeason, in.GradedSlips),
		RecentSlips:   recent,
		Schedules:     buildScheduleViews(in.Schedules),
		Members:       members,
	}
}

func buildGroupRecordSnapshot(season int, gradedSlips []db.GradedSlip) GroupRecordSnapshot {
	book := buildRecordBook(season, gradedSlips, nil, nil, nil)
	rec := fmt.Sprintf("%d-%d", book.Wins, book.Losses)
	if book.Pushes > 0 {
		rec = fmt.Sprintf("%d-%d-%d", book.Wins, book.Losses, book.Pushes)
	}
	streak := "–"
	if book.CurrentWin > 0 {
		streak = fmt.Sprintf("W%d", book.CurrentWin)
	} else if book.CurrentLoss > 0 {
		streak = fmt.Sprintf("L%d", book.CurrentLoss)
	}
	return GroupRecordSnapshot{
		HasSlips:   book.TotalSlips > 0,
		Season:     season,
		Record:     rec,
		Wins:       book.Wins,
		Losses:     book.Losses,
		Pushes:     book.Pushes,
		TotalSlips: book.TotalSlips,
		Streak:     streak,
	}
}

func recentSlipResult(slip db.Slip) (label, tone string) {
	if slip.Status != db.SlipGraded {
		return "Closed", "closed"
	}
	switch slip.Result {
	case parlay.Won:
		return "Won", "won"
	case parlay.Lost:
		return "Lost", "lost"
	case parlay.Push, parlay.Void:
		return "Push", "push"
	default:
		return "Closed", "closed"
	}
}

func slipProgressSummary(members []db.SlipMember, legs []db.Leg, legsPerMember int) (lockedCount int, progress string) {
	for _, m := range members {
		if m.LockedAt != nil {
			lockedCount++
		}
	}
	memberLabel := fmt.Sprintf("%d members", len(members))
	if len(members) == 1 {
		memberLabel = "1 member"
	}
	return lockedCount, fmt.Sprintf("%s · %d/%d picks", memberLabel, len(legs), len(members)*legsPerMember)
}

func buildScheduleViews(schedules []db.GroupSlipSchedule) []ScheduleView {
	views := make([]ScheduleView, 0, len(schedules))
	for _, s := range schedules {
		views = append(views, ScheduleView{
			ID:            s.ID,
			Name:          s.Name,
			SlateScope:    s.SlateScope,
			ScopeBadge:    s.SlateScope.Label(),
			LegsPerMember: s.LegsPerMember,
			IsActive:      s.IsActive,
		})
	}
	return views
}

// scopeBadge returns the display label for a slip's slate scope badge.
func scopeBadge(slip db.Slip) string {
	return slips.ScopeBadge(slip)
}

// home lists every group the person belongs to, oldest first. A person with one
// group skips the list and lands on that group's week. The list carries no live
// stream; app.js refetches it when the tab becomes visible.
func (h *slipHandlers) home(w http.ResponseWriter, r *http.Request) {
	page := h.pageWithTab(r, "Groups", TabGroups)
	if h.deps.Store == nil {
		h.renderHome(w, r, page, nil)
		return
	}

	user, ok := h.user(r.Context())
	if !ok {
		h.renderHome(w, r, page, nil)
		return
	}
	groupList, err := h.deps.Store.ForUser(r.Context(), user.ID)
	if err != nil {
		h.serverError(w, err)
		return
	}

	cards := make([]HomeCard, 0, len(groupList))
	now := h.deps.now()
	week, hasWeek, ok := h.latestWeek(w, r)
	if !ok {
		return
	}
	var games []db.Game
	if hasWeek && len(groupList) > 0 {
		var err error
		games, err = h.deps.Store.GamesForWeek(r.Context(), week.ID)
		if err != nil {
			h.serverError(w, err)
			return
		}
	}
	for _, group := range groupList {
		summary := "No slip yet"
		if hasWeek {
			groupSlips, err := h.deps.Store.SlipsForWeek(r.Context(), group.ID, week.ID)
			if err != nil {
				h.serverError(w, err)
				return
			}
			if len(groupSlips) > 0 {
				var ok bool
				summary, ok = h.groupWeekSummary(w, r, groupSlips, games, user.ID, now)
				if !ok {
					return
				}
			}
		}
		cards = append(cards, HomeCard{ID: group.ID, Name: group.Name, Summary: summary})
	}

	if len(cards) == 1 && !isHTMX(r) {
		h.redirect(w, r, fmt.Sprintf("/groups/%d/week", cards[0].ID))
		return
	}
	h.renderHome(w, r, page, cards)
}

// latestWeek returns the latest stored week, and whether there is one.
func (h *slipHandlers) latestWeek(w http.ResponseWriter, r *http.Request) (db.Week, bool, bool) {
	week, err := h.deps.Store.LatestWeek(r.Context())
	if errors.Is(err, db.ErrWeekNotFound) {
		return db.Week{}, false, true
	}
	if err != nil {
		h.serverError(w, err)
		return db.Week{}, false, false
	}
	return week, true, true
}

// groupWeekSummary picks the highest-priority next-step summary across all of
// a group's slips in the week. It writes the error response and returns false
// when a query fails.
func (h *slipHandlers) groupWeekSummary(
	w http.ResponseWriter,
	r *http.Request,
	groupSlips []db.Slip,
	games []db.Game,
	viewerID int64,
	now time.Time,
) (string, bool) {
	var best slips.Next
	hasBest := false
	for _, slip := range groupSlips {
		members, err := h.deps.Store.SlipMembers(r.Context(), slip.ID)
		if err != nil {
			h.serverError(w, err)
			return "", false
		}
		legs, err := h.deps.Store.LegsForSlip(r.Context(), slip.ID)
		if err != nil {
			h.serverError(w, err)
			return "", false
		}
		next := slips.NextStep(slip, members, legs, games, viewerID, now)
		if !hasBest || betterStep(next, best) {
			best = next
			hasBest = true
		}
	}
	if !hasBest {
		return "No slip yet", true
	}
	return best.Label(), true
}

// betterStep reports whether candidate should headline a group card ahead of
// current: actionable steps beat waiting/export, which beat closed, which beat
// done; ties break by earliest upcoming deadline, then existing slice order.
func betterStep(candidate, current slips.Next) bool {
	cp, curP := stepPriority(candidate.Step), stepPriority(current.Step)
	if cp != curP {
		return cp > curP
	}
	if candidate.Deadline == nil {
		return false
	}
	if current.Deadline == nil {
		return true
	}
	return candidate.Deadline.Before(*current.Deadline)
}

func stepPriority(step slips.Step) int {
	switch step {
	case slips.StepPick, slips.StepLockIn, slips.StepJoin:
		return 4
	case slips.StepWaiting, slips.StepExport:
		return 3
	case slips.StepClosed:
		return 2
	case slips.StepDone:
		return 1
	default:
		return 0
	}
}

// groupWeek switches the active group and renders the group overview page.
func (h *slipHandlers) groupWeek(w http.ResponseWriter, r *http.Request) {
	if h.deps.Store == nil {
		http.NotFound(w, r)
		return
	}
	user, ok := h.user(r.Context())
	if !ok {
		h.redirect(w, r, "/login")
		return
	}
	groupID, valid := parseID(r.PathValue("id"))
	if !valid {
		http.NotFound(w, r)
		return
	}
	ctx := r.Context()
	member, err := h.deps.Store.IsMember(ctx, groupID, user.ID)
	if err != nil {
		h.serverError(w, err)
		return
	}
	if !member {
		http.NotFound(w, r)
		return
	}
	group, err := h.deps.Store.GroupByID(ctx, groupID)
	if errors.Is(err, db.ErrGroupNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		h.serverError(w, err)
		return
	}

	h.setGroup(w, groupID)
	page := h.pageForGroup(r, "This week", groupID)
	now := h.deps.now()

	upcoming, err := h.deps.Store.UpcomingWeeks(ctx, now)
	if err != nil {
		h.serverError(w, err)
		return
	}

	var activeWeek *db.Week
	var nextWeek *db.Week
	if len(upcoming) > 0 {
		w0 := upcoming[0]
		activeWeek = &w0
		nextWeek = &w0
	} else {
		latest, hasLatest, ok := h.latestWeek(w, r)
		if !ok {
			return
		}
		if hasLatest {
			activeWeek = &latest
		}
	}

	var weekGames []db.Game
	var weekSlips []groupOverviewSlip
	if activeWeek != nil {
		page.WeekLabel = weekLabel(*activeWeek)
		groupSlips, err := slips.EnsureWeekSlips(ctx, h.deps.Store, group.ID, activeWeek.ID, now)
		if err != nil {
			h.serverError(w, err)
			return
		}
		weekGames, err = h.deps.Store.GamesForWeek(ctx, activeWeek.ID)
		if err != nil {
			h.serverError(w, err)
			return
		}
		weekSlips = make([]groupOverviewSlip, 0, len(groupSlips))
		for _, slip := range groupSlips {
			members, err := h.deps.Store.SlipMembers(ctx, slip.ID)
			if err != nil {
				h.serverError(w, err)
				return
			}
			legs, err := h.deps.Store.LegsForSlip(ctx, slip.ID)
			if err != nil {
				h.serverError(w, err)
				return
			}
			weekSlips = append(weekSlips, groupOverviewSlip{
				Slip:    slip,
				Week:    *activeWeek,
				Members: members,
				Legs:    legs,
			})
		}
	}

	recentSlips, err := h.deps.Store.RecentSlipsForGroup(ctx, group.ID, recentSlipsLimit+len(weekSlips))
	if err != nil {
		h.serverError(w, err)
		return
	}

	recordSeason, ok := h.overviewRecordSeason(w, r, group.ID, activeWeek, now)
	if !ok {
		return
	}
	gradedSlips, err := h.deps.Store.GradedSlipsForGroupSeason(ctx, group.ID, recordSeason)
	if err != nil {
		h.serverError(w, err)
		return
	}

	schedules, err := h.deps.Store.GroupSlipSchedules(ctx, group.ID)
	if err != nil {
		h.serverError(w, err)
		return
	}
	groupMembers, err := h.deps.Store.Members(ctx, group.ID)
	if err != nil {
		h.serverError(w, err)
		return
	}

	overview := buildGroupOverviewPage(groupOverviewInput{
		Page:         page,
		Group:        group,
		ActiveWeek:   activeWeek,
		NextWeek:     nextWeek,
		WeekSlips:    weekSlips,
		WeekGames:    weekGames,
		RecentSlips:  recentSlips,
		RecordSeason: recordSeason,
		GradedSlips:  gradedSlips,
		Schedules:    schedules,
		GroupMembers: groupMembers,
		ViewerID:     user.ID,
		Now:          now,
	})
	h.renderPage(w, "week", overview)
}

func (h *slipHandlers) overviewRecordSeason(
	w http.ResponseWriter,
	r *http.Request,
	groupID int64,
	activeWeek *db.Week,
	now time.Time,
) (int, bool) {
	ctx := r.Context()
	gradedSeasons, err := h.deps.Store.GradedSeasonsForGroup(ctx, groupID)
	if err != nil {
		h.serverError(w, err)
		return 0, false
	}
	if len(gradedSeasons) > 0 {
		return gradedSeasons[0], true
	}
	allSeasons, err := h.deps.Store.SeasonsForGroup(ctx, groupID)
	if err != nil {
		h.serverError(w, err)
		return 0, false
	}
	if len(allSeasons) > 0 {
		return allSeasons[0], true
	}
	if activeWeek != nil {
		return activeWeek.Season, true
	}
	return now.Year(), true
}

// renderHome writes the group list, or just its list element for an HTMX
// refresh.
func (h *slipHandlers) renderHome(w http.ResponseWriter, r *http.Request, page Page, cards []HomeCard) {
	view := HomePage{Page: page, Cards: cards}
	if isHTMX(r) {
		h.renderPartial(w, "group-list", view)
		return
	}
	h.renderPage(w, "home", view)
}

// setGroup makes groupID the browser's active group. A nil seam leaves the
// shell's small constructor without a cookie.
func (h *slipHandlers) setGroup(w http.ResponseWriter, groupID int64) {
	if h.deps.SetGroup != nil {
		h.deps.SetGroup(w, groupID)
	}
}

// isHTMX reports whether HTMX sent the request.
func isHTMX(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("Hx-Request"), "true")
}
