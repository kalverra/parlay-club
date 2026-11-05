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
	"POST /login": true,
	"POST /who":   true,
	"POST /users": true,
}

// RequirePasscode sends a browser without a valid passcode cookie to the
// passcode screen.
func (h *Handlers) RequirePasscode(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !h.cookies.HasPasscode(r) {
			redirect(w, r, "/login")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireUser sends a browser without a valid, active user cookie to the
// picker, and hands the loaded user to the next handler.
func (h *Handlers) RequireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := h.cookies.UserID(r)
		if !ok {
			redirect(w, r, "/who")
			return
		}

		user, err := h.store.UserByID(r.Context(), id)
		switch {
		case errors.Is(err, db.ErrUserNotFound):
			redirect(w, r, "/who")
			return
		case err != nil:
			h.serverError(w, err)
			return
		case !user.Active:
			redirect(w, r, "/who")
			return
		}

		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userKey{}, user)))
	})
}

// CSRF rejects a non-GET request that did not come from HTMX. A custom header
// cannot be sent cross-site without CORS, and SameSite=Lax covers the three
// plain forms that skip the check.
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

// redirect sends a browser to target. An HTMX request gets a 200 with
// HX-Redirect instead of a 302, because HTMX would swap a redirect's empty body
// into the page rather than navigate.
func redirect(w http.ResponseWriter, r *http.Request, target string) {
	if strings.EqualFold(r.Header.Get(hxRequest), "true") {
		w.Header().Set(hxRedirect, target)
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, target, http.StatusFound)
}
