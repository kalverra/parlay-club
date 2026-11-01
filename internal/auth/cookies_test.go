package auth_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/auth"
)

const (
	testSessionKey = "test-session-key-long-enough-for-anything"
	testPasscode   = "hut-hut"
)

// testCookies builds a signer over the test key and passcode.
func testCookies(secure bool) *auth.Cookies {
	return auth.NewCookies(testSessionKey, testPasscode, secure)
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

func TestSetPasscodeCookie(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		secure bool
	}{
		"dev":  {secure: false},
		"prod": {secure: true},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			w := httptest.NewRecorder()
			testCookies(test.secure).SetPasscode(w)

			cookie := responseCookie(t, w, auth.PasscodeCookie)
			assert.Equal(t, signedValue(t, testPasscode), cookie.Value)
			assert.Equal(t, "/", cookie.Path)
			assert.Equal(t, 365*24*60*60, cookie.MaxAge)
			assert.True(t, cookie.HttpOnly)
			assert.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
			assert.Equal(t, test.secure, cookie.Secure)
		})
	}
}

func TestHasPasscode(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	testCookies(false).SetPasscode(w)

	assert.True(t, testCookies(false).HasPasscode(requestWithCookie(t, responseCookie(t, w, auth.PasscodeCookie))))
}

func TestHasPasscodeWithoutCookie(t *testing.T) {
	t.Parallel()

	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	assert.False(t, testCookies(false).HasPasscode(request))
}

func TestHasPasscodeRejectsTamperedValue(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	testCookies(true).SetPasscode(w)
	//nolint:gosec // the handler set these attributes; the test tampers with the value.
	cookie := responseCookie(t, w, auth.PasscodeCookie)
	cookie.Value = tampered(cookie.Value)

	assert.False(t, testCookies(true).HasPasscode(requestWithCookie(t, cookie)))
}

func TestHasPasscodeRejectsAnotherPasscode(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	testCookies(false).SetPasscode(w)
	request := requestWithCookie(t, responseCookie(t, w, auth.PasscodeCookie))

	// A new passcode issues different MACs, which logs every browser out.
	assert.False(t, auth.NewCookies(testSessionKey, "new-passcode", false).HasPasscode(request))
}

func TestHasPasscodeRejectsAnotherSessionKey(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	testCookies(false).SetPasscode(w)
	request := requestWithCookie(t, responseCookie(t, w, auth.PasscodeCookie))

	assert.False(t, auth.NewCookies("another-session-key-long-enough", testPasscode, false).HasPasscode(request))
}

func TestPasscodeMatches(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		input string
		want  bool
	}{
		"exact":     {input: testPasscode, want: true},
		"wrong":     {input: "hut-hutt", want: false},
		"prefix":    {input: testPasscode[:len(testPasscode)-1], want: false},
		"empty":     {input: "", want: false},
		"padded":    {input: " " + testPasscode, want: false},
		"uppercase": {input: "HUT-HUT", want: false},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.want, testCookies(false).PasscodeMatches(test.input))
		})
	}
}

func TestSetUserCookie(t *testing.T) {
	t.Parallel()

	for _, id := range []int64{1, 42, 9007199254740993} {
		w := httptest.NewRecorder()
		testCookies(true).SetUser(w, id)

		cookie := responseCookie(t, w, auth.UserCookie)
		assert.Equal(t, "/", cookie.Path)
		assert.Equal(t, 365*24*60*60, cookie.MaxAge)
		assert.True(t, cookie.HttpOnly)
		assert.True(t, cookie.Secure)
		assert.Equal(t, http.SameSiteLaxMode, cookie.SameSite)

		idText := strconv.FormatInt(id, 10)
		assert.Equal(t, idText+"."+signedValue(t, "user:"+idText), cookie.Value)
	}
}

func TestUserID(t *testing.T) {
	t.Parallel()

	for _, id := range []int64{1, 42, 9007199254740993} {
		w := httptest.NewRecorder()
		testCookies(false).SetUser(w, id)

		got, ok := testCookies(false).UserID(requestWithCookie(t, responseCookie(t, w, auth.UserCookie)))
		require.True(t, ok)
		assert.Equal(t, id, got)
	}
}

func TestUserIDWithoutCookie(t *testing.T) {
	t.Parallel()

	_, ok := testCookies(false).UserID(httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	assert.False(t, ok)
}

func TestUserIDRejectsTamperedSignature(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	testCookies(true).SetUser(w, 42)
	//nolint:gosec // the handler set these attributes; the test tampers with the value.
	cookie := responseCookie(t, w, auth.UserCookie)
	cookie.Value = "42." + tampered(cookie.Value[len("42."):])

	_, ok := testCookies(true).UserID(requestWithCookie(t, cookie))
	assert.False(t, ok)
}

func TestUserIDRejectsTamperedID(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	testCookies(true).SetUser(w, 42)
	//nolint:gosec // the handler set these attributes; the test tampers with the value.
	cookie := responseCookie(t, w, auth.UserCookie)
	cookie.Value = "43." + cookie.Value[len("42."):]

	_, ok := testCookies(true).UserID(requestWithCookie(t, cookie))
	assert.False(t, ok)
}

func TestUserIDRejectsMalformedCookie(t *testing.T) {
	t.Parallel()

	signature := signedValue(t, "user:42")
	tests := map[string]string{
		"empty":          "",
		"no dot":         "42",
		"no id":          ".signature",
		"no signature":   "42.",
		"non-numeric id": "forty-two." + signature,
		"zero id":        "0." + signedValue(t, "user:0"),
		"negative id":    "-1." + signedValue(t, "user:-1"),
		"leading zero":   "042." + signedValue(t, "user:042"),
		"padded id":      " 42." + signedValue(t, "user: 42"),
		"extra dot":      "42." + signature + ".extra",
	}

	for name, value := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cookie := &http.Cookie{
				Name:     auth.UserCookie,
				Value:    value,
				Secure:   true,
				HttpOnly: true,
				SameSite: http.SameSiteLaxMode,
			}
			_, ok := testCookies(true).UserID(requestWithCookie(t, cookie))
			assert.False(t, ok)
		})
	}
}

func TestUserIDRejectsAnotherSessionKey(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	testCookies(false).SetUser(w, 42)
	request := requestWithCookie(t, responseCookie(t, w, auth.UserCookie))

	_, ok := auth.NewCookies("another-session-key-long-enough", testPasscode, false).UserID(request)
	assert.False(t, ok)
}
