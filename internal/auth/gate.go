package auth

import (
	"strings"

	"github.com/kalverra/parlay-club/internal/config"
)

// Gate is the email allow list. It is the only place that knows the list, so a
// later change can replace it without touching sign-in.
type Gate struct {
	entries map[string]gateEntry
}

// gateEntry is one allowed email and the history-import name it may link to.
type gateEntry struct {
	linkName string
}

// NewGate builds a gate from parsed ALLOWED_EMAILS entries. Emails compare
// without case. An empty list allows everyone, which only happens in dev.
func NewGate(allowed []config.AllowedEmail) Gate {
	entries := make(map[string]gateEntry, len(allowed))
	for _, entry := range allowed {
		email := normalizeEmail(entry.Email)
		if email == "" {
			continue
		}
		entries[email] = gateEntry{linkName: strings.TrimSpace(entry.Name)}
	}
	return Gate{entries: entries}
}

// Allows reports whether email may sign in.
func (g Gate) Allows(email string) bool {
	if len(g.entries) == 0 {
		return true
	}
	_, ok := g.entries[normalizeEmail(email)]
	return ok
}

// LinkName returns the history-import name this email may claim, or "" when the
// entry carries no name.
func (g Gate) LinkName(email string) string {
	return g.entries[normalizeEmail(email)].linkName
}

// normalizeEmail trims and lowercases an address for comparison.
func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
