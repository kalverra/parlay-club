package nfelo

import "strings"

// teamCodes maps the nfelo abbreviations that differ from ESPN's. Every other
// code nfelo publishes already matches ESPN. The first column of the CSV is
// pandas's nameless row index, so it carries no team.
var teamCodes = map[string]string{
	"OAK": "LV",
	"WAS": "WSH",
}

// mapTeam converts an nfelo team code to ESPN's abbreviation. An unknown code
// passes through unchanged so the caller can log an unmatched row.
func mapTeam(code string) string {
	code = strings.ToUpper(strings.TrimSpace(code))
	if mapped, ok := teamCodes[code]; ok {
		return mapped
	}
	return code
}
