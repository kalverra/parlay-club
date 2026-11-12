package web

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/rs/zerolog"

	"github.com/kalverra/parlay-club/internal/db"
)

// screenshotCache matches staticCache: capability tokens are 128-bit random
// values and screenshots are immutable once written.
const screenshotCache = staticCache

// ScreenshotHandler serves bug report screenshots by capability token at
// /screenshots/{token}.jpg without requiring authentication.
func ScreenshotHandler(store *db.Store, logger zerolog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if store == nil {
			http.NotFound(w, r)
			return
		}

		token, ok := screenshotToken(r.PathValue("file"))
		if !ok {
			http.NotFound(w, r)
			return
		}

		data, mime, err := store.ScreenshotByToken(r.Context(), token)
		if err != nil {
			if errors.Is(err, db.ErrBugReportNotFound) {
				http.NotFound(w, r)
				return
			}
			logger.Error().Err(err).Msg("load screenshot")
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		if len(data) == 0 {
			http.NotFound(w, r)
			return
		}

		if mime == "" {
			mime = "image/jpeg"
		}
		w.Header().Set("Content-Type", mime)
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		w.Header().Set("Cache-Control", screenshotCache)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		//nolint:gosec // G705: binary image bytes served with explicit image Content-Type and nosniff.
		_, _ = w.Write(data)
	})
}

// screenshotToken extracts the capability token from a "{token}.jpg" path segment.
func screenshotToken(file string) (string, bool) {
	token, ok := strings.CutSuffix(file, ".jpg")
	if !ok || token == "" {
		return "", false
	}
	return token, true
}
