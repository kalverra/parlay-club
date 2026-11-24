package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/rs/zerolog"

	"github.com/kalverra/parlay-club/internal/db"
	webui "github.com/kalverra/parlay-club/internal/web"
)

// maxFormBytes caps a submitted form; these forms hold a word or two.
const maxFormBytes = 4 << 10

const (
	signInFailed = "Sign-in didn't work. Try again."
	blankEmail   = "Add an email first."
)

// Renderer renders a full page by name.
type Renderer interface {
	Render(name string, data any) ([]byte, error)
}

// Handlers serves sign-in, the Google round trip, and logout.
type Handlers struct {
	logger   zerolog.Logger
	store    *db.Store
	cookies  *Cookies
	renderer Renderer
	provider Provider
	dev      bool
}

// NewHandlers builds the auth screens. provider is nil when Google is not
// configured, and dev turns on the local sign-in routes.
func NewHandlers(
	logger zerolog.Logger,
	store *db.Store,
	cookies *Cookies,
	renderer Renderer,
	provider Provider,
	dev bool,
) *Handlers {
	return &Handlers{
		logger:   logger,
		store:    store,
		cookies:  cookies,
		renderer: renderer,
		provider: provider,
		dev:      dev,
	}
}

// Routes mounts the sign-in screens. POST /logout and POST /dev/login are
// plain forms, so CSRF exempts them.
func (h *Handlers) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /login", h.loginForm)
	mux.HandleFunc("POST /logout", h.logout)

	if h.provider != nil {
		mux.HandleFunc("GET /auth/google/start", h.googleStart)
		mux.HandleFunc("GET /auth/google/callback", h.googleCallback)
	}

	if h.dev {
		mux.HandleFunc("GET /dev/login", h.devLoginForm)
		mux.HandleFunc("POST /dev/login", h.devLogin)
	}
}

// LoginPage is the data for the sign-in screen.
type LoginPage struct {
	webui.Page
	Error  string
	Google bool
	Dev    bool
}

// DevLoginPage is the data for the local sign-in form.
type DevLoginPage struct {
	webui.Page
	Error string
	Email string
	Name  string
}

// loginForm shows the sign-in screen.
func (h *Handlers) loginForm(w http.ResponseWriter, r *http.Request) {
	h.renderLogin(w, r, http.StatusOK, "")
}

// logout clears the session and active-group cookies and returns to the
// sign-in screen.
func (h *Handlers) logout(w http.ResponseWriter, r *http.Request) {
	h.cookies.ClearSession(w)
	h.cookies.ClearGroup(w)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// googleStart begins the authorization code flow. It stores state, nonce, and
// the PKCE verifier in a signed cookie, then sends the browser to Google.
func (h *Handlers) googleStart(w http.ResponseWriter, r *http.Request) {
	state, err := randomToken()
	if err != nil {
		h.serverError(w, err)
		return
	}
	nonce, err := randomToken()
	if err != nil {
		h.serverError(w, err)
		return
	}
	verifier, err := randomToken()
	if err != nil {
		h.serverError(w, err)
		return
	}

	h.cookies.SetOAuth(w, OAuthState{
		State:    state,
		Nonce:    nonce,
		Verifier: verifier,
		ReturnTo: h.cookies.ReturnTo(r),
	})
	http.Redirect(w, r, h.provider.AuthURL(state, nonce, pkceChallenge(verifier)), http.StatusFound)
}

// googleCallback checks the round trip, exchanges the code, signs the person
// in, and returns them to the app.
func (h *Handlers) googleCallback(w http.ResponseWriter, r *http.Request) {
	saved, ok := h.cookies.OAuthState(r)
	if !ok {
		h.renderLogin(w, r, http.StatusBadRequest, signInFailed)
		return
	}
	if r.URL.Query().Get("state") != saved.State {
		h.cookies.ClearOAuth(w)
		h.renderLogin(w, r, http.StatusBadRequest, signInFailed)
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		h.cookies.ClearOAuth(w)
		h.renderLogin(w, r, http.StatusBadRequest, signInFailed)
		return
	}

	identity, err := h.provider.Exchange(r.Context(), code, saved.Verifier, saved.Nonce)
	if err != nil {
		h.logger.Warn().Err(err).Msg("google sign-in")
		h.cookies.ClearOAuth(w)
		h.renderLogin(w, r, http.StatusBadRequest, signInFailed)
		return
	}

	user, ok := h.signIn(w, r, identity)
	if !ok {
		return
	}

	h.cookies.ClearOAuth(w)
	h.cookies.SetSession(w, user.ID)
	http.Redirect(w, r, destination(saved.ReturnTo), http.StatusSeeOther)
}

// devLoginForm shows the local sign-in form.
func (h *Handlers) devLoginForm(w http.ResponseWriter, r *http.Request) {
	h.renderDevLogin(w, r, http.StatusOK, "", "", "")
}

// devLogin signs in a local identity without Google.
func (h *Handlers) devLogin(w http.ResponseWriter, r *http.Request) {
	form, err := readForm(w, r)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	email := strings.TrimSpace(form.Get("email"))
	name := strings.TrimSpace(form.Get("name"))
	if !validEmail(email) {
		h.renderDevLogin(w, r, http.StatusUnprocessableEntity, email, name, blankEmail)
		return
	}

	user, ok := h.signIn(w, r, Identity{
		Sub:   "dev:" + email,
		Email: email,
		Name:  name,
	})
	if !ok {
		return
	}

	target := destination(h.cookies.ReturnTo(r))
	h.cookies.ClearOAuth(w)
	h.cookies.SetSession(w, user.ID)
	//nolint:gosec // destination only returns a /join/ path or the home page.
	http.Redirect(w, r, target, http.StatusSeeOther)
}

// signIn resolves a verified identity and renders the failure itself. It
// reports whether the caller should continue with a session.
func (h *Handlers) signIn(
	w http.ResponseWriter,
	r *http.Request,
	identity Identity,
) (db.User, bool) {
	user, err := SignIn(r.Context(), h.store, identity)
	switch {
	case errors.Is(err, ErrIncompleteIdentity):
		h.renderLogin(w, r, http.StatusBadRequest, signInFailed)
		return db.User{}, false
	case err != nil:
		h.serverError(w, err)
		return db.User{}, false
	}
	return user, true
}

// renderLogin writes the sign-in screen with an optional error.
func (h *Handlers) renderLogin(w http.ResponseWriter, r *http.Request, status int, message string) {
	//nolint:modernize // naming the embedded layout field keeps the data readable.
	page := LoginPage{
		Page:   webui.Page{Title: "Sign in", Theme: webui.ThemeFromRequest(r).ID},
		Error:  message,
		Google: h.provider != nil,
		Dev:    h.dev,
	}
	h.render(w, status, "login", page)
}

// renderDevLogin writes the local sign-in form with an optional error. email
// and name are what the fields should keep showing after a rejected submit.
func (h *Handlers) renderDevLogin(
	w http.ResponseWriter,
	r *http.Request,
	status int,
	email, name, message string,
) {
	//nolint:modernize // naming the embedded layout field keeps the data readable.
	page := DevLoginPage{
		Page:  webui.Page{Title: "Dev sign-in", Theme: webui.ThemeFromRequest(r).ID},
		Error: message,
		Email: email,
		Name:  name,
	}
	h.render(w, status, "dev-login", page)
}

// render writes a full page. Auth pages carry names, so browsers must not store
// them.
func (h *Handlers) render(w http.ResponseWriter, status int, page string, data any) {
	html, err := h.renderer.Render(page, data)
	if err != nil {
		h.logger.Error().Err(err).Str("page", page).Msg("render page")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = w.Write(html)
}

// serverError logs err and answers 500.
func (h *Handlers) serverError(w http.ResponseWriter, err error) {
	h.logger.Error().Err(err).Msg("auth request")
	http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}

// readForm parses a small submitted form.
func readForm(w http.ResponseWriter, r *http.Request) (url.Values, error) {
	r.Body = http.MaxBytesReader(w, r.Body, maxFormBytes)
	if err := r.ParseForm(); err != nil {
		return nil, err
	}
	return r.PostForm, nil
}

// randomToken returns 32 random bytes as URL-safe base64 without padding.
func randomToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// pkceChallenge returns the S256 challenge for a verifier.
func pkceChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// validEmail rejects an obvious typo without pulling in address parsing that
// would accept display names.
func validEmail(email string) bool {
	at := strings.LastIndex(email, "@")
	return at > 0 && at < len(email)-1 && !strings.ContainsAny(email, " \t")
}

// safeReturn accepts only a join path as a post-sign-in destination, so the
// return cookie can never send a browser off-site.
func safeReturn(path string) string {
	if !strings.HasPrefix(path, "/join/") || strings.HasPrefix(path, "//") {
		return ""
	}
	if strings.ContainsAny(path, "\\\r\n") {
		return ""
	}
	return path
}

// destination is the saved join path, or the home page when there is none.
func destination(path string) string {
	if target := safeReturn(path); target != "" {
		return target
	}
	return "/"
}
