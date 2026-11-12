package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/slips"
)

// scheduleForMember loads a schedule from r.PathValue("scheduleID"), verifies that
// r.PathValue("id") matches sched.GroupID, and verifies that the current user is a member of the group.
func (h *slipHandlers) scheduleForMember(w http.ResponseWriter, r *http.Request) (db.GroupSlipSchedule, bool) {
	user, ok := h.user(r.Context())
	if !ok {
		h.redirect(w, r, "/login")
		return db.GroupSlipSchedule{}, false
	}
	if h.deps.Store == nil {
		http.NotFound(w, r)
		return db.GroupSlipSchedule{}, false
	}

	groupID, valid := parseID(r.PathValue("id"))
	if !valid {
		http.NotFound(w, r)
		return db.GroupSlipSchedule{}, false
	}
	scheduleID, valid := parseID(r.PathValue("scheduleID"))
	if !valid {
		http.NotFound(w, r)
		return db.GroupSlipSchedule{}, false
	}

	sched, err := h.deps.Store.GroupSlipScheduleByID(r.Context(), scheduleID)
	if errors.Is(err, db.ErrScheduleNotFound) {
		http.NotFound(w, r)
		return db.GroupSlipSchedule{}, false
	}
	if err != nil {
		h.serverError(w, err)
		return db.GroupSlipSchedule{}, false
	}
	if sched.GroupID != groupID {
		http.NotFound(w, r)
		return db.GroupSlipSchedule{}, false
	}

	member, err := h.deps.Store.IsMember(r.Context(), groupID, user.ID)
	if err != nil {
		h.serverError(w, err)
		return db.GroupSlipSchedule{}, false
	}
	if !member {
		http.NotFound(w, r)
		return db.GroupSlipSchedule{}, false
	}

	return sched, true
}

type scheduleInput struct {
	Name          string
	SlateScope    db.SlateScope
	LegsPerMember int
}

func decodeScheduleInput(w http.ResponseWriter, r *http.Request) (scheduleInput, bool) {
	var input scheduleInput
	if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		var payload struct {
			Name          string `json:"name"`
			SlateScope    string `json:"slate_scope"`
			LegsPerMember int    `json:"legs_per_member"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxFormBytes)).
			Decode(&payload); err != nil &&
			!errors.Is(err, io.EOF) {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return scheduleInput{}, false
		}
		input.Name = payload.Name
		input.SlateScope = db.SlateScope(payload.SlateScope)
		input.LegsPerMember = payload.LegsPerMember
		return input, true
	}

	if err := readForm(w, r); err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return scheduleInput{}, false
	}
	input.Name = r.PostForm.Get("name")
	input.SlateScope = db.SlateScope(r.PostForm.Get("slate_scope"))
	if rawLegs := r.PostForm.Get("legs_per_member"); rawLegs != "" {
		parsed, err := strconv.Atoi(strings.TrimSpace(rawLegs))
		if err != nil {
			http.Error(w, "Legs per member must be a number.", http.StatusUnprocessableEntity)
			return scheduleInput{}, false
		}
		input.LegsPerMember = parsed
	}
	return input, true
}

// createSchedule adds a recurring slip schedule for a group.
func (h *slipHandlers) createSchedule(w http.ResponseWriter, r *http.Request) {
	user, ok := h.user(r.Context())
	if !ok {
		h.redirect(w, r, "/login")
		return
	}
	if h.deps.Store == nil {
		http.NotFound(w, r)
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

	input, ok := decodeScheduleInput(w, r)
	if !ok {
		return
	}
	if input.LegsPerMember == 0 {
		input.LegsPerMember = slips.DefaultLegsPerMember
	}

	sched, err := h.deps.Store.CreateGroupSlipSchedule(
		r.Context(),
		groupID,
		input.Name,
		input.SlateScope,
		input.LegsPerMember,
	)
	if err != nil {
		h.respondError(w, err)
		return
	}

	if strings.Contains(r.Header.Get("Accept"), "application/json") {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(sched)
		return
	}
	if isHTMX(r) {
		w.Header().Set("Hx-Refresh", "true")
		w.WriteHeader(http.StatusOK)
		return
	}
	h.redirect(w, r, fmt.Sprintf("/groups/%d/week", groupID))
}

// setScheduleActive turns a recurring schedule on or off.
func (h *slipHandlers) setScheduleActive(w http.ResponseWriter, r *http.Request) {
	sched, ok := h.scheduleForMember(w, r)
	if !ok {
		return
	}

	var (
		active      bool
		validActive bool
	)
	if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		var payload struct {
			Active *bool `json:"active"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxFormBytes)).
			Decode(&payload); err != nil &&
			!errors.Is(err, io.EOF) {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}
		if payload.Active != nil {
			active = *payload.Active
		} else {
			active = !sched.IsActive
		}
		validActive = true
	} else {
		if err := readForm(w, r); err != nil {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}
		active, validActive = parseActive(r.PostForm.Get("active"), sched.IsActive)
	}

	if !validActive {
		h.writeFormError(w, "Active must be on or off.")
		return
	}

	updated, err := h.deps.Store.SetGroupSlipScheduleActive(r.Context(), sched.ID, active)
	if err != nil {
		h.respondError(w, err)
		return
	}

	if strings.Contains(r.Header.Get("Accept"), "application/json") {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(updated)
		return
	}
	if isHTMX(r) {
		w.Header().Set("Hx-Refresh", "true")
		w.WriteHeader(http.StatusOK)
		return
	}
	h.redirect(w, r, fmt.Sprintf("/groups/%d/week", sched.GroupID))
}

// listSchedules returns the recurring schedules for a group.
func (h *slipHandlers) listSchedules(w http.ResponseWriter, r *http.Request) {
	user, ok := h.user(r.Context())
	if !ok {
		h.redirect(w, r, "/login")
		return
	}
	if h.deps.Store == nil {
		http.NotFound(w, r)
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

	schedules, err := h.deps.Store.GroupSlipSchedules(r.Context(), groupID)
	if err != nil {
		h.serverError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(schedules)
}

// updateSchedule changes a schedule's name, slate scope, or legs per member.
func (h *slipHandlers) updateSchedule(w http.ResponseWriter, r *http.Request) {
	sched, ok := h.scheduleForMember(w, r)
	if !ok {
		return
	}

	input, ok := decodeScheduleInput(w, r)
	if !ok {
		return
	}

	name := input.Name
	if name == "" {
		name = sched.Name
	}
	slateScope := input.SlateScope
	if slateScope == "" {
		slateScope = sched.SlateScope
	}
	legsPerMember := input.LegsPerMember
	if legsPerMember == 0 {
		legsPerMember = sched.LegsPerMember
	}

	updated, err := h.deps.Store.UpdateGroupSlipSchedule(r.Context(), sched.ID, name, slateScope, legsPerMember)
	if err != nil {
		h.respondError(w, err)
		return
	}

	if strings.Contains(r.Header.Get("Accept"), "application/json") {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(updated)
		return
	}
	if isHTMX(r) {
		w.Header().Set("Hx-Refresh", "true")
		w.WriteHeader(http.StatusOK)
		return
	}
	h.redirect(w, r, fmt.Sprintf("/groups/%d/week", sched.GroupID))
}

// deleteSchedule removes a recurring slip schedule.
func (h *slipHandlers) deleteSchedule(w http.ResponseWriter, r *http.Request) {
	sched, ok := h.scheduleForMember(w, r)
	if !ok {
		return
	}

	if err := h.deps.Store.DeleteGroupSlipSchedule(r.Context(), sched.ID); err != nil {
		h.respondError(w, err)
		return
	}

	if strings.Contains(r.Header.Get("Accept"), "application/json") {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if isHTMX(r) {
		w.Header().Set("Hx-Refresh", "true")
		w.WriteHeader(http.StatusOK)
		return
	}
	h.redirect(w, r, fmt.Sprintf("/groups/%d/week", sched.GroupID))
}
