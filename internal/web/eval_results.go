package web

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/kalverra/parlay-club/internal/analyzer"
	"github.com/kalverra/parlay-club/internal/eval"
)

// EvalResultsPage is the in-app Grading Results screen at GET /analyzer/results.
type EvalResultsPage struct {
	Page
	BackURL           string
	GuideURL          string
	ScopeLabel        string
	Source            string
	IsMarket          bool
	SlipsURL          string
	Sources           []EvalSourceOption
	HasData           bool
	Season            int
	SeasonLabel       string
	Seasons           []EvalSeasonOption
	SelectedEngine    string
	Engines           []EvalEngineOption
	ShowEngineDiffs   bool
	EngineComparisons []EvalEngineComparisonView
	Calibration       EvalCalibrationView
	Monotonic         bool
	MonotonicStatus   string
	LegBands          []EvalBandView
	SlipBands         []EvalBandView
	Diffs             []EvalDiffView
	Weekly            []EvalWeekView
	Factors           []EvalFactorView
	SlipFactors       []EvalFactorView
	Coverage          EvalCoverageView
}

// EvalSourceOption is one pill in the source switcher (User slips vs. Market-wide).
type EvalSourceOption struct {
	Label   string
	Value   string
	Current bool
	URL     string
}

// EvalSeasonOption is one pill in the season filter.
type EvalSeasonOption struct {
	Label   string
	Value   string
	Current bool
	URL     string
}

// EvalEngineOption is one pill in the grading engine switcher.
type EvalEngineOption struct {
	Name    string
	Active  bool
	Current bool
	URL     string
}

// EvalEngineComparisonView summarizes one grading engine's calibration and
// SGP performance for side-by-side comparison.
type EvalEngineComparisonView struct {
	Name          string
	Active        bool
	Current       bool
	URL           string
	HasModelProbs bool
	LogLoss       string
	BrierScore    string
	ClasswiseECE  string
	BinSpread     string
	BinSpreadOK   bool
	HasSGP        bool
	SGPCount      int
	SGPLogLoss    string
	NaiveSGPLL    string
	SGPLift       string
	SGPLiftPos    bool
	SGPLiftNeg    bool
	Monotonic     bool
}

// EvalCalibrationView formats proper scoring rules, market baselines,
// reliability bins, and SGP correlation lift for the selected grading engine.
type EvalCalibrationView struct {
	HasData           bool
	HasModelProbs     bool
	LogLoss           string
	BrierScore        string
	ClasswiseECE      string
	BinSpread         string
	BinSpreadOK       bool
	HasMarket         bool
	MarketLogLoss     string
	MarketBrier       string
	BeatsMarketLL     bool
	TrailsMarketLL    bool
	BeatsMarketBrier  bool
	TrailsMarketBrier bool
	HasSGP            bool
	SGPCount          int
	SGPLabel          string
	SGPLogLoss        string
	NaiveIndepLogLoss string
	SGPLift           string
	SGPLiftPositive   bool
	SGPLiftNegative   bool
	Reliability       []EvalReliabilityBinView
}

// EvalReliabilityBinView formats one 10% probability bucket in the reliability table.
type EvalReliabilityBinView struct {
	Label      string
	Count      int
	CountLabel string
	Empty      bool
	MeanPred   string
	Empirical  string
	Gap        string
}

// EvalBandView formats one A-F (or ungraded) calibration row.
type EvalBandView struct {
	Band         string
	Class        string
	Record       string
	Decided      int
	HitPct       string
	ImpliedPct   string
	Lift         string
	LiftPositive bool
	LiftNegative bool
	FlatROI      string
	ROIPositive  bool
	ROINegative  bool
}

// EvalDiffView formats one baseline-vs-candidate rubric comparison.
type EvalDiffView struct {
	BaselineName  string
	CandidateName string
	LegBandDiffs  []EvalBandDiffView
	SlipBandDiffs []EvalBandDiffView
	Migrations    []EvalMigrationView
}

// EvalBandDiffView formats one band's delta between two rubrics.
type EvalBandDiffView struct {
	Band         string
	Class        string
	SampleChange string
	WinRateDelta string
	WinPositive  bool
	WinNegative  bool
	LiftDelta    string
	LiftPositive bool
	LiftNegative bool
	FlatROIDelta string
	ROIPositive  bool
	ROINegative  bool
}

// EvalMigrationView formats a band-to-band leg migration cell.
type EvalMigrationView struct {
	FromBand  string
	FromClass string
	ToBand    string
	ToClass   string
	Total     int
	Record    string
	HitPct    string
}

// EvalWeekView formats one chronological week row in the progression section.
type EvalWeekView struct {
	Label       string
	DecidedLegs int
	HitPct      string
	Lift        string
	LiftPos     bool
	LiftNeg     bool
	FlatROI     string
	ROIPos      bool
	ROINeg      bool
	ABSummary   string
	CFSummary   string
}

// EvalFactorView formats one triggered scoring factor's attribution row.
type EvalFactorView struct {
	Rule         string
	Label        string
	Count        int
	Record       string
	HitPct       string
	Lift         string
	LiftPositive bool
	LiftNegative bool
	AvgDelta     string
	DeltaPos     bool
	DeltaNeg     bool
}

// EvalCoverageView formats the pre-kickoff signal coverage counts.
type EvalCoverageView struct {
	TotalLegs      int
	WithGameLogs   int
	WithL5Fallback int
	WithInjuries   int
	WithNFLVerse   int
	WithNfelo      int
	WithOdds       int
	WithSharp      int
}

// evalResults serves GET /analyzer/results, showing global calibration across
// all groups (or market-wide pre-kickoff lines when ?source=market), week-over-week
// progression, factor attribution, and data coverage.
func (h *slipHandlers) evalResults(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.user(r.Context()); !ok {
		h.redirect(w, r, "/login")
		return
	}

	source, ok := parseEvalSource(w, r)
	if !ok {
		return
	}

	backURL := safeReturnPath(r.URL.Query().Get("from"))
	page := h.page(r, "Grading Results")
	page.Tab = ""
	isMarket := source == eval.ModeMarket
	scopeLabel := "All groups"
	if isMarket {
		scopeLabel = "Market-wide"
	}

	if h.deps.Store == nil {
		h.renderPage(w, "analyzer-results", EvalResultsPage{
			Page:        page,
			BackURL:     backURL,
			GuideURL:    buildGuideURL(backURL),
			ScopeLabel:  scopeLabel,
			Source:      string(source),
			IsMarket:    isMarket,
			SlipsURL:    buildResultsURL(string(eval.ModeSlips), "all", "", backURL),
			Sources:     buildEvalSources(source, "all", "", backURL),
			SeasonLabel: "All seasons",
		})
		return
	}

	ctx := r.Context()
	allSeasons, err := h.deps.Store.Seasons(ctx)
	if err != nil {
		h.serverError(w, err)
		return
	}
	preferredSeasons, err := h.preferredEvalSeasons(ctx, source)
	if err != nil {
		h.serverError(w, err)
		return
	}

	seasonChoices := allSeasons
	if isMarket {
		seasonChoices = mergeEvalSeasons(preferredSeasons, allSeasons)
	}

	selectedSeason, ok := parseEvalSeason(w, r, seasonChoices, preferredSeasons)
	if !ok {
		return
	}

	suite, err := h.deps.EvalCache.GetOrComputeMode(ctx, h.deps.Store, source, 0, selectedSeason)
	if err != nil {
		h.serverError(w, err)
		return
	}

	requestedEngine := strings.TrimSpace(r.URL.Query().Get("engine"))
	if requestedEngine == "" {
		requestedEngine = strings.TrimSpace(r.URL.Query().Get("rubric"))
	}
	selectedReport := selectEvalReport(suite, requestedEngine)
	data := buildEvalResultsPage(
		page,
		backURL,
		source,
		selectedSeason,
		seasonChoices,
		suite,
		selectedReport,
	)
	h.renderPage(w, "analyzer-results", data)
}

func (h *slipHandlers) preferredEvalSeasons(ctx context.Context, source eval.ReplayMode) ([]int, error) {
	if source == eval.ModeMarket {
		return h.deps.Store.FinalSeasons(ctx)
	}
	return h.deps.Store.GradedSeasons(ctx)
}

func mergeEvalSeasons(primary, secondary []int) []int {
	if len(primary) == 0 {
		return secondary
	}
	seen := make(map[int]struct{}, len(primary)+len(secondary))
	out := make([]int, 0, len(primary)+len(secondary))
	for _, s := range primary {
		if _, ok := seen[s]; !ok {
			seen[s] = struct{}{}
			out = append(out, s)
		}
	}
	for _, s := range secondary {
		if _, ok := seen[s]; !ok {
			seen[s] = struct{}{}
			out = append(out, s)
		}
	}
	slices.SortFunc(out, func(a, b int) int { return b - a })
	return out
}

func parseEvalSource(w http.ResponseWriter, r *http.Request) (eval.ReplayMode, bool) {
	raw := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("source")))
	switch raw {
	case "", string(eval.ModeSlips):
		return eval.ModeSlips, true
	case string(eval.ModeMarket):
		return eval.ModeMarket, true
	default:
		http.Error(w, "invalid source", http.StatusBadRequest)
		return "", false
	}
}

func parseEvalSeason(
	w http.ResponseWriter,
	r *http.Request,
	allSeasons []int,
	gradedSeasons []int,
) (int, bool) {
	if raw := strings.TrimSpace(r.URL.Query().Get("season")); raw != "" {
		if strings.EqualFold(raw, "all") || raw == "0" {
			return 0, true
		}
		season, err := strconv.Atoi(raw)
		if err != nil || season <= 0 {
			http.Error(w, "invalid season", http.StatusBadRequest)
			return 0, false
		}
		return season, true
	}

	if len(gradedSeasons) > 0 {
		return gradedSeasons[0], true
	}
	if len(allSeasons) > 0 {
		return allSeasons[0], true
	}
	return 0, true
}

func selectEvalReport(suite eval.SuiteReport, requested string) eval.Report {
	if len(suite.Reports) == 0 {
		name := analyzer.RubricV1.Name
		if analyzer.ActiveEngine != nil {
			name = analyzer.ActiveEngine.Name()
		}
		return eval.Report{RubricName: name, Coverage: suite.Coverage}
	}
	if requested != "" {
		for _, rep := range suite.Reports {
			if rep.RubricName == requested {
				return rep
			}
		}
	}
	for _, rep := range suite.Reports {
		if rep.RubricName == suite.ActiveRubric {
			return rep
		}
	}
	return suite.Reports[0]
}

func buildEvalResultsPage(
	page Page,
	backURL string,
	source eval.ReplayMode,
	selectedSeason int,
	allSeasons []int,
	suite eval.SuiteReport,
	report eval.Report,
) EvalResultsPage {
	seasonYears := slices.Clone(allSeasons)
	if selectedSeason > 0 && !slices.Contains(seasonYears, selectedSeason) {
		seasonYears = append(seasonYears, selectedSeason)
		slices.SortFunc(seasonYears, func(a, b int) int { return b - a })
	}

	sourceStr := string(source)
	isMarket := source == eval.ModeMarket
	scopeLabel := "All groups"
	if isMarket {
		scopeLabel = "Market-wide"
	}

	seasons := make([]EvalSeasonOption, 0, len(seasonYears)+1)
	seasons = append(seasons, EvalSeasonOption{
		Label:   "All seasons",
		Value:   "all",
		Current: selectedSeason == 0,
		URL:     buildResultsURL(sourceStr, "all", report.RubricName, backURL),
	})
	for _, year := range seasonYears {
		val := strconv.Itoa(year)
		seasons = append(seasons, EvalSeasonOption{
			Label:   val,
			Value:   val,
			Current: year == selectedSeason,
			URL:     buildResultsURL(sourceStr, val, report.RubricName, backURL),
		})
	}

	seasonLabel := "All seasons"
	seasonParam := "all"
	if selectedSeason > 0 {
		seasonLabel = fmt.Sprintf("%d season", selectedSeason)
		seasonParam = strconv.Itoa(selectedSeason)
	}

	engines := make([]EvalEngineOption, 0, len(suite.Reports))
	for _, rep := range suite.Reports {
		engines = append(engines, EvalEngineOption{
			Name:    rep.RubricName,
			Active:  rep.RubricName == suite.ActiveRubric,
			Current: rep.RubricName == report.RubricName,
			URL:     buildResultsURL(sourceStr, seasonParam, rep.RubricName, backURL),
		})
	}

	legBands := buildEvalBands(report.LegBands)
	var slipBands []EvalBandView
	var slipFactors []EvalFactorView
	if !isMarket {
		slipBands = buildEvalBands(report.SlipBands)
		slipFactors = buildEvalFactors(report.SlipFactors)
	}
	totalDecidedLegs := 0
	for _, b := range report.LegBands {
		if b.Band != "ungraded" {
			totalDecidedLegs += b.Decided
		}
	}

	monotonicStatus := "Lower bands are keeping pace with A-grade legs in this sample."
	if report.LegMonotonic {
		monotonicStatus = "A-grade legs are outperforming lower bands."
	} else if totalDecidedLegs < 20 {
		monotonicStatus = "Early sample — more settled legs will sharpen the grade curve."
	}

	hasData := suite.Coverage.TotalLegs > 0 || len(report.Weekly) > 0

	return EvalResultsPage{
		Page:              page,
		BackURL:           backURL,
		GuideURL:          buildGuideURL(backURL),
		ScopeLabel:        scopeLabel,
		Source:            sourceStr,
		IsMarket:          isMarket,
		SlipsURL:          buildResultsURL(string(eval.ModeSlips), seasonParam, report.RubricName, backURL),
		Sources:           buildEvalSources(source, seasonParam, report.RubricName, backURL),
		HasData:           hasData,
		Season:            selectedSeason,
		SeasonLabel:       seasonLabel,
		Seasons:           seasons,
		SelectedEngine:    report.RubricName,
		Engines:           engines,
		ShowEngineDiffs:   len(suite.Diffs) > 0,
		EngineComparisons: buildEvalEngineComparisons(suite, report.RubricName, sourceStr, seasonParam, backURL),
		Calibration:       buildEvalCalibration(report.Calibration),
		Monotonic:         report.LegMonotonic,
		MonotonicStatus:   monotonicStatus,
		LegBands:          legBands,
		SlipBands:         slipBands,
		Diffs:             buildEvalDiffs(suite.Diffs),
		Weekly:            buildEvalWeekly(report.Weekly),
		Factors:           buildEvalFactors(report.Factors),
		SlipFactors:       slipFactors,
		Coverage: EvalCoverageView{
			TotalLegs:      suite.Coverage.TotalLegs,
			WithGameLogs:   suite.Coverage.WithGameLogs,
			WithL5Fallback: suite.Coverage.WithL5Fallback,
			WithInjuries:   suite.Coverage.WithInjuries,
			WithNFLVerse:   suite.Coverage.WithNFLVerse,
			WithNfelo:      suite.Coverage.WithNfelo,
			WithOdds:       suite.Coverage.WithOdds,
			WithSharp:      suite.Coverage.WithSharp,
		},
	}
}

func buildEvalSources(current eval.ReplayMode, season, engine, backURL string) []EvalSourceOption {
	return []EvalSourceOption{
		{
			Label:   "User slips",
			Value:   string(eval.ModeSlips),
			Current: current != eval.ModeMarket,
			URL:     buildResultsURL(string(eval.ModeSlips), season, engine, backURL),
		},
		{
			Label:   "Market-wide",
			Value:   string(eval.ModeMarket),
			Current: current == eval.ModeMarket,
			URL:     buildResultsURL(string(eval.ModeMarket), season, engine, backURL),
		},
	}
}

func buildGuideURL(backURL string) string {
	if backURL == "" {
		return "/analyzer/guide"
	}
	return "/analyzer/guide?from=" + url.QueryEscape(backURL)
}

func buildResultsURL(source, season, engine, backURL string) string {
	params := url.Values{}
	if source != "" {
		params.Set("source", source)
	}
	if season != "" {
		params.Set("season", season)
	}
	if engine != "" {
		params.Set("engine", engine)
	}
	if backURL != "" {
		params.Set("from", backURL)
	}
	encoded := params.Encode()
	if encoded == "" {
		return "/analyzer/results"
	}
	return "/analyzer/results?" + encoded
}

func buildEvalEngineComparisons(
	suite eval.SuiteReport,
	selectedEngine, source, season, backURL string,
) []EvalEngineComparisonView {
	out := make([]EvalEngineComparisonView, 0, len(suite.Reports))
	for _, rep := range suite.Reports {
		cal := buildEvalCalibration(rep.Calibration)
		out = append(out, EvalEngineComparisonView{
			Name:          rep.RubricName,
			Active:        rep.RubricName == suite.ActiveRubric,
			Current:       rep.RubricName == selectedEngine,
			URL:           buildResultsURL(source, season, rep.RubricName, backURL),
			HasModelProbs: cal.HasModelProbs,
			LogLoss:       cal.LogLoss,
			BrierScore:    cal.BrierScore,
			ClasswiseECE:  cal.ClasswiseECE,
			BinSpread:     cal.BinSpread,
			BinSpreadOK:   cal.BinSpreadOK,
			HasSGP:        cal.HasSGP,
			SGPCount:      cal.SGPCount,
			SGPLogLoss:    cal.SGPLogLoss,
			NaiveSGPLL:    cal.NaiveIndepLogLoss,
			SGPLift:       cal.SGPLift,
			SGPLiftPos:    cal.SGPLiftPositive,
			SGPLiftNeg:    cal.SGPLiftNegative,
			Monotonic:     rep.LegMonotonic,
		})
	}
	return out
}

func buildEvalCalibration(c eval.CalibrationMetrics) EvalCalibrationView {
	hasModel := len(c.Reliability) > 0
	hasMarket := c.MarketLogLoss > 0 || c.MarketBrier > 0
	hasSGP := c.SGPCount > 0

	view := EvalCalibrationView{
		HasData:           hasModel || hasMarket || hasSGP,
		HasModelProbs:     hasModel,
		LogLoss:           "–",
		BrierScore:        "–",
		ClasswiseECE:      "–",
		BinSpread:         "–",
		BinSpreadOK:       c.BinSpreadOK,
		HasMarket:         hasMarket,
		MarketLogLoss:     "–",
		MarketBrier:       "–",
		HasSGP:            hasSGP,
		SGPCount:          c.SGPCount,
		SGPLogLoss:        "–",
		NaiveIndepLogLoss: "–",
		SGPLift:           "–",
	}

	if hasModel {
		view.LogLoss = fmt.Sprintf("%.4f", c.LogLoss)
		view.BrierScore = fmt.Sprintf("%.4f", c.BrierScore)
		view.ClasswiseECE = fmt.Sprintf("%.4f", c.ClasswiseECE)
		totalBins := c.TotalBins
		if totalBins <= 0 {
			totalBins = len(c.Reliability)
		}
		view.BinSpread = fmt.Sprintf("%d/%d", c.PopulatedBins, totalBins)
		view.Reliability = buildEvalReliabilityBins(c.Reliability)
	}

	if hasMarket {
		view.MarketLogLoss = fmt.Sprintf("%.4f", c.MarketLogLoss)
		view.MarketBrier = fmt.Sprintf("%.4f", c.MarketBrier)
		if hasModel {
			diffLL := c.MarketLogLoss - c.LogLoss
			view.BeatsMarketLL = diffLL > 0.00005
			view.TrailsMarketLL = diffLL < -0.00005
			diffBrier := c.MarketBrier - c.BrierScore
			view.BeatsMarketBrier = diffBrier > 0.00005
			view.TrailsMarketBrier = diffBrier < -0.00005
		}
	}

	if hasSGP {
		if c.SGPCount == 1 {
			view.SGPLabel = "1 settled SGP"
		} else {
			view.SGPLabel = fmt.Sprintf("%d settled SGPs", c.SGPCount)
		}
		view.SGPLogLoss = fmt.Sprintf("%.4f", c.SGPLogLoss)
		view.NaiveIndepLogLoss = fmt.Sprintf("%.4f", c.NaiveIndepLogLoss)
		lift := c.NaiveIndepLogLoss - c.SGPLogLoss
		view.SGPLift = fmt.Sprintf("%+.4f", lift)
		view.SGPLiftPositive = lift > 0.00005
		view.SGPLiftNegative = lift < -0.00005
	}

	return view
}

func buildEvalReliabilityBins(bins []eval.ReliabilityBin) []EvalReliabilityBinView {
	out := make([]EvalReliabilityBinView, 0, len(bins))
	for _, b := range bins {
		countLabel := fmt.Sprintf("%d legs", b.Count)
		if b.Count == 1 {
			countLabel = "1 leg"
		}
		item := EvalReliabilityBinView{
			Label:      fmt.Sprintf("%.0f–%.0f%%", b.BinLow*100, b.BinHigh*100),
			Count:      b.Count,
			CountLabel: countLabel,
			Empty:      b.Count == 0,
			MeanPred:   "–",
			Empirical:  "–",
			Gap:        "–",
		}
		if b.Count > 0 {
			item.MeanPred = fmt.Sprintf("%.1f%%", b.MeanPredProb*100)
			item.Empirical = fmt.Sprintf("%.1f%%", b.EmpiricalHit*100)
			item.Gap = fmt.Sprintf("%+.1f pp", (b.EmpiricalHit-b.MeanPredProb)*100)
		}
		out = append(out, item)
	}
	return out
}

func buildEvalBands(bands []eval.BandStats) []EvalBandView {
	out := make([]EvalBandView, 0, len(bands))
	for _, b := range bands {
		if b.Band == "ungraded" && b.Total == 0 {
			continue
		}
		record := fmt.Sprintf("%d-%d-%d", b.Wins, b.Losses, b.Pushes)
		hitPct := "–"
		impliedPct := "–"
		lift := "–"
		flatROI := "–"
		if b.Decided > 0 {
			hitPct = fmt.Sprintf("%.1f%%", b.WinRate*100)
		}
		if b.AvgImpliedProb > 0 {
			impliedPct = fmt.Sprintf("%.1f%%", b.AvgImpliedProb*100)
			lift = fmt.Sprintf("%+.1f%%", b.Lift*100)
			flatROI = fmt.Sprintf("%+.1f%%", b.FlatROI*100)
		}
		out = append(out, EvalBandView{
			Band:         b.Band,
			Class:        gradeClass(b.Band),
			Record:       record,
			Decided:      b.Decided,
			HitPct:       hitPct,
			ImpliedPct:   impliedPct,
			Lift:         lift,
			LiftPositive: b.AvgImpliedProb > 0 && b.Lift > 0.0005,
			LiftNegative: b.AvgImpliedProb > 0 && b.Lift < -0.0005,
			FlatROI:      flatROI,
			ROIPositive:  b.AvgImpliedProb > 0 && b.FlatROI > 0.0005,
			ROINegative:  b.AvgImpliedProb > 0 && b.FlatROI < -0.0005,
		})
	}
	return out
}

func buildEvalDiffs(diffs []eval.DiffReport) []EvalDiffView {
	out := make([]EvalDiffView, 0, len(diffs))
	for _, d := range diffs {
		migrations := make([]EvalMigrationView, 0, len(d.Migrations))
		for _, m := range d.Migrations {
			if m.Total == 0 || m.FromBand == m.ToBand {
				continue
			}
			hitPct := "–"
			if m.Decided > 0 {
				hitPct = fmt.Sprintf("%.1f%%", m.WinRate*100)
			}
			migrations = append(migrations, EvalMigrationView{
				FromBand:  m.FromBand,
				FromClass: gradeClass(m.FromBand),
				ToBand:    m.ToBand,
				ToClass:   gradeClass(m.ToBand),
				Total:     m.Total,
				Record:    fmt.Sprintf("%d-%d", m.Wins, m.Losses),
				HitPct:    hitPct,
			})
		}
		out = append(out, EvalDiffView{
			BaselineName:  d.BaselineName,
			CandidateName: d.CandidateName,
			LegBandDiffs:  buildEvalBandDiffs(d.LegBandDiffs),
			SlipBandDiffs: buildEvalBandDiffs(d.SlipBandDiffs),
			Migrations:    migrations,
		})
	}
	return out
}

func buildEvalBandDiffs(diffs []eval.BandDiff) []EvalBandDiffView {
	out := make([]EvalBandDiffView, 0, len(diffs))
	for _, d := range diffs {
		if d.Band == "ungraded" && d.BaselineDecided == 0 && d.CandidateDecided == 0 {
			continue
		}
		out = append(out, EvalBandDiffView{
			Band:         d.Band,
			Class:        gradeClass(d.Band),
			SampleChange: fmt.Sprintf("%d → %d", d.BaselineDecided, d.CandidateDecided),
			WinRateDelta: fmt.Sprintf("%+.1f pp", d.WinRateDelta*100),
			WinPositive:  d.WinRateDelta > 0.0005,
			WinNegative:  d.WinRateDelta < -0.0005,
			LiftDelta:    fmt.Sprintf("%+.1f pp", d.LiftDelta*100),
			LiftPositive: d.LiftDelta > 0.0005,
			LiftNegative: d.LiftDelta < -0.0005,
			FlatROIDelta: fmt.Sprintf("%+.1f pp", d.FlatROIDelta*100),
			ROIPositive:  d.FlatROIDelta > 0.0005,
			ROINegative:  d.FlatROIDelta < -0.0005,
		})
	}
	return out
}

func buildEvalWeekly(weekly []eval.WeekSummary) []EvalWeekView {
	out := make([]EvalWeekView, 0, len(weekly))
	for _, w := range weekly {
		label := fmt.Sprintf("%d Wk %d", w.Season, w.WeekNum)
		if w.WeekType != "" {
			label = fmt.Sprintf("%d %s Wk %d", w.Season, w.WeekType, w.WeekNum)
		}
		hitPct := "–"
		if w.DecidedLegs > 0 {
			hitPct = fmt.Sprintf("%.1f%%", w.LegWinRate*100)
		}
		abSummary := summarizeBandGroup(w.LegBands, "A", "B")
		cfSummary := summarizeBandGroup(w.LegBands, "C", "D", "F")
		out = append(out, EvalWeekView{
			Label:       label,
			DecidedLegs: w.DecidedLegs,
			HitPct:      hitPct,
			Lift:        fmt.Sprintf("%+.1f%%", w.LegLift*100),
			LiftPos:     w.LegLift > 0.0005,
			LiftNeg:     w.LegLift < -0.0005,
			FlatROI:     fmt.Sprintf("%+.1f%%", w.LegFlatROI*100),
			ROIPos:      w.LegFlatROI > 0.0005,
			ROINeg:      w.LegFlatROI < -0.0005,
			ABSummary:   abSummary,
			CFSummary:   cfSummary,
		})
	}
	return out
}

func summarizeBandGroup(bands []eval.BandStats, targets ...string) string {
	var wins, decided int
	for _, b := range bands {
		if slices.Contains(targets, b.Band) {
			wins += b.Wins
			decided += b.Decided
		}
	}
	if decided == 0 {
		return "–"
	}
	return fmt.Sprintf("%d/%d (%.0f%%)", wins, decided, float64(wins)/float64(decided)*100)
}

func buildEvalFactors(factors []eval.FactorStats) []EvalFactorView {
	out := make([]EvalFactorView, 0, len(factors))
	for _, f := range factors {
		hitPct := "–"
		if f.Wins+f.Losses > 0 {
			hitPct = fmt.Sprintf("%.1f%%", f.WinRate*100)
		}
		lift := "–"
		if f.AvgImpliedProb > 0 {
			lift = fmt.Sprintf("%+.1f%%", f.Lift*100)
		}
		out = append(out, EvalFactorView{
			Rule:         f.Rule,
			Label:        f.Label,
			Count:        f.Count,
			Record:       fmt.Sprintf("%d-%d", f.Wins, f.Losses),
			HitPct:       hitPct,
			Lift:         lift,
			LiftPositive: f.AvgImpliedProb > 0 && f.Lift > 0.0005,
			LiftNegative: f.AvgImpliedProb > 0 && f.Lift < -0.0005,
			AvgDelta:     fmt.Sprintf("%+.1f", f.AvgDelta),
			DeltaPos:     f.AvgDelta > 0.05,
			DeltaNeg:     f.AvgDelta < -0.05,
		})
	}
	return out
}
