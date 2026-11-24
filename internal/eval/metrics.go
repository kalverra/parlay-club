package eval

import (
	"cmp"
	"slices"

	"github.com/kalverra/parlay-club/internal/analyzer"
	"github.com/kalverra/parlay-club/internal/parlay"
)

const ungradedBand = "ungraded"

var bandOrder = []string{"A", "B", "C", "D", "F", ungradedBand}

// BandStats holds calibration and return metrics for one grade band.
type BandStats struct {
	Band           string  `json:"band"`
	Total          int     `json:"total"`
	Decided        int     `json:"decided"`
	Wins           int     `json:"wins"`
	Losses         int     `json:"losses"`
	Pushes         int     `json:"pushes"`
	WinRate        float64 `json:"win_rate"`
	AvgImpliedProb float64 `json:"avg_implied_prob"`
	Lift           float64 `json:"lift"`
	FlatROI        float64 `json:"flat_roi"`
	BrierScore     float64 `json:"brier_score"`
}

// FactorStats holds per-factor attribution across decided legs.
type FactorStats struct {
	Rule           string  `json:"rule"`
	Label          string  `json:"label"`
	Count          int     `json:"count"`
	Wins           int     `json:"wins"`
	Losses         int     `json:"losses"`
	WinRate        float64 `json:"win_rate"`
	AvgImpliedProb float64 `json:"avg_implied_prob"`
	Lift           float64 `json:"lift"`
	AvgDelta       float64 `json:"avg_delta"`
}

// WeekSummary holds per-week calibration and overall leg performance.
type WeekSummary struct {
	Season      int         `json:"season"`
	WeekNum     int         `json:"week_num"`
	WeekType    string      `json:"week_type,omitempty"`
	LegBands    []BandStats `json:"leg_bands"`
	SlipBands   []BandStats `json:"slip_bands"`
	DecidedLegs int         `json:"decided_legs"`
	LegWinRate  float64     `json:"leg_win_rate"`
	LegLift     float64     `json:"leg_lift"`
	LegFlatROI  float64     `json:"leg_flat_roi"`
}

// Report summarizes calibration, factor attribution, and monotonicity for one
// rubric evaluated on a ReplayDataset.
type Report struct {
	RubricName       string        `json:"rubric_name"`
	Coverage         DataCoverage  `json:"coverage"`
	LegBands         []BandStats   `json:"leg_bands"`
	SlipBands        []BandStats   `json:"slip_bands"`
	Weekly           []WeekSummary `json:"weekly"`
	Factors          []FactorStats `json:"factors"`
	SlipFactors      []FactorStats `json:"slip_factors,omitempty"`
	LegMonotonic     bool          `json:"leg_monotonic"`
	LegMonotonicWin  bool          `json:"leg_monotonic_win"`
	LegMonotonicLift bool          `json:"leg_monotonic_lift"`
	legOutcomes      []legOutcome
}

// SuiteReport holds evaluation reports for a suite of rubrics and pairwise
// comparisons against the first rubric.
type SuiteReport struct {
	ActiveRubric string       `json:"active_rubric"`
	Coverage     DataCoverage `json:"coverage"`
	Reports      []Report     `json:"reports"`
	Diffs        []DiffReport `json:"diffs"`
}

// BandDiff records the metric delta from baseline to candidate for one band.
type BandDiff struct {
	Band             string  `json:"band"`
	BaselineDecided  int     `json:"baseline_decided"`
	CandidateDecided int     `json:"candidate_decided"`
	WinRateDelta     float64 `json:"win_rate_delta"`
	LiftDelta        float64 `json:"lift_delta"`
	FlatROIDelta     float64 `json:"flat_roi_delta"`
}

// MigrationCell records legs that moved from FromBand to ToBand between
// baseline and candidate rubrics.
type MigrationCell struct {
	FromBand string  `json:"from_band"`
	ToBand   string  `json:"to_band"`
	Total    int     `json:"total"`
	Decided  int     `json:"decided"`
	Wins     int     `json:"wins"`
	Losses   int     `json:"losses"`
	WinRate  float64 `json:"win_rate"`
}

// DiffReport compares a candidate Report against a baseline Report.
type DiffReport struct {
	BaselineName  string          `json:"baseline_name"`
	CandidateName string          `json:"candidate_name"`
	LegBandDiffs  []BandDiff      `json:"leg_band_diffs"`
	SlipBandDiffs []BandDiff      `json:"slip_band_diffs"`
	Migrations    []MigrationCell `json:"migrations"`
}

type legOutcome struct {
	slipID  int64
	legID   int64
	legIdx  int
	band    string
	decided bool
	won     bool
}

type legOutcomeKey struct {
	slipID   int64
	legID    int64
	legIdx   int
	hasLegID bool
}

func (o legOutcome) key() legOutcomeKey {
	if o.legID != 0 {
		return legOutcomeKey{
			slipID:   o.slipID,
			legID:    o.legID,
			hasLegID: true,
		}
	}
	return legOutcomeKey{
		slipID: o.slipID,
		legIdx: o.legIdx,
	}
}

type bandAccum struct {
	band           string
	total          int
	wins           int
	losses         int
	pushes         int
	oddsCount      int
	oddsWins       int
	impliedProbSum float64
	unitsSum       float64
	brierSum       float64
}

func (a *bandAccum) add(result parlay.Result, decimalOdds float64) {
	a.total++
	switch result {
	case parlay.Push, parlay.Void:
		a.pushes++
		return
	case parlay.Won:
		a.wins++
	case parlay.Lost:
		a.losses++
	default:
		return
	}

	if decimalOdds > 1 {
		implied := 1.0 / decimalOdds
		outcome := 0.0
		if result == parlay.Won {
			outcome = 1.0
			a.oddsWins++
		}
		diff := implied - outcome
		a.oddsCount++
		a.impliedProbSum += implied
		a.unitsSum += parlay.Units(result, decimalOdds)
		a.brierSum += diff * diff
	}
}

func (a *bandAccum) stats() BandStats {
	decided := a.wins + a.losses
	s := BandStats{
		Band:    a.band,
		Total:   a.total,
		Decided: decided,
		Wins:    a.wins,
		Losses:  a.losses,
		Pushes:  a.pushes,
	}
	if decided > 0 {
		s.WinRate = float64(a.wins) / float64(decided)
	}
	if a.oddsCount > 0 {
		s.AvgImpliedProb = a.impliedProbSum / float64(a.oddsCount)
		s.Lift = float64(a.oddsWins)/float64(a.oddsCount) - s.AvgImpliedProb
		s.FlatROI = a.unitsSum / float64(a.oddsCount)
		s.BrierScore = a.brierSum / float64(a.oddsCount)
	}
	return s
}

type factorAccum struct {
	rule           string
	label          string
	count          int
	wins           int
	losses         int
	oddsCount      int
	oddsWins       int
	impliedProbSum float64
	deltaSum       int
}

func (a *factorAccum) add(result parlay.Result, decimalOdds float64, delta int) {
	a.count++
	a.deltaSum += delta
	if result == parlay.Won {
		a.wins++
	} else {
		a.losses++
	}
	if decimalOdds > 1 {
		a.oddsCount++
		if result == parlay.Won {
			a.oddsWins++
		}
		a.impliedProbSum += 1.0 / decimalOdds
	}
}

func (a *factorAccum) stats() FactorStats {
	s := FactorStats{
		Rule:   a.rule,
		Label:  a.label,
		Count:  a.count,
		Wins:   a.wins,
		Losses: a.losses,
	}
	if a.count > 0 {
		s.WinRate = float64(a.wins) / float64(a.count)
		s.AvgDelta = float64(a.deltaSum) / float64(a.count)
	}
	if a.oddsCount > 0 {
		s.AvgImpliedProb = a.impliedProbSum / float64(a.oddsCount)
		s.Lift = float64(a.oddsWins)/float64(a.oddsCount) - s.AvgImpliedProb
	}
	return s
}

func rubricFactorLabels(rubric analyzer.Rubric) map[string]string {
	doc := analyzer.GradeRubricFor(rubric)
	labels := make(map[string]string, len(doc.LegFactors)+len(doc.SlipFactors))
	for _, f := range doc.LegFactors {
		labels[f.Rule] = f.Label
	}
	for _, f := range doc.SlipFactors {
		labels[f.Rule] = f.Label
	}
	return labels
}

func recordFactors(
	acc map[string]*factorAccum,
	labels map[string]string,
	applied []analyzer.GradeFactor,
	result parlay.Result,
	decimalOdds float64,
) {
	for _, f := range applied {
		fa, ok := acc[f.Rule]
		if !ok {
			label := labels[f.Rule]
			if label == "" {
				label = f.Rule
			}
			fa = &factorAccum{rule: f.Rule, label: label}
			acc[f.Rule] = fa
		}
		fa.add(result, decimalOdds, f.Delta)
	}
}

func finalizeFactorStats(acc map[string]*factorAccum) []FactorStats {
	if len(acc) == 0 {
		return nil
	}
	out := make([]FactorStats, 0, len(acc))
	for _, fa := range acc {
		out = append(out, fa.stats())
	}
	slices.SortFunc(out, func(a, b FactorStats) int {
		if c := cmp.Compare(b.Count, a.Count); c != 0 {
			return c
		}
		return cmp.Compare(a.Rule, b.Rule)
	})
	return out
}

// Evaluate scores every slip and leg in dataset under rubric and returns leg
// and slip calibration by band, weekly progression, lift over implied odds,
// flat ROI, per-factor attribution, and monotonicity checks.
func Evaluate(dataset ReplayDataset, rubric analyzer.Rubric) Report {
	legBands := newBandAccumMap()
	slipBands := newBandAccumMap()
	factors := make(map[string]*factorAccum)
	slipFactors := make(map[string]*factorAccum)
	factorLabels := rubricFactorLabels(rubric)
	var outcomes []legOutcome
	weekly := make([]WeekSummary, 0, len(dataset.Weeks))

	for _, week := range dataset.Weeks {
		weekLegBands := newBandAccumMap()
		weekSlipBands := newBandAccumMap()
		weekOverallLegs := &bandAccum{band: "all"}

		for _, slip := range week.Slips {
			analyzerLegs := make([]analyzer.Leg, len(slip.Legs))
			for i, rl := range slip.Legs {
				analyzerLegs[i] = rl.Leg
			}

			slipGrade := analyzer.ScoreSlipWithRubric(analyzerLegs, week.Games, week.Stats, rubric)
			slipBand := normalizeBand(slipGrade.Letter)
			slipDec := slipDecimalOdds(slip)
			slipBands[slipBand].add(slip.Result, slipDec)
			weekSlipBands[slipBand].add(slip.Result, slipDec)
			if slip.Result == parlay.Won || slip.Result == parlay.Lost {
				recordFactors(slipFactors, factorLabels, slipGrade.Factors, slip.Result, slipDec)
			}

			for i, rl := range slip.Legs {
				if !isSettledResult(rl.Result) {
					continue
				}
				var lg analyzer.LegGrade
				if i < len(slipGrade.LegGrades) {
					lg = slipGrade.LegGrades[i]
				}
				band := normalizeBand(lg.Letter)
				dec := legDecimalOdds(rl)
				legBands[band].add(rl.Result, dec)
				weekLegBands[band].add(rl.Result, dec)
				weekOverallLegs.add(rl.Result, dec)

				decided := rl.Result == parlay.Won || rl.Result == parlay.Lost
				outcomes = append(outcomes, legOutcome{
					slipID:  slip.SlipID,
					legID:   rl.Leg.ID,
					legIdx:  i,
					band:    band,
					decided: decided,
					won:     rl.Result == parlay.Won,
				})

				if !decided {
					continue
				}
				recordFactors(factors, factorLabels, lg.Factors, rl.Result, dec)
			}
		}

		overall := weekOverallLegs.stats()
		weekly = append(weekly, WeekSummary{
			Season:      week.Season,
			WeekNum:     week.WeekNum,
			WeekType:    week.WeekType,
			LegBands:    finalizeBandStats(weekLegBands),
			SlipBands:   finalizeBandStats(weekSlipBands),
			DecidedLegs: overall.Decided,
			LegWinRate:  overall.WinRate,
			LegLift:     overall.Lift,
			LegFlatROI:  overall.FlatROI,
		})
	}

	slices.SortFunc(weekly, func(a, b WeekSummary) int {
		if c := cmp.Compare(a.Season, b.Season); c != 0 {
			return c
		}
		if c := cmp.Compare(weekTypeRank(a.WeekType), weekTypeRank(b.WeekType)); c != 0 {
			return c
		}
		return cmp.Compare(a.WeekNum, b.WeekNum)
	})

	legBandStats := finalizeBandStats(legBands)
	slipBandStats := finalizeBandStats(slipBands)
	monoWin, monoLift := checkMonotonicity(legBandStats)

	return Report{
		RubricName:       rubric.Name,
		Coverage:         dataset.Coverage,
		LegBands:         legBandStats,
		SlipBands:        slipBandStats,
		Weekly:           weekly,
		Factors:          finalizeFactorStats(factors),
		SlipFactors:      finalizeFactorStats(slipFactors),
		LegMonotonic:     monoWin && monoLift,
		LegMonotonicWin:  monoWin,
		LegMonotonicLift: monoLift,
		legOutcomes:      outcomes,
	}
}

// EvaluateAll evaluates every rubric in rubrics against dataset and compares
// each subsequent rubric against the first.
func EvaluateAll(dataset ReplayDataset, rubrics []analyzer.Rubric) SuiteReport {
	reports := make([]Report, 0, len(rubrics))
	for _, r := range rubrics {
		reports = append(reports, Evaluate(dataset, r))
	}
	diffs := make([]DiffReport, 0)
	if len(reports) > 1 {
		diffs = make([]DiffReport, 0, len(reports)-1)
		for _, cand := range reports[1:] {
			diffs = append(diffs, Compare(reports[0], cand))
		}
	}
	return SuiteReport{
		ActiveRubric: analyzer.ActiveRubric.Name,
		Coverage:     dataset.Coverage,
		Reports:      reports,
		Diffs:        diffs,
	}
}

func weekTypeRank(weekType string) int {
	if weekType == "POST" {
		return 1
	}
	return 0
}

// CompareRubrics evaluates baseline and candidate rubrics against dataset and
// returns both reports along with their DiffReport.
func CompareRubrics(dataset ReplayDataset, baseline, candidate analyzer.Rubric) (Report, Report, DiffReport) {
	baseReport := Evaluate(dataset, baseline)
	candReport := Evaluate(dataset, candidate)
	return baseReport, candReport, Compare(baseReport, candReport)
}

// Compare computes per-band calibration deltas and the leg band migration
// matrix between baseline and candidate reports.
func Compare(baseline, candidate Report) DiffReport {
	return DiffReport{
		BaselineName:  baseline.RubricName,
		CandidateName: candidate.RubricName,
		LegBandDiffs:  diffBandStats(baseline.LegBands, candidate.LegBands),
		SlipBandDiffs: diffBandStats(baseline.SlipBands, candidate.SlipBands),
		Migrations:    computeMigrations(baseline.legOutcomes, candidate.legOutcomes),
	}
}

func newBandAccumMap() map[string]*bandAccum {
	m := make(map[string]*bandAccum, len(bandOrder))
	for _, b := range bandOrder {
		m[b] = &bandAccum{band: b}
	}
	return m
}

func finalizeBandStats(m map[string]*bandAccum) []BandStats {
	out := make([]BandStats, len(bandOrder))
	for i, b := range bandOrder {
		out[i] = m[b].stats()
	}
	return out
}

func normalizeBand(letter string) string {
	switch letter {
	case "A", "B", "C", "D", "F":
		return letter
	default:
		return ungradedBand
	}
}

func legDecimalOdds(rl ReplayedLeg) float64 {
	if rl.DecimalOdds > 1 {
		return rl.DecimalOdds
	}
	if rl.Leg.Odds != 0 {
		return parlay.AmericanToDecimal(rl.Leg.Odds)
	}
	return 0
}

func slipDecimalOdds(rs ReplayedSlip) float64 {
	if rs.CombinedOdds > 1 {
		return rs.CombinedOdds
	}
	parlayLegs := make([]parlay.Leg, 0, len(rs.Legs))
	for _, rl := range rs.Legs {
		if !isSettledResult(rl.Result) || !rl.Result.Counts() {
			continue
		}
		dec := legDecimalOdds(rl)
		if dec <= 1 {
			return 0
		}
		parlayLegs = append(parlayLegs, parlay.Leg{
			Decimal: dec,
			Result:  rl.Result,
		})
	}
	if len(parlayLegs) == 0 {
		return 0
	}
	return parlay.CombinedOdds(parlayLegs)
}

func checkMonotonicity(bands []BandStats) (winMono, liftMono bool) {
	winMono = true
	liftMono = true
	hasPrev := false
	var prev BandStats
	for _, b := range bands {
		if b.Band == ungradedBand || b.Decided == 0 {
			continue
		}
		if hasPrev {
			if b.WinRate > prev.WinRate {
				winMono = false
			}
			if b.Lift > prev.Lift {
				liftMono = false
			}
		}
		prev = b
		hasPrev = true
	}
	return winMono, liftMono
}

func diffBandStats(base, cand []BandStats) []BandDiff {
	baseMap := make(map[string]BandStats, len(base))
	for _, b := range base {
		baseMap[b.Band] = b
	}
	candMap := make(map[string]BandStats, len(cand))
	for _, c := range cand {
		candMap[c.Band] = c
	}

	diffs := make([]BandDiff, len(bandOrder))
	for i, band := range bandOrder {
		b := baseMap[band]
		c := candMap[band]
		diffs[i] = BandDiff{
			Band:             band,
			BaselineDecided:  b.Decided,
			CandidateDecided: c.Decided,
			WinRateDelta:     c.WinRate - b.WinRate,
			LiftDelta:        c.Lift - b.Lift,
			FlatROIDelta:     c.FlatROI - b.FlatROI,
		}
	}
	return diffs
}

func computeMigrations(base, cand []legOutcome) []MigrationCell {
	if len(base) == 0 || len(cand) == 0 {
		return nil
	}
	candByKey := make(map[legOutcomeKey]legOutcome, len(cand))
	for _, c := range cand {
		candByKey[c.key()] = c
	}

	type key struct {
		from string
		to   string
	}
	type counts struct {
		total  int
		wins   int
		losses int
	}
	cells := make(map[key]*counts)
	for _, b := range base {
		candOutcome, ok := candByKey[b.key()]
		if !ok {
			continue
		}
		k := key{from: b.band, to: candOutcome.band}
		c, ok := cells[k]
		if !ok {
			c = &counts{}
			cells[k] = c
		}
		c.total++
		if b.decided {
			if b.won {
				c.wins++
			} else {
				c.losses++
			}
		}
	}

	var out []MigrationCell
	for _, from := range bandOrder {
		for _, to := range bandOrder {
			c, ok := cells[key{from: from, to: to}]
			if !ok {
				continue
			}
			decided := c.wins + c.losses
			winRate := 0.0
			if decided > 0 {
				winRate = float64(c.wins) / float64(decided)
			}
			out = append(out, MigrationCell{
				FromBand: from,
				ToBand:   to,
				Total:    c.total,
				Decided:  decided,
				Wins:     c.wins,
				Losses:   c.losses,
				WinRate:  winRate,
			})
		}
	}
	return out
}
