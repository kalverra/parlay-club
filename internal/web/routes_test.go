package web_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/db"
	webui "github.com/kalverra/parlay-club/internal/web"
	assets "github.com/kalverra/parlay-club/web"
)

func TestShellPages(t *testing.T) {
	t.Parallel()

	handler := newShell(t)

	tests := []struct {
		path    string
		heading string
	}{
		{"/", "Slip"},
		{"/games", "Games"},
		{"/research", "Research"},
		{"/record", "Record Book"},
	}

	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			t.Parallel()

			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodGet, test.path, nil))

			assert.Equal(t, http.StatusOK, recorder.Code)
			assert.Equal(t, "text/html; charset=utf-8", recorder.Header().Get("Content-Type"))
			assert.Contains(t, recorder.Body.String(), `<h1 class="text-xl font-semibold">`+test.heading+"</h1>")

			current := regexp.MustCompile(
				`<a\s+href="` + regexp.QuoteMeta(test.path) + `"[^>]*aria-current="page"`,
			)
			assert.Regexp(t, current, recorder.Body.String())
		})
	}
}

func TestShellServesStatic(t *testing.T) {
	t.Parallel()

	handler := newShell(t)

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		webui.AssetURL("app.css"),
		nil,
	))

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "public, max-age=31536000, immutable", recorder.Header().Get("Cache-Control"))
	assert.NotEmpty(t, recorder.Body.String())
}

func TestShellUnknownPath(t *testing.T) {
	t.Parallel()

	handler := newShell(t)

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/nope", nil))

	assert.Equal(t, http.StatusNotFound, recorder.Code)
}

func TestShellRendersSignedInUser(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		user db.User
		want []string
		deny []string
	}{
		{
			name: "initials",
			user: db.User{Name: "Adam Hamrick", AvatarColor: db.AvatarColor("sky")},
			want: []string{`href="/me"`, `title="Adam Hamrick"`, ">AH</span", "bg-sky-500"},
			deny: []string{"<img"},
		},
		{
			name: "picture",
			user: db.User{
				Name:        "Adam Hamrick",
				AvatarColor: db.AvatarColor("sky"),
				PictureURL:  "https://example.com/ada.png",
			},
			want: []string{
				`href="/me"`,
				`title="Adam Hamrick"`,
				`src="https://example.com/ada.png"`,
				`referrerpolicy="no-referrer"`,
			},
			deny: []string{">AH</span"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			handler := newShellWithUser(t, func(context.Context) (db.User, bool) {
				return test.user, true
			})

			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequestWithContext(
				t.Context(),
				http.MethodGet,
				"/",
				nil,
			))

			require.Equal(t, http.StatusOK, recorder.Code)
			body := recorder.Body.String()
			for _, want := range test.want {
				assert.Contains(t, body, want)
			}
			for _, deny := range test.deny {
				assert.NotContains(t, body, deny)
			}
		})
	}
}

// newShell builds the web routes the way the server does.
func newShell(t *testing.T) http.Handler {
	t.Helper()

	return newShellWithUser(t, nil)
}

// newShellWithUser builds the web routes with a stub user lookup.
func newShellWithUser(t *testing.T, userFrom webui.UserFrom) http.Handler {
	t.Helper()

	renderer, err := webui.NewRenderer(assets.Templates, false)
	require.NoError(t, err)

	return webui.New(zerolog.Nop(), renderer, userFrom, nil)
}
