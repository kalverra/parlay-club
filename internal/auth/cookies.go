// Package auth signs and verifies sign-in cookies and serves the sign-in flow.
package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
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
	GroupCookie   = "pc_group"
)

const (
	// sessionMaxAge keeps a browser signed in for thirty days.
	sessionMaxAge = 30 * 24 * time.Hour
	// oauthMaxAge is how long a browser may take to finish the Google round trip.
	oauthMaxAge = 10 * time.Minute
	// oauthPath lets every route read the round-trip cookie, because a join
	// link sets the return path before sign-in and the dev form needs it too.
	oauthPath = "/"
)

// OAuthState is the signed state a browser carries through the Google round
// trip. The verifier proves this browser started the flow; the state and nonce
// stop a callback from being replayed or forged. ReturnTo is the join path to
// land on after sign-in, if any.
type OAuthState struct {
	State    string
	Nonce    string
	Verifier string
	ReturnTo string
}

// Cookies signs and verifies the session, OAuth, and group cookies.
//
// The session cookie holds the user ID, an expiry, and an HMAC over
// "session:{userID}.{expiry}". The OAuth cookie holds state, nonce, the PKCE
// verifier, an issue time, and an optional return path, with an HMAC over
// "oauth:{payload}". The group cookie holds the active group ID and an expiry,
// with an HMAC over "group:{groupID}.{expiry}". Every MAC uses the session key
// and is compared in constant time.
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
	return c.signedID(r, SessionCookie, "session")
}

// SetGroup writes the cookie that names id as the active group.
func (c *Cookies) SetGroup(w http.ResponseWriter, id int64) {
	idText := strconv.FormatInt(id, 10)
	expiryText := strconv.FormatInt(time.Now().Add(sessionMaxAge).Unix(), 10)
	payload := idText + "." + expiryText
	http.SetCookie(w, c.cookie(GroupCookie, payload+"."+c.mac("group:"+payload), sessionMaxAge, "/"))
}

// GroupID returns the group ID carried by a valid group cookie. A missing,
// tampered, malformed, or expired cookie reports false, so the caller can fall
// back to a group the person belongs to.
func (c *Cookies) GroupID(r *http.Request) (int64, bool) {
	return c.signedID(r, GroupCookie, "group")
}

// ClearGroup expires the active-group cookie.
func (c *Cookies) ClearGroup(w http.ResponseWriter) {
	http.SetCookie(w, c.expiredCookie(GroupCookie, "/"))
}

// signedID reads the ID a signed cookie carries. scope separates the MACs of
// cookies that share this format, so a session value cannot pass as a group.
func (c *Cookies) signedID(r *http.Request, name, scope string) (int64, bool) {
	cookie, err := r.Cookie(name)
	if err != nil {
		return 0, false
	}

	parts := strings.Split(cookie.Value, ".")
	if len(parts) != 3 {
		return 0, false
	}
	idText, expiryText, signature := parts[0], parts[1], parts[2]
	payload := idText + "." + expiryText
	if !hmac.Equal([]byte(signature), []byte(c.mac(scope+":"+payload))) {
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
	payload := strings.Join([]string{
		state.State,
		state.Nonce,
		state.Verifier,
		issued,
		encodeReturn(state.ReturnTo),
	}, ".")
	http.SetCookie(w, c.cookie(OAuthCookie, payload+"."+c.mac("oauth:"+payload), oauthMaxAge, oauthPath))
}

// SetReturnTo writes a round-trip cookie that only remembers where to land
// after sign-in. The next sign-in replaces it with a real round trip.
func (c *Cookies) SetReturnTo(w http.ResponseWriter, path string) {
	c.SetOAuth(w, OAuthState{ReturnTo: path})
}

// OAuthState returns the round trip a browser carried. A missing, tampered,
// malformed, expired, or return-only cookie reports false.
func (c *Cookies) OAuthState(r *http.Request) (OAuthState, bool) {
	parts, ok := c.oauthParts(r)
	if !ok {
		return OAuthState{}, false
	}
	if parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return OAuthState{}, false
	}
	return OAuthState{
		State:    parts[0],
		Nonce:    parts[1],
		Verifier: parts[2],
		ReturnTo: decodeReturn(parts[4]),
	}, true
}

// ReturnTo returns the join path a round-trip cookie carries, whether or not
// the cookie carries a usable round trip. A cookie with no join path reports an
// empty string.
func (c *Cookies) ReturnTo(r *http.Request) string {
	parts, ok := c.oauthParts(r)
	if !ok {
		return ""
	}
	return decodeReturn(parts[4])
}

// oauthParts verifies the OAuth cookie's signature and age and returns its five
// payload fields: state, nonce, verifier, issued time, and encoded return path.
func (c *Cookies) oauthParts(r *http.Request) ([]string, bool) {
	cookie, err := r.Cookie(OAuthCookie)
	if err != nil {
		return nil, false
	}

	parts := strings.Split(cookie.Value, ".")
	if len(parts) != 6 {
		return nil, false
	}
	payload := strings.Join(parts[:5], ".")
	if !hmac.Equal([]byte(parts[5]), []byte(c.mac("oauth:"+payload))) {
		return nil, false
	}

	issued, err := strconv.ParseInt(parts[3], 10, 64)
	if err != nil || time.Now().Unix()-issued > int64(oauthMaxAge.Seconds()) {
		return nil, false
	}
	return parts[:5], true
}

// encodeReturn makes a return path safe inside a dot-separated cookie value.
func encodeReturn(path string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(path))
}

// decodeReturn reverses encodeReturn. A malformed value reports an empty path,
// which the caller treats as no return target.
func decodeReturn(encoded string) string {
	if encoded == "" {
		return ""
	}
	path, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return ""
	}
	return string(path)
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
