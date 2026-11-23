package web

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/kalverra/parlay-club/internal/analyzer"
	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/parlay"
	"github.com/kalverra/parlay-club/internal/slips"
)

// maxFormBytes caps a submitted slip form.
const maxFormBytes = 16 << 10

// defaultStake is the payout calculator's starting stake.
const defaultStake = 10

// ruleErrors are the refusals a person can fix, so they answer 422 with the
// service's short message. Anything else is a 500.
var ruleErrors = []error{
	slips.ErrNotMember,
	slips.ErrNotOwner,
	slips.ErrLegLimit,
	slips.ErrLegsPerMemberTooLow,
	slips.ErrKickoffLocked,
	slips.ErrGameOutsideScope,
	slips.ErrLockedIn,
	slips.ErrLegsNotReady,
	slips.ErrSlipNotOpen,
	slips.ErrNotLocked,
	slips.ErrMemberHasLegs,
	slips.ErrOwnVote,
	slips.ErrVotesClosed,
	slips.ErrVoteInvalid,
	slips.ErrWagerStakeInvalid,
	slips.ErrWagerOddsInvalid,
	slips.ErrWagerBoostInvalid,
	slips.ErrWagerBookTooLong,
	db.ErrSlipNameRequired,
	db.ErrSlipNameTaken,
	db.ErrScheduleNameRequired,
	db.ErrScheduleNameTaken,
	db.ErrSlateScopeInvalid,
	db.ErrLegsPerMemberRange,
	db.ErrSlipMembersRequired,
	db.ErrGameNotFound,
	db.ErrLegResultInvalid,
}

// slipHandlers serves the Slip tab and the profile screen.
type slipHandlers struct {
	deps Dependencies
}

// Routes mounts every slip route. They sit inside RequireUser and the group
// scope, so the signed-in person and active group are already on the context.
func (h *slipHandlers) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /{$}", h.home)
	mux.HandleFunc("GET /analyzer/guide", h.analyzerGuide)
	mux.HandleFunc("GET /analyzer/results", h.evalResults)
	mux.HandleFunc("GET /record", h.recordBook)
	mux.HandleFunc("GET /groups/{id}/week", h.groupWeek)
	mux.HandleFunc("GET /me", h.profile)
	mux.HandleFunc("POST /me", h.updateProfile)
	mux.HandleFunc("POST /users/{id}/active", h.toggleActive)

	mux.HandleFunc("GET /groups/{id}/schedules", h.listSchedules)
	mux.HandleFunc("POST /groups/{id}/schedules", h.createSchedule)
	mux.HandleFunc("POST /groups/{id}/schedules/{scheduleID}/active", h.setScheduleActive)
	mux.HandleFunc("PATCH /groups/{id}/schedules/{scheduleID}", h.updateSchedule)
	mux.HandleFunc("DELETE /groups/{id}/schedules/{scheduleID}", h.deleteSchedule)

	mux.HandleFunc("GET /slips/new", h.newSlipForm)
	mux.HandleFunc("POST /slips", h.createSlip)
	mux.HandleFunc("GET /slips/{id}", h.slipPage)
	mux.HandleFunc("GET /slips/{id}/pick", h.pickBoard)
	mux.HandleFunc("GET /slips/{id}/pick/results", h.pickResults)
	mux.HandleFunc("GET /slips/{id}/pick/slate", h.pickSlate)
	mux.HandleFunc("GET /slips/{id}/pick/detail", h.pickDetail)
	mux.HandleFunc("POST /slips/{id}/pick/refresh", h.pickRefresh)
	mux.HandleFunc("GET /slips/{id}/pick/swap", h.pickSwap)
	mux.HandleFunc("GET /slips/{id}/pick/dock", h.pickDock)
	mux.HandleFunc("GET /slips/{id}/tray", h.pickTray)
	mux.HandleFunc("GET /slips/{id}/panel", h.panel)
	mux.HandleFunc("GET /slips/{id}/export", h.export)
	mux.HandleFunc("GET /slips/{id}/payout", h.payout)
	mux.HandleFunc("PUT /slips/{id}/wagers/me", h.setWager)
	mux.HandleFunc("DELETE /slips/{id}/wagers/me", h.deleteWager)
	mux.HandleFunc("GET /slips/{id}/activity", h.activity)
	mux.HandleFunc("POST /slips/{id}/members", h.addMember)
	mux.HandleFunc("DELETE /slips/{id}/members/{userID}", h.removeMember)
	mux.HandleFunc("PATCH /slips/{id}", h.updateSlip)
	mux.HandleFunc("POST /slips/{id}/legs", h.addLeg)
	mux.HandleFunc("GET /legs/{id}/edit", h.editLegForm)
	mux.HandleFunc("PUT /legs/{id}", h.updateLeg)
	mux.HandleFunc("PUT /legs/{id}/result", h.setLegResult)
	mux.HandleFunc("POST /legs/{id}/vote", h.voteLeg)
	mux.HandleFunc("DELETE /legs/{id}", h.deleteLeg)
	mux.HandleFunc("POST /slips/{id}/lock", h.lockIn)
	mux.HandleFunc("POST /slips/{id}/unlock", h.unlock)
	mux.HandleFunc("GET /slips/{id}/events", h.events)
}

// SlipPage is the full Slip tab. Slip is nil when the group has no slip yet,
// and Creating turns the page into the create form.
type SlipPage struct {
	Page
	LiveURL  string
	Slip     *SlipView
	Export   ExportView
	Creating bool
	Create   SlipCreateView
	Next     NextView
	Siblings []SiblingSlipView
}

// SiblingSlipView is one choice in the sibling slip switcher.
type SiblingSlipView struct {
	ID      int64
	Name    string
	Current bool
	Summary string
}

// NextView is the hub's step banner and its one primary action.
type NextView struct {
	Step     slips.Step
	Title    string
	Detail   string
	Deadline *DeadlineView
	Action   *NextAction
	Nudges   []NextNudge
}

// NextNudge is the share-sheet button for one member the slip waits on. The
// message is built in app.js so its time reads in the sender's zone and its
// link is absolute; the server sends only the parts.
type NextNudge struct {
	Name     string
	SlipName string
	CloseISO string
	Path     string
}

// DeadlineView is the first kickoff still ahead in the slip's week.
type DeadlineView struct {
	Matchup string
	ISO     string
	Label   string
}

// NextAction is the button under the step banner. Href makes a link and
// PostURL makes an HTMX form. UserID fills the join form's hidden field.
type NextAction struct {
	Label   string
	Href    string
	PostURL string
	UserID  int64
}

// SlipView is everything the slip panel draws.
type SlipView struct {
	ID            int64
	Name          string
	Status        string
	LegsPerMember int
	Open          bool
	LockedIn      int
	MemberCount   int
	Members       []SlipMemberView
	Payout        PayoutView
	Wager         WagerView
	Activity      []ActivityView
	Analyzer      AnalyzerView
	Siblings      []SiblingSlipView
}

// SlipMemberView is one member's section of the panel.
type SlipMemberView struct {
	User      User
	Locked    bool
	IsCurrent bool
	CanLockIn bool
	Legs      []LegView
	LegCount  int
}

// LegView is one leg row.
type LegView struct {
	ID              int64
	Selection       string
	Odds            string
	Model           string // nfelo chip; "" when there is no fresh model
	Grade           GradeView
	Result          string
	GradedBy        string
	OwnerName       string
	CanEdit         bool
	CanGrade        bool
	ResearchURL     string
	PlayerHeadshot  string
	PlayerInitials  string
	TeamLogo        string
	MatchupAwayLogo string
	MatchupHomeLogo string
	Votes           VoteView
}

// VoteView is one leg's tally and the viewer's own vote. UpValue and DownValue
// are what the next tap sends: the matching value again clears the vote.
type VoteView struct {
	Up        int
	Down      int
	Mine      int
	CanVote   bool
	Has       bool
	UpValue   int
	DownValue int
}

// ActivityView is one activity line.
type ActivityView struct {
	Message string
	When    string
}

// SlipCreateView is the create-slip form. Season, SeasonType, and WeekNum hold
// a past week the slate sync never stored; filling them overrides WeekID.
// Solo says the group has only the viewer so far.
type SlipCreateView struct {
	Weeks         []WeekOption
	WeekID        int64
	Season        int
	SeasonType    int
	WeekNum       int
	Name          string
	SlateScope    string
	SlateScopes   []SlateScopeOption
	LegsPerMember int
	Members       []MemberOption
	Selected      []int64
	Solo          bool
	Error         string
}

// SlateScopeOption is one choice in the slip create form's slate scope picker.
type SlateScopeOption struct {
	Value    string
	Label    string
	Selected bool
}

// WeekOption is one week in the create form.
type WeekOption struct {
	ID    int64
	Label string
}

// MemberOption is one checklist entry in the create form.
type MemberOption struct {
	ID       int64
	Name     string
	Selected bool
}

// PayoutView is the payout calculator's rendered values.
type PayoutView struct {
	SlipID            int64
	HasLegs           bool
	Stake             string
	Boost             string
	Combined          string
	CombinedAmerican  string
	Reference         string
	ReferenceAmerican string
	Return            string
	Profit            string
}

// WagerView is the signed-in user's logged bet form on a slip.
type WagerView struct {
	SlipID   int64
	HasWager bool
	Book     string
	Stake    string
	Odds     string
	Boost    string
	Return   string
	Profit   string
}

// LegFormView is the add or edit leg form.
type LegFormView struct {
	SlipID       int64
	LegID        int64
	Action       string
	Target       string
	Editing      bool
	Quick        bool
	Market       string
	Selection    string
	Team         string
	PlayerName   string
	Side         string
	Line         string
	Odds         string
	GameID       string
	GameOptional bool
	Games        []GameOption
	Players      []string
}

// GameOption is one choice in the leg form's game picker.
type GameOption struct {
	ID       string
	Label    string
	Selected bool
}

// LegAddedView is the confirmation a quick-add pill swaps in.
type LegAddedView struct {
	Selection string
}

// ProfilePage is the profile screen.
type ProfilePage struct {
	Page
	HomeBook string
	Members  []ProfileMember
}

// ProfileMember is one person in the profile's member list.
type ProfileMember struct {
	ID     int64
	Name   string
	Avatar User
	Active bool
	Self   bool
}

// newSlipForm shows the create form for a chosen or latest week.
func (h *slipHandlers) newSlipForm(w http.ResponseWriter, r *http.Request) {
	user, group, ok := h.requireActor(w, r)
	if !ok {
		return
	}

	page := h.page(r, "New slip")
	var week *db.Week
	if raw := r.URL.Query().Get("week"); raw != "" {
		id, valid := parseID(raw)
		if !valid {
			http.NotFound(w, r)
			return
		}
		found, err := h.deps.Store.WeekByID(r.Context(), id)
		if errors.Is(err, db.ErrWeekNotFound) {
			http.NotFound(w, r)
			return
		}
		if err != nil {
			h.serverError(w, err)
			return
		}
		week = &found
	} else {
		found, err := h.deps.Store.LatestWeek(r.Context())
		switch {
		case errors.Is(err, db.ErrWeekNotFound):
		case err != nil:
			h.serverError(w, err)
			return
		default:
			week = &found
		}
	}

	h.renderCreate(w, r, page, user, group, week, nil)
}

// createSlip writes a new slip and sends the browser to it.
func (h *slipHandlers) createSlip(w http.ResponseWriter, r *http.Request) {
	user, group, ok := h.requireActor(w, r)
	if !ok {
		return
	}
	if err := readForm(w, r); err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	weekID, ok := h.weekFromCreateForm(w, r, user, group)
	if !ok {
		return
	}

	legsPerMember := slips.DefaultLegsPerMember
	if raw := strings.TrimSpace(r.PostForm.Get("legs_per_member")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			h.renderCreateError(w, r, user, group, "Legs per member must be a number.")
			return
		}
		legsPerMember = parsed
	}

	memberIDs := make([]int64, 0, len(r.PostForm["member_id"]))
	for _, raw := range r.PostForm["member_id"] {
		id, valid := parseID(raw)
		if !valid {
			continue
		}
		member, err := h.deps.Store.IsMember(r.Context(), group.ID, id)
		if err != nil {
			h.serverError(w, err)
			return
		}
		if member {
			memberIDs = append(memberIDs, id)
		}
	}

	scope := r.PostForm.Get("slate_scope")
	if scope == "" {
		scope = r.PostForm.Get("scope")
	}

	created, err := h.deps.Store.CreateSlip(
		r.Context(),
		group.ID,
		weekID,
		nil,
		r.PostForm.Get("name"),
		db.SlateScope(scope),
		legsPerMember,
		memberIDs,
	)
	if err != nil {
		if message, ok := fixableMessage(err); ok {
			h.renderCreateError(w, r, user, group, message)
			return
		}
		h.serverError(w, err)
		return
	}

	h.redirect(w, r, fmt.Sprintf("/slips/%d", created.ID))
}

// weekFromCreateForm resolves the create form's week and writes the refusal. A
// filled season and week number make a week the slate sync never stored;
// otherwise the chosen week_id is used.
func (h *slipHandlers) weekFromCreateForm(
	w http.ResponseWriter,
	r *http.Request,
	user db.User,
	group db.Group,
) (int64, bool) {
	seasonRaw := strings.TrimSpace(r.PostForm.Get("season"))
	weekRaw := strings.TrimSpace(r.PostForm.Get("week"))
	if seasonRaw != "" || weekRaw != "" {
		season, err := strconv.Atoi(seasonRaw)
		if err != nil || season < 1900 || season > 2100 {
			h.renderCreateError(w, r, user, group, "Season must be a year.")
			return 0, false
		}
		weekNum, err := strconv.Atoi(weekRaw)
		if err != nil || weekNum < 1 || weekNum > 22 {
			h.renderCreateError(w, r, user, group, "Week must be a number from 1 to 22.")
			return 0, false
		}
		seasonType, ok := parseSeasonType(r.PostForm.Get("season_type"))
		if !ok {
			h.renderCreateError(w, r, user, group, "Pick a season type.")
			return 0, false
		}
		weekID := db.WeekID(season, seasonType, weekNum)
		if !h.weekIDIsPast(w, r, user, group, weekID) {
			return 0, false
		}
		week, err := h.deps.Store.UpsertWeek(r.Context(), season, seasonType, weekNum)
		if err != nil {
			h.serverError(w, err)
			return 0, false
		}
		return week.ID, true
	}

	weekID, valid := parseID(r.PostForm.Get("week_id"))
	if !valid {
		h.renderCreateError(w, r, user, group, "Pick a week.")
		return 0, false
	}
	if _, err := h.deps.Store.WeekByID(r.Context(), weekID); errors.Is(err, db.ErrWeekNotFound) {
		http.NotFound(w, r)
		return 0, false
	} else if err != nil {
		h.serverError(w, err)
		return 0, false
	}
	return weekID, true
}

// weekIDIsPast refuses a typed week later than the latest known one. With no
// weeks on record there is nothing to compare against, so any week is allowed.
func (h *slipHandlers) weekIDIsPast(
	w http.ResponseWriter,
	r *http.Request,
	user db.User,
	group db.Group,
	weekID int64,
) bool {
	latest, err := h.deps.Store.LatestWeek(r.Context())
	if errors.Is(err, db.ErrWeekNotFound) {
		return true
	}
	if err != nil {
		h.serverError(w, err)
		return false
	}
	if weekID > latest.ID {
		h.renderCreateError(w, r, user, group, "That week hasn't started yet.")
		return false
	}
	return true
}

// slipPage shows one slip in full.
func (h *slipHandlers) slipPage(w http.ResponseWriter, r *http.Request) {
	slip, _, _, ok := h.loadSlip(w, r, r.PathValue("id"))
	if !ok {
		return
	}
	page := h.pageForGroup(r, slip.Name, slip.GroupID)
	h.renderSlip(w, r, page, slip, defaultStake, 0)
}

// panel returns the slip panel partial for HTMX refreshes.
func (h *slipHandlers) panel(w http.ResponseWriter, r *http.Request) {
	slip, _, _, ok := h.loadSlip(w, r, r.PathValue("id"))
	if !ok {
		return
	}
	h.renderPanel(w, r, slip)
}

// payout returns the payout calculator partial.
func (h *slipHandlers) payout(w http.ResponseWriter, r *http.Request) {
	slip, _, _, ok := h.loadSlip(w, r, r.PathValue("id"))
	if !ok {
		return
	}
	stake := parseFloatDefault(r.URL.Query().Get("stake"), defaultStake)
	boost := parseFloatDefault(r.URL.Query().Get("boost"), 0)
	payout, err := h.payoutView(r.Context(), slip, stake, boost)
	if err != nil {
		h.serverError(w, err)
		return
	}
	h.renderPartial(w, "slip-payout", payout)
}

// setWager saves the signed-in user's personal wager on the slip and renders
// the updated "My wager" partial.
func (h *slipHandlers) setWager(w http.ResponseWriter, r *http.Request) {
	slip, user, _, ok := h.loadSlip(w, r, r.PathValue("id"))
	if !ok {
		return
	}
	if err := readForm(w, r); err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	stake, err := strconv.ParseFloat(strings.TrimSpace(r.PostForm.Get("stake")), 64)
	if err != nil {
		h.writeFormError(w, "Stake needs to be a number.")
		return
	}
	odds, err := strconv.Atoi(strings.TrimSpace(r.PostForm.Get("odds")))
	if err != nil {
		h.writeFormError(w, "Odds need to be a whole number.")
		return
	}
	boost := 0.0
	if rawBoost := strings.TrimSpace(r.PostForm.Get("boost")); rawBoost != "" {
		boost, err = strconv.ParseFloat(rawBoost, 64)
		if err != nil {
			h.writeFormError(w, "Boost needs to be a number.")
			return
		}
	}

	_, err = h.deps.Slips.SetWager(r.Context(), user.ID, slip.ID, slips.WagerInput{
		Book:     r.PostForm.Get("book"),
		Stake:    stake,
		Odds:     odds,
		BoostPct: boost,
	})
	if err != nil {
		h.respondError(w, err)
		return
	}
	view, err := h.wagerView(r.Context(), slip, user.ID)
	if err != nil {
		h.serverError(w, err)
		return
	}
	h.renderPartial(w, "slip-wager", view)
}

// deleteWager clears the signed-in user's personal wager on the slip and
// renders the reset "My wager" partial.
func (h *slipHandlers) deleteWager(w http.ResponseWriter, r *http.Request) {
	slip, user, _, ok := h.loadSlip(w, r, r.PathValue("id"))
	if !ok {
		return
	}
	if err := h.deps.Slips.DeleteWager(r.Context(), user.ID, slip.ID); err != nil {
		h.respondError(w, err)
		return
	}
	view, err := h.wagerView(r.Context(), slip, user.ID)
	if err != nil {
		h.serverError(w, err)
		return
	}
	h.renderPartial(w, "slip-wager", view)
}

// activity returns the activity feed partial.
func (h *slipHandlers) activity(w http.ResponseWriter, r *http.Request) {
	slip, _, _, ok := h.loadSlip(w, r, r.PathValue("id"))
	if !ok {
		return
	}
	page, err := h.slipPageData(r.Context(), slip)
	if err != nil {
		h.serverError(w, err)
		return
	}
	h.renderPartial(w, "slip-activity", page)
}

// addMember puts another group member on the slip.
func (h *slipHandlers) addMember(w http.ResponseWriter, r *http.Request) {
	slip, user, group, ok := h.loadSlip(w, r, r.PathValue("id"))
	if !ok {
		return
	}
	if err := readForm(w, r); err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	userID, valid := parseID(r.PostForm.Get("user_id"))
	if !valid {
		http.NotFound(w, r)
		return
	}
	member, err := h.deps.Store.IsMember(r.Context(), group.ID, userID)
	if err != nil {
		h.serverError(w, err)
		return
	}
	if !member {
		http.NotFound(w, r)
		return
	}

	if err := h.deps.Slips.AddMember(r.Context(), user.ID, slip.ID, userID); err != nil {
		h.respondError(w, err)
		return
	}
	h.renderPanel(w, r, slip)
}

// removeMember takes a member off the slip.
func (h *slipHandlers) removeMember(w http.ResponseWriter, r *http.Request) {
	slip, user, _, ok := h.loadSlip(w, r, r.PathValue("id"))
	if !ok {
		return
	}
	userID, valid := parseID(r.PathValue("userID"))
	if !valid {
		http.NotFound(w, r)
		return
	}
	if err := h.deps.Slips.RemoveMember(r.Context(), user.ID, slip.ID, userID); err != nil {
		h.respondError(w, err)
		return
	}
	h.renderPanel(w, r, slip)
}

// updateSlip changes the name or legs per member.
func (h *slipHandlers) updateSlip(w http.ResponseWriter, r *http.Request) {
	slip, user, _, ok := h.loadSlip(w, r, r.PathValue("id"))
	if !ok {
		return
	}
	if err := readForm(w, r); err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	legsPerMember, valid := parseID(r.PostForm.Get("legs_per_member"))
	if !valid {
		h.writeFormError(w, "Legs per member must be a number.")
		return
	}
	name := r.PostForm.Get("name")
	if name == "" {
		name = slip.Name
	}
	updated, err := h.deps.Slips.UpdateSlip(r.Context(), user.ID, slip.ID, name, int(legsPerMember))
	if err != nil {
		h.respondError(w, err)
		return
	}
	h.renderPanel(w, r, updated)
}

// editLegForm shows the leg form filled from an existing leg.
func (h *slipHandlers) editLegForm(w http.ResponseWriter, r *http.Request) {
	leg, _, ok := h.loadLeg(w, r, r.PathValue("id"))
	if !ok {
		return
	}
	slip, err := h.deps.Store.SlipByID(r.Context(), leg.SlipID)
	if err != nil {
		h.respondError(w, err)
		return
	}
	form, err := h.withGames(r.Context(), legFormView(leg), slip)
	if err != nil {
		h.serverError(w, err)
		return
	}
	h.renderPartial(w, "leg-form", form)
}

// withGames fills the leg form's game picker. Games that already kicked off
// stay off a current week's list; a past week lists them all so history can be
// entered. The game stays optional on a past week, on an edit that had none, or
// when the week has no game to pick.
func (h *slipHandlers) withGames(
	ctx context.Context,
	form LegFormView,
	slip db.Slip,
) (LegFormView, error) {
	now := h.deps.now()
	past, err := h.deps.Store.WeekPast(ctx, slip.WeekID, now)
	if err != nil {
		return LegFormView{}, err
	}
	games, err := h.deps.Store.GamesForWeek(ctx, slip.WeekID)
	if err != nil {
		return LegFormView{}, err
	}
	games = slips.FilterGamesByScope(games, slip.SlateScope)

	selected := false
	for _, game := range games {
		if !past && !now.Before(game.Kickoff) {
			continue
		}
		option := GameOption{
			ID:    game.ID,
			Label: gameOptionLabel(game),
		}
		if game.ID == form.GameID {
			option.Selected = true
			selected = true
		}
		form.Games = append(form.Games, option)
	}
	form.GameOptional = past || len(form.Games) == 0 ||
		(form.Editing && form.GameID == "")
	if !selected && form.GameID != "" {
		form.Games = append(
			[]GameOption{{Label: "Pick a game", Selected: true}},
			form.Games...,
		)
		form.GameOptional = false
	}
	if h.deps.Store != nil {
		players, err := h.deps.Store.WeekPlayerNames(ctx, slip.WeekID)
		if err != nil {
			return LegFormView{}, err
		}
		form.Players = players
	}
	return form, nil
}

// gameOptionLabel is one game's line in the leg form's picker.
func gameOptionLabel(game db.Game) string {
	kickoff := game.Kickoff.UTC()
	return fmt.Sprintf(
		"%s @ %s · %s",
		game.AwayTeam,
		game.HomeTeam,
		kickoff.Format("Mon 3:04 PM")+" UTC",
	)
}

// addLeg adds one leg for the signed-in member.
func (h *slipHandlers) addLeg(w http.ResponseWriter, r *http.Request) {
	slip, user, _, ok := h.loadSlip(w, r, r.PathValue("id"))
	if !ok {
		return
	}
	leg, valid := h.legFromForm(w, r, slip.ID)
	if !valid {
		return
	}

	created, err := h.deps.Slips.AddLeg(r.Context(), user.ID, leg)
	if err != nil {
		h.respondError(w, err)
		return
	}
	if r.PostForm.Get("from") == "board" {
		w.Header().Set("Hx-Trigger", "tray-updated")
		h.renderPartial(w, "pick-line-group", h.pickPairRow(r.Context(), slip, created, true))
		return
	}
	// Tell the browser the bottom sheet can close now that the leg landed.
	w.Header().Set("Hx-Trigger", "leg-saved")
	if r.PostForm.Get("quick") != "" {
		h.renderPartial(w, "leg-added", LegAddedView{Selection: created.Selection})
		return
	}
	h.renderPanel(w, r, slip)
}

// updateLeg saves the pick fields of an existing leg.
func (h *slipHandlers) updateLeg(w http.ResponseWriter, r *http.Request) {
	existing, user, ok := h.loadLeg(w, r, r.PathValue("id"))
	if !ok {
		return
	}
	leg, valid := h.legFromForm(w, r, existing.SlipID)
	if !valid {
		return
	}
	leg.ID = existing.ID

	updated, err := h.deps.Slips.UpdateLeg(r.Context(), user.ID, leg)
	if err != nil {
		h.respondError(w, err)
		return
	}
	slip, err := h.deps.Store.SlipByID(r.Context(), existing.SlipID)
	if err != nil {
		h.respondError(w, err)
		return
	}
	switch r.PostForm.Get("from") {
	case "board":
		trigger := "tray-updated"
		if !sameBoardLine(existing, updated) {
			// The swapped-out line is still on screen. Refresh the board so it
			// does not keep a delete button for a leg that moved away.
			trigger = "tray-updated, board-updated"
		}
		w.Header().Set("Hx-Trigger", trigger)
		h.renderPartial(w, "pick-line-group", h.pickPairRow(r.Context(), slip, updated, true))
		return
	case "tray":
		dock, err := h.pickDockData(r.Context(), slip)
		if err != nil {
			h.serverError(w, err)
			return
		}
		w.Header().Set("Hx-Trigger", "tray-updated, board-updated")
		h.renderPartial(w, "pick-tray", dock)
		return
	}
	w.Header().Set("Hx-Trigger", "leg-saved")
	h.renderPanel(w, r, slip)
}

// setLegResult grades one leg by hand. Any slip member can set any leg's
// result, and the refreshed panel shows the recomputed slip.
func (h *slipHandlers) setLegResult(w http.ResponseWriter, r *http.Request) {
	leg, user, ok := h.loadLeg(w, r, r.PathValue("id"))
	if !ok {
		return
	}
	if err := readForm(w, r); err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	result := parlay.Result(strings.ToUpper(strings.TrimSpace(r.PostForm.Get("result"))))
	slip, err := h.deps.Slips.SetLegResult(r.Context(), user.ID, leg.ID, result)
	if err != nil {
		h.respondError(w, err)
		return
	}
	h.renderPanel(w, r, slip)
}

// voteLeg records the signed-in member's vote on someone else's leg. Up, down,
// and clear refresh the panel so the tally updates without a reload.
func (h *slipHandlers) voteLeg(w http.ResponseWriter, r *http.Request) {
	leg, user, ok := h.loadLeg(w, r, r.PathValue("id"))
	if !ok {
		return
	}
	if err := readForm(w, r); err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	value, err := strconv.Atoi(strings.TrimSpace(r.PostForm.Get("value")))
	if err != nil || (value != -1 && value != 0 && value != 1) {
		h.writeFormError(w, slips.ErrVoteInvalid.Error())
		return
	}
	if err := h.deps.Slips.Vote(r.Context(), user.ID, leg.ID, value); err != nil {
		h.respondError(w, err)
		return
	}
	slip, err := h.deps.Store.SlipByID(r.Context(), leg.SlipID)
	if err != nil {
		h.respondError(w, err)
		return
	}
	h.renderPanel(w, r, slip)
}

// deleteLeg removes one leg.
func (h *slipHandlers) deleteLeg(w http.ResponseWriter, r *http.Request) {
	leg, user, ok := h.loadLeg(w, r, r.PathValue("id"))
	if !ok {
		return
	}
	if err := h.deps.Slips.DeleteLeg(r.Context(), user.ID, leg.ID); err != nil {
		h.respondError(w, err)
		return
	}
	slip, err := h.deps.Store.SlipByID(r.Context(), leg.SlipID)
	if err != nil {
		h.respondError(w, err)
		return
	}
	switch r.URL.Query().Get("from") {
	case "tray":
		dock, err := h.pickDockData(r.Context(), slip)
		if err != nil {
			h.serverError(w, err)
			return
		}
		w.Header().Set("Hx-Trigger", "tray-updated, board-updated")
		h.renderPartial(w, "pick-tray", dock)
		return
	case "board":
		w.Header().Set("Hx-Trigger", "tray-updated")
		h.renderPartial(w, "pick-line-group", h.pickPairRow(r.Context(), slip, leg, false))
		return
	}
	h.renderPanel(w, r, slip)
}

// lockIn records the signed-in member's lock.
func (h *slipHandlers) lockIn(w http.ResponseWriter, r *http.Request) {
	slip, user, _, ok := h.loadSlip(w, r, r.PathValue("id"))
	if !ok {
		return
	}
	if err := h.deps.Slips.LockIn(r.Context(), user.ID, slip.ID); err != nil {
		h.respondError(w, err)
		return
	}
	if h.deps.Refresh != nil {
		updated, err := h.deps.Store.SlipByID(r.Context(), slip.ID)
		if err != nil {
			h.serverError(w, err)
			return
		}
		if updated.Status == db.SlipLocked {
			h.deps.Refresh.KickSlipExportEnrich(r.Context(), updated.ID)
		}
		slip = updated
	}
	// The export card is a sibling of the panel, so it needs its own refresh
	// signal even when no SSE stream is open.
	w.Header().Set("Hx-Trigger", "slip-updated")
	h.renderPanel(w, r, slip)
}

// unlock clears every member's lock.
func (h *slipHandlers) unlock(w http.ResponseWriter, r *http.Request) {
	slip, user, _, ok := h.loadSlip(w, r, r.PathValue("id"))
	if !ok {
		return
	}
	if err := h.deps.Slips.Unlock(r.Context(), user.ID, slip.ID); err != nil {
		h.respondError(w, err)
		return
	}
	// Hide the export card for the person who unlocked, without waiting on SSE.
	w.Header().Set("Hx-Trigger", "slip-updated")
	h.renderPanel(w, r, slip)
}

// events serves the slip's SSE stream.
func (h *slipHandlers) events(w http.ResponseWriter, r *http.Request) {
	slip, _, _, ok := h.loadSlip(w, r, r.PathValue("id"))
	if !ok {
		return
	}
	if slip.Status == db.SlipGraded {
		header := w.Header()
		header.Set("Content-Type", "text/event-stream")
		header.Set("Cache-Control", "no-cache")
		header.Set("Connection", "keep-alive")
		header.Set("X-Accel-Buffering", "no")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, "event: bye\ndata: {}\n\n")
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		return
	}
	if h.deps.Stream == nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	h.deps.Stream.ServeSlip(w, r, slip.ID, slip.WeekID)
}

// profile shows the signed-in person's home book and group members.
func (h *slipHandlers) profile(w http.ResponseWriter, r *http.Request) {
	user, group, ok := h.requireActor(w, r)
	if !ok {
		return
	}
	members, err := h.deps.Store.GroupMembers(r.Context(), group.ID)
	if err != nil {
		h.serverError(w, err)
		return
	}

	page := h.pageWithTab(r, "Profile", "")
	memberViews := make([]ProfileMember, 0, len(members))
	for _, member := range members {
		memberViews = append(memberViews, ProfileMember{
			ID:     member.ID,
			Name:   member.Name,
			Avatar: NewUser(member),
			Active: member.Active,
			Self:   member.ID == user.ID,
		})
	}

	homeBook := ""
	if user.HomeBook != nil {
		homeBook = *user.HomeBook
	}
	h.renderPage(w, "profile", ProfilePage{
		Page:     page,
		HomeBook: homeBook,
		Members:  memberViews,
	})
}

// updateProfile saves the home book.
func (h *slipHandlers) updateProfile(w http.ResponseWriter, r *http.Request) {
	user, _, ok := h.requireActor(w, r)
	if !ok {
		return
	}
	if err := readForm(w, r); err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	homeBook := strings.TrimSpace(r.PostForm.Get("home_book"))
	var book *string
	if homeBook != "" {
		book = &homeBook
	}
	if _, err := h.deps.Store.SetUserHomeBook(r.Context(), user.ID, book); err != nil {
		h.serverError(w, err)
		return
	}
	h.redirect(w, r, "/me")
}

// toggleActive turns a group member on or off. An empty value toggles.
func (h *slipHandlers) toggleActive(w http.ResponseWriter, r *http.Request) {
	user, group, ok := h.requireActor(w, r)
	if !ok {
		return
	}
	targetID, valid := parseID(r.PathValue("id"))
	if !valid {
		http.NotFound(w, r)
		return
	}
	member, err := h.deps.Store.IsMember(r.Context(), group.ID, targetID)
	if err != nil {
		h.serverError(w, err)
		return
	}
	if !member {
		http.NotFound(w, r)
		return
	}
	if err := readForm(w, r); err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	target, err := h.deps.Store.UserByID(r.Context(), targetID)
	if err != nil {
		h.respondError(w, err)
		return
	}
	active, valid := parseActive(r.PostForm.Get("active"), target.Active)
	if !valid {
		h.writeFormError(w, "Active must be on or off.")
		return
	}
	if target.ID == user.ID && !active {
		h.writeFormError(w, "You can't turn yourself off.")
		return
	}
	if _, err := h.deps.Store.SetUserActive(r.Context(), target.ID, active); err != nil {
		h.serverError(w, err)
		return
	}
	h.redirect(w, r, "/me")
}

// renderSlip writes the full Slip tab for slip.
func (h *slipHandlers) renderSlip(
	w http.ResponseWriter,
	r *http.Request,
	page Page,
	slip db.Slip,
	stake, boost float64,
) {
	if week, err := h.deps.Store.WeekByID(r.Context(), slip.WeekID); err == nil {
		page.WeekLabel = weekLabel(week)
	}
	page.Slips = h.slipOptions(r.Context(), slip)
	data, err := h.slipView(r.Context(), slip, stake, boost)
	if err != nil {
		h.serverError(w, err)
		return
	}
	export, err := h.exportView(r.Context(), slip)
	if err != nil {
		h.serverError(w, err)
		return
	}
	data.Page = page
	data.Export = export
	h.renderPage(w, "slip", data)
}

// slipOptions lists the group's slips in the slip's week for the top bar. The
// current slip is marked so the switcher can show it.
func (h *slipHandlers) slipOptions(ctx context.Context, slip db.Slip) []SlipOption {
	groupSlips, err := h.deps.Store.SlipsForWeek(ctx, slip.GroupID, slip.WeekID)
	if err != nil {
		return nil
	}
	options := make([]SlipOption, 0, len(groupSlips))
	for _, other := range groupSlips {
		options = append(options, SlipOption{
			ID:      other.ID,
			Name:    other.Name,
			Current: other.ID == slip.ID,
		})
	}
	return options
}

// siblingSlips returns the sibling slips for the switcher when a group has multiple
// slips in the week. If 1 or 0 slips exist, it returns nil.
func (h *slipHandlers) siblingSlips(
	ctx context.Context,
	slip db.Slip,
	currentMembers []db.SlipMember,
	currentLegs []db.Leg,
	viewerID int64,
) ([]SiblingSlipView, error) {
	groupSlips, err := h.deps.Store.SlipsForWeek(ctx, slip.GroupID, slip.WeekID)
	if err != nil {
		return nil, err
	}
	if len(groupSlips) <= 1 {
		return nil, nil
	}

	views := make([]SiblingSlipView, 0, len(groupSlips))
	for _, other := range groupSlips {
		otherMembers := currentMembers
		otherLegs := currentLegs
		if other.ID != slip.ID {
			m, err := h.deps.Store.SlipMembers(ctx, other.ID)
			if err != nil {
				return nil, err
			}
			l, err := h.deps.Store.LegsForSlip(ctx, other.ID)
			if err != nil {
				return nil, err
			}
			otherMembers = m
			otherLegs = l
		}

		views = append(views, SiblingSlipView{
			ID:      other.ID,
			Name:    other.Name,
			Current: other.ID == slip.ID,
			Summary: siblingSummary(other, otherMembers, otherLegs, viewerID),
		})
	}
	return views, nil
}

// siblingSummary formats a short status for a slip in the sibling switcher
// (e.g., "Locked", "1/2 Picks", "Won").
func siblingSummary(
	slip db.Slip,
	members []db.SlipMember,
	legs []db.Leg,
	viewerID int64,
) string {
	if slip.Status == db.SlipGraded {
		switch slip.Result {
		case parlay.Won:
			return "Won"
		case parlay.Lost:
			return "Lost"
		case parlay.Push, parlay.Void:
			return "Push"
		default:
			return "Graded"
		}
	}
	if slip.Status == db.SlipLocked {
		return "Locked"
	}

	var viewerMember *db.SlipMember
	viewerLegs := 0
	for i := range members {
		if members[i].User.ID == viewerID {
			viewerMember = &members[i]
		}
	}
	for _, leg := range legs {
		if leg.UserID == viewerID {
			viewerLegs++
		}
	}

	if viewerMember != nil {
		if viewerMember.LockedAt != nil {
			return "Locked"
		}
		return fmt.Sprintf("%d/%d Picks", viewerLegs, slip.LegsPerMember)
	}

	if len(members) > 0 && allMembersLocked(members) {
		return "Locked"
	}

	totalNeeded := len(members) * slip.LegsPerMember
	return fmt.Sprintf("%d/%d Picks", len(legs), totalNeeded)
}

// allMembersLocked reports whether all slip members are locked in.
func allMembersLocked(members []db.SlipMember) bool {
	if len(members) == 0 {
		return false
	}
	for _, m := range members {
		if m.LockedAt == nil {
			return false
		}
	}
	return true
}

// renderPanel writes the slip panel partial for slip. The payout calculator is
// its own partial, so refreshing the panel does not reset the stake a person
// typed.
func (h *slipHandlers) renderPanel(w http.ResponseWriter, r *http.Request, slip db.Slip) {
	page, err := h.slipPageData(r.Context(), slip)
	if err != nil {
		h.serverError(w, err)
		return
	}
	h.renderPartial(w, "slip-panel", page)
}

// defaultSlateScopeOptions returns the available slate scopes for creating a slip.
func defaultSlateScopeOptions(selected string) []SlateScopeOption {
	if selected == "" {
		selected = string(db.SlateScopeSundayMain)
	}
	scopes := []db.SlateScope{db.SlateScopeSundayMain, db.SlateScopeThursday, db.SlateScopeFullWeek}
	options := make([]SlateScopeOption, len(scopes))
	for i, scope := range scopes {
		options[i] = SlateScopeOption{
			Value:    string(scope),
			Label:    scope.Label(),
			Selected: selected == string(scope),
		}
	}
	return options
}

// renderCreate shows the create form, filling defaults from the previous slip.
func (h *slipHandlers) renderCreate(
	w http.ResponseWriter,
	r *http.Request,
	page Page,
	user db.User,
	group db.Group,
	week *db.Week,
	form *SlipCreateView,
) {
	create, err := h.createView(r.Context(), user, group, week)
	if err != nil {
		h.serverError(w, err)
		return
	}
	if week != nil {
		page.WeekLabel = weekLabel(*week)
	}
	if form != nil {
		create.WeekID = form.WeekID
		create.Name = form.Name
		if form.SlateScope != "" {
			create.SlateScope = form.SlateScope
		}
		create.LegsPerMember = form.LegsPerMember
		create.Season = form.Season
		create.SeasonType = form.SeasonType
		create.WeekNum = form.WeekNum
		create.Error = form.Error
		if form.Selected != nil {
			chosen := make(map[int64]bool, len(form.Selected))
			for _, id := range form.Selected {
				chosen[id] = true
			}
			for i := range create.Members {
				create.Members[i].Selected = chosen[create.Members[i].ID]
			}
		}
	} else {
		reqScope := r.URL.Query().Get("slate_scope")
		if reqScope == "" {
			reqScope = r.URL.Query().Get("scope")
		}
		if db.SlateScope(reqScope).Valid() {
			create.SlateScope = reqScope
		}
	}
	create.SlateScopes = defaultSlateScopeOptions(create.SlateScope)
	h.renderPage(w, "slip", SlipPage{Page: page, Creating: true, Create: create})
}

// createView builds the create form's weeks, defaults, and member checklist.
func (h *slipHandlers) createView(
	ctx context.Context,
	user db.User,
	group db.Group,
	week *db.Week,
) (SlipCreateView, error) {
	weeks, err := h.deps.Store.Weeks(ctx)
	if err != nil {
		return SlipCreateView{}, err
	}
	options := make([]WeekOption, 0, len(weeks))
	for _, known := range weeks {
		options = append(options, WeekOption{ID: known.ID, Label: weekOptionLabel(known)})
	}

	members, err := h.deps.Store.Members(ctx, group.ID)
	if err != nil {
		return SlipCreateView{}, err
	}
	memberOptions := make([]MemberOption, 0, len(members))
	for _, member := range members {
		memberOptions = append(memberOptions, MemberOption{
			ID:       member.ID,
			Name:     member.Name,
			Selected: member.ID == user.ID,
		})
	}

	view := SlipCreateView{
		Weeks:         options,
		SeasonType:    2,
		SlateScope:    string(db.SlateScopeSundayMain),
		LegsPerMember: slips.DefaultLegsPerMember,
		Members:       memberOptions,
		Solo:          len(memberOptions) == 1,
	}
	if week == nil {
		return view, nil
	}

	view.WeekID = week.ID
	view.Name = weekLabel(*week)
	defaults, err := slips.CreateDefaults(ctx, h.deps.Store, group.ID, week.ID)
	if err != nil {
		return SlipCreateView{}, err
	}
	view.LegsPerMember = defaults.LegsPerMember
	if len(defaults.MemberIDs) > 0 {
		selected := make(map[int64]bool, len(defaults.MemberIDs))
		for _, id := range defaults.MemberIDs {
			selected[id] = true
		}
		for i := range view.Members {
			view.Members[i].Selected = selected[view.Members[i].ID]
		}
	}
	view.Name = h.nextSlipName(ctx, group.ID, week.ID, view.Name)
	return view, nil
}

// nextSlipName appends " #2", " #3" until the name is free in the week.
func (h *slipHandlers) nextSlipName(
	ctx context.Context,
	groupID, weekID int64,
	base string,
) string {
	groupSlips, err := h.deps.Store.SlipsForWeek(ctx, groupID, weekID)
	if err != nil {
		return base
	}
	used := make(map[string]bool, len(groupSlips))
	for _, slip := range groupSlips {
		used[slip.Name] = true
	}
	if !used[base] {
		return base
	}
	for n := 2; ; n++ {
		candidate := fmt.Sprintf("%s #%d", base, n)
		if !used[candidate] {
			return candidate
		}
	}
}

// renderCreateError re-renders the form with a short message and the values
// the person already typed.
func (h *slipHandlers) renderCreateError(
	w http.ResponseWriter,
	r *http.Request,
	user db.User,
	group db.Group,
	message string,
) {
	page := h.page(r, "New slip")
	var week *db.Week
	if id, ok := parseID(r.PostForm.Get("week_id")); ok {
		found, err := h.deps.Store.WeekByID(r.Context(), id)
		if err == nil {
			week = &found
		}
	}
	form := &SlipCreateView{
		WeekID:        weekIDFromForm(r),
		Season:        intFromForm(r, "season"),
		SeasonType:    seasonTypeFromForm(r),
		WeekNum:       intFromForm(r, "week"),
		Name:          r.PostForm.Get("name"),
		SlateScope:    r.PostForm.Get("slate_scope"),
		LegsPerMember: slips.DefaultLegsPerMember,
		Error:         message,
	}
	if form.SlateScope == "" {
		form.SlateScope = r.PostForm.Get("scope")
	}
	if raw := r.PostForm.Get("legs_per_member"); raw != "" {
		if legs, err := strconv.Atoi(raw); err == nil {
			form.LegsPerMember = legs
		}
	}
	for _, raw := range r.PostForm["member_id"] {
		if id, ok := parseID(raw); ok {
			form.Selected = append(form.Selected, id)
		}
	}
	h.renderCreate(w, r, page, user, group, week, form)
}

// slipView loads and shapes one slip for the panel.
func (h *slipHandlers) slipView(
	ctx context.Context,
	slip db.Slip,
	stake, boost float64,
) (SlipPage, error) {
	page, err := h.slipPageData(ctx, slip)
	if err != nil {
		return SlipPage{}, err
	}
	payout, err := h.payoutView(ctx, slip, stake, boost)
	if err != nil {
		return SlipPage{}, err
	}
	page.Slip.Payout = payout
	if user, ok := h.user(ctx); ok {
		wager, err := h.wagerView(ctx, slip, user.ID)
		if err != nil {
			return SlipPage{}, err
		}
		page.Slip.Wager = wager
	}
	return page, nil
}

// slipPageData loads the slip's members, legs, and activity.
func (h *slipHandlers) slipPageData(ctx context.Context, slip db.Slip) (SlipPage, error) {
	user, hasUser := h.user(ctx)
	members, err := h.deps.Store.SlipMembers(ctx, slip.ID)
	if err != nil {
		return SlipPage{}, err
	}
	legs, err := h.deps.Store.LegsForSlip(ctx, slip.ID)
	if err != nil {
		return SlipPage{}, err
	}
	allWeekGames, err := h.deps.Store.GamesForWeek(ctx, slip.WeekID)
	if err != nil {
		return SlipPage{}, err
	}
	games := slips.FilterGamesByScope(allWeekGames, slip.SlateScope)
	entries, err := h.deps.Store.ActivityForSlip(ctx, slip.ID)
	if err != nil {
		return SlipPage{}, err
	}
	votes, err := h.deps.Store.VotesForSlip(ctx, slip.ID)
	if err != nil {
		return SlipPage{}, err
	}

	names := make(map[int64]string, len(members))
	viewerIsMember := false
	for _, person := range members {
		names[person.User.ID] = person.User.Name
		if hasUser && person.User.ID == user.ID {
			viewerIsMember = true
		}
	}

	open := slip.Status == db.SlipOpen
	gamesByID, playerStats := analyzer.LoadSlipSignals(ctx, h.deps.Store, h.deps.now(), slip.WeekID, games, legs)
	analysis, legGrades := analyzerView(legs, gamesByID, playerStats, names)
	media, err := h.legMedia(ctx, legs, allWeekGames)
	if err != nil {
		return SlipPage{}, err
	}
	viewerID := user.ID
	votesByLeg := voteViews(votes, viewerID)
	legsByUser := legViews(
		legs,
		names,
		gamesByID,
		legGrades,
		media,
		open,
		hasUser,
		viewerIsMember,
		viewerID,
		votesByLeg,
	)

	locked := 0
	for _, member := range members {
		if member.LockedAt != nil {
			locked++
		}
	}

	memberViews := make([]SlipMemberView, 0, len(members))
	for _, member := range members {
		own := legsByUser[member.User.ID]
		isCurrent := hasUser && member.User.ID == user.ID
		memberViews = append(memberViews, SlipMemberView{
			User:      NewUser(member.User),
			Locked:    member.LockedAt != nil,
			IsCurrent: isCurrent,
			CanLockIn: open && isCurrent && member.LockedAt == nil && len(own) == slip.LegsPerMember,
			Legs:      own,
			LegCount:  len(own),
		})
	}

	activity := make([]ActivityView, 0, len(entries))
	for _, entry := range entries {
		activity = append(activity, ActivityView{
			Message: entry.Message,
			When:    entry.CreatedAt.Format(time.RFC3339),
		})
	}

	siblings, err := h.siblingSlips(ctx, slip, members, legs, viewerID)
	if err != nil {
		return SlipPage{}, err
	}

	view := &SlipView{
		ID:            slip.ID,
		Name:          slip.Name,
		Status:        string(slip.Status),
		LegsPerMember: slip.LegsPerMember,
		Open:          open,
		LockedIn:      locked,
		MemberCount:   len(members),
		Members:       memberViews,
		Activity:      activity,
		Analyzer:      analysis,
		Siblings:      siblings,
	}

	next := slips.NextStep(slip, members, legs, games, viewerID, h.deps.now())
	return SlipPage{
		Page:     h.page(nil, slip.Name),
		LiveURL:  slipLiveURL(slip),
		Slip:     view,
		Next:     nextView(next, slip, viewerID, games),
		Siblings: siblings,
	}, nil
}

// slipLiveURL returns the SSE endpoint for non-graded slips and empty once
// a slip is settled so clients do not hold open streams on finished weeks.
func slipLiveURL(slip db.Slip) string {
	if slip.Status == db.SlipGraded {
		return ""
	}
	return fmt.Sprintf("/slips/%d/events", slip.ID)
}

// voteViews folds the stored votes into one tally per leg and the viewer's own
// vote. The caller fills CanVote, Has, and the next-tap values per leg.
func voteViews(votes []db.LegVote, viewerID int64) map[int64]VoteView {
	views := make(map[int64]VoteView, len(votes))
	for _, vote := range votes {
		view := views[vote.LegID]
		if vote.Value > 0 {
			view.Up++
		} else if vote.Value < 0 {
			view.Down++
		}
		if viewerID != 0 && vote.UserID == viewerID {
			view.Mine = vote.Value
		}
		views[vote.LegID] = view
	}
	return views
}

// legMediaIndex holds preloaded player portraits, team logos, and week games
// for rendering LegView badges without per-leg queries.
type legMediaIndex struct {
	players   map[string]db.RosterPlayer
	teamLogos map[string]string
	games     map[string]db.Game
}

func (h *slipHandlers) legMedia(ctx context.Context, legs []db.Leg, weekGames []db.Game) (legMediaIndex, error) {
	idx := legMediaIndex{
		teamLogos: make(map[string]string, len(weekGames)*2),
		games:     make(map[string]db.Game, len(weekGames)),
	}
	for _, g := range weekGames {
		idx.games[g.ID] = g
		if logo := db.StringValue(g.AwayLogo); logo != "" {
			idx.teamLogos[g.AwayTeam] = logo
		}
		if logo := db.StringValue(g.HomeLogo); logo != "" {
			idx.teamLogos[g.HomeTeam] = logo
		}
	}

	seenPlayers := make(map[string]struct{})
	var playerIDs []string
	for _, leg := range legs {
		if leg.PlayerID == nil || *leg.PlayerID == "" {
			continue
		}
		if _, ok := seenPlayers[*leg.PlayerID]; ok {
			continue
		}
		seenPlayers[*leg.PlayerID] = struct{}{}
		playerIDs = append(playerIDs, *leg.PlayerID)
	}
	if len(playerIDs) > 0 {
		players, err := h.deps.Store.RosterPlayersByIDs(ctx, playerIDs)
		if err != nil {
			return legMediaIndex{}, err
		}
		idx.players = players
	}

	needsGlobalLogos := false
	for _, leg := range legs {
		if team := db.StringValue(leg.Team); team != "" && idx.teamLogos[team] == "" {
			needsGlobalLogos = true
			break
		}
		if leg.PlayerID != nil && *leg.PlayerID != "" {
			if p, ok := idx.players[*leg.PlayerID]; ok && p.Team != "" && idx.teamLogos[p.Team] == "" {
				needsGlobalLogos = true
				break
			}
		}
	}
	if needsGlobalLogos {
		globalLogos, err := h.deps.Store.TeamLogos(ctx)
		if err != nil {
			return legMediaIndex{}, err
		}
		for team, logo := range globalLogos {
			if idx.teamLogos[team] == "" {
				idx.teamLogos[team] = logo
			}
		}
	}
	return idx, nil
}

// legViews groups a slip's legs by owner for the panel, with each leg's vote
// tally and the viewer's part in it.
func legViews(
	legs []db.Leg,
	names map[int64]string,
	games map[string]analyzer.Game,
	legGrades map[int64]GradeView,
	media legMediaIndex,
	open, hasUser, viewerIsMember bool,
	viewerID int64,
	votes map[int64]VoteView,
) map[int64][]LegView {
	byUser := make(map[int64][]LegView, len(names))
	for _, leg := range legs {
		canVote := open && hasUser && viewerIsMember && leg.UserID != viewerID
		view := LegView{
			ID:          leg.ID,
			Selection:   leg.Selection,
			Odds:        formatOdds(leg.Odds),
			Model:       legModel(leg, games),
			Grade:       legGrades[leg.ID],
			Result:      string(leg.Result),
			GradedBy:    legGradedBy(leg.GradedBy),
			OwnerName:   names[leg.UserID],
			CanEdit:     open && hasUser && leg.UserID == viewerID,
			CanGrade:    viewerIsMember,
			ResearchURL: legResearchURL(leg),
			Votes:       legVoteView(votes, leg, canVote),
		}
		applyLegMedia(&view, leg, media)
		byUser[leg.UserID] = append(byUser[leg.UserID], view)
	}
	return byUser
}

func applyLegMedia(view *LegView, leg db.Leg, media legMediaIndex) {
	if leg.PlayerID != nil && *leg.PlayerID != "" {
		player, ok := media.players[*leg.PlayerID]
		name := db.StringValue(leg.PlayerName)
		if name == "" && ok {
			name = player.FullName
		}
		view.PlayerInitials = Initials(name)
		if ok {
			view.PlayerHeadshot = player.HeadshotURL
		}
		if view.PlayerHeadshot == "" {
			team := db.StringValue(leg.Team)
			if team == "" && ok {
				team = player.Team
			}
			if team != "" {
				view.TeamLogo = media.teamLogos[team]
			}
		}
		return
	}
	if leg.Team != nil && *leg.Team != "" {
		view.TeamLogo = media.teamLogos[*leg.Team]
		return
	}
	if leg.GameID != nil && *leg.GameID != "" {
		if g, ok := media.games[*leg.GameID]; ok {
			view.MatchupAwayLogo = db.StringValue(g.AwayLogo)
			view.MatchupHomeLogo = db.StringValue(g.HomeLogo)
		}
	}
}

// legResearchURL links a leg to the research view for its player or game.
func legResearchURL(leg db.Leg) string {
	if leg.PlayerID != nil && *leg.PlayerID != "" {
		u := url.Values{}
		u.Set("player_id", *leg.PlayerID)
		u.Set("leg_id", strconv.FormatInt(leg.ID, 10))
		if leg.MarketType != "" {
			u.Set("market", leg.MarketType)
		}
		if leg.Line != nil {
			u.Set("line", fmt.Sprintf("%g", *leg.Line))
		}
		return "/research?" + u.Encode()
	}
	if leg.GameID != nil && *leg.GameID != "" {
		u := url.Values{}
		u.Set("game_id", *leg.GameID)
		u.Set("leg_id", strconv.FormatInt(leg.ID, 10))
		return "/research?" + u.Encode()
	}
	return ""
}

// legVoteView returns one leg's tally, decides whether the viewer can vote on
// it, and sets what the next tap sends. Tapping the highlighted vote clears it.
func legVoteView(votes map[int64]VoteView, leg db.Leg, canVote bool) VoteView {
	view := votes[leg.ID]
	view.CanVote = canVote
	view.Has = canVote || view.Up+view.Down > 0
	view.UpValue = 1
	if view.Mine == 1 {
		view.UpValue = 0
	}
	view.DownValue = -1
	if view.Mine == -1 {
		view.DownValue = 0
	}
	return view
}

// nextView turns the pure step into the hub banner's copy and one action.
func nextView(next slips.Next, slip db.Slip, viewerID int64, games []db.Game) NextView {
	view := NextView{
		Step:  next.Step,
		Title: next.Label(),
	}
	switch next.Step {
	case slips.StepPick:
		view.Detail = "Fill your part of the slip, then lock in."
		view.Action = &NextAction{
			Label: "Pick your legs",
			Href:  fmt.Sprintf("/slips/%d/pick", slip.ID),
		}
	case slips.StepLockIn:
		view.Detail = "All your legs are in."
		view.Action = &NextAction{
			Label:   "Lock in",
			PostURL: fmt.Sprintf("/slips/%d/lock", slip.ID),
		}
	case slips.StepWaiting:
		view.Detail = "You're locked in. The slip waits on the rest."
		for _, name := range next.WaitingOn {
			nudge := NextNudge{
				Name:     name,
				SlipName: slip.Name,
				Path:     fmt.Sprintf("/slips/%d", slip.ID),
			}
			if next.Deadline != nil {
				nudge.CloseISO = next.Deadline.UTC().Format(time.RFC3339)
			}
			view.Nudges = append(view.Nudges, nudge)
		}
	case slips.StepExport:
		view.Detail = "Everyone is locked in."
	case slips.StepDone:
		view.Detail = "This slip is graded."
	case slips.StepClosed:
		if len(next.WaitingOn) > 0 {
			view.Detail = slips.NameList(next.WaitingOn) +
				" never finished picking, so this slip can't lock in this week."
		} else {
			view.Detail = "Every game this week has kicked off."
		}
	case slips.StepJoin:
		view.Detail = "You're in this group, but not on this slip yet."
		view.Action = &NextAction{
			Label:   "Join this slip",
			PostURL: fmt.Sprintf("/slips/%d/members", slip.ID),
			UserID:  viewerID,
		}
	}
	switch next.Step {
	case slips.StepPick, slips.StepLockIn, slips.StepWaiting, slips.StepJoin:
		if next.Deadline != nil {
			view.Deadline = newDeadlineView(games, *next.Deadline)
		}
	}
	return view
}

// newDeadlineView names the game whose kickoff closes the picks.
func newDeadlineView(games []db.Game, at time.Time) *DeadlineView {
	matchup := ""
	for _, game := range games {
		if game.Kickoff.Equal(at) {
			matchup = game.AwayTeam + " @ " + game.HomeTeam
			break
		}
	}
	return &DeadlineView{
		Matchup: matchup,
		ISO:     at.UTC().Format(time.RFC3339),
		Label:   at.UTC().Format("Mon 3:04 PM") + " UTC",
	}
}

// payoutView computes the combined odds and return for the calculator.
func (h *slipHandlers) payoutView(
	ctx context.Context,
	slip db.Slip,
	stake, boost float64,
) (PayoutView, error) {
	legs, err := h.deps.Store.LegsForSlip(ctx, slip.ID)
	if err != nil {
		return PayoutView{}, err
	}
	combinedLegs := make([]parlay.Leg, 0, len(legs))
	for _, leg := range legs {
		combinedLegs = append(combinedLegs, parlay.Leg{
			Decimal: parlay.AmericanToDecimal(leg.Odds),
			Result:  leg.Result,
		})
	}
	combined := parlay.CombinedOdds(combinedLegs)
	reference, err := h.referenceOdds(ctx, slip, legs)
	if err != nil {
		return PayoutView{}, err
	}
	total := parlay.Payout(stake, combined, boost)
	return PayoutView{
		SlipID:            slip.ID,
		HasLegs:           len(legs) > 0,
		Stake:             formatFloat(stake),
		Boost:             formatFloat(boost),
		Combined:          fmt.Sprintf("%.2fx", combined),
		CombinedAmerican:  formatOdds(parlay.DecimalToAmerican(combined)),
		Reference:         fmt.Sprintf("%.2fx", reference),
		ReferenceAmerican: formatOdds(parlay.DecimalToAmerican(reference)),
		Return:            money(total),
		Profit:            money(total - stake),
	}, nil
}

// wagerView builds the "My wager" form for userID on slip, pre-filling any
// saved wager or defaulting odds to the slip's current combined odds.
func (h *slipHandlers) wagerView(ctx context.Context, slip db.Slip, userID int64) (WagerView, error) {
	wager, found, err := h.deps.Store.WagerForUser(ctx, slip.ID, userID)
	if err != nil {
		return WagerView{}, err
	}
	if found {
		book := ""
		if wager.Book != nil {
			book = *wager.Book
		}
		total := parlay.Payout(wager.Stake, parlay.AmericanToDecimal(wager.Odds), wager.BoostPct)
		return WagerView{
			SlipID:   slip.ID,
			HasWager: true,
			Book:     book,
			Stake:    formatFloat(wager.Stake),
			Odds:     strconv.Itoa(wager.Odds),
			Boost:    formatFloat(wager.BoostPct),
			Return:   money(total),
			Profit:   money(total - wager.Stake),
		}, nil
	}

	legs, err := h.deps.Store.LegsForSlip(ctx, slip.ID)
	if err != nil {
		return WagerView{}, err
	}
	combinedLegs := make([]parlay.Leg, 0, len(legs))
	for _, leg := range legs {
		combinedLegs = append(combinedLegs, parlay.Leg{
			Decimal: parlay.AmericanToDecimal(leg.Odds),
			Result:  leg.Result,
		})
	}
	oddsStr := ""
	if american := parlay.DecimalToAmerican(parlay.CombinedOdds(combinedLegs)); american != 0 {
		oddsStr = strconv.Itoa(american)
	}
	return WagerView{
		SlipID: slip.ID,
		Odds:   oddsStr,
		Boost:  "0",
	}, nil
}

// referenceOdds is the product of each counting leg's reference decimal. A
// leg's reference price comes from the prices its slip members' home books
// carry for the same game, market, player, side, and line, and falls back to
// the leg's own price when none of them do.
func (h *slipHandlers) referenceOdds(
	ctx context.Context,
	slip db.Slip,
	legs []db.Leg,
) (float64, error) {
	members, err := h.deps.Store.SlipMembers(ctx, slip.ID)
	if err != nil {
		return 0, err
	}
	books := memberBooks(members)

	byGame := make(map[string][]db.Odds, len(legs))
	referenceLegs := make([]parlay.Leg, 0, len(legs))
	for _, leg := range legs {
		own := parlay.AmericanToDecimal(leg.Odds)
		var candidates []float64
		if leg.GameID != nil && len(books) > 0 {
			rows, seen := byGame[*leg.GameID]
			if !seen {
				rows, err = h.deps.Store.OddsForGame(ctx, *leg.GameID)
				if err != nil {
					return 0, err
				}
				byGame[*leg.GameID] = rows
			}
			candidates = candidatePrices(rows, leg, books)
		}
		referenceLegs = append(referenceLegs, parlay.Leg{
			Decimal: parlay.ReferencePrice(candidates, own),
			Result:  leg.Result,
		})
	}
	return parlay.CombinedOdds(referenceLegs), nil
}

// memberBooks is the set of home books on a slip, keyed the way the odds rows
// spell a book. A book shared by two members counts once.
func memberBooks(members []db.SlipMember) map[string]bool {
	books := make(map[string]bool, len(members))
	for _, member := range members {
		if member.User.HomeBook == nil {
			continue
		}
		book := db.NormalizeBook(*member.User.HomeBook)
		if book != "" {
			books[book] = true
		}
	}
	return books
}

// candidatePrices collects one decimal price per member home book for the
// selection a leg picked, preferring an exact line match and falling back to a
// moved line for that book. Rows from an older sync are already dropped.
func candidatePrices(rows []db.Odds, leg db.Leg, books map[string]bool) []float64 {
	prices := make([]float64, 0, len(books))
	for book := range books {
		if row, ok := db.MatchBookOdds(leg, rows, book); ok {
			prices = append(prices, parlay.AmericanToDecimal(row.Price))
		}
	}
	return prices
}

// legFormView maps a stored leg to the edit form.
func legFormView(leg db.Leg) LegFormView {
	view := LegFormView{
		SlipID:    leg.SlipID,
		LegID:     leg.ID,
		Action:    fmt.Sprintf("/legs/%d", leg.ID),
		Target:    "#slip-panel",
		Editing:   true,
		Market:    leg.MarketType,
		Selection: leg.Selection,
		Odds:      strconv.Itoa(leg.Odds),
	}
	if leg.Team != nil {
		view.Team = *leg.Team
	}
	if leg.PlayerName != nil {
		view.PlayerName = *leg.PlayerName
	}
	if leg.Side != nil {
		view.Side = *leg.Side
	}
	if leg.GameID != nil {
		view.GameID = *leg.GameID
	}
	if leg.Line != nil {
		view.Line = formatFloat(*leg.Line)
	}
	return view
}

// legFromForm reads the leg form or writes a 422 and returns false.
func (h *slipHandlers) legFromForm(w http.ResponseWriter, r *http.Request, slipID int64) (db.Leg, bool) {
	if err := readForm(w, r); err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return db.Leg{}, false
	}

	market := strings.ToUpper(strings.TrimSpace(r.PostForm.Get("market_type")))
	if market == "" {
		market = "OTHER"
	}
	odds := 0
	if raw := strings.TrimSpace(r.PostForm.Get("odds")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			h.writeFormError(w, "Odds need to be a number.")
			return db.Leg{}, false
		}
		odds = parsed
	}
	line, ok := optionalFloat(w, r.PostForm.Get("line"))
	if !ok {
		return db.Leg{}, false
	}

	selection := strings.TrimSpace(r.PostForm.Get("selection"))
	if selection == "" {
		selection = db.BuildLegSelection(
			market,
			r.PostForm.Get("team"),
			r.PostForm.Get("player_name"),
			r.PostForm.Get("side"),
			line,
		)
	}
	if selection == "" {
		h.writeFormError(w, "Add a selection.")
		return db.Leg{}, false
	}

	leg := db.Leg{
		SlipID:     slipID,
		MarketType: market,
		Selection:  selection,
		Odds:       odds,
		Team:       db.OptionalString(r.PostForm.Get("team")),
		PlayerName: db.OptionalString(r.PostForm.Get("player_name")),
		PlayerID:   db.OptionalString(r.PostForm.Get("player_id")),
		Side:       db.OptionalString(strings.ToUpper(strings.TrimSpace(r.PostForm.Get("side")))),
		Line:       line,
		GameID:     db.OptionalString(r.PostForm.Get("game_id")),
	}
	if market == "ANYTIME_TD" && leg.Side == nil {
		side := "YES"
		leg.Side = &side
	}

	hasPlayer := leg.PlayerName != nil && *leg.PlayerName != ""
	hasGame := leg.GameID != nil && *leg.GameID != ""
	if (isPlayerMarket(market) || hasPlayer) && hasGame && hasPlayer && h.deps.Store != nil {
		if game, err := h.deps.Store.GameByID(r.Context(), *leg.GameID); err == nil {
			athleteID, team, err := h.deps.Store.ResolvePlayerMatch(
				r.Context(),
				*leg.PlayerName,
				game.HomeTeam,
				game.AwayTeam,
			)
			if err == nil {
				if athleteID != "" && (leg.PlayerID == nil || *leg.PlayerID == "") {
					leg.PlayerID = &athleteID
				}
				if team != "" && (leg.Team == nil || *leg.Team == "") {
					leg.Team = &team
				}
			}
		}
	}
	return leg, true
}

// isPlayerMarket reports whether a market type targets an athlete rather than a game or team.
func isPlayerMarket(market string) bool {
	return db.IsPlayerMarket(market)
}

// loadSlip parses the path id, then resolves the slip's group. A slip in
// another of the viewer's groups makes that group active; a slip outside every
// group they belong to answers 404, the same as a missing slip.
func (h *slipHandlers) loadSlip(
	w http.ResponseWriter,
	r *http.Request,
	rawID string,
) (db.Slip, db.User, db.Group, bool) {
	if h.deps.Store == nil {
		http.NotFound(w, r)
		return db.Slip{}, db.User{}, db.Group{}, false
	}
	user, group, ok := h.requireActor(w, r)
	if !ok {
		return db.Slip{}, db.User{}, db.Group{}, false
	}
	id, valid := parseID(rawID)
	if !valid {
		http.NotFound(w, r)
		return db.Slip{}, db.User{}, db.Group{}, false
	}

	slip, err := h.deps.Store.SlipByID(r.Context(), id)
	if errors.Is(err, db.ErrSlipNotFound) {
		http.NotFound(w, r)
		return db.Slip{}, db.User{}, db.Group{}, false
	}
	if err != nil {
		h.serverError(w, err)
		return db.Slip{}, db.User{}, db.Group{}, false
	}
	if slip.GroupID != group.ID {
		member, err := h.deps.Store.IsMember(r.Context(), slip.GroupID, user.ID)
		if err != nil {
			h.serverError(w, err)
			return db.Slip{}, db.User{}, db.Group{}, false
		}
		if !member {
			http.NotFound(w, r)
			return db.Slip{}, db.User{}, db.Group{}, false
		}
		group, err = h.deps.Store.GroupByID(r.Context(), slip.GroupID)
		if errors.Is(err, db.ErrGroupNotFound) {
			http.NotFound(w, r)
			return db.Slip{}, db.User{}, db.Group{}, false
		}
		if err != nil {
			h.serverError(w, err)
			return db.Slip{}, db.User{}, db.Group{}, false
		}
		h.setGroup(w, slip.GroupID)
	}
	return slip, user, group, true
}

// loadLeg parses the path id, then guards the leg's slip by active group.
func (h *slipHandlers) loadLeg(w http.ResponseWriter, r *http.Request, rawID string) (db.Leg, db.User, bool) {
	if h.deps.Store == nil {
		http.NotFound(w, r)
		return db.Leg{}, db.User{}, false
	}
	id, valid := parseID(rawID)
	if !valid {
		http.NotFound(w, r)
		return db.Leg{}, db.User{}, false
	}
	leg, err := h.deps.Store.LegByID(r.Context(), id)
	if errors.Is(err, db.ErrLegNotFound) {
		http.NotFound(w, r)
		return db.Leg{}, db.User{}, false
	}
	if err != nil {
		h.serverError(w, err)
		return db.Leg{}, db.User{}, false
	}
	slip, user, _, ok := h.loadSlip(w, r, strconv.FormatInt(leg.SlipID, 10))
	if !ok {
		return db.Leg{}, db.User{}, false
	}
	if slip.ID != leg.SlipID {
		http.NotFound(w, r)
		return db.Leg{}, db.User{}, false
	}
	return leg, user, true
}

// requireActor returns the signed-in person and active group.
func (h *slipHandlers) requireActor(w http.ResponseWriter, r *http.Request) (db.User, db.Group, bool) {
	if h.deps.Store == nil || h.deps.Slips == nil {
		http.NotFound(w, r)
		return db.User{}, db.Group{}, false
	}
	user, ok := h.user(r.Context())
	if !ok {
		h.redirect(w, r, "/login")
		return db.User{}, db.Group{}, false
	}
	group, ok := h.group(r.Context())
	if !ok {
		http.NotFound(w, r)
		return db.User{}, db.Group{}, false
	}
	return user, group, true
}

// user returns the signed-in person.
func (h *slipHandlers) user(ctx context.Context) (db.User, bool) {
	if h.deps.UserFrom == nil {
		return db.User{}, false
	}
	return h.deps.UserFrom(ctx)
}

// group returns the active group.
func (h *slipHandlers) group(ctx context.Context) (db.Group, bool) {
	if h.deps.GroupFrom == nil {
		return db.Group{}, false
	}
	return h.deps.GroupFrom(ctx)
}

// page builds the shell's data for a This week screen.
func (h *slipHandlers) page(r *http.Request, title string) Page {
	return h.pageWithTab(r, title, TabWeek)
}

// pageWithTab builds the shell's data for a page with an explicit tab. An empty
// tab leaves the bottom bar off.
func (h *slipHandlers) pageWithTab(r *http.Request, title string, tab Tab) Page {
	page := Page{Title: title, Tab: tab}
	if r == nil {
		return page
	}
	if user, ok := h.user(r.Context()); ok {
		view := NewUser(user)
		page.User = &view
	}
	if h.deps.GroupsFrom != nil {
		page.Groups = h.deps.GroupsFrom(r.Context())
	}
	return page
}

// pageForGroup marks groupID as the active group in the top bar. Opening a
// slip or week from another of the viewer's groups switches the group, so the
// shell must show the one the page acts in.
func (h *slipHandlers) pageForGroup(r *http.Request, title string, groupID int64) Page {
	page := h.page(r, title)
	for i := range page.Groups {
		page.Groups[i].Current = page.Groups[i].ID == groupID
	}
	return page
}

// renderPage writes a full page.
func (h *slipHandlers) renderPage(w http.ResponseWriter, name string, data any) {
	h.renderPageCtx(context.Background(), w, name, data)
}

// renderPageCtx writes a full page and records render phase timing on ctx.
func (h *slipHandlers) renderPageCtx(ctx context.Context, w http.ResponseWriter, name string, data any) {
	start := time.Now()
	html, err := h.deps.Renderer.Render(name, data)
	AddRenderTiming(ctx, time.Since(start))
	if err != nil {
		h.deps.Logger.Error().Err(err).Str("page", name).Msg("render page")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(html)
}

// renderPartial writes one HTMX fragment.
func (h *slipHandlers) renderPartial(w http.ResponseWriter, name string, data any) {
	h.renderPartialCtx(context.Background(), w, name, data)
}

// renderPartialCtx writes one HTMX fragment and records render phase timing on ctx.
func (h *slipHandlers) renderPartialCtx(ctx context.Context, w http.ResponseWriter, name string, data any) {
	start := time.Now()
	html, err := h.deps.Renderer.RenderPartial(name, data)
	AddRenderTiming(ctx, time.Since(start))
	if err != nil {
		h.deps.Logger.Error().Err(err).Str("partial", name).Msg("render partial")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(html)
}

// respondError maps a service refusal to 422 and a missing row to 404.
func (h *slipHandlers) respondError(w http.ResponseWriter, err error) {
	if message, ok := fixableMessage(err); ok {
		http.Error(w, message, http.StatusUnprocessableEntity)
		return
	}
	switch {
	case errors.Is(err, db.ErrSlipNotFound),
		errors.Is(err, db.ErrLegNotFound),
		errors.Is(err, db.ErrSlipMemberNotFound),
		errors.Is(err, db.ErrUserNotFound),
		errors.Is(err, db.ErrScheduleNotFound):
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
	default:
		h.serverError(w, err)
	}
}

// fixableMessage returns the short message a 422 shows.
func fixableMessage(err error) (string, bool) {
	for _, target := range ruleErrors {
		if errors.Is(err, target) {
			return target.Error(), true
		}
	}
	return "", false
}

// serverError logs err and answers 500.
func (h *slipHandlers) serverError(w http.ResponseWriter, err error) {
	h.deps.Logger.Error().Err(err).Msg("slip request")
	http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}

// writeFormError answers 422 with a message.
func (h *slipHandlers) writeFormError(w http.ResponseWriter, message string) {
	http.Error(w, message, http.StatusUnprocessableEntity)
}

// redirect sends the browser to target, or HTMX if the request came from it.
func (h *slipHandlers) redirect(w http.ResponseWriter, r *http.Request, target string) {
	if strings.EqualFold(r.Header.Get("Hx-Request"), "true") {
		w.Header().Set("Hx-Redirect", target)
		w.WriteHeader(http.StatusOK)
		return
	}
	//nolint:gosec // target is an internal app path built by our handlers.
	http.Redirect(w, r, target, http.StatusSeeOther)
}

// readForm parses a small submitted form.
func readForm(w http.ResponseWriter, r *http.Request) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxFormBytes)
	return r.ParseForm()
}

// parseID reads a positive integer.
func parseID(raw string) (int64, bool) {
	id, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}

// optionalFloat parses a blank-able line.
func optionalFloat(w http.ResponseWriter, raw string) (*float64, bool) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil, true
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		http.Error(w, "Line needs to be a number.", http.StatusUnprocessableEntity)
		return nil, false
	}
	return &parsed, true
}

// parseFloatDefault parses a query value, falling back to fallback.
func parseFloatDefault(raw string, fallback float64) float64 {
	value := strings.TrimSpace(raw)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || parsed < 0 {
		return fallback
	}
	return parsed
}

// parseActive reads a toggle form value. An empty value flips current.
func parseActive(raw string, current bool) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "":
		return !current, true
	case "1", "true", "on", "yes":
		return true, true
	case "0", "false", "off", "no":
		return false, true
	default:
		return false, false
	}
}

// parseSeasonType reads a create-form season type. ESPN uses 2 for the regular
// season and 3 for the postseason.
func parseSeasonType(raw string) (int, bool) {
	switch strings.TrimSpace(raw) {
	case "", "2":
		return 2, true
	case "3":
		return 3, true
	default:
		return 0, false
	}
}

// seasonTypeFromForm reads the create form's season type, defaulting to regular.
func seasonTypeFromForm(r *http.Request) int {
	seasonType, ok := parseSeasonType(r.PostForm.Get("season_type"))
	if !ok {
		return 2
	}
	return seasonType
}

// intFromForm reads a whole number from the submitted form, or 0.
func intFromForm(r *http.Request, name string) int {
	value, err := strconv.Atoi(strings.TrimSpace(r.PostForm.Get(name)))
	if err != nil {
		return 0
	}
	return value
}

// weekIDFromForm reads the create form's week.
func weekIDFromForm(r *http.Request) int64 {
	id, _ := parseID(r.PostForm.Get("week_id"))
	return id
}

// weekLabel is the display name and default slip name for a week.
func weekLabel(week db.Week) string {
	return fmt.Sprintf("Week %d", week.WeekNum)
}

// weekOptionLabel tells weeks from different seasons apart in the create form.
func weekOptionLabel(week db.Week) string {
	if week.SeasonType == 3 {
		return fmt.Sprintf("%d Postseason %s", week.Season, weekLabel(week))
	}
	return fmt.Sprintf("%d %s", week.Season, weekLabel(week))
}

// titleSide capitalizes a side for display.
func titleSide(raw string) string {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case "OVER":
		return "Over"
	case "UNDER":
		return "Under"
	case "YES":
		return "Yes"
	default:
		return ""
	}
}

// signedLine formats a spread line with its sign.
func signedLine(line float64) string {
	if line > 0 {
		return "+" + formatFloat(line)
	}
	return formatFloat(line)
}

// formatFloat prints a float without a trailing zero tail.
func formatFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

// formatOdds prints an American price with a plus sign when positive.
func formatOdds(odds int) string {
	if odds > 0 {
		return fmt.Sprintf("+%d", odds)
	}
	return strconv.Itoa(odds)
}

// money prints a dollar amount with two decimals.
func money(value float64) string {
	return fmt.Sprintf("$%.2f", value)
}

// legGradedBy formats a leg's grader attribution for display, turning the
// internal "auto" marker into "Auto".
func legGradedBy(gradedBy *string) string {
	name := db.StringValue(gradedBy)
	if name == slips.AutoGraderName {
		return "Auto"
	}
	return name
}

// resultLabel formats a leg's result chip text, showing "Needs grading" for
// NEEDS_GRADING and appending the grader attribution when present.
func resultLabel(result, gradedBy string) string {
	if result == string(parlay.NeedsGrading) {
		return "Needs grading"
	}
	if gradedBy != "" {
		return result + " · " + gradedBy
	}
	return result
}

// resultClass colors a leg's result chip: green for a win, rose for a loss,
// amber when manual grading is needed, muted for a push or void, and slate
// while it waits.
func resultClass(result string) string {
	switch result {
	case "WON":
		return "border-emerald-800 bg-emerald-950 text-emerald-300"
	case "LOST":
		return "border-rose-800 bg-rose-950 text-rose-300"
	case "NEEDS_GRADING":
		return "border-amber-700 bg-amber-950 text-amber-300"
	case "PUSH", "VOID":
		return "border-slate-700 bg-slate-800 text-slate-300"
	default:
		return "border-slate-700 bg-slate-800 text-slate-400"
	}
}
