package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/kalverra/parlay-club/internal/db"
)

// hxRequest marks a request made by HTMX. hxRedirect tells HTMX to navigate.
// Header names are case-insensitive; these are the canonical Go forms.
const (
	hxRequest  = "Hx-Request"
	hxRedirect = "Hx-Redirect"
)

// csrfExempt lists the plain forms that may post without the HX-Request
// header. Everything else non-GET must come from HTMX.
var csrfExempt = map[string]bool{
	"POST /logout":    true,
	"POST /dev/login": true,
}

// RequireUser sends a browser without a valid session to the sign-in screen,
// and hands the loaded user to the next handler. It also refuses an inactive
// user.
func (h *Handlers) RequireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := h.cookies.SessionID(r)
		if !ok {
			h.redirect(w, r)
			return
		}

		user, err := h.store.UserByID(r.Context(), id)
		switch {
		case errors.Is(err, db.ErrUserNotFound):
			h.redirect(w, r)
			return
		case err != nil:
			h.serverError(w, err)
			return
		case !user.Active:
			h.redirect(w, r)
			return
		}

		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userKey{}, user)))
	})
}

// CSRF rejects a non-GET request that did not come from HTMX. A custom header
// cannot be sent cross-site without CORS, and SameSite=Lax covers the plain
// forms that skip the check.
func (h *Handlers) CSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet || csrfExempt[r.Method+" "+r.URL.Path] {
			next.ServeHTTP(w, r)
			return
		}
		if strings.EqualFold(r.Header.Get(hxRequest), "true") {
			next.ServeHTTP(w, r)
			return
		}

		h.logger.Warn().
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Msg("reject cross-site request")
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
	})
}

// userKey is the context key for the user RequireUser loaded.
type userKey struct{}

// UserFrom returns the user RequireUser put on ctx.
func UserFrom(ctx context.Context) (db.User, bool) {
	user, ok := ctx.Value(userKey{}).(db.User)
	return user, ok
}

// redirect sends a browser to the sign-in screen. It remembers a join path in
// the signed pc_oauth cookie so sign-in lands back on the invite. An HTMX
// request gets a 200 with HX-Redirect instead of a 302, because HTMX would swap
// a redirect's empty body into the page rather than navigate.
func (h *Handlers) redirect(w http.ResponseWriter, r *http.Request) {
	if target := safeReturn(r.URL.Path); target != "" {
		h.cookies.SetReturnTo(w, target)
	}

	if strings.EqualFold(r.Header.Get(hxRequest), "true") {
		w.Header().Set(hxRedirect, "/login")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/login", http.StatusFound)
}
