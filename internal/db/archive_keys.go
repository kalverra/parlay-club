package db

// Canonical archive and sync_state keys shared by refresh and replay.
const (
	// ArchiveOddsLinesKey is the archive and sync_state key for the week's game lines.
	ArchiveOddsLinesKey = "odds:lines"
)

// ArchiveOddsPropsKey returns the archive and sync_state key for one game's props.
func ArchiveOddsPropsKey(gameID string) string {
	return "odds:props:" + gameID
}

// ArchiveRosterKey returns the archive and sync_state key for one team's roster.
func ArchiveRosterKey(teamID string) string {
	return "espn:roster:" + teamID
}

// ArchiveGameLogKey returns the archive and sync_state key for one athlete's game log.
func ArchiveGameLogKey(athleteID string) string {
	return "espn:gamelog:" + athleteID
}

// ArchiveSummaryKey returns the archive and sync_state key for one game's summary.
func ArchiveSummaryKey(gameID string) string {
	return "espn:summary:" + gameID
}
