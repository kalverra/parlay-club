// Package seed imports the group's past slips from seed/history.yaml. The file
// is read on every boot, and a slip already stored for the same week and name
// is skipped, so a restart changes nothing. Every slip is created through the
// normal slip rules, then each graded leg settles the slip result.
package seed

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/parlay"
	"github.com/kalverra/parlay-club/internal/slips"
)

// DefaultPath is the history file's path, relative to the working directory.
const DefaultPath = "seed/history.yaml"

// maxLegsPerMember matches the create-slip form's range.
const maxLegsPerMember = 5

// Report counts what one Load changed.
type Report struct {
	Slips   int // slips created
	Skipped int // slips already present
	Users   int // users created
}

// historyFile is the decoded YAML. A file with no slips is valid and changes
// nothing.
type historyFile struct {
	Slips []fileSlip `yaml:"slips"`
}

// fileSlip is one historical parlay.
type fileSlip struct {
	Season        int       `yaml:"season"`
	SeasonType    int       `yaml:"season_type"`
	Week          int       `yaml:"week"`
	Name          string    `yaml:"name"`
	LegsPerMember int       `yaml:"legs_per_member"`
	Legs          []fileLeg `yaml:"legs"`
}

// fileLeg is one historical pick. Historic picks are usually OTHER with a
// typed selection, so the pick fields other than selection are optional.
type fileLeg struct {
	User      string   `yaml:"user"`
	Selection string   `yaml:"selection"`
	Market    string   `yaml:"market"`
	Team      string   `yaml:"team"`
	Player    string   `yaml:"player"`
	PlayerID  string   `yaml:"player_id"`
	Side      string   `yaml:"side"`
	Line      *float64 `yaml:"line"`
	GameID    string   `yaml:"game_id"`
	Odds      int      `yaml:"odds"`
	Result    string   `yaml:"result"`
}

// Load reads the history file at path and imports its slips into groupID. A
// missing slip is created with its week, people, and legs; an existing slip is
// skipped before any legs are added. A bad file imports only the slips before
// the problem and reports it.
func Load(ctx context.Context, store *db.Store, groupID int64, path string) (Report, error) {
	history, err := read(path)
	if err != nil {
		return Report{}, err
	}
	if len(history.Slips) == 0 {
		return Report{}, nil
	}
	if err := validate(history); err != nil {
		return Report{}, fmt.Errorf("%s: %w", path, err)
	}

	var report Report
	for _, slip := range history.Slips {
		name := strings.TrimSpace(slip.Name)
		weekID := db.WeekID(slip.Season, slip.SeasonType, slip.Week)
		exists, err := slipExists(ctx, store, groupID, weekID, name)
		if err != nil {
			return report, fmt.Errorf("%s: %w", path, err)
		}
		if exists {
			report.Skipped++
			continue
		}

		users, created, err := resolveUsers(ctx, store, slip.Legs)
		if err != nil {
			return report, fmt.Errorf("%s: slip %q: %w", path, name, err)
		}
		report.Users += created
		if err := loadSlip(ctx, store, groupID, slip, users); err != nil {
			return report, fmt.Errorf("%s: slip %q: %w", path, name, err)
		}
		report.Slips++
	}
	return report, nil
}

// read decodes path. A comments-only file is the same as an empty one.
func read(path string) (historyFile, error) {
	raw, err := os.ReadFile(path) //nolint:gosec // The path is the app's own history file.
	if err != nil {
		return historyFile{}, fmt.Errorf("read history file: %w", err)
	}

	decoder := yaml.NewDecoder(bytes.NewReader(raw))
	decoder.KnownFields(true)
	var history historyFile
	err = decoder.Decode(&history)
	if errors.Is(err, io.EOF) {
		return historyFile{}, nil
	}
	if err != nil {
		return historyFile{}, fmt.Errorf("parse history file: %w", err)
	}
	return history, nil
}

// validate reports every problem in the file at once, so one boot shows the
// whole list. It checks the data, not the database.
func validate(history historyFile) error {
	var problems []error
	for i, slip := range history.Slips {
		label := fmt.Sprintf("slip %d", i+1)
		if name := strings.TrimSpace(slip.Name); name != "" {
			label = fmt.Sprintf("slip %q", name)
		}
		if slip.Season <= 0 {
			problems = append(problems, fmt.Errorf("%s: season is required", label))
		}
		if slip.SeasonType < 1 || slip.SeasonType > 3 {
			problems = append(problems, fmt.Errorf("%s: season_type must be 1, 2, or 3", label))
		}
		if slip.Week < 1 || slip.Week > 99 {
			problems = append(problems, fmt.Errorf("%s: week must be from 1 to 99", label))
		}
		if strings.TrimSpace(slip.Name) == "" {
			problems = append(problems, fmt.Errorf("%s: name is required", label))
		}
		limit := legsPerMember(slip)
		if limit < 1 || limit > maxLegsPerMember {
			problems = append(problems, fmt.Errorf(
				"%s: legs_per_member must be from 1 to %d", label, maxLegsPerMember))
		}
		if len(slip.Legs) == 0 {
			problems = append(problems, fmt.Errorf("%s: add at least one leg", label))
		}

		counts := make(map[string]int, len(slip.Legs))
		for j, leg := range slip.Legs {
			legLabel := fmt.Sprintf("%s leg %d", label, j+1)
			user := strings.TrimSpace(leg.User)
			if user == "" {
				problems = append(problems, fmt.Errorf("%s: user is required", legLabel))
			}
			if strings.TrimSpace(leg.Selection) == "" {
				problems = append(problems, fmt.Errorf("%s: selection is required", legLabel))
			}
			if !knownMarket(leg) {
				problems = append(problems, fmt.Errorf("%s: %q is not a market the app knows", legLabel, leg.Market))
			}
			if result := legResult(leg); !result.IsLegResult() {
				problems = append(problems, fmt.Errorf("%s: %q is not a leg result", legLabel, leg.Result))
			}
			if user != "" {
				counts[userKey(user)]++
			}
		}
		for user, count := range counts {
			if count > limit {
				problems = append(problems, fmt.Errorf(
					"%s: %s has %d legs, more than legs_per_member %d", label, user, count, limit))
			}
		}
	}
	return errors.Join(problems...)
}

// slipExists reports whether the group already has a slip with this name in the
// week.
func slipExists(ctx context.Context, store *db.Store, groupID, weekID int64, name string) (bool, error) {
	existing, err := store.SlipsForWeek(ctx, groupID, weekID)
	if err != nil {
		return false, err
	}
	for _, slip := range existing {
		if strings.EqualFold(slip.Name, name) {
			return true, nil
		}
	}
	return false, nil
}

// resolveUsers finds or creates the people a slip's legs name. It returns the
// users keyed by userKey and how many it created.
func resolveUsers(
	ctx context.Context,
	store *db.Store,
	legs []fileLeg,
) (map[string]db.User, int, error) {
	users := make(map[string]db.User, len(legs))
	created := 0
	for _, leg := range legs {
		key := userKey(leg.User)
		if _, ok := users[key]; ok {
			continue
		}
		user, wasCreated, err := resolveUser(ctx, store, strings.TrimSpace(leg.User))
		if err != nil {
			return nil, created, err
		}
		users[key] = user
		if wasCreated {
			created++
		}
	}
	return users, created, nil
}

// resolveUser is the one place a history name becomes a person. Today it
// matches a name, then creates an active user. Sign-in and groups can change
// this function to match an email or alias without touching the file.
func resolveUser(ctx context.Context, store *db.Store, name string) (db.User, bool, error) {
	user, err := store.UserByName(ctx, name)
	switch {
	case err == nil:
		return user, false, nil
	case errors.Is(err, db.ErrUserNotFound):
		created, err := store.CreateUserWithIdentity(ctx, name, "", "", "")
		if err != nil {
			return db.User{}, false, err
		}
		return created, true, nil
	default:
		return db.User{}, false, err
	}
}

// loadSlip creates one slip, its legs, and their results in one transaction, so
// a bad leg leaves no half-built slip behind.
func loadSlip(
	ctx context.Context,
	store *db.Store,
	groupID int64,
	slip fileSlip,
	users map[string]db.User,
) error {
	return store.Tx(ctx, func(tx *db.Store) error {
		week, err := tx.UpsertWeek(ctx, slip.Season, slip.SeasonType, slip.Week)
		if err != nil {
			return err
		}
		created, err := tx.CreateSlip(
			ctx,
			groupID,
			week.ID,
			strings.TrimSpace(slip.Name),
			legsPerMember(slip),
			memberIDs(slip.Legs, users),
		)
		if err != nil {
			return err
		}

		service := slips.New(tx, nil, time.Now)
		for _, leg := range slip.Legs {
			owner := users[userKey(leg.User)]
			added, err := service.AddLeg(ctx, owner.ID, legFrom(created.ID, leg))
			if err != nil {
				return err
			}
			if result := legResult(leg); result != parlay.Pending {
				if _, err := service.SetLegResult(ctx, owner.ID, added.ID, result); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

// memberIDs lists the distinct users on a slip in the order their first leg
// appears.
func memberIDs(legs []fileLeg, users map[string]db.User) []int64 {
	ids := make([]int64, 0, len(legs))
	seen := make(map[int64]bool, len(legs))
	for _, leg := range legs {
		user := users[userKey(leg.User)]
		if seen[user.ID] {
			continue
		}
		seen[user.ID] = true
		ids = append(ids, user.ID)
	}
	return ids
}

// legFrom turns one file leg into the stored leg the rules expect.
func legFrom(slipID int64, leg fileLeg) db.Leg {
	market := marketName(leg)
	stored := db.Leg{
		SlipID:     slipID,
		GameID:     optional(leg.GameID),
		MarketType: market,
		Selection:  strings.TrimSpace(leg.Selection),
		Team:       optional(leg.Team),
		PlayerName: optional(leg.Player),
		PlayerID:   optional(leg.PlayerID),
		Side:       optional(strings.ToUpper(strings.TrimSpace(leg.Side))),
		Line:       leg.Line,
		Odds:       leg.Odds,
	}
	if market == "ANYTIME_TD" && stored.Side == nil {
		stored.Side = new("YES")
	}
	return stored
}

// legsPerMember returns the file's limit, defaulting to the create-slip
// default when the file leaves it out.
func legsPerMember(slip fileSlip) int {
	if slip.LegsPerMember == 0 {
		return slips.DefaultLegsPerMember
	}
	return slip.LegsPerMember
}

// legResult reads a leg's result, treating a missing one as still pending.
func legResult(leg fileLeg) parlay.Result {
	raw := strings.ToUpper(strings.TrimSpace(leg.Result))
	if raw == "" {
		return parlay.Pending
	}
	return parlay.Result(raw)
}

// marketName reads a leg's market, defaulting to OTHER.
func marketName(leg fileLeg) string {
	market := strings.ToUpper(strings.TrimSpace(leg.Market))
	if market == "" {
		return "OTHER"
	}
	return market
}

// knownMarket reports whether the leg's market is one the app supports.
func knownMarket(leg fileLeg) bool {
	switch marketName(leg) {
	case "SPREAD", "ML", "TOTAL", "TEAM_TOTAL", "PASS_YDS", "PASS_TDS",
		"RUSH_YDS", "RUSH_ATT", "REC_YDS", "RECEPTIONS", "ANYTIME_TD", "KICKER_PTS", "OTHER":
		return true
	default:
		return false
	}
}

// userKey identifies a person in the file, ignoring case and surrounding space
// the same way the users table does.
func userKey(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

// optional returns a trimmed value, or nil when it is empty.
func optional(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}
