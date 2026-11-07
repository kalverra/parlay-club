package archive

import (
	"net/url"
	"strings"
)

// redactURL returns the URL as text with The Odds API's apiKey value replaced
// by REDACTED. The other query parameters stay in their original order, because
// markets, regions, and week are what make a response meaningful. Any other
// appearance of the same value is replaced too, so a redirect that copies the
// key elsewhere cannot leak it.
func redactURL(u *url.URL) string {
	if u == nil {
		return ""
	}

	secret := u.Query().Get("apiKey")
	stored := *u
	if secret != "" {
		stored.RawQuery = redactQuery(u.RawQuery)
	}

	text := stored.String()
	if secret == "" {
		return text
	}
	for _, value := range []string{secret, url.QueryEscape(secret), url.PathEscape(secret)} {
		text = strings.ReplaceAll(text, value, redacted)
	}
	return text
}

// redactQuery replaces each apiKey value while leaving every other parameter
// byte-for-byte as it was.
func redactQuery(raw string) string {
	parts := strings.Split(raw, "&")
	for i, part := range parts {
		name, _, found := strings.Cut(part, "=")
		if found && name == "apiKey" {
			parts[i] = name + "=" + redacted
		}
	}
	return strings.Join(parts, "&")
}
