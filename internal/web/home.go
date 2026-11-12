package web

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/kalverra/parlay-club/internal/db"
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

// WeekPage is the multi-slip hub rendered for a group's week when multiple slips exist.
type WeekPage struct {
	Page
	Group     db.Group
	Week      db.Week
	Cards     []WeekSlipCard
	Schedules []ScheduleView
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

// WeekSlipCard is one slip card shown in the multi-slip week view.
type WeekSlipCard struct {
	ID          int64
	Name        string
	ScopeBadge  string
	NextStep    string
	Deadline    *DeadlineView
	MemberCount int
	PicksCount  int
	TotalPicks  int
	LockedCount int
	Progress    string
}

// scopeBadge returns the display label for a slip's slate scope badge.
func scopeBadge(scope db.SlateScope) string {
	return scope.Label()
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
	for _, group := range groupList {
		summary := "No slip yet"
		if hasWeek {
			groupSlips, err := h.deps.Store.SlipsForWeek(r.Context(), group.ID, week.ID)
			if err != nil {
				h.serverError(w, err)
				return
			}
			if len(groupSlips) > 0 {
				summary = h.groupStep(w, r, groupSlips[0], user.ID, now)
				if summary == "" {
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

// groupStep is the viewer's next-step summary on a group's newest slip. It
// writes the error response and returns "" when a query fails.
func (h *slipHandlers) groupStep(
	w http.ResponseWriter,
	r *http.Request,
	slip db.Slip,
	viewerID int64,
	now time.Time,
) string {
	members, err := h.deps.Store.SlipMembers(r.Context(), slip.ID)
	if err != nil {
		h.serverError(w, err)
		return ""
	}
	legs, err := h.deps.Store.LegsForSlip(r.Context(), slip.ID)
	if err != nil {
		h.serverError(w, err)
		return ""
	}
	games, err := h.deps.Store.GamesForWeek(r.Context(), slip.WeekID)
	if err != nil {
		h.serverError(w, err)
		return ""
	}
	return slips.NextStep(slip, members, legs, games, viewerID, now).Label()
}

// groupWeek switches the active group and opens the newest slip of the latest
// week, or the create form when the group has none yet. When multiple slips
// exist for the week, it renders the multi-slip week hub.
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
	member, err := h.deps.Store.IsMember(r.Context(), groupID, user.ID)
	if err != nil {
		h.serverError(w, err)
		return
	}
	if !member {
		http.NotFound(w, r)
		return
	}
	group, err := h.deps.Store.GroupByID(r.Context(), groupID)
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

	week, err := h.deps.Store.LatestWeek(r.Context())
	if errors.Is(err, db.ErrWeekNotFound) {
		h.renderCreate(w, r, page, user, group, nil, nil)
		return
	}
	if err != nil {
		h.serverError(w, err)
		return
	}
	page.WeekLabel = weekLabel(week)

	groupSlips, err := slips.EnsureWeekSlips(r.Context(), h.deps.Store, group.ID, week.ID)
	if err != nil {
		h.serverError(w, err)
		return
	}
	if len(groupSlips) == 0 {
		h.renderCreate(w, r, page, user, group, &week, nil)
		return
	}
	if len(groupSlips) == 1 {
		h.redirect(w, r, fmt.Sprintf("/slips/%d", groupSlips[0].ID))
		return
	}

	games, err := h.deps.Store.GamesForWeek(r.Context(), week.ID)
	if err != nil {
		h.serverError(w, err)
		return
	}

	now := h.deps.now()
	cards := make([]WeekSlipCard, 0, len(groupSlips))
	for _, slip := range groupSlips {
		members, err := h.deps.Store.SlipMembers(r.Context(), slip.ID)
		if err != nil {
			h.serverError(w, err)
			return
		}
		legs, err := h.deps.Store.LegsForSlip(r.Context(), slip.ID)
		if err != nil {
			h.serverError(w, err)
			return
		}

		next := slips.NextStep(slip, members, legs, games, user.ID, now)
		var deadline *DeadlineView
		if next.Deadline != nil {
			deadline = newDeadlineView(games, *next.Deadline)
		}

		lockedCount := 0
		for _, m := range members {
			if m.LockedAt != nil {
				lockedCount++
			}
		}

		memberLabel := fmt.Sprintf("%d members", len(members))
		if len(members) == 1 {
			memberLabel = "1 member"
		}
		progress := fmt.Sprintf("%s · %d/%d picks", memberLabel, len(legs), len(members)*slip.LegsPerMember)

		cards = append(cards, WeekSlipCard{
			ID:          slip.ID,
			Name:        slip.Name,
			ScopeBadge:  scopeBadge(slip.SlateScope),
			NextStep:    next.Label(),
			Deadline:    deadline,
			MemberCount: len(members),
			PicksCount:  len(legs),
			TotalPicks:  len(members) * slip.LegsPerMember,
			LockedCount: lockedCount,
			Progress:    progress,
		})
	}

	schedules, err := h.deps.Store.GroupSlipSchedules(r.Context(), group.ID)
	if err != nil {
		h.serverError(w, err)
		return
	}
	schedViews := make([]ScheduleView, 0, len(schedules))
	for _, s := range schedules {
		schedViews = append(schedViews, ScheduleView{
			ID:            s.ID,
			Name:          s.Name,
			SlateScope:    s.SlateScope,
			ScopeBadge:    s.SlateScope.Label(),
			LegsPerMember: s.LegsPerMember,
			IsActive:      s.IsActive,
		})
	}

	h.renderPage(w, "week", WeekPage{
		Page:      page,
		Group:     group,
		Week:      week,
		Cards:     cards,
		Schedules: schedViews,
	})
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
