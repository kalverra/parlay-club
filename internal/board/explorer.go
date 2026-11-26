package board

import (
	"cmp"
	"math"
	"slices"
	"strings"

	"github.com/kalverra/parlay-club/internal/analyzer"
	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/parlay"
)

// ExploreSort orders the research explorer's candidate bets.
type ExploreSort string

// ExploreQuery is one slate-wide research request. Q, GameID, and PlayerID
// narrow the slate to the bets someone already has in mind; Category, Grades,
// Signal, and Window are the open-browse chips. Signals fills in every badge a
// row matches so the signal chips and their counts see more than the single
// badge a row displays.
type ExploreQuery struct {
	Q        string   // case/accent-insensitive search (player, team, matchup, market)
	GameID   string   // optional targeted game filter
	PlayerID string   // optional targeted player filter
	Category Category // lines, team_totals, passing, rushing, receiving, tds, kicking
	Grades   []string // e.g. ["A"], ["A", "B"]
	Signal   string   // smash, volume, role_bump, fits_slip
	Window   Window   // kickoff window
	Sort     ExploreSort
	Limit    int // default 50 for fast phone rendering

	// Signals supplies the nflverse, injury, and open-slip context used for the
	// signal chips. Empty falls back to each row's already-applied Badge.
	Signals BoardSignals
}

// ScoredRow is one candidate bet with its analyzer grade and a concise reason.
type ScoredRow struct {
	Row        Row
	Grade      analyzer.LegGrade
	Reason     string // concise top positive/negative factor
	ImpliedPct int
}

// ExploreResult is one explorer page plus the facet counts and paging state its
// chips need.
type ExploreResult struct {
	Rows           []ScoredRow
	TotalMatched   int
	CategoryCounts map[Category]int
	GradeCounts    map[string]int
	SignalCounts   map[string]int
	HasMore        bool
}

// The ways the research explorer can order its rows.
const (
	ExploreSortGrade   ExploreSort = "grade"   // highest analyzer score, then best price
	ExploreSortMatchup ExploreSort = "matchup" // strongest signal + softest opp def EPA
	ExploreSortOdds    ExploreSort = "odds"    // shortest American odds first
	ExploreSortLine    ExploreSort = "line"    // highest line first
	ExploreSortKickoff ExploreSort = "kickoff" // earliest kickoff first
	ExploreSortPlayer  ExploreSort = "player"  // alphabetical player/team
)

// ExploreSorts lists the explorer's sort chips in display order.
var ExploreSorts = []ExploreSort{
	ExploreSortGrade,
	ExploreSortMatchup,
	ExploreSortOdds,
	ExploreSortLine,
	ExploreSortKickoff,
	ExploreSortPlayer,
}

// Signal tokens the explorer's signal chips filter by. Each maps to one badge
// label the signals pass attaches to a row.
const (
	SignalSmash    = "smash"
	SignalVolume   = "volume"
	SignalRoleBump = "role_bump"
	SignalFitsSlip = "fits_slip"
)

// ExploreSignals lists the signal chips in display order.
var ExploreSignals = []string{
	SignalSmash,
	SignalVolume,
	SignalRoleBump,
	SignalFitsSlip,
}

const (
	// DefaultExploreLimit is the phone-sized page the explorer shows before
	// "Show more" asks for a larger one.
	DefaultExploreLimit = 50

	// Signal weights used to order the matchup sort and to score a row's
	// primary badge, mirroring the pick board's signal ranking.
	signalWeightSmash    = 40
	signalWeightFitsSlip = 30
	signalWeightRoleBump = 25
	signalWeightVolume   = 20

	defenseRankMax = 32
)

// gradeLetters lists every letter a scored leg can carry, in display order.
var gradeLetters = []string{"A", "B", "C", "D", "F"}

// Label is the chip text.
func (s ExploreSort) Label() string {
	switch s {
	case ExploreSortGrade:
		return "Best grade"
	case ExploreSortMatchup:
		return "Matchup"
	case ExploreSortOdds:
		return "Odds"
	case ExploreSortLine:
		return "Line"
	case ExploreSortKickoff:
		return "Kickoff"
	case ExploreSortPlayer:
		return "Player"
	default:
		return string(s)
	}
}

// SignalLabel is the chip text for a signal token.
func SignalLabel(signal string) string {
	switch signalToken(signal) {
	case SignalSmash:
		return BadgeSmashMatchup
	case SignalVolume:
		return BadgeHighVolume
	case SignalRoleBump:
		return BadgeRoleBump
	case SignalFitsSlip:
		return BadgeFitsSlip
	default:
		return signal
	}
}

// Explore scores every cached row with the analyzer, applies the query's
// targeted and open-exploration filters, counts the facet chips, and returns
// one deterministic page. It reads nothing itself and never fetches odds, so
// slate-wide browsing stays free of paid API calls. Rows with the OTHER market
// are left out; games that already kicked off are filtered before this point by
// Build.
//
// Facet counts tally each chip as if only that chip's own filter were cleared,
// so the counts stay useful while someone toggles the others.
func Explore(
	rows []Row,
	games map[string]analyzer.Game,
	stats map[string]analyzer.PlayerStats,
	q ExploreQuery,
) ExploreResult {
	result := newExploreResult()
	filters := newExploreFilters(q)
	sortMode := q.Sort
	if !slices.Contains(ExploreSorts, sortMode) {
		sortMode = ExploreSortGrade
	}
	limit := q.Limit
	if limit <= 0 {
		limit = DefaultExploreLimit
	}

	candidates := make([]exploreCandidate, 0, len(rows))
	for _, row := range rows {
		cand, ok := filters.match(row, games, stats)
		if ok {
			candidates = append(candidates, cand)
		}
	}

	tallyFacets(&result, candidates)
	matched := matchingRows(candidates)
	result.TotalMatched = len(matched)

	sortExploreRows(matched, games, sortMode)
	if len(matched) > limit {
		matched = matched[:limit]
		result.HasMore = true
	}
	result.Rows = matched
	return result
}

// exploreCandidate is one scored row the targeted filters kept, along with
// each facet's own verdict so the counts can drop just that facet's filter.
type exploreCandidate struct {
	scored     ScoredRow
	signals    []string
	categoryOK bool
	gradeOK    bool
	signalOK   bool
	windowOK   bool
}

// exploreFilters holds the query's normalized filter values.
type exploreFilters struct {
	gameID   string
	playerID string
	category Category
	window   Window
	signal   string
	grades   map[string]bool
	signals  BoardSignals
	needle   string
}

// newExploreFilters normalizes a query so unknown chip values read as no
// filter and text search folds case and accents.
func newExploreFilters(q ExploreQuery) exploreFilters {
	return exploreFilters{
		gameID:   q.GameID,
		playerID: q.PlayerID,
		category: knownCategory(q.Category),
		window:   knownWindow(q.Window),
		signal:   signalToken(q.Signal),
		grades:   gradeSet(q.Grades),
		signals:  q.Signals,
		needle:   normalize(q.Q),
	}
}

// match reports whether a row survives the targeted filters, scoring it and
// recording which facets it satisfies. OTHER markets never match.
func (f exploreFilters) match(
	row Row,
	games map[string]analyzer.Game,
	stats map[string]analyzer.PlayerStats,
) (exploreCandidate, bool) {
	if row.Market == "OTHER" {
		return exploreCandidate{}, false
	}
	if f.gameID != "" && row.GameID != f.gameID {
		return exploreCandidate{}, false
	}
	if f.playerID != "" && row.PlayerID != f.playerID {
		return exploreCandidate{}, false
	}
	game, hasGame := games[row.GameID]
	if f.needle != "" && !exploreRowMatches(row, game, hasGame, f.needle) {
		return exploreCandidate{}, false
	}

	// Fill in every matching badge so the signal chips see secondary signals
	// too; without signals, fall back to the badge the caller already applied.
	var signals []string
	if hasGame && !f.signals.empty() {
		signals = badgesFor(game.Home, game.Away, row, f.signals)
		row.Badge = ""
		if len(signals) > 0 {
			row.Badge = signals[0]
		}
	} else if row.Badge != "" {
		signals = []string{row.Badge}
	}

	grade := analyzer.ScoreLeg(RowToAnalyzerLeg(row), games, stats)
	return exploreCandidate{
		scored: ScoredRow{
			Row:        row,
			Grade:      grade,
			Reason:     ExploreReason(grade),
			ImpliedPct: impliedPct(row.Odds),
		},
		signals:    signals,
		categoryOK: f.category == "" || row.Category == f.category,
		gradeOK:    len(f.grades) == 0 || f.grades[grade.Letter],
		signalOK:   f.signal == "" || slices.Contains(signalTokens(signals), f.signal),
		windowOK:   f.window == "" || row.Window == f.window,
	}, true
}

// newExploreResult starts an empty result with every facet chip present at
// zero so templates always have a count to print.
func newExploreResult() ExploreResult {
	result := ExploreResult{
		Rows:           []ScoredRow{},
		CategoryCounts: make(map[Category]int, len(Categories)),
		GradeCounts:    make(map[string]int, len(gradeLetters)),
		SignalCounts:   make(map[string]int, len(ExploreSignals)),
	}
	for _, category := range Categories {
		result.CategoryCounts[category] = 0
	}
	for _, letter := range gradeLetters {
		result.GradeCounts[letter] = 0
	}
	for _, signal := range ExploreSignals {
		result.SignalCounts[signal] = 0
	}
	return result
}

// tallyFacets counts each chip as if only that chip's own filter were cleared.
func tallyFacets(result *ExploreResult, candidates []exploreCandidate) {
	for _, cand := range candidates {
		if cand.gradeOK && cand.signalOK && cand.windowOK {
			result.CategoryCounts[cand.scored.Row.Category]++
		}
		if cand.categoryOK && cand.signalOK && cand.windowOK {
			if letter := cand.scored.Grade.Letter; letter != "" {
				result.GradeCounts[letter]++
			}
		}
		if cand.categoryOK && cand.gradeOK && cand.windowOK {
			for _, token := range signalTokens(cand.signals) {
				result.SignalCounts[token]++
			}
		}
	}
}

// matchingRows keeps the candidates that pass every facet filter.
func matchingRows(candidates []exploreCandidate) []ScoredRow {
	matched := make([]ScoredRow, 0, len(candidates))
	for _, cand := range candidates {
		if !cand.categoryOK || !cand.gradeOK || !cand.signalOK || !cand.windowOK {
			continue
		}
		matched = append(matched, cand.scored)
	}
	return matched
}

// knownCategory returns the category when it is one of the chips, or "" for an
// empty or unknown value so a stale link never empties the list.
func knownCategory(category Category) Category {
	if slices.Contains(Categories, category) {
		return category
	}
	return ""
}

// knownWindow returns the window when it is one of the chips, or "" for an
// empty or unknown value.
func knownWindow(window Window) Window {
	if slices.Contains(Windows, window) {
		return window
	}
	return ""
}

// gradeSet folds the requested grade letters into a lookup set. Empty entries
// drop out, so an empty or all-blank list means no grade filter.
func gradeSet(letters []string) map[string]bool {
	set := make(map[string]bool, len(letters))
	for _, letter := range letters {
		letter = strings.ToUpper(strings.TrimSpace(letter))
		if letter == "" {
			continue
		}
		set[letter] = true
	}
	return set
}

// signalToken maps a signal chip value or badge label to its token. An unknown
// value returns "" so a stale link reads as no signal filter.
func signalToken(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case SignalSmash, "smash matchup":
		return SignalSmash
	case SignalVolume, "high volume":
		return SignalVolume
	case SignalRoleBump, "role bump":
		return SignalRoleBump
	case SignalFitsSlip, "fits slip":
		return SignalFitsSlip
	default:
		return ""
	}
}

// signalTokens maps a row's badge labels to their tokens, dropping unknown
// labels and duplicates.
func signalTokens(badges []string) []string {
	tokens := make([]string, 0, len(badges))
	for _, badge := range badges {
		token := signalToken(badge)
		if token == "" || slices.Contains(tokens, token) {
			continue
		}
		tokens = append(tokens, token)
	}
	return tokens
}

// signalWeight is the matchup weight of one badge, matching the pick board's
// signal ranking.
func signalWeight(badge string) int {
	switch signalToken(badge) {
	case SignalSmash:
		return signalWeightSmash
	case SignalFitsSlip:
		return signalWeightFitsSlip
	case SignalRoleBump:
		return signalWeightRoleBump
	case SignalVolume:
		return signalWeightVolume
	default:
		return 0
	}
}

// exploreRowMatches reports whether needle appears in a row's text, team,
// player, market, or matchup. needle is already normalized.
func exploreRowMatches(row Row, game analyzer.Game, hasGame bool, needle string) bool {
	haystack := []string{
		row.Text,
		row.Team,
		row.PlayerName,
		row.Market,
		db.MarketLabel(row.Market),
	}
	if hasGame {
		haystack = append(haystack, game.Away, game.Home, game.Away+" @ "+game.Home)
	}
	for _, part := range haystack {
		if part != "" && strings.Contains(normalize(part), needle) {
			return true
		}
	}
	return false
}

// ExploreReason names the factor that moved the grade the most, preferring the
// positive one when a positive and a negative tie. The explorer and the per-bet
// page share it so one rule names the top factor everywhere.
func ExploreReason(grade analyzer.LegGrade) string {
	best := analyzer.GradeFactor{}
	found := false
	for _, factor := range grade.Factors {
		if factor.Delta == 0 {
			continue
		}
		if !found || absInt(factor.Delta) > absInt(best.Delta) ||
			(absInt(factor.Delta) == absInt(best.Delta) && factor.Delta > best.Delta) {
			best = factor
			found = true
		}
	}
	if !found {
		return "Balanced profile"
	}
	return best.Label
}

// impliedPct is a price's implied probability as a whole percentage.
func impliedPct(odds int) int {
	prob := parlay.ImpliedProb(odds)
	if prob <= 0 {
		return 0
	}
	return int(math.Round(prob * 100))
}

// absInt is the absolute value of v.
func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// sortExploreRows orders one page of scored rows for the query's sort, falling
// back to a fixed tiebreak so the order never depends on input order.
func sortExploreRows(rows []ScoredRow, games map[string]analyzer.Game, mode ExploreSort) {
	slices.SortStableFunc(rows, func(left, right ScoredRow) int {
		switch mode {
		case ExploreSortGrade:
			if c := cmp.Compare(right.Grade.Score, left.Grade.Score); c != 0 {
				return c
			}
			if c := cmp.Compare(priceValue(right.Row.Odds), priceValue(left.Row.Odds)); c != 0 {
				return c
			}
		case ExploreSortMatchup:
			if c := cmp.Compare(
				matchupScore(right.Row, games[right.Row.GameID]),
				matchupScore(left.Row, games[left.Row.GameID]),
			); c != 0 {
				return c
			}
			if c := cmp.Compare(right.Grade.Score, left.Grade.Score); c != 0 {
				return c
			}
		case ExploreSortOdds:
			if c := cmp.Compare(priceValue(left.Row.Odds), priceValue(right.Row.Odds)); c != 0 {
				return c
			}
		case ExploreSortLine:
			if c := cmp.Compare(lineValue(right.Row.Line), lineValue(left.Row.Line)); c != 0 {
				return c
			}
		case ExploreSortKickoff:
			if c := left.Row.Kickoff.Compare(right.Row.Kickoff); c != 0 {
				return c
			}
		case ExploreSortPlayer:
			if c := compareExploreNames(left.Row, right.Row); c != 0 {
				return c
			}
		}
		return compareExploreTiebreak(left.Row, right.Row)
	})
}

// matchupScore ranks a row by its signals and how soft the opposing defense is
// for the row's stat, using the same shape as the pick board's signal score.
func matchupScore(row Row, game analyzer.Game) int {
	score := signalWeight(row.Badge)
	if row.Team == "" {
		return score
	}
	_, rank, ok := analyzer.MatchupDefenseRank(row.Market, game.OpponentEPA(row.Team))
	if !ok {
		return score
	}
	switch row.Side {
	case "OVER":
		score += rank
	case "UNDER":
		score += defenseRankMax + 1 - rank
	}
	return score
}

// priceValue is a price as decimal odds, with a missing price sorting last.
func priceValue(odds int) float64 {
	if odds == 0 {
		return math.Inf(1)
	}
	return parlay.AmericanToDecimal(odds)
}

// compareExploreNames orders the player sort by the player's name, falling back
// to the team for game lines and putting an unnamed row last.
func compareExploreNames(left, right Row) int {
	leftName, rightName := exploreName(left), exploreName(right)
	if leftName != rightName {
		if leftName == "" {
			return 1
		}
		if rightName == "" {
			return -1
		}
		return cmp.Compare(leftName, rightName)
	}
	return cmp.Compare(left.Team, right.Team)
}

// exploreName is the name a row sorts under: the player when it has one, the
// team otherwise.
func exploreName(row Row) string {
	if row.PlayerName != "" {
		return row.PlayerName
	}
	return row.Team
}

// compareExploreTiebreak is the fixed order every sort falls back to.
func compareExploreTiebreak(left, right Row) int {
	if c := cmp.Compare(left.GameID, right.GameID); c != 0 {
		return c
	}
	if c := cmp.Compare(left.Market, right.Market); c != 0 {
		return c
	}
	if c := cmp.Compare(left.PlayerName, right.PlayerName); c != 0 {
		return c
	}
	if c := cmp.Compare(left.Side, right.Side); c != 0 {
		return c
	}
	if c := cmp.Compare(left.Team, right.Team); c != 0 {
		return c
	}
	if c := cmp.Compare(lineValue(left.Line), lineValue(right.Line)); c != 0 {
		return c
	}
	return cmp.Compare(left.Odds, right.Odds)
}
