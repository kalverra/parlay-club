package odds

import "time"

// MatchWindow is how far an event's commence time may sit from a game's
// kickoff and still count as the same game. Kickoffs move by minutes, and the
// two sources round differently, so the window is generous without reaching
// the next week's rematch.
const MatchWindow = 12 * time.Hour

// MatchGame is the ESPN side of a match: the game's ID, its two team display
// names, and its kickoff in UTC. A caller adapts an ESPN or stored game to this
// shape so the odds package does not depend on either.
type MatchGame struct {
	ID       string
	HomeName string
	AwayName string
	Kickoff  time.Time
}

// MatchEvents pairs each event with one game. An event matches a game when its
// home and away team names equal the game's display names and its commence
// time is within MatchWindow of kickoff. The result maps game ID to event ID;
// unmatched holds the events that found no game, in input order. Each event
// and each game is used at most once, so input order decides the rare tie.
func MatchEvents(events []Event, games []MatchGame) (map[string]string, []Event) {
	matches := make(map[string]string, len(games))
	usedGame := make(map[string]bool, len(games))
	unmatched := make([]Event, 0, len(events))

	for _, event := range events {
		gameID, ok := findGame(event, games, usedGame)
		if !ok {
			unmatched = append(unmatched, event)
			continue
		}
		matches[gameID] = event.ID
		usedGame[gameID] = true
	}
	return matches, unmatched
}

// findGame returns the first unused game that event matches.
func findGame(event Event, games []MatchGame, used map[string]bool) (string, bool) {
	for _, game := range games {
		if used[game.ID] {
			continue
		}
		if event.HomeTeam != game.HomeName || event.AwayTeam != game.AwayName {
			continue
		}
		if withinMatchWindow(event.CommenceTime, game.Kickoff) {
			return game.ID, true
		}
	}
	return "", false
}

// withinMatchWindow reports whether the two times are no more than MatchWindow
// apart, in either direction.
func withinMatchWindow(commence, kickoff time.Time) bool {
	delta := commence.Sub(kickoff)
	if delta < 0 {
		delta = -delta
	}
	return delta <= MatchWindow
}
