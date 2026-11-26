package groups

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/rs/zerolog"

	"github.com/kalverra/parlay-club/internal/db"
	webui "github.com/kalverra/parlay-club/internal/web"
)

// maxFormBytes caps a submitted form; these forms hold a name or an ID.
const maxFormBytes = 4 << 10

const (
	blankGroupName   = "Give the group a name."
	lastGroupMessage = "You can't leave your only group."
)

// Renderer renders a full page by name.
type Renderer interface {
	Render(name string, data any) ([]byte, error)
}

// Handlers serves the invite screens. They sit inside RequireUser and Scope, so
// the signed-in person is already on the context.
type Handlers struct {
	logger   zerolog.Logger
	store    InviteStore
	userFrom UserFrom
	cookies  GroupCookie
	renderer Renderer
	baseURL  string
}

// NewHandlers builds the invite screens. baseURL is the public origin, such as
// https://parlay.example.com, used to build invite links. Empty means build
// them from the request instead.
func NewHandlers(
	logger zerolog.Logger,
	store InviteStore,
	userFrom UserFrom,
	cookies GroupCookie,
	renderer Renderer,
	baseURL string,
) *Handlers {
	return &Handlers{
		logger:   logger,
		store:    store,
		userFrom: userFrom,
		cookies:  cookies,
		renderer: renderer,
		baseURL:  strings.TrimRight(baseURL, "/"),
	}
}

// Routes mounts the group pages. They sit inside RequireUser and Scope.
func (h *Handlers) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /groups", h.groupsPage)
	mux.HandleFunc("POST /groups", h.createGroup)
	mux.HandleFunc("POST /groups/switch", h.switchGroup)
	mux.HandleFunc("POST /groups/{id}/reset-invite", h.resetInvite)
	mux.HandleFunc("POST /groups/{id}/leave", h.leave)

	mux.HandleFunc("GET /join/{token}", h.joinForm)
	mux.HandleFunc("POST /join/{token}", h.join)
}

// GroupCard is one group on the /groups page: its name, its invite link, and
// the actions the person can take on it.
type GroupCard struct {
	ID        int64
	Name      string
	InviteURL string
	Current   bool
	CanLeave  bool
}

// Page is the group list and create form.
type Page struct {
	webui.Page
	Cards []GroupCard
}

// groupsPage lists the caller's groups, their invite links, and the create
// form. It sits inside Scope, so the active group is already chosen.
func (h *Handlers) groupsPage(w http.ResponseWriter, r *http.Request) {
	user, ok := h.userFrom(r.Context())
	if !ok {
		h.redirectToLogin(w, r)
		return
	}
	h.renderGroups(w, r, user, http.StatusOK)
}

// createGroup adds a group, makes the caller its first member, and makes it
// their active group.
func (h *Handlers) createGroup(w http.ResponseWriter, r *http.Request) {
	user, ok := h.userFrom(r.Context())
	if !ok {
		h.redirectToLogin(w, r)
		return
	}
	if err := readForm(w, r); err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	group, err := h.store.Create(r.Context(), user.ID, r.PostForm.Get("name"))
	if errors.Is(err, db.ErrGroupNameRequired) {
		http.Error(w, blankGroupName, http.StatusUnprocessableEntity)
		return
	}
	if err != nil {
		h.serverError(w, err)
		return
	}

	h.cookies.SetGroup(w, group.ID)
	h.redirect(w, r, "/groups")
}

// switchGroup makes a group the caller belongs to the active one.
func (h *Handlers) switchGroup(w http.ResponseWriter, r *http.Request) {
	user, ok := h.userFrom(r.Context())
	if !ok {
		h.redirectToLogin(w, r)
		return
	}
	if err := readForm(w, r); err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	groupID, ok := parseID(r.PostForm.Get("group_id"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	member, err := h.store.IsMember(r.Context(), groupID, user.ID)
	if err != nil {
		h.serverError(w, err)
		return
	}
	if !member {
		http.NotFound(w, r)
		return
	}

	h.cookies.SetGroup(w, groupID)
	h.refresh(w)
}

// resetInvite replaces one group's invite token, which kills the old link.
func (h *Handlers) resetInvite(w http.ResponseWriter, r *http.Request) {
	user, ok := h.userFrom(r.Context())
	if !ok {
		h.redirectToLogin(w, r)
		return
	}
	groupID, ok := h.memberGroupID(w, r, user.ID)
	if !ok {
		return
	}

	if _, err := h.store.ResetInvite(r.Context(), groupID); err != nil {
		h.serverError(w, err)
		return
	}
	h.refresh(w)
}

// leave removes the caller's membership in one group, and nothing else. It
// moves the active cookie to another group when the one left was active.
func (h *Handlers) leave(w http.ResponseWriter, r *http.Request) {
	user, ok := h.userFrom(r.Context())
	if !ok {
		h.redirectToLogin(w, r)
		return
	}
	groupID, ok := h.memberGroupID(w, r, user.ID)
	if !ok {
		return
	}

	err := h.store.Leave(r.Context(), groupID, user.ID)
	switch {
	case errors.Is(err, db.ErrLastGroup):
		http.Error(w, lastGroupMessage, http.StatusUnprocessableEntity)
		return
	case errors.Is(err, db.ErrNotMember):
		http.NotFound(w, r)
		return
	case err != nil:
		h.serverError(w, err)
		return
	}

	if active, ok := From(r.Context()); ok && active.ID == groupID {
		remaining, err := h.store.ForUser(r.Context(), user.ID)
		if err != nil {
			h.serverError(w, err)
			return
		}
		if len(remaining) > 0 {
			h.cookies.SetGroup(w, remaining[0].ID)
		}
	}
	h.redirect(w, r, "/groups")
}

// memberGroupID parses the group in the path and reports 404 unless the caller
// belongs to it, so a stranger cannot learn which groups exist.
func (h *Handlers) memberGroupID(
	w http.ResponseWriter,
	r *http.Request,
	userID int64,
) (int64, bool) {
	groupID, ok := parseID(r.PathValue("id"))
	if !ok {
		http.NotFound(w, r)
		return 0, false
	}

	member, err := h.store.IsMember(r.Context(), groupID, userID)
	if err != nil {
		h.serverError(w, err)
		return 0, false
	}
	if !member {
		http.NotFound(w, r)
		return 0, false
	}
	return groupID, true
}

// renderGroups writes the group list for user.
func (h *Handlers) renderGroups(w http.ResponseWriter, r *http.Request, user db.User, status int) {
	memberships, ok := membershipsFrom(r.Context())
	if !ok {
		var err error
		memberships, err = h.store.ForUser(r.Context(), user.ID)
		if err != nil {
			h.serverError(w, err)
			return
		}
	}

	active, _ := From(r.Context())
	canLeave := len(memberships) > 1
	cards := make([]GroupCard, 0, len(memberships))
	for _, group := range memberships {
		cards = append(cards, GroupCard{
			ID:        group.ID,
			Name:      group.Name,
			InviteURL: h.inviteURL(r, group.InviteToken),
			Current:   group.ID == active.ID,
			CanLeave:  canLeave,
		})
	}

	view := webui.NewUser(user)
	//nolint:modernize // naming the embedded layout field keeps the data readable.
	page := Page{
		Page: webui.Page{
			Title:  "Groups",
			User:   &view,
			Groups: Options(r.Context()),
			Theme:  webui.ThemeForRequest(r, user.Name).ID,
		},
		Cards: cards,
	}
	h.render(w, status, "groups", page)
}

// JoinPage is the data for "Join {group}?".
type JoinPage struct {
	webui.Page
	GroupName string
	Members   []webui.User
	Token     string
	Already   bool
}

// joinForm shows who is in the group behind an invite link.
func (h *Handlers) joinForm(w http.ResponseWriter, r *http.Request) {
	user, ok := h.userFrom(r.Context())
	if !ok {
		h.redirectToLogin(w, r)
		return
	}

	group, err := h.store.ByToken(r.Context(), r.PathValue("token"))
	if errors.Is(err, db.ErrInviteNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		h.serverError(w, err)
		return
	}

	members, err := h.store.Members(r.Context(), group.ID)
	if err != nil {
		h.serverError(w, err)
		return
	}
	already, err := h.store.IsMember(r.Context(), group.ID, user.ID)
	if err != nil {
		h.serverError(w, err)
		return
	}

	view := webui.NewUser(user)
	page := JoinPage{
		//nolint:modernize // naming the embedded layout field keeps the data readable.
		Page: webui.Page{
			Title:  "Join " + group.Name,
			User:   &view,
			Groups: Options(r.Context()),
			Theme:  webui.ThemeForRequest(r, user.Name).ID,
		},
		GroupName: group.Name,
		Members:   memberViews(members),
		Token:     group.InviteToken,
		Already:   already,
	}
	h.render(w, http.StatusOK, "join", page)
}

// join adds the caller to the group and makes it their active group. Joining
// twice changes nothing.
func (h *Handlers) join(w http.ResponseWriter, r *http.Request) {
	user, ok := h.userFrom(r.Context())
	if !ok {
		h.redirectToLogin(w, r)
		return
	}

	group, err := h.store.JoinByToken(r.Context(), user.ID, r.PathValue("token"))
	if errors.Is(err, db.ErrInviteNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		h.serverError(w, err)
		return
	}

	h.cookies.SetGroup(w, group.ID)
	h.redirect(w, r, "/")
}

// memberViews maps stored members to the avatars the layout draws.
func memberViews(members []db.User) []webui.User {
	views := make([]webui.User, 0, len(members))
	for _, member := range members {
		views = append(views, webui.NewUser(member))
	}
	return views
}

// inviteURL builds the absolute link a person can paste into a message. A
// configured base URL wins, because a proxy may rewrite Host to an internal
// hostname. Without one, the request's own host and scheme are the best guess.
func (h *Handlers) inviteURL(r *http.Request, token string) string {
	if h.baseURL != "" {
		return h.baseURL + "/join/" + token
	}
	scheme := "http"
	if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	if r.Host == "" {
		return "/join/" + token
	}
	return scheme + "://" + r.Host + "/join/" + token
}

// parseID reads a positive integer form or path value.
func parseID(raw string) (int64, bool) {
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}

// readForm parses a small submitted form.
func readForm(w http.ResponseWriter, r *http.Request) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxFormBytes)
	return r.ParseForm()
}

// refresh tells HTMX to reload the current page. The group list and the active
// group in the top bar both change, so a full reload is simpler than a swap.
func (h *Handlers) refresh(w http.ResponseWriter) {
	w.Header().Set("Hx-Refresh", "true")
	w.WriteHeader(http.StatusOK)
}

// redirect sends the browser to target. An HTMX request gets a 200 with
// HX-Redirect so HTMX navigates instead of swapping in an empty body.
func (h *Handlers) redirect(w http.ResponseWriter, r *http.Request, target string) {
	if strings.EqualFold(r.Header.Get("Hx-Request"), "true") {
		w.Header().Set("Hx-Redirect", target)
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}

// redirectToLogin sends an unexpected signed-out request to the sign-in screen.
// RequireUser normally handles this before the handler runs.
func (h *Handlers) redirectToLogin(w http.ResponseWriter, r *http.Request) {
	h.redirect(w, r, "/login")
}

// render writes a full page.
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
	h.logger.Error().Err(err).Msg("groups request")
	http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}
