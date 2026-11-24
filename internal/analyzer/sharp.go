package analyzer

import (
	"cmp"
	"math"
	"slices"
	"strings"

	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/parlay"
)

const (
	lineMatchEpsilon   = 1e-6
	minTier3BookCount  = 3
	sharpTier1         = 1
	sharpTier2         = 2
	sharpTier3         = 3
	consensusBookLabel = "market consensus"
)

// SharpKey identifies a single pickable selection for sharp-book comparison.
type SharpKey struct {
	MarketType string
	Team       string // normalized upper; "" for TOTAL
	PlayerName string // normalized lower via db.NormalizePlayerName; "" for game/team markets
	Side       string // TEAM, OVER, UNDER
}

// SharpQuote holds the de-vigged fair benchmark for one selection.
type SharpQuote struct {
	Line         *float64
	FairProb     float64 // de-vigged probability (0..1)
	FairAmerican int     // DecimalToAmerican(1 / FairProb)
	Source       string  // "Pinnacle", "Circa", "Bookmaker", "LowVig", "BetOnline", or "market consensus"
	Tier         int     // 1 = sharpest, 2 = sharp US-region, 3 = consensus
}

type bookTierInfo struct {
	tier     int
	priority int
	label    string
}

var sharpBookTiers = map[string]bookTierInfo{
	"pinnacle":    {tier: sharpTier1, priority: 1, label: "Pinnacle"},
	"circa":       {tier: sharpTier1, priority: 2, label: "Circa"},
	"bookmaker":   {tier: sharpTier1, priority: 3, label: "Bookmaker"},
	"lowvig":      {tier: sharpTier2, priority: 1, label: "LowVig"},
	"betonlineag": {tier: sharpTier2, priority: 2, label: "BetOnline"},
}

// excludedTier3Books lists DFS pick'em apps and peer-to-peer exchanges whose
// pricing models or liquidity differ from traditional sportsbooks.
var excludedTier3Books = map[string]bool{
	"prizepicks":       true,
	"underdog":         true,
	"sleeper":          true,
	"betr":             true,
	"betr_us_dfs":      true,
	"dabble":           true,
	"parlayplay":       true,
	"boom":             true,
	"chalkboard":       true,
	"jockmkt":          true,
	"ownersbox":        true,
	"splash":           true,
	"thrive":           true,
	"novig":            true,
	"prophetx":         true,
	"prophet_exchange": true,
	"betopenly":        true,
	"kalshi":           true,
	"polymarket":       true,
	"sporttrade":       true,
	"betfair":          true,
	"betfair_ex_eu":    true,
	"betfair_ex_uk":    true,
	"smarkets":         true,
	"matchbook":        true,
}

type bookCandidate struct {
	book     string
	tier     int
	priority int
	label    string
	line     *float64
	fairProb float64
}

type gameBookGroup struct {
	gameID string
	book   string
}

type overUnderKey struct {
	marketType string
	team       string
	playerName string
}

// BuildSharpBenchmarks computes tiered, de-vigged sharp benchmarks per game and
// selection from current odds rows (typically db.CurrentOdds). One-sided
// markets, DFS/exchange books in Tier 3, and pairs with invalid overround are
// skipped cleanly.
func BuildSharpBenchmarks(rows []db.Odds) map[string]map[SharpKey]SharpQuote {
	if len(rows) == 0 {
		return nil
	}

	byGameBook := make(map[gameBookGroup][]db.Odds)
	for _, row := range rows {
		book := db.NormalizeBook(row.Book)
		if row.GameID == "" || book == "" || row.Price == 0 {
			continue
		}
		market := strings.ToUpper(strings.TrimSpace(row.MarketType))
		side := strings.ToUpper(strings.TrimSpace(row.Side))
		if market == "" || market == "ANYTIME_TD" || side == "YES" {
			continue
		}
		if side != "TEAM" && side != "OVER" && side != "UNDER" {
			continue
		}
		key := gameBookGroup{gameID: row.GameID, book: book}
		byGameBook[key] = append(byGameBook[key], row)
	}

	// candidates[gameID][SharpKey] -> list of per-book de-vigged candidates.
	candidates := make(map[string]map[SharpKey][]bookCandidate)

	for gb, bookRows := range byGameBook {
		info, isSharp := sharpBookTiers[gb.book]
		if !isSharp {
			if excludedTier3Books[gb.book] {
				continue
			}
			info = bookTierInfo{tier: sharpTier3, priority: 99, label: consensusBookLabel}
		}

		paired := pairBookRows(bookRows)
		if len(paired) == 0 {
			continue
		}
		gameMap := candidates[gb.gameID]
		if gameMap == nil {
			gameMap = make(map[SharpKey][]bookCandidate)
			candidates[gb.gameID] = gameMap
		}
		for key, quote := range paired {
			gameMap[key] = append(gameMap[key], bookCandidate{
				book:     gb.book,
				tier:     info.tier,
				priority: info.priority,
				label:    info.label,
				line:     quote.line,
				fairProb: quote.fairProb,
			})
		}
	}

	out := make(map[string]map[SharpKey]SharpQuote, len(candidates))
	for gameID, byKey := range candidates {
		gameOut := make(map[SharpKey]SharpQuote)
		for key, list := range byKey {
			if sq, ok := selectTieredQuote(list); ok {
				gameOut[key] = sq
			}
		}
		if len(gameOut) > 0 {
			out[gameID] = gameOut
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

type pairedSide struct {
	line     *float64
	fairProb float64
}

func pairBookRows(rows []db.Odds) map[SharpKey]pairedSide {
	var mlRows, spreadRows, ouRows []db.Odds
	for _, row := range rows {
		market := strings.ToUpper(strings.TrimSpace(row.MarketType))
		side := strings.ToUpper(strings.TrimSpace(row.Side))
		switch {
		case market == "ML" && side == "TEAM":
			mlRows = append(mlRows, row)
		case market == "SPREAD" && side == "TEAM":
			spreadRows = append(spreadRows, row)
		case side == "OVER" || side == "UNDER":
			ouRows = append(ouRows, row)
		}
	}

	out := make(map[SharpKey]pairedSide)
	pairML(mlRows, out)
	pairSpread(spreadRows, out)
	pairOverUnder(ouRows, out)
	return out
}

func pairML(rows []db.Odds, out map[SharpKey]pairedSide) {
	if len(rows) < 2 {
		return
	}
	seen := make(map[string]db.Odds, 2)
	teams := make([]string, 0, 2)
	for _, row := range rows {
		if row.Line != nil {
			continue
		}
		team := strings.ToUpper(strings.TrimSpace(row.Team))
		if team == "" {
			continue
		}
		if _, exists := seen[team]; !exists {
			seen[team] = row
			teams = append(teams, team)
		}
	}
	if len(teams) != 2 {
		return
	}
	a, b := seen[teams[0]], seen[teams[1]]
	probA, probB, ok := parlay.DevigTwoWay(a.Price, b.Price)
	if !ok {
		return
	}
	out[SharpKey{MarketType: "ML", Team: teams[0], Side: "TEAM"}] = pairedSide{fairProb: probA}
	out[SharpKey{MarketType: "ML", Team: teams[1], Side: "TEAM"}] = pairedSide{fairProb: probB}
}

func pairSpread(rows []db.Odds, out map[SharpKey]pairedSide) {
	if len(rows) < 2 {
		return
	}
	type spreadEntry struct {
		team  string
		line  float64
		price int
	}
	entries := make([]spreadEntry, 0, len(rows))
	for _, row := range rows {
		if row.Line == nil {
			continue
		}
		team := strings.ToUpper(strings.TrimSpace(row.Team))
		if team == "" {
			continue
		}
		entries = append(entries, spreadEntry{
			team:  team,
			line:  *row.Line,
			price: row.Price,
		})
	}
	if len(entries) < 2 {
		return
	}

	var bestFound bool
	var bestBalance float64
	var bestA, bestB spreadEntry
	var bestProbA, bestProbB float64

	for i := range entries {
		for j := i + 1; j < len(entries); j++ {
			a, b := entries[i], entries[j]
			if a.team == b.team || math.Abs(a.line+b.line) > lineMatchEpsilon {
				continue
			}
			probA, probB, ok := parlay.DevigTwoWay(a.price, b.price)
			if !ok {
				continue
			}
			balance := math.Abs(probA - 0.5)
			if !bestFound || balance < bestBalance {
				bestFound = true
				bestBalance = balance
				bestA, bestB = a, b
				bestProbA, bestProbB = probA, probB
			}
		}
	}
	if !bestFound {
		return
	}

	lineA := bestA.line
	lineB := bestB.line
	out[SharpKey{MarketType: "SPREAD", Team: bestA.team, Side: "TEAM"}] = pairedSide{line: &lineA, fairProb: bestProbA}
	out[SharpKey{MarketType: "SPREAD", Team: bestB.team, Side: "TEAM"}] = pairedSide{line: &lineB, fairProb: bestProbB}
}

func pairOverUnder(rows []db.Odds, out map[SharpKey]pairedSide) {
	if len(rows) < 2 {
		return
	}
	type sideSides struct {
		overs  []db.Odds
		unders []db.Odds
	}
	groups := make(map[overUnderKey]*sideSides)

	for _, row := range rows {
		if row.Line == nil {
			continue
		}
		market := strings.ToUpper(strings.TrimSpace(row.MarketType))
		team := strings.ToUpper(strings.TrimSpace(row.Team))
		if market == "TOTAL" {
			team = ""
		}
		var player string
		if db.IsPlayerMarket(market) || row.PlayerName != "" {
			player = db.NormalizePlayerName(row.PlayerName)
			if player == "" {
				continue
			}
		}
		k := overUnderKey{marketType: market, team: team, playerName: player}
		g := groups[k]
		if g == nil {
			g = &sideSides{}
			groups[k] = g
		}
		switch strings.ToUpper(strings.TrimSpace(row.Side)) {
		case "OVER":
			g.overs = append(g.overs, row)
		case "UNDER":
			g.unders = append(g.unders, row)
		}
	}

	for k, g := range groups {
		if len(g.overs) == 0 || len(g.unders) == 0 {
			continue
		}
		var bestFound bool
		var bestBalance float64
		var bestLine float64
		var bestProbOver, bestProbUnder float64

		for _, over := range g.overs {
			for _, under := range g.unders {
				if math.Abs(*over.Line-*under.Line) > lineMatchEpsilon {
					continue
				}
				probOver, probUnder, ok := parlay.DevigTwoWay(over.Price, under.Price)
				if !ok {
					continue
				}
				balance := math.Abs(probOver - 0.5)
				if !bestFound || balance < bestBalance {
					bestFound = true
					bestBalance = balance
					bestLine = *over.Line
					bestProbOver = probOver
					bestProbUnder = probUnder
				}
			}
		}
		if !bestFound {
			continue
		}

		lineOver := bestLine
		lineUnder := bestLine
		out[SharpKey{MarketType: k.marketType, Team: k.team, PlayerName: k.playerName, Side: "OVER"}] = pairedSide{
			line:     &lineOver,
			fairProb: bestProbOver,
		}
		out[SharpKey{MarketType: k.marketType, Team: k.team, PlayerName: k.playerName, Side: "UNDER"}] = pairedSide{
			line:     &lineUnder,
			fairProb: bestProbUnder,
		}
	}
}

func selectTieredQuote(candidates []bookCandidate) (SharpQuote, bool) {
	var tier1, tier2, tier3 []bookCandidate
	for _, c := range candidates {
		switch c.tier {
		case sharpTier1:
			tier1 = append(tier1, c)
		case sharpTier2:
			tier2 = append(tier2, c)
		case sharpTier3:
			tier3 = append(tier3, c)
		}
	}

	if len(tier1) > 0 {
		return aggregateSharpTier(tier1, sharpTier1), true
	}
	if len(tier2) > 0 {
		return aggregateSharpTier(tier2, sharpTier2), true
	}
	return aggregateTier3Consensus(tier3)
}

func aggregateSharpTier(group []bookCandidate, tier int) SharpQuote {
	probs := make([]float64, 0, len(group))
	lines := make([]float64, 0, len(group))
	bestSource := group[0]
	for _, c := range group {
		probs = append(probs, c.fairProb)
		if c.line != nil {
			lines = append(lines, *c.line)
		}
		if c.priority < bestSource.priority || (c.priority == bestSource.priority && c.book < bestSource.book) {
			bestSource = c
		}
	}

	fairProb := medianFloat64(probs)
	var linePtr *float64
	if len(lines) > 0 {
		med := medianFloat64(lines)
		linePtr = &med
	}

	return SharpQuote{
		Line:         linePtr,
		FairProb:     fairProb,
		FairAmerican: parlay.DecimalToAmerican(1 / fairProb),
		Source:       bestSource.label,
		Tier:         tier,
	}
}

func aggregateTier3Consensus(group []bookCandidate) (SharpQuote, bool) {
	if len(group) < minTier3BookCount {
		return SharpQuote{}, false
	}

	// Group by exact line (scaled by 1e6) so consensus only averages books
	// quoting the same line.
	type lineBucket struct {
		hasLine bool
		scaled  int64
	}
	buckets := make(map[lineBucket][]bookCandidate)
	for _, c := range group {
		var b lineBucket
		if c.line != nil {
			b = lineBucket{hasLine: true, scaled: int64(math.Round(*c.line * 1e6))}
		}
		buckets[b] = append(buckets[b], c)
	}

	var bestBucket lineBucket
	var bestList []bookCandidate
	for b, list := range buckets {
		if len(list) < minTier3BookCount {
			continue
		}
		if len(list) > len(bestList) || (len(list) == len(bestList) && cmp.Compare(b.scaled, bestBucket.scaled) < 0) {
			bestBucket = b
			bestList = list
		}
	}
	if len(bestList) < minTier3BookCount {
		return SharpQuote{}, false
	}

	probs := make([]float64, 0, len(bestList))
	for _, c := range bestList {
		probs = append(probs, c.fairProb)
	}
	fairProb := medianFloat64(probs)

	var linePtr *float64
	if bestList[0].line != nil {
		lineVal := *bestList[0].line
		linePtr = &lineVal
	}

	return SharpQuote{
		Line:         linePtr,
		FairProb:     fairProb,
		FairAmerican: parlay.DecimalToAmerican(1 / fairProb),
		Source:       consensusBookLabel,
		Tier:         sharpTier3,
	}, true
}

func medianFloat64(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := slices.Clone(values)
	slices.Sort(sorted)
	mid := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return sorted[mid]
	}
	return (sorted[mid-1] + sorted[mid]) / 2
}
