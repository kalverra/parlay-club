package board

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/kalverra/parlay-club/internal/analyzer"
	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/parlay"
)

const (
	defaultMaxLegs      = 3
	defaultMaxParlays   = 2
	defaultMaxPerPlayer = 1

	maxComboPoolA      = 12
	maxComboPoolBStack = 6
)

// LegHighlight is an A-grade pick-board row paired with its analyzer grade
// and a concise summary of its top positive factors.
type LegHighlight struct {
	Row    Row
	Grade  analyzer.LegGrade
	Reason string
}

// ParlayHighlight is an A-grade 2- or 3-leg combo built from cached board
// rows without script conflicts.
type ParlayHighlight struct {
	Legs         []LegHighlight
	Grade        analyzer.SlipGrade
	CombinedOdds int
	Label        string
	Reason       string
}

// HighlightOptions configures how SelectHighlights filters, bounds, and caps
// returned leg and parlay highlights. Zero values select defaults (3 legs,
// 2 parlays, 1 leg per player); negative values disable that cap (0 legs/parlays
// or unlimited per player).
type HighlightOptions struct {
	MaxLegs       int
	MaxParlays    int
	MaxPerPlayer  int
	SameGameOnly  bool
	ExcludePicked bool
}

// HighlightSet holds the top A-grade single legs and parlay combos for a
// board slice.
type HighlightSet struct {
	Legs    []LegHighlight
	Parlays []ParlayHighlight
}

// RowToAnalyzerLeg converts a pick-board Row into an analyzer.Leg for scoring.
func RowToAnalyzerLeg(row Row) analyzer.Leg {
	return analyzer.Leg{
		ID:         row.Picked,
		GameID:     row.GameID,
		Market:     row.Market,
		Team:       row.Team,
		PlayerID:   row.PlayerID,
		PlayerName: row.PlayerName,
		Side:       row.Side,
		Line:       row.Line,
		Odds:       row.Odds,
	}
}

// SelectHighlights scores cached board rows with analyzer.ScoreLeg and
// analyzer.EvaluateSlip, returning top A-grade legs and conflict-free A-grade
// 2- and 3-leg parlay combos.
func SelectHighlights(
	rows []Row,
	games map[string]analyzer.Game,
	stats map[string]analyzer.PlayerStats,
	opts HighlightOptions,
) HighlightSet {
	maxLegs := resolveLimit(opts.MaxLegs, defaultMaxLegs)
	maxParlays := resolveLimit(opts.MaxParlays, defaultMaxParlays)
	maxPerPlayer := resolveLimit(opts.MaxPerPlayer, defaultMaxPerPlayer)

	if len(rows) == 0 || len(games) == 0 || (maxLegs == 0 && maxParlays == 0) {
		return HighlightSet{}
	}

	var aLegs []LegHighlight
	var bStackLegs []LegHighlight

	for _, row := range rows {
		if opts.ExcludePicked && row.Picked != 0 {
			continue
		}
		if row.GameID == "" || row.Market == "OTHER" {
			continue
		}
		grade := analyzer.ScoreLeg(RowToAnalyzerLeg(row), games, stats)
		switch grade.Letter {
		case "A":
			aLegs = append(aLegs, LegHighlight{
				Row:    row,
				Grade:  grade,
				Reason: legHighlightReason(grade),
			})
		case "B":
			if maxParlays > 0 && isStackPartnerCandidate(row) {
				bStackLegs = append(bStackLegs, LegHighlight{
					Row:    row,
					Grade:  grade,
					Reason: legHighlightReason(grade),
				})
			}
		}
	}

	slices.SortStableFunc(aLegs, compareLegHighlights)
	slices.SortStableFunc(bStackLegs, compareLegHighlights)

	selectedLegs := selectTopLegs(aLegs, maxLegs, maxPerPlayer)
	var selectedParlays []ParlayHighlight
	if maxParlays > 0 {
		selectedParlays = selectTopParlays(aLegs, bStackLegs, games, stats, opts.SameGameOnly, maxParlays)
	}

	return HighlightSet{
		Legs:    selectedLegs,
		Parlays: selectedParlays,
	}
}

func resolveLimit(val, fallback int) int {
	if val == 0 {
		return fallback
	}
	if val < 0 {
		return 0
	}
	return val
}

func compareLegHighlights(a, b LegHighlight) int {
	if c := cmp.Compare(b.Grade.Score, a.Grade.Score); c != 0 {
		return c
	}
	if c := cmp.Compare(parlay.AmericanToDecimal(b.Row.Odds), parlay.AmericanToDecimal(a.Row.Odds)); c != 0 {
		return c
	}
	if c := cmp.Compare(a.Row.GameID, b.Row.GameID); c != 0 {
		return c
	}
	if c := cmp.Compare(a.Row.Market, b.Row.Market); c != 0 {
		return c
	}
	if c := cmp.Compare(a.Row.PlayerName, b.Row.PlayerName); c != 0 {
		return c
	}
	if c := cmp.Compare(a.Row.Side, b.Row.Side); c != 0 {
		return c
	}
	return cmp.Compare(a.Row.Team, b.Row.Team)
}

func legHighlightReason(grade analyzer.LegGrade) string {
	var pos []analyzer.GradeFactor
	for _, f := range grade.Factors {
		if f.Delta > 0 {
			pos = append(pos, f)
		}
	}
	if len(pos) == 0 {
		return "Strong overall profile"
	}
	slices.SortStableFunc(pos, func(a, b analyzer.GradeFactor) int {
		return cmp.Compare(b.Delta, a.Delta)
	})
	limit := min(len(pos), 2)
	labels := make([]string, limit)
	for i := range limit {
		labels[i] = pos[i].Label
	}
	return strings.Join(labels, ", ")
}

func selectTopLegs(aLegs []LegHighlight, maxLegs, maxPerPlayer int) []LegHighlight {
	if maxLegs == 0 || len(aLegs) == 0 {
		return nil
	}
	out := make([]LegHighlight, 0, min(maxLegs, len(aLegs)))
	for _, cand := range aLegs {
		if conflictsWithSelected(cand.Row, out, maxPerPlayer) {
			continue
		}
		out = append(out, cand)
		if len(out) == maxLegs {
			break
		}
	}
	return out
}

func conflictsWithSelected(row Row, selected []LegHighlight, maxPerPlayer int) bool {
	candKey := rowSelectionKey(row)
	isPlayer := db.IsPlayerMarket(row.Market) || row.PlayerID != "" || row.PlayerName != ""
	playerCount := 0
	for _, s := range selected {
		if db.SameLineGroup(candKey, rowSelectionKey(s.Row)) {
			return true
		}
		if isPlayer && maxPerPlayer > 0 && db.SamePlayer(candKey, rowSelectionKey(s.Row)) {
			playerCount++
			if playerCount >= maxPerPlayer {
				return true
			}
		}
	}
	return false
}

func isStackPartnerCandidate(row Row) bool {
	switch row.Market {
	case "PASS_YDS":
		return row.Side == "OVER"
	case "REC_YDS", "RECEPTIONS":
		return row.Side == "OVER"
	case "TOTAL":
		return row.Side == "OVER"
	case "SPREAD":
		return row.Line != nil && *row.Line < 0
	case "RUSH_YDS", "RUSH_ATT":
		return row.Side == "OVER"
	case "TEAM_TOTAL":
		return row.Side == "UNDER"
	default:
		return false
	}
}

type parlayCandidate struct {
	highlight ParlayHighlight
	hasStack  bool
}

func selectTopParlays(
	aLegs []LegHighlight,
	bStackLegs []LegHighlight,
	games map[string]analyzer.Game,
	stats map[string]analyzer.PlayerStats,
	sameGameOnly bool,
	maxParlays int,
) []ParlayHighlight {
	aPool := boundedUniqueLinePool(aLegs, maxComboPoolA)
	bPool := boundedBStackPool(bStackLegs, aPool, maxComboPoolBStack)

	if len(aPool)+len(bPool) < 2 {
		return nil
	}

	candidates := collectAParlayCandidates(aPool, games, stats, sameGameOnly)
	candidates = append(candidates, collectBStackCandidates(aPool, bPool, games, stats)...)

	if len(candidates) == 0 {
		return nil
	}

	slices.SortStableFunc(candidates, compareParlayCandidates)

	out := make([]ParlayHighlight, 0, min(maxParlays, len(candidates)))
	for _, cand := range candidates {
		if sharesMoreThanOneLeg(cand.highlight.Legs, out) {
			continue
		}
		out = append(out, cand.highlight)
		if len(out) == maxParlays {
			break
		}
	}
	return out
}

func collectAParlayCandidates(
	aPool []LegHighlight,
	games map[string]analyzer.Game,
	stats map[string]analyzer.PlayerStats,
	sameGameOnly bool,
) []parlayCandidate {
	var candidates []parlayCandidate

	// 2-leg combos from the A-grade pool.
	for i := range aPool {
		for j := i + 1; j < len(aPool); j++ {
			legs := []LegHighlight{aPool[i], aPool[j]}
			if !validCombo(legs, sameGameOnly) {
				continue
			}
			if cand, ok := scoreCombo(legs, games, stats); ok {
				candidates = append(candidates, cand)
			}
		}
	}

	// 3-leg combos from the A-grade pool.
	for i := range aPool {
		for j := i + 1; j < len(aPool); j++ {
			if sameGameOnly && aPool[i].Row.GameID != aPool[j].Row.GameID {
				continue
			}
			if db.SameLineGroup(rowSelectionKey(aPool[i].Row), rowSelectionKey(aPool[j].Row)) {
				continue
			}
			for k := j + 1; k < len(aPool); k++ {
				legs := []LegHighlight{aPool[i], aPool[j], aPool[k]}
				if !validCombo(legs, sameGameOnly) {
					continue
				}
				if cand, ok := scoreCombo(legs, games, stats); ok {
					candidates = append(candidates, cand)
				}
			}
		}
	}

	return candidates
}

func collectBStackCandidates(
	aPool []LegHighlight,
	bPool []LegHighlight,
	games map[string]analyzer.Game,
	stats map[string]analyzer.PlayerStats,
) []parlayCandidate {
	if len(bPool) == 0 {
		return nil
	}
	combined := make([]LegHighlight, 0, len(aPool)+len(bPool))
	combined = append(combined, aPool...)
	combined = append(combined, bPool...)
	byGame := make(map[string][]int)
	for idx, leg := range combined {
		byGame[leg.Row.GameID] = append(byGame[leg.Row.GameID], idx)
	}

	aCount := len(aPool)
	var candidates []parlayCandidate
	for _, idxs := range byGame {
		if len(idxs) < 3 {
			continue
		}
		for ii := range idxs {
			i := idxs[ii]
			for jj := ii + 1; jj < len(idxs); jj++ {
				j := idxs[jj]
				if db.SameLineGroup(rowSelectionKey(combined[i].Row), rowSelectionKey(combined[j].Row)) {
					continue
				}
				for kk := jj + 1; kk < len(idxs); kk++ {
					k := idxs[kk]
					if i < aCount && j < aCount && k < aCount {
						continue
					}
					legs := []LegHighlight{combined[i], combined[j], combined[k]}
					if !validCombo(legs, true) {
						continue
					}
					if cand, ok := scoreCombo(legs, games, stats); ok && cand.hasStack {
						candidates = append(candidates, cand)
					}
				}
			}
		}
	}
	return candidates
}

func boundedUniqueLinePool(legs []LegHighlight, limit int) []LegHighlight {
	if limit <= 0 || len(legs) == 0 {
		return nil
	}
	pool := make([]LegHighlight, 0, min(limit, len(legs)))
	for _, leg := range legs {
		key := rowSelectionKey(leg.Row)
		dup := false
		for _, existing := range pool {
			if db.SameLineGroup(key, rowSelectionKey(existing.Row)) {
				dup = true
				break
			}
		}
		if dup {
			continue
		}
		pool = append(pool, leg)
		if len(pool) == limit {
			break
		}
	}
	return pool
}

func boundedBStackPool(bLegs, aPool []LegHighlight, limit int) []LegHighlight {
	if limit <= 0 || len(bLegs) == 0 {
		return nil
	}
	// Prefer B-grade stack partners in games that also have A-grade legs, then
	// fill remaining slots in score order.
	aGames := make(map[string]bool, len(aPool))
	for _, leg := range aPool {
		aGames[leg.Row.GameID] = true
	}

	pool := make([]LegHighlight, 0, min(limit, len(bLegs)))
	appendUnique := func(leg LegHighlight) bool {
		key := rowSelectionKey(leg.Row)
		for _, existing := range pool {
			if db.SameLineGroup(key, rowSelectionKey(existing.Row)) {
				return false
			}
		}
		for _, existing := range aPool {
			if db.SameLineGroup(key, rowSelectionKey(existing.Row)) {
				return false
			}
		}
		pool = append(pool, leg)
		return len(pool) == limit
	}

	for _, leg := range bLegs {
		if aGames[leg.Row.GameID] && appendUnique(leg) {
			return pool
		}
	}
	for _, leg := range bLegs {
		if !aGames[leg.Row.GameID] && appendUnique(leg) {
			return pool
		}
	}
	return pool
}

func validCombo(legs []LegHighlight, sameGameOnly bool) bool {
	if len(legs) < 2 {
		return false
	}
	firstGame := legs[0].Row.GameID
	for i := range legs {
		if sameGameOnly && legs[i].Row.GameID != firstGame {
			return false
		}
		ki := rowSelectionKey(legs[i].Row)
		for j := i + 1; j < len(legs); j++ {
			if db.SameLineGroup(ki, rowSelectionKey(legs[j].Row)) {
				return false
			}
		}
	}
	return true
}

func scoreCombo(
	legs []LegHighlight,
	games map[string]analyzer.Game,
	stats map[string]analyzer.PlayerStats,
) (parlayCandidate, bool) {
	analyzerLegs := make([]analyzer.Leg, len(legs))
	parlayLegs := make([]parlay.Leg, len(legs))
	for i, leg := range legs {
		al := RowToAnalyzerLeg(leg.Row)
		al.ID = int64(i + 1)
		analyzerLegs[i] = al
		parlayLegs[i] = parlay.Leg{
			Decimal: parlay.AmericanToDecimal(leg.Row.Odds),
			Result:  parlay.Pending,
		}
	}

	eval := analyzer.EvaluateSlip(analyzerLegs, games, stats)
	if eval.Grade.Letter != "A" {
		return parlayCandidate{}, false
	}
	for _, w := range eval.Warnings {
		if w.Severity >= analyzer.Warn {
			return parlayCandidate{}, false
		}
	}

	stackRule := ""
	for _, f := range eval.Grade.Factors {
		if f.Delta > 0 && (f.Rule == "stack_shootout" || f.Rule == "stack_clock_drain") {
			stackRule = f.Rule
			break
		}
	}

	label := fmt.Sprintf("%d-leg A-grade combo", len(legs))
	switch stackRule {
	case "stack_shootout":
		label = "Shootout stack"
	case "stack_clock_drain":
		label = "Clock-drain stack"
	}

	combinedOdds := parlay.DecimalToAmerican(parlay.CombinedOdds(parlayLegs))

	return parlayCandidate{
		hasStack: stackRule != "",
		highlight: ParlayHighlight{
			Legs:         legs,
			Grade:        eval.Grade,
			CombinedOdds: combinedOdds,
			Label:        label,
			Reason:       eval.Grade.Summary,
		},
	}, true
}

func compareParlayCandidates(a, b parlayCandidate) int {
	if c := cmp.Compare(b.highlight.Grade.Score, a.highlight.Grade.Score); c != 0 {
		return c
	}
	if a.hasStack != b.hasStack {
		if a.hasStack {
			return -1
		}
		return 1
	}
	if c := cmp.Compare(
		parlay.AmericanToDecimal(b.highlight.CombinedOdds),
		parlay.AmericanToDecimal(a.highlight.CombinedOdds),
	); c != 0 {
		return c
	}
	if c := cmp.Compare(len(a.highlight.Legs), len(b.highlight.Legs)); c != 0 {
		return c
	}
	for i := range a.highlight.Legs {
		if c := compareLegHighlights(a.highlight.Legs[i], b.highlight.Legs[i]); c != 0 {
			return c
		}
	}
	return 0
}

func sharesMoreThanOneLeg(legs []LegHighlight, selected []ParlayHighlight) bool {
	for _, existing := range selected {
		shared := 0
		for _, l := range legs {
			kl := rowSelectionKey(l.Row)
			for _, e := range existing.Legs {
				if db.SameSelection(kl, rowSelectionKey(e.Row)) {
					shared++
					if shared > 1 {
						return true
					}
					break
				}
			}
		}
	}
	return false
}
