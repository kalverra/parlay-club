package auth

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/rs/zerolog"

	"github.com/kalverra/parlay-club/internal/db"
	webui "github.com/kalverra/parlay-club/internal/web"
)

const (
	// maxNameLength is the longest user name, in characters.
	maxNameLength = 24
	// failDelay is how long a wrong passcode waits before the form returns.
	failDelay = time.Second
	// maxFormBytes caps a submitted form; these forms hold a word or two.
	maxFormBytes = 4 << 10

	wrongPasscode = "That passcode didn't match. Try again."
	badUser       = "Pick a name from the list."
	goneUser      = "That name is no longer on the list."
	inactiveUser  = "That name is no longer active."
	takenName     = "That name is already taken."
	blankName     = "Add a name first."
)

// Renderer renders a full page by name.
type Renderer interface {
	Render(name string, data any) ([]byte, error)
}

// Handlers serves the passcode screen and the user picker.
type Handlers struct {
	logger   zerolog.Logger
	store    *db.Store
	cookies  *Cookies
	renderer Renderer
	wait     func(time.Duration)
}

// NewHandlers builds the auth screens. wait receives the pause after a wrong
// passcode; nil uses time.Sleep.
func NewHandlers(
	logger zerolog.Logger,
	store *db.Store,
	cookies *Cookies,
	renderer Renderer,
	wait func(time.Duration),
) *Handlers {
	if wait == nil {
		wait = time.Sleep
	}
	return &Handlers{logger: logger, store: store, cookies: cookies, renderer: renderer, wait: wait}
}

// Routes mounts the passcode and picker screens. The three POST forms are the
// only non-GET requests exempt from the CSRF header check.
func (h *Handlers) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /login", h.loginForm)
	mux.HandleFunc("POST /login", h.login)
	mux.HandleFunc("GET /who", h.whoForm)
	mux.HandleFunc("POST /who", h.who)
	mux.HandleFunc("POST /users", h.addUser)
}

// LoginPage is the data for the passcode screen.
type LoginPage struct {
	webui.Page
	Error string
}

// WhoPage is the data for the user picker.
type WhoPage struct {
	webui.Page
	Users []PickerUser
	Error string
	Name  string
}

// PickerUser is one row in the user picker.
type PickerUser struct {
	ID          int64
	Name        string
	Initials    string
	AvatarClass string
}

// loginForm shows the passcode form.
func (h *Handlers) loginForm(w http.ResponseWriter, _ *http.Request) {
	h.renderLogin(w, http.StatusOK, "")
}

// login checks the submitted passcode. A wrong one waits a second, so guessing
// stays slow even if the answer looks cheap.
func (h *Handlers) login(w http.ResponseWriter, r *http.Request) {
	form, err := readForm(w, r)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	if !h.cookies.PasscodeMatches(form.Get("passcode")) {
		h.wait(failDelay)
		h.renderLogin(w, http.StatusUnauthorized, wrongPasscode)
		return
	}

	h.cookies.SetPasscode(w)
	http.Redirect(w, r, "/who", http.StatusSeeOther)
}

// whoForm shows the picker and the add-name form.
func (h *Handlers) whoForm(w http.ResponseWriter, r *http.Request) {
	h.renderWho(w, r, http.StatusOK, "", "")
}

// who signs in the chosen user. A stale or tampered ID comes back as a form
// error, so a person can pick again without losing the page.
func (h *Handlers) who(w http.ResponseWriter, r *http.Request) {
	form, err := readForm(w, r)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	id, err := strconv.ParseInt(form.Get("user_id"), 10, 64)
	if err != nil {
		h.renderWho(w, r, http.StatusUnprocessableEntity, "", badUser)
		return
	}

	user, err := h.store.UserByID(r.Context(), id)
	switch {
	case errors.Is(err, db.ErrUserNotFound):
		h.renderWho(w, r, http.StatusUnprocessableEntity, "", goneUser)
		return
	case err != nil:
		h.serverError(w, err)
		return
	case !user.Active:
		h.renderWho(w, r, http.StatusUnprocessableEntity, "", inactiveUser)
		return
	}

	h.cookies.SetUser(w, user.ID)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// addUser creates a name and picks it. Only the person who added it signs in as
// that name; everyone else finds it in the picker next time.
func (h *Handlers) addUser(w http.ResponseWriter, r *http.Request) {
	form, err := readForm(w, r)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(form.Get("name"))
	if message := nameProblem(name); message != "" {
		h.renderWho(w, r, http.StatusUnprocessableEntity, name, message)
		return
	}

	user, err := h.store.CreateUser(r.Context(), name)
	if errors.Is(err, db.ErrNameTaken) {
		h.renderWho(w, r, http.StatusUnprocessableEntity, name, takenName)
		return
	}
	if err != nil {
		h.serverError(w, err)
		return
	}

	h.cookies.SetUser(w, user.ID)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// nameProblem returns the message for an invalid name, or "" when it is valid.
func nameProblem(name string) string {
	switch {
	case name == "":
		return blankName
	case utf8.RuneCountInString(name) > maxNameLength:
		return fmt.Sprintf("Names can be at most %d characters.", maxNameLength)
	default:
		return ""
	}
}

// renderLogin writes the passcode screen with an optional error.
func (h *Handlers) renderLogin(w http.ResponseWriter, status int, message string) {
	//nolint:modernize // naming the embedded layout field keeps the data readable.
	page := LoginPage{Page: webui.Page{Title: "Passcode"}, Error: message}
	h.render(w, status, "login", page)
}

// renderWho loads the active users and renders the picker. name is what the
// add-name field should keep showing after a rejected submission.
func (h *Handlers) renderWho(w http.ResponseWriter, r *http.Request, status int, name, message string) {
	users, err := h.store.ListActiveUsers(r.Context())
	if err != nil {
		h.serverError(w, err)
		return
	}

	picker := make([]PickerUser, len(users))
	for i, user := range users {
		picker[i] = PickerUser{
			ID:          user.ID,
			Name:        user.Name,
			Initials:    webui.Initials(user.Name),
			AvatarClass: user.AvatarColor.Class(),
		}
	}

	//nolint:modernize // naming the embedded layout field keeps the data readable.
	page := WhoPage{Page: webui.Page{Title: "Who are you?"}, Users: picker, Error: message, Name: name}
	h.render(w, status, "who", page)
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
		return nil, fmt.Errorf("parse form: %w", err)
	}
	return r.PostForm, nil
}
