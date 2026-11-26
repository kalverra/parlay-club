package eval

import (
	"cmp"
	"math"
	"slices"

	"github.com/kalverra/parlay-club/internal/analyzer"
	"github.com/kalverra/parlay-club/internal/parlay"
)

const (
	ungradedBand = "ungraded"

	defaultReliabilityBins = 10
	minBinSpreadRatio      = 0.80
	minCalibProb           = 1e-6
	maxCalibProb           = 1.0 - 1e-6
)

var bandOrder = []string{"A", "B", "C", "D", "F", ungradedBand}

// ReliabilityBin holds predicted vs. empirical hit rate in one probability bucket.
type ReliabilityBin struct {
	BinLow       float64 `json:"bin_low"`
	BinHigh      float64 `json:"bin_high"`
	Count        int     `json:"count"`
	MeanPredProb float64 `json:"mean_pred_prob"`
	EmpiricalHit float64 `json:"empirical_hit"`
}

// CalibrationMetrics holds proper scoring rules, classwise ECE, reliability
// bin spread, market baseline, and SGP copula-vs-independence comparisons.
type CalibrationMetrics struct {
	LogLoss           float64          `json:"log_loss"`
	BrierScore        float64          `json:"brier_score"`
	ClasswiseECE      float64          `json:"classwise_ece"`
	PopulatedBins     int              `json:"populated_bins"`
	TotalBins         int              `json:"total_bins"`
	BinSpreadOK       bool             `json:"bin_spread_ok"` // >= 80% bins populated
	MarketLogLoss     float64          `json:"market_log_loss,omitempty"`
	MarketBrier       float64          `json:"market_brier,omitempty"`
	SGPLogLoss        float64          `json:"sgp_log_loss,omitempty"`
	NaiveIndepLogLoss float64          `json:"naive_indep_log_loss,omitempty"`
	SGPCount          int              `json:"sgp_count,omitempty"`
	Reliability       []ReliabilityBin `json:"reliability,omitempty"`
}

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
// rubric or GradingEngine evaluated on a ReplayDataset.
type Report struct {
	RubricName       string             `json:"rubric_name"`
	Coverage         DataCoverage       `json:"coverage"`
	Calibration      CalibrationMetrics `json:"calibration"`
	LegBands         []BandStats        `json:"leg_bands"`
	SlipBands        []BandStats        `json:"slip_bands"`
	Weekly           []WeekSummary      `json:"weekly"`
	Factors          []FactorStats      `json:"factors"`
	SlipFactors      []FactorStats      `json:"slip_factors,omitempty"`
	LegMonotonic     bool               `json:"leg_monotonic"`
	LegMonotonicWin  bool               `json:"leg_monotonic_win"`
	LegMonotonicLift bool               `json:"leg_monotonic_lift"`
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
	season   int
	weekType string
	weekNum  int
	isMarket bool
	slipID   int64
	legID    int64
	legIdx   int
	band     string
	decided  bool
	won      bool
}

type legOutcomeKey struct {
	season   int
	weekType string
	weekNum  int
	isMarket bool
	slipID   int64
	legID    int64
	legIdx   int
	hasLegID bool
}

func (o legOutcome) key() legOutcomeKey {
	if o.legID != 0 {
		return legOutcomeKey{
			isMarket: o.isMarket,
			slipID:   o.slipID,
			legID:    o.legID,
			hasLegID: true,
		}
	}
	return legOutcomeKey{
		season:   o.season,
		weekType: o.weekType,
		weekNum:  o.weekNum,
		isMarket: o.isMarket,
		slipID:   o.slipID,
		legIdx:   o.legIdx,
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
				label = f.Label
			}
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

type rubricEngine struct {
	rubric analyzer.Rubric
}

func (e rubricEngine) Name() string {
	return e.rubric.Name
}

func (e rubricEngine) ScoreLeg(
	leg analyzer.Leg,
	games map[string]analyzer.Game,
	stats map[string]analyzer.PlayerStats,
) analyzer.LegGrade {
	return analyzer.ScoreLegWithRubric(leg, games, stats, e.rubric)
}

func (e rubricEngine) EvaluateSlip(
	legs []analyzer.Leg,
	games map[string]analyzer.Game,
	stats map[string]analyzer.PlayerStats,
) analyzer.SlipEvaluation {
	return analyzer.EvaluateSlipWithRubric(legs, games, stats, e.rubric)
}

// Evaluate scores every slip and leg in dataset under rubric and returns leg
// and slip calibration by band, weekly progression, lift over implied odds,
// flat ROI, per-factor attribution, and monotonicity checks.
func Evaluate(dataset ReplayDataset, rubric analyzer.Rubric) Report {
	return evaluateWithEngine(dataset, rubricEngine{rubric: rubric}, rubricFactorLabels(rubric))
}

// EvaluateEngine scores every slip and leg in dataset under engine and returns
// band calibration, proper scoring CalibrationMetrics, weekly progression,
// factor attribution, and monotonicity checks.
func EvaluateEngine(dataset ReplayDataset, engine analyzer.GradingEngine) Report {
	if engine == nil {
		engine = resolveDefaultEngine()
	}
	return evaluateWithEngine(dataset, engine, rubricFactorLabels(analyzer.ActiveRubric))
}

func resolveDefaultEngine() analyzer.GradingEngine {
	if analyzer.ActiveEngine != nil {
		return analyzer.ActiveEngine
	}
	return analyzer.EngineV1
}

func activeEngineName() string {
	if analyzer.ActiveEngine != nil {
		return analyzer.ActiveEngine.Name()
	}
	return analyzer.ActiveRubric.Name
}

func evaluateWithEngine(
	dataset ReplayDataset,
	engine analyzer.GradingEngine,
	factorLabels map[string]string,
) Report {
	legBands := newBandAccumMap()
	slipBands := newBandAccumMap()
	factors := make(map[string]*factorAccum)
	slipFactors := make(map[string]*factorAccum)
	var calib calibAccum
	var outcomes []legOutcome
	weekly := make([]WeekSummary, 0, len(dataset.Weeks))

	for _, week := range dataset.Weeks {
		weekLegBands := newBandAccumMap()
		weekSlipBands := newBandAccumMap()
		weekOverallLegs := &bandAccum{band: "all"}

		recordLeg := func(rl ReplayedLeg, lg analyzer.LegGrade, slipID int64, legIdx int, isMarket bool) {
			band := normalizeBand(lg.Letter)
			dec := legDecimalOdds(rl)
			legBands[band].add(rl.Result, dec)
			weekLegBands[band].add(rl.Result, dec)
			weekOverallLegs.add(rl.Result, dec)
			calib.addLeg(rl, lg, week.Games)

			decided := rl.Result == parlay.Won || rl.Result == parlay.Lost
			outcomes = append(outcomes, legOutcome{
				season:   week.Season,
				weekType: week.WeekType,
				weekNum:  week.WeekNum,
				isMarket: isMarket,
				slipID:   slipID,
				legID:    rl.Leg.ID,
				legIdx:   legIdx,
				band:     band,
				decided:  decided,
				won:      rl.Result == parlay.Won,
			})

			if decided {
				recordFactors(factors, factorLabels, lg.Factors, rl.Result, dec)
			}
		}

		for _, slip := range week.Slips {
			analyzerLegs := make([]analyzer.Leg, len(slip.Legs))
			for i, rl := range slip.Legs {
				analyzerLegs[i] = rl.Leg
			}

			slipGrade := engine.EvaluateSlip(analyzerLegs, week.Games, week.Stats).Grade
			slipBand := normalizeBand(slipGrade.Letter)
			slipDec := slipDecimalOdds(slip)
			slipBands[slipBand].add(slip.Result, slipDec)
			weekSlipBands[slipBand].add(slip.Result, slipDec)
			calib.addSlip(slip, slipGrade)
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
				recordLeg(rl, lg, slip.SlipID, i, false)
			}
		}

		for i, rl := range week.MarketLegs {
			if !isSettledResult(rl.Result) {
				continue
			}
			lg := engine.ScoreLeg(rl.Leg, week.Games, week.Stats)
			recordLeg(rl, lg, 0, i, true)
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
		RubricName:       engine.Name(),
		Coverage:         dataset.Coverage,
		Calibration:      calib.metrics(),
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
	return buildSuiteReport(analyzer.ActiveRubric.Name, dataset.Coverage, reports)
}

// EvaluateAllEngines evaluates every GradingEngine in engines side-by-side
// against dataset and compares each subsequent engine against the first.
func EvaluateAllEngines(dataset ReplayDataset, engines []analyzer.GradingEngine) SuiteReport {
	reports := make([]Report, 0, len(engines))
	for _, e := range engines {
		if e == nil {
			continue
		}
		reports = append(reports, EvaluateEngine(dataset, e))
	}
	return buildSuiteReport(activeEngineName(), dataset.Coverage, reports)
}

func buildSuiteReport(activeName string, cov DataCoverage, reports []Report) SuiteReport {
	diffs := make([]DiffReport, 0)
	if len(reports) > 1 {
		diffs = make([]DiffReport, 0, len(reports)-1)
		for _, cand := range reports[1:] {
			diffs = append(diffs, Compare(reports[0], cand))
		}
	}
	return SuiteReport{
		ActiveRubric: activeName,
		Coverage:     cov,
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

// CompareEngines evaluates baseline and candidate GradingEngines against
// dataset and returns both reports along with their DiffReport.
func CompareEngines(
	dataset ReplayDataset,
	baseline, candidate analyzer.GradingEngine,
) (Report, Report, DiffReport) {
	baseReport := EvaluateEngine(dataset, baseline)
	candReport := EvaluateEngine(dataset, candidate)
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

type calibAccum struct {
	modelCount  int
	modelLLSum  float64
	modelBrier  float64
	binCounts   [defaultReliabilityBins]int
	binPredSum  [defaultReliabilityBins]float64
	binHitSum   [defaultReliabilityBins]float64
	marketCount int
	marketLLSum float64
	marketBrier float64
	sgpCount    int
	sgpLLSum    float64
	naiveLLSum  float64
}

func (a *calibAccum) addLeg(rl ReplayedLeg, lg analyzer.LegGrade, games map[string]analyzer.Game) {
	if rl.Result != parlay.Won && rl.Result != parlay.Lost {
		return
	}
	outcome := 0.0
	if rl.Result == parlay.Won {
		outcome = 1.0
	}

	if lg.CalibratedProb > 0 && lg.CalibratedProb < 1 {
		p := clampCalibProb(lg.CalibratedProb)
		a.modelCount++
		a.modelLLSum += binaryLogLossSample(p, outcome)
		diff := p - outcome
		a.modelBrier += diff * diff
		idx := reliabilityBinIndex(p, defaultReliabilityBins)
		a.binCounts[idx]++
		a.binPredSum[idx] += p
		a.binHitSum[idx] += outcome
	}

	if lg.Letter != "" {
		if mp := resolveMarketBaselineProb(rl, lg, games); mp > 0 && mp < 1 {
			p := clampCalibProb(mp)
			a.marketCount++
			a.marketLLSum += binaryLogLossSample(p, outcome)
			diff := p - outcome
			a.marketBrier += diff * diff
		}
	}
}

func (a *calibAccum) addSlip(slip ReplayedSlip, sg analyzer.SlipGrade) {
	if slip.Result != parlay.Won && slip.Result != parlay.Lost {
		return
	}
	if !(sg.CalibratedProb > 0 && sg.CalibratedProb < 1) {
		return
	}
	if !isSameGameParlay(slip.Legs, sg.LegGrades) {
		return
	}

	indepProb := 1.0
	gradedCount := 0
	for _, lg := range sg.LegGrades {
		if lg.Letter == "" {
			continue
		}
		if !(lg.CalibratedProb > 0 && lg.CalibratedProb < 1) {
			return
		}
		indepProb *= lg.CalibratedProb
		gradedCount++
	}
	if gradedCount < 2 {
		return
	}

	outcome := 0.0
	if slip.Result == parlay.Won {
		outcome = 1.0
	}
	jointP := clampCalibProb(sg.CalibratedProb)
	indepP := clampCalibProb(indepProb)
	a.sgpCount++
	a.sgpLLSum += binaryLogLossSample(jointP, outcome)
	a.naiveLLSum += binaryLogLossSample(indepP, outcome)
}

func (a *calibAccum) metrics() CalibrationMetrics {
	m := CalibrationMetrics{
		TotalBins: defaultReliabilityBins,
	}
	if a.modelCount > 0 {
		n := float64(a.modelCount)
		m.LogLoss = a.modelLLSum / n
		m.BrierScore = a.modelBrier / n

		bins := make([]ReliabilityBin, defaultReliabilityBins)
		populated := 0
		ece := 0.0
		for i := range defaultReliabilityBins {
			lo := float64(i) / float64(defaultReliabilityBins)
			hi := float64(i+1) / float64(defaultReliabilityBins)
			cnt := a.binCounts[i]
			var meanPred, empHit float64
			if cnt > 0 {
				populated++
				meanPred = a.binPredSum[i] / float64(cnt)
				empHit = a.binHitSum[i] / float64(cnt)
				// Matches ml/distributions.py reliability_summary (2-class symmetric ECE).
				ece += (float64(cnt) / n) * math.Abs(meanPred-empHit)
			}
			bins[i] = ReliabilityBin{
				BinLow:       lo,
				BinHigh:      hi,
				Count:        cnt,
				MeanPredProb: meanPred,
				EmpiricalHit: empHit,
			}
		}
		m.ClasswiseECE = ece
		m.PopulatedBins = populated
		m.BinSpreadOK = populated >= int(math.Ceil(minBinSpreadRatio*float64(defaultReliabilityBins)))
		m.Reliability = bins
	}
	if a.marketCount > 0 {
		n := float64(a.marketCount)
		m.MarketLogLoss = a.marketLLSum / n
		m.MarketBrier = a.marketBrier / n
	}
	if a.sgpCount > 0 {
		n := float64(a.sgpCount)
		m.SGPLogLoss = a.sgpLLSum / n
		m.NaiveIndepLogLoss = a.naiveLLSum / n
		m.SGPCount = a.sgpCount
	}
	return m
}

func isSameGameParlay(legs []ReplayedLeg, grades []analyzer.LegGrade) bool {
	countsByGame := make(map[string]int, len(legs))
	for i, rl := range legs {
		if i >= len(grades) || grades[i].Letter == "" || rl.Leg.GameID == "" {
			continue
		}
		countsByGame[rl.Leg.GameID]++
		if countsByGame[rl.Leg.GameID] >= 2 {
			return true
		}
	}
	return false
}

func resolveMarketBaselineProb(rl ReplayedLeg, lg analyzer.LegGrade, games map[string]analyzer.Game) float64 {
	if lg.MarketProb > 0 && lg.MarketProb < 1 {
		return lg.MarketProb
	}
	if rl.Leg.GameID != "" {
		if g, ok := games[rl.Leg.GameID]; ok {
			if p, ok := analyzer.SharpMarketProb(g, rl.Leg); ok {
				return p
			}
		}
	}
	if dec := legDecimalOdds(rl); dec > 1 {
		return 1.0 / dec
	}
	return 0
}

func binaryLogLossSample(p, outcome float64) float64 {
	return -(outcome*math.Log(p) + (1.0-outcome)*math.Log1p(-p))
}

func clampCalibProb(p float64) float64 {
	if math.IsNaN(p) {
		return 0.5
	}
	if p < minCalibProb {
		return minCalibProb
	}
	if p > maxCalibProb {
		return maxCalibProb
	}
	return p
}

func reliabilityBinIndex(p float64, nBins int) int {
	for i := range nBins - 1 {
		if p < float64(i+1)/float64(nBins) {
			return i
		}
	}
	return nBins - 1
}
