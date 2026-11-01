// Package auth signs and verifies the passcode and user cookies.
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
	PasscodeCookie = "pc_auth"
	UserCookie     = "pc_user"
)

// cookieMaxAge keeps a browser signed in for a year.
const cookieMaxAge = 365 * 24 * time.Hour

// Cookies signs and verifies the two cookies that stand in for accounts.
//
// The passcode cookie holds an HMAC of the passcode, so changing the passcode
// invalidates every cookie already issued. The user cookie holds the user ID
// plus an HMAC of "user:" and that ID. Both MACs use the same session key and
// are compared in constant time.
//
// A Cookies is safe for concurrent use once built.
type Cookies struct {
	sessionKey []byte
	passcode   string
	secure     bool
}

// NewCookies returns a signer for sessionKey that accepts passcode. When secure
// is true, cookies carry the Secure attribute, as they must over HTTPS in prod.
func NewCookies(sessionKey, passcode string, secure bool) *Cookies {
	return &Cookies{sessionKey: []byte(sessionKey), passcode: passcode, secure: secure}
}

// SetPasscode writes the cookie that proves this browser knows the group
// passcode.
func (c *Cookies) SetPasscode(w http.ResponseWriter) {
	http.SetCookie(w, c.cookie(PasscodeCookie, c.mac(c.passcode)))
}

// HasPasscode reports whether r carries a valid passcode cookie. A missing,
// tampered, or stale cookie (one signed for another passcode or key) is false.
func (c *Cookies) HasPasscode(r *http.Request) bool {
	cookie, err := r.Cookie(PasscodeCookie)
	if err != nil {
		return false
	}
	return hmac.Equal([]byte(cookie.Value), []byte(c.mac(c.passcode)))
}

// SetUser writes the cookie that names the chosen user.
func (c *Cookies) SetUser(w http.ResponseWriter, id int64) {
	idText := strconv.FormatInt(id, 10)
	http.SetCookie(w, c.cookie(UserCookie, idText+"."+c.mac("user:"+idText)))
}

// UserID returns the user ID carried by a valid user cookie. A missing,
// tampered, or malformed cookie reports false.
func (c *Cookies) UserID(r *http.Request) (int64, bool) {
	cookie, err := r.Cookie(UserCookie)
	if err != nil {
		return 0, false
	}

	idText, signature, found := strings.Cut(cookie.Value, ".")
	if !found {
		return 0, false
	}
	if !hmac.Equal([]byte(signature), []byte(c.mac("user:"+idText))) {
		return 0, false
	}

	id, err := strconv.ParseInt(idText, 10, 64)
	if err != nil || id < 1 || strconv.FormatInt(id, 10) != idText {
		return 0, false
	}
	return id, true
}

// cookie builds the shared attributes of both cookies.
//
//nolint:gosec // Secure is on in prod; dev serves plain HTTP, so it follows the flag.
func (c *Cookies) cookie(name, value string) *http.Cookie {
	return &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		MaxAge:   int(cookieMaxAge.Seconds()),
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
