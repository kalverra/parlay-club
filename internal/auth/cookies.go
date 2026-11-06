// Package auth signs and verifies sign-in cookies and serves the sign-in flow.
package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Cookie names the browser carries.
const (
	SessionCookie = "pc_session"
	OAuthCookie   = "pc_oauth"
)

const (
	// sessionMaxAge keeps a browser signed in for thirty days.
	sessionMaxAge = 30 * 24 * time.Hour
	// oauthMaxAge is how long a browser may take to finish the Google round trip.
	oauthMaxAge = 10 * time.Minute
	// oauthPath scopes the round-trip cookie to the auth routes.
	oauthPath = "/auth"
)

// OAuthState is the signed state a browser carries through the Google round
// trip. The verifier proves this browser started the flow; the state and nonce
// stop a callback from being replayed or forged.
type OAuthState struct {
	State    string
	Nonce    string
	Verifier string
}

// Cookies signs and verifies the session and OAuth cookies.
//
// The session cookie holds the user ID, an expiry, and an HMAC over
// "session:{userID}.{expiry}". The OAuth cookie holds state, nonce, the PKCE
// verifier, and an issue time, with an HMAC over "oauth:{payload}". Both MACs
// use the session key and are compared in constant time.
//
// A Cookies is safe for concurrent use once built.
type Cookies struct {
	sessionKey []byte
	secure     bool
}

// NewCookies returns a signer for sessionKey. When secure is true, cookies
// carry the Secure attribute, as they must over HTTPS in prod.
func NewCookies(sessionKey string, secure bool) *Cookies {
	return &Cookies{sessionKey: []byte(sessionKey), secure: secure}
}

// SetSession writes the cookie that signs this browser in as id.
func (c *Cookies) SetSession(w http.ResponseWriter, id int64) {
	idText := strconv.FormatInt(id, 10)
	expiryText := strconv.FormatInt(time.Now().Add(sessionMaxAge).Unix(), 10)
	payload := idText + "." + expiryText
	http.SetCookie(w, c.cookie(SessionCookie, payload+"."+c.mac("session:"+payload), sessionMaxAge, "/"))
}

// SessionID returns the user ID carried by a valid session cookie. A missing,
// tampered, malformed, or expired cookie reports false.
func (c *Cookies) SessionID(r *http.Request) (int64, bool) {
	cookie, err := r.Cookie(SessionCookie)
	if err != nil {
		return 0, false
	}

	parts := strings.Split(cookie.Value, ".")
	if len(parts) != 3 {
		return 0, false
	}
	idText, expiryText, signature := parts[0], parts[1], parts[2]
	payload := idText + "." + expiryText
	if !hmac.Equal([]byte(signature), []byte(c.mac("session:"+payload))) {
		return 0, false
	}

	id, err := strconv.ParseInt(idText, 10, 64)
	if err != nil || id < 1 || strconv.FormatInt(id, 10) != idText {
		return 0, false
	}
	expiry, err := strconv.ParseInt(expiryText, 10, 64)
	if err != nil || time.Now().Unix() >= expiry {
		return 0, false
	}
	return id, true
}

// ClearSession expires the session cookie.
func (c *Cookies) ClearSession(w http.ResponseWriter) {
	http.SetCookie(w, c.expiredCookie(SessionCookie, "/"))
}

// SetOAuth writes the signed cookie that carries one Google round trip.
func (c *Cookies) SetOAuth(w http.ResponseWriter, state OAuthState) {
	issued := strconv.FormatInt(time.Now().Unix(), 10)
	payload := state.State + "." + state.Nonce + "." + state.Verifier + "." + issued
	http.SetCookie(w, c.cookie(OAuthCookie, payload+"."+c.mac("oauth:"+payload), oauthMaxAge, oauthPath))
}

// OAuthState returns the round trip a browser carried. A missing, tampered,
// malformed, or expired cookie reports false.
func (c *Cookies) OAuthState(r *http.Request) (OAuthState, bool) {
	cookie, err := r.Cookie(OAuthCookie)
	if err != nil {
		return OAuthState{}, false
	}

	parts := strings.Split(cookie.Value, ".")
	if len(parts) != 5 {
		return OAuthState{}, false
	}
	payload := strings.Join(parts[:4], ".")
	if !hmac.Equal([]byte(parts[4]), []byte(c.mac("oauth:"+payload))) {
		return OAuthState{}, false
	}

	issued, err := strconv.ParseInt(parts[3], 10, 64)
	if err != nil || time.Now().Unix()-issued > int64(oauthMaxAge.Seconds()) {
		return OAuthState{}, false
	}
	if parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return OAuthState{}, false
	}
	return OAuthState{State: parts[0], Nonce: parts[1], Verifier: parts[2]}, true
}

// ClearOAuth expires the round-trip cookie.
func (c *Cookies) ClearOAuth(w http.ResponseWriter) {
	http.SetCookie(w, c.expiredCookie(OAuthCookie, oauthPath))
}

// cookie builds a cookie with the shared attributes.
//
//nolint:gosec // Secure is on in prod; dev serves plain HTTP, so it follows the flag.
func (c *Cookies) cookie(name, value string, maxAge time.Duration, path string) *http.Cookie {
	return &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     path,
		MaxAge:   int(maxAge.Seconds()),
		HttpOnly: true,
		Secure:   c.secure,
		SameSite: http.SameSiteLaxMode,
	}
}

// expiredCookie clears a cookie at path.
func (c *Cookies) expiredCookie(name, path string) *http.Cookie {
	//nolint:gosec // clearing a cookie; Secure follows the deployment as above.
	return &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     path,
		MaxAge:   -1,
		Expires:  time.Unix(1, 0),
		HttpOnly: true,
		Secure:   c.secure,
		SameSite: http.SameSiteLaxMode,
	}
}

// mac returns the lowercase hex HMAC-SHA256 of payload under the session key.
func (c *Cookies) mac(payload string) string {
	mac := hmac.New(sha256.New, c.sessionKey)
	_, _ = mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}
