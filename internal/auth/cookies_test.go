package auth_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/auth"
)

const testSessionKey = "test-session-key-long-enough-for-anything"

// testCookies builds a signer over the test key.
func testCookies(secure bool) *auth.Cookies {
	return auth.NewCookies(testSessionKey, secure)
}

// signedValue repeats the documented MAC so a test fails if the wire format
// drifts: HMAC-SHA256(session key, payload) as lowercase hex.
func signedValue(t *testing.T, payload string) string {
	t.Helper()

	mac := hmac.New(sha256.New, []byte(testSessionKey))
	_, err := mac.Write([]byte(payload))
	require.NoError(t, err)
	return hex.EncodeToString(mac.Sum(nil))
}

// sessionValue builds a session cookie value for id and expiry.
func sessionValue(t *testing.T, id, expiry int64) string {
	t.Helper()

	payload := strconv.FormatInt(id, 10) + "." + strconv.FormatInt(expiry, 10)
	return payload + "." + signedValue(t, "session:"+payload)
}

// oauthValue builds a pc_oauth cookie value for one round trip.
func oauthValue(t *testing.T, state, nonce, verifier string, issued time.Time) string {
	t.Helper()

	payload := state + "." + nonce + "." + verifier + "." + strconv.FormatInt(issued.Unix(), 10)
	return payload + "." + signedValue(t, "oauth:"+payload)
}

// responseCookie returns the named cookie a handler set, failing the test when
// it is absent.
func responseCookie(t *testing.T, w *httptest.ResponseRecorder, name string) *http.Cookie {
	t.Helper()

	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == name {
			return cookie
		}
	}
	require.FailNow(t, "response set no cookie", "cookie %q missing", name)
	return nil
}

// requestWithCookie builds a GET request carrying cookie.
func requestWithCookie(t *testing.T, cookie *http.Cookie) *http.Request {
	t.Helper()

	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	request.AddCookie(cookie)
	return request
}

// tampered changes a cookie value without shortening it.
func tampered(value string) string {
	replacement := byte('0')
	if value[0] == replacement {
		replacement = '1'
	}
	return string(replacement) + value[1:]
}

func TestSetSessionCookie(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		secure bool
		id     int64
	}{
		"dev":        {secure: false, id: 42},
		"prod":       {secure: true, id: 42},
		"large id":   {secure: true, id: 9007199254740993},
		"first user": {secure: false, id: 1},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			w := httptest.NewRecorder()
			testCookies(test.secure).SetSession(w, test.id)

			cookie := responseCookie(t, w, auth.SessionCookie)
			assert.Equal(t, "/", cookie.Path)
			assert.Equal(t, 30*24*60*60, cookie.MaxAge)
			assert.True(t, cookie.HttpOnly)
			assert.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
			assert.Equal(t, test.secure, cookie.Secure)

			parts := splitCookie(t, cookie.Value, 3)
			assert.Equal(t, strconv.FormatInt(test.id, 10), parts[0])
			expiry, err := strconv.ParseInt(parts[1], 10, 64)
			require.NoError(t, err)
			assert.WithinDuration(t,
				time.Now().Add(30*24*time.Hour), time.Unix(expiry, 0), time.Minute)
			assert.Equal(t, signedValue(t, "session:"+parts[0]+"."+parts[1]), parts[2])
		})
	}
}

func TestSessionID(t *testing.T) {
	t.Parallel()

	for _, id := range []int64{1, 42, 9007199254740993} {
		w := httptest.NewRecorder()
		testCookies(false).SetSession(w, id)

		got, ok := testCookies(false).SessionID(requestWithCookie(t, responseCookie(t, w, auth.SessionCookie)))
		require.True(t, ok)
		assert.Equal(t, id, got)
	}
}

func TestSessionIDWithoutCookie(t *testing.T) {
	t.Parallel()

	_, ok := testCookies(false).SessionID(
		httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	assert.False(t, ok)
}

func TestSessionIDRejectsTamperedSignature(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	testCookies(true).SetSession(w, 42)
	//nolint:gosec // the handler set these attributes; the test tampers with the value.
	cookie := responseCookie(t, w, auth.SessionCookie)
	cookie.Value = tampered(cookie.Value)

	_, ok := testCookies(true).SessionID(requestWithCookie(t, cookie))
	assert.False(t, ok)
}

func TestSessionIDRejectsTamperedID(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	testCookies(true).SetSession(w, 42)
	//nolint:gosec // the handler set these attributes; the test tampers with the value.
	cookie := responseCookie(t, w, auth.SessionCookie)
	parts := splitCookie(t, cookie.Value, 3)
	cookie.Value = "43." + parts[1] + "." + parts[2]

	_, ok := testCookies(true).SessionID(requestWithCookie(t, cookie))
	assert.False(t, ok)
}

func TestSessionIDRejectsExpiredCookie(t *testing.T) {
	t.Parallel()

	cookie := &http.Cookie{
		Name:     auth.SessionCookie,
		Value:    sessionValue(t, 42, time.Now().Add(-time.Minute).Unix()),
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}

	_, ok := testCookies(false).SessionID(requestWithCookie(t, cookie))
	assert.False(t, ok)
}

func TestSessionIDRejectsAnotherSessionKey(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	testCookies(false).SetSession(w, 42)
	request := requestWithCookie(t, responseCookie(t, w, auth.SessionCookie))

	_, ok := auth.NewCookies("another-session-key-long-enough", false).SessionID(request)
	assert.False(t, ok)
}

func TestSessionIDRejectsMalformedCookie(t *testing.T) {
	t.Parallel()

	signature := signedValue(t, "session:42.4102444800")
	tests := map[string]string{
		"empty":          "",
		"no dots":        "42",
		"two parts":      "42.4102444800",
		"no id":          ".4102444800." + signature,
		"no expiry":      "42.." + signature,
		"non-numeric id": "forty-two.4102444800." + signedValue(t, "session:forty-two.4102444800"),
		"zero id":        "0.4102444800." + signedValue(t, "session:0.4102444800"),
		"negative id":    "-1.4102444800." + signedValue(t, "session:-1.4102444800"),
		"leading zero":   "042.4102444800." + signedValue(t, "session:042.4102444800"),
		"extra dot":      "42.4102444800." + signature + ".extra",
	}

	for name, value := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cookie := &http.Cookie{
				Name:     auth.SessionCookie,
				Value:    value,
				Secure:   true,
				HttpOnly: true,
				SameSite: http.SameSiteLaxMode,
			}
			_, ok := testCookies(false).SessionID(requestWithCookie(t, cookie))
			assert.False(t, ok)
		})
	}
}

func TestClearSessionCookie(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	testCookies(true).ClearSession(w)

	cookie := responseCookie(t, w, auth.SessionCookie)
	assert.Empty(t, cookie.Value)
	assert.Equal(t, "/", cookie.Path)
	assert.Negative(t, cookie.MaxAge)
	assert.True(t, cookie.Secure)

	_, ok := testCookies(true).SessionID(requestWithCookie(t, cookie))
	assert.False(t, ok)
}

func TestSetOAuthCookie(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	testCookies(true).SetOAuth(w, auth.OAuthState{
		State: "state-1", Nonce: "nonce-1", Verifier: "verifier-1",
	})

	cookie := responseCookie(t, w, auth.OAuthCookie)
	assert.Equal(t, "/auth", cookie.Path)
	assert.Equal(t, 10*60, cookie.MaxAge)
	assert.True(t, cookie.HttpOnly)
	assert.True(t, cookie.Secure)
	assert.Equal(t, http.SameSiteLaxMode, cookie.SameSite)

	state, ok := testCookies(true).OAuthState(requestWithCookie(t, cookie))
	require.True(t, ok)
	assert.Equal(t, auth.OAuthState{State: "state-1", Nonce: "nonce-1", Verifier: "verifier-1"}, state)
}

func TestOAuthStateRejectsTamperedValue(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	testCookies(false).SetOAuth(w, auth.OAuthState{
		State: "state-1", Nonce: "nonce-1", Verifier: "verifier-1",
	})
	//nolint:gosec // the handler set these attributes; the test tampers with the value.
	cookie := responseCookie(t, w, auth.OAuthCookie)
	cookie.Value = tampered(cookie.Value)

	_, ok := testCookies(false).OAuthState(requestWithCookie(t, cookie))
	assert.False(t, ok)
}

func TestOAuthStateRejectsExpiredCookie(t *testing.T) {
	t.Parallel()

	cookie := &http.Cookie{
		Name:     auth.OAuthCookie,
		Value:    oauthValue(t, "state", "nonce", "verifier", time.Now().Add(-11*time.Minute)),
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}

	_, ok := testCookies(false).OAuthState(requestWithCookie(t, cookie))
	assert.False(t, ok)
}

func TestOAuthStateWithoutCookie(t *testing.T) {
	t.Parallel()

	_, ok := testCookies(false).OAuthState(
		httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	assert.False(t, ok)
}

func TestClearOAuthCookie(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	testCookies(false).ClearOAuth(w)

	cookie := responseCookie(t, w, auth.OAuthCookie)
	assert.Empty(t, cookie.Value)
	assert.Equal(t, "/auth", cookie.Path)
	assert.Negative(t, cookie.MaxAge)

	_, ok := testCookies(false).OAuthState(requestWithCookie(t, cookie))
	assert.False(t, ok)
}

func TestClearGroupCookie(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	testCookies(true).ClearGroup(w)

	cookie := responseCookie(t, w, auth.GroupCookie)
	assert.Empty(t, cookie.Value)
	assert.Equal(t, "/", cookie.Path)
	assert.Negative(t, cookie.MaxAge)
	assert.True(t, cookie.Secure)

	_, ok := testCookies(true).GroupID(requestWithCookie(t, cookie))
	assert.False(t, ok)
}

func TestSetGroupCookie(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		secure bool
		id     int64
	}{
		"dev":         {secure: false, id: 42},
		"prod":        {secure: true, id: 42},
		"large id":    {secure: true, id: 9007199254740993},
		"first group": {secure: false, id: 1},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			w := httptest.NewRecorder()
			testCookies(test.secure).SetGroup(w, test.id)

			cookie := responseCookie(t, w, auth.GroupCookie)
			assert.Equal(t, "/", cookie.Path)
			assert.Equal(t, 30*24*60*60, cookie.MaxAge)
			assert.True(t, cookie.HttpOnly)
			assert.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
			assert.Equal(t, test.secure, cookie.Secure)

			parts := splitCookie(t, cookie.Value, 3)
			assert.Equal(t, strconv.FormatInt(test.id, 10), parts[0])
			expiry, err := strconv.ParseInt(parts[1], 10, 64)
			require.NoError(t, err)
			assert.WithinDuration(t,
				time.Now().Add(30*24*time.Hour), time.Unix(expiry, 0), time.Minute)
			assert.Equal(t, signedValue(t, "group:"+parts[0]+"."+parts[1]), parts[2])
		})
	}
}

func TestGroupID(t *testing.T) {
	t.Parallel()

	for _, id := range []int64{1, 42, 9007199254740993} {
		w := httptest.NewRecorder()
		testCookies(false).SetGroup(w, id)

		got, ok := testCookies(false).GroupID(
			requestWithCookie(t, responseCookie(t, w, auth.GroupCookie)))
		require.True(t, ok)
		assert.Equal(t, id, got)
	}
}

func TestGroupIDWithoutCookie(t *testing.T) {
	t.Parallel()

	_, ok := testCookies(false).GroupID(
		httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	assert.False(t, ok)
}

func TestGroupIDRejectsTamperedSignature(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	testCookies(true).SetGroup(w, 42)
	//nolint:gosec // the handler set these attributes; the test tampers with the value.
	cookie := responseCookie(t, w, auth.GroupCookie)
	cookie.Value = tampered(cookie.Value)

	_, ok := testCookies(true).GroupID(requestWithCookie(t, cookie))
	assert.False(t, ok)
}

func TestGroupIDRejectsASessionValue(t *testing.T) {
	t.Parallel()

	cookie := &http.Cookie{
		Name:     auth.GroupCookie,
		Value:    sessionValue(t, 42, time.Now().Add(time.Hour).Unix()),
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}

	_, ok := testCookies(false).GroupID(requestWithCookie(t, cookie))
	assert.False(t, ok, "a session MAC must not pass as a group MAC")
}

func TestGroupIDRejectsExpiredCookie(t *testing.T) {
	t.Parallel()

	cookie := &http.Cookie{
		Name:     auth.GroupCookie,
		Value:    groupValue(t, 42, time.Now().Add(-time.Minute).Unix()),
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}

	_, ok := testCookies(false).GroupID(requestWithCookie(t, cookie))
	assert.False(t, ok)
}

func TestGroupIDRejectsAnotherSessionKey(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	testCookies(false).SetGroup(w, 42)
	request := requestWithCookie(t, responseCookie(t, w, auth.GroupCookie))

	_, ok := auth.NewCookies("another-session-key-long-enough", false).GroupID(request)
	assert.False(t, ok)
}

func TestGroupIDRejectsMalformedCookie(t *testing.T) {
	t.Parallel()

	signature := signedValue(t, "group:42.4102444800")
	tests := map[string]string{
		"empty":          "",
		"no dots":        "42",
		"two parts":      "42.4102444800",
		"no id":          ".4102444800." + signature,
		"no expiry":      "42.." + signature,
		"non-numeric id": "forty-two.4102444800." + signedValue(t, "group:forty-two.4102444800"),
		"zero id":        "0.4102444800." + signedValue(t, "group:0.4102444800"),
		"negative id":    "-1.4102444800." + signedValue(t, "group:-1.4102444800"),
		"leading zero":   "042.4102444800." + signedValue(t, "group:042.4102444800"),
		"extra dot":      "42.4102444800." + signature + ".extra",
	}

	for name, value := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cookie := &http.Cookie{
				Name:     auth.GroupCookie,
				Value:    value,
				Secure:   true,
				HttpOnly: true,
				SameSite: http.SameSiteLaxMode,
			}
			_, ok := testCookies(false).GroupID(requestWithCookie(t, cookie))
			assert.False(t, ok)
		})
	}
}

// groupValue builds a pc_group cookie value for id and expiry.
func groupValue(t *testing.T, id, expiry int64) string {
	t.Helper()

	payload := strconv.FormatInt(id, 10) + "." + strconv.FormatInt(expiry, 10)
	return payload + "." + signedValue(t, "group:"+payload)
}

// splitCookie splits value on dots and fails the test when the part count is
// wrong.
func splitCookie(t *testing.T, value string, want int) []string {
	t.Helper()

	parts := splitDots(value)
	require.Len(t, parts, want, "cookie value %q", value)
	return parts
}

// splitDots splits on dots without importing strings in every test.
func splitDots(value string) []string {
	var parts []string
	start := 0
	for i := range len(value) {
		if value[i] == '.' {
			parts = append(parts, value[start:i])
			start = i + 1
		}
	}
	return append(parts, value[start:])
}
