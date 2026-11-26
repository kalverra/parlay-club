package web

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/push"
)

// maxPushBodyBytes caps a subscription post. A browser endpoint is a couple
// hundred characters, so this leaves room for the keys and nothing else.
const maxPushBodyBytes = 8 << 10

// pushHandlers serves the browser's push subscription. The routes sit behind
// RequireUser and CSRF like every other mutation, and they answer 404 when the
// server has no VAPID key pair.
type pushHandlers struct {
	deps Dependencies
}

// Routes mounts the push routes.
func (h *pushHandlers) Routes(mux *http.ServeMux) {
	mux.HandleFunc("POST /push/subscription", h.subscribe)
	mux.HandleFunc("DELETE /push/subscription", h.unsubscribe)
	mux.HandleFunc("POST /push/test", h.sendTest)
}

// subscribe stores the browser's subscription for the signed-in person. An
// endpoint that already exists is reassigned to them, which keeps a shared
// phone on its last subscriber.
func (h *pushHandlers) subscribe(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireUser(w, r)
	if !ok {
		return
	}
	var sub push.Subscription
	if !decodePushBody(w, r, &sub) {
		return
	}
	if err := h.deps.Push.Subscribe(r.Context(), user.ID, sub); err != nil {
		h.respondPushError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// unsubscribe removes the browser's endpoint, but only when it belongs to the
// caller. Turning notifications off twice is harmless.
func (h *pushHandlers) unsubscribe(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireUser(w, r)
	if !ok {
		return
	}
	var body struct {
		Endpoint string `json:"endpoint"`
	}
	if !decodePushBody(w, r, &body) {
		return
	}
	if err := h.deps.Push.Unsubscribe(r.Context(), user.ID, body.Endpoint); err != nil {
		h.respondPushError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// sendTest pushes a short confirmation to the caller's own devices, so someone
// can check that the switch works without waiting for a slip event.
func (h *pushHandlers) sendTest(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireUser(w, r)
	if !ok {
		return
	}
	h.deps.Push.SendToUsers(r.Context(), []int64{user.ID}, push.Message{
		Title: "Sunday Parlay Club",
		Body:  "Notifications are on for this device.",
		URL:   "/me",
		Tag:   "push-test",
	})
	w.WriteHeader(http.StatusNoContent)
}

// requireUser returns the signed-in person, or answers 404 when push is off.
// A missing user is an expired session, which goes to sign-in like the rest of
// the app.
func (h *pushHandlers) requireUser(w http.ResponseWriter, r *http.Request) (db.User, bool) {
	if !h.deps.Push.Enabled() {
		http.NotFound(w, r)
		return db.User{}, false
	}
	if h.deps.UserFrom == nil {
		h.redirect(w, r, "/login")
		return db.User{}, false
	}
	user, ok := h.deps.UserFrom(r.Context())
	if !ok {
		h.redirect(w, r, "/login")
		return db.User{}, false
	}
	return user, true
}

// respondPushError maps a refusal to the status the browser expects. An
// invalid subscription is the person's to fix, so it gets 400 with the reason.
func (h *pushHandlers) respondPushError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, push.ErrDisabled):
		http.NotFound(w, r)
	case errors.Is(err, push.ErrInvalidSubscription):
		http.Error(w, err.Error(), http.StatusBadRequest)
	default:
		h.deps.Logger.Error().Err(err).Msg("push request")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}

// redirect sends the browser to target, or answers an HTMX request with
// HX-Redirect so it navigates instead of swapping an empty body.
func (h *pushHandlers) redirect(w http.ResponseWriter, r *http.Request, target string) {
	if strings.EqualFold(r.Header.Get("Hx-Request"), "true") {
		w.Header().Set("Hx-Redirect", target)
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}

// decodePushBody reads a small JSON body, answering 400 when it is not one.
func decodePushBody(w http.ResponseWriter, r *http.Request, target any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxPushBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(target); err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return false
	}
	return true
}
