package groups

import (
	"errors"
	"net/http"
	"strings"

	"github.com/rs/zerolog"

	"github.com/kalverra/parlay-club/internal/db"
	webui "github.com/kalverra/parlay-club/internal/web"
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
}

// NewHandlers builds the invite screens.
func NewHandlers(
	logger zerolog.Logger,
	store InviteStore,
	userFrom UserFrom,
	cookies GroupCookie,
	renderer Renderer,
) *Handlers {
	return &Handlers{
		logger:   logger,
		store:    store,
		userFrom: userFrom,
		cookies:  cookies,
		renderer: renderer,
	}
}

// Routes mounts the invite pages.
func (h *Handlers) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /join/{token}", h.joinForm)
	mux.HandleFunc("POST /join/{token}", h.join)
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

	page := JoinPage{
		//nolint:modernize // naming the embedded layout field keeps the data readable.
		Page:      webui.Page{Title: "Join " + group.Name},
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
		views = append(views, webui.User{
			Name:        member.Name,
			Initials:    webui.Initials(member.Name),
			AvatarClass: member.AvatarColor.Class(),
			PictureURL:  member.PictureURL,
		})
	}
	return views
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
