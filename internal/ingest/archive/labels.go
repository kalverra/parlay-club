package archive

import (
	"context"
	"strings"
)

// Source names stored in api_responses.source.
const (
	// SourceESPN covers the ESPN scoreboard.
	SourceESPN = "espn"
	// SourceWeather covers Open-Meteo forecasts.
	SourceWeather = "weather"
	// SourceOdds covers Parlay API and The Odds API event lists, lines, and props.
	SourceOdds = "odds"
)

// labelKey is the private context key for a response's archive label.
type labelKey struct{}

// WithKey labels every request made with ctx as one source key. The key is the
// same string sync_state.source uses, such as "odds:props:401".
func WithKey(ctx context.Context, key string) context.Context {
	return context.WithValue(ctx, labelKey{}, key)
}

// KeyFrom returns the archive label set by WithKey. A missing or empty label
// reports false, so the transport can fall back to host + path.
func KeyFrom(ctx context.Context) (string, bool) {
	key, ok := ctx.Value(labelKey{}).(string)
	return key, ok && key != ""
}

// sourceForKey takes the source from the key's prefix, or from the host when
// the key has no prefix. The key is host + path for an unlabelled request.
func sourceForKey(key, host string) string {
	if prefix, _, found := strings.Cut(key, ":"); found && prefix != "" {
		return strings.ToLower(prefix)
	}
	return sourceForHost(host)
}

// sourceForHost maps a known API host to its source name. An unknown host is
// its own source, so the row still says where it came from.
func sourceForHost(host string) string {
	host = strings.ToLower(host)
	switch {
	case strings.Contains(host, "espn.com"):
		return SourceESPN
	case strings.Contains(host, "open-meteo.com"):
		return SourceWeather
	case strings.Contains(host, "parlay-api.com"), strings.Contains(host, "the-odds-api.com"):
		return SourceOdds
	default:
		return host
	}
}
