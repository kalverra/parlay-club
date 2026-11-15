package nflverse

import "strings"

// teamCodes maps nflverse abbreviations that differ from ESPN's, including
// historical franchise codes that appear in players.csv.
var teamCodes = map[string]string{
	"JAC": "JAX",
	"LA":  "LAR",
	"OAK": "LV",
	"SD":  "LAC",
	"STL": "LAR",
	"WAS": "WSH",
}

// mapTeam converts an nflverse team code to ESPN's abbreviation. An empty or
// unknown code passes through unchanged (uppercased) so callers can see it.
func mapTeam(code string) string {
	code = strings.ToUpper(strings.TrimSpace(code))
	if mapped, ok := teamCodes[code]; ok {
		return mapped
	}
	return code
}
