package web

import (
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/kalverra/parlay-club/internal/bugs"
	"github.com/kalverra/parlay-club/internal/db"
)

const (
	// maxBugReportBodyBytes caps POST /bug-reports form bodies at 2 MiB to
	// protect the single Cloud Run instance and SQLite size.
	maxBugReportBodyBytes  = 2 << 20
	maxScreenshotBytes     = 1 << 20
	maxBugDescriptionRunes = 4000
	maxBugPageURLRunes     = 2000
	maxBugUserAgentRunes   = 512
	maxBugScreenSizeRunes  = 64
)

type bugHandlers struct {
	deps Dependencies
}

// Routes mounts the authenticated bug report submission endpoint.
func (h *bugHandlers) Routes(mux *http.ServeMux) {
	mux.HandleFunc("POST /bug-reports", h.submit)
}

// BugReportSuccessView is rendered inside #bug-report-body after a report is saved.
type BugReportSuccessView struct {
	IssueURL string
	IssueNum int
}

// BugReportErrorView is rendered on 400 validation errors for POST /bug-reports.
type BugReportErrorView struct {
	Message string
}

// submit validates a bug report submission, stores it in SQLite, optionally
// files a GitHub issue, and renders the HTMX result partial.
func (h *bugHandlers) submit(w http.ResponseWriter, r *http.Request) {
	user, ok := h.user(r)
	if !ok {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}
	if h.deps.Bugs == nil {
		h.serverError(w, errors.New("bug service not configured"))
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxBugReportBodyBytes)
	if err := r.ParseForm(); err != nil {
		if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
			h.renderError(w, "Bug report is too large (max 2 MB).")
			return
		}
		h.renderError(w, "Invalid bug report submission.")
		return
	}

	description := strings.TrimSpace(r.FormValue("description"))
	if description == "" {
		h.renderError(w, "Please describe what broke or looks wrong.")
		return
	}
	if utf8.RuneCountInString(description) > maxBugDescriptionRunes {
		h.renderError(w, "Description is too long (max 4000 characters).")
		return
	}

	var screenshot []byte
	var screenshotMIME string
	if raw := strings.TrimSpace(r.FormValue("screenshot")); raw != "" {
		data, mime, err := decodeScreenshotDataURL(raw)
		if err != nil {
			h.renderError(w, err.Error())
			return
		}
		screenshot = data
		screenshotMIME = mime
	}

	userAgent := strings.TrimSpace(r.FormValue("user_agent"))
	if userAgent == "" {
		userAgent = r.UserAgent()
	}

	result, err := h.deps.Bugs.SubmitReport(r.Context(), user, bugs.SubmitInput{
		PageURL:        truncateRunes(r.FormValue("page_url"), maxBugPageURLRunes),
		Description:    description,
		UserAgent:      truncateRunes(userAgent, maxBugUserAgentRunes),
		ScreenSize:     truncateRunes(r.FormValue("screen_size"), maxBugScreenSizeRunes),
		Screenshot:     screenshot,
		ScreenshotMIME: screenshotMIME,
	})
	if err != nil {
		if errors.Is(err, db.ErrBugReportDescriptionRequired) {
			h.renderError(w, "Please describe what broke or looks wrong.")
			return
		}
		h.serverError(w, err)
		return
	}

	h.renderPartial(w, http.StatusOK, "bug-report-success", BugReportSuccessView{
		IssueURL: result.IssueURL,
		IssueNum: result.IssueNum,
	})
}

func (h *bugHandlers) user(r *http.Request) (db.User, bool) {
	if h.deps.UserFrom == nil {
		return db.User{}, false
	}
	return h.deps.UserFrom(r.Context())
}

func (h *bugHandlers) renderError(w http.ResponseWriter, message string) {
	h.renderPartial(w, http.StatusBadRequest, "bug-report-error", BugReportErrorView{
		Message: message,
	})
}

func (h *bugHandlers) renderPartial(w http.ResponseWriter, status int, name string, data any) {
	html, err := h.deps.Renderer.RenderPartial(name, data)
	if err != nil {
		h.serverError(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = w.Write(html)
}

func (h *bugHandlers) serverError(w http.ResponseWriter, err error) {
	h.deps.Logger.Error().Err(err).Msg("bug report request")
	http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}

// decodeScreenshotDataURL parses a base64 image data URL into raw bytes and MIME type.
func decodeScreenshotDataURL(raw string) ([]byte, string, error) {
	rest, ok := strings.CutPrefix(raw, "data:")
	if !ok {
		return nil, "", errors.New("screenshot must be a base64 data URL")
	}
	meta, encoded, ok := strings.Cut(rest, ",")
	if !ok {
		return nil, "", errors.New("screenshot data URL is malformed")
	}
	mime, ok := strings.CutSuffix(meta, ";base64")
	if !ok || !isAllowedScreenshotMIME(mime) {
		return nil, "", errors.New("screenshot must be a base64 JPEG, PNG, or WebP image")
	}
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, "", errors.New("screenshot image data is invalid")
	}
	if len(decoded) > maxScreenshotBytes {
		return nil, "", errors.New("screenshot exceeds maximum size (max 2 MB)")
	}
	return decoded, mime, nil
}

func truncateRunes(s string, maxRunes int) string {
	if utf8.RuneCountInString(s) <= maxRunes {
		return s
	}
	return string([]rune(s)[:maxRunes])
}

func isAllowedScreenshotMIME(mime string) bool {
	return mime == "image/jpeg" || mime == "image/png" || mime == "image/webp"
}
