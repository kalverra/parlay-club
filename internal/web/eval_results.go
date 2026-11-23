package web

import (
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
	BackURL         string
	GuideURL        string
	GroupName       string
	HasGroup        bool
	HasData         bool
	Season          int
	SeasonLabel     string
	Seasons         []EvalSeasonOption
	SelectedRubric  string
	Rubrics         []EvalRubricOption
	ShowRubricDiffs bool
	Monotonic       bool
	MonotonicStatus string
	LegBands        []EvalBandView
	SlipBands       []EvalBandView
	Diffs           []EvalDiffView
	Weekly          []EvalWeekView
	Factors         []EvalFactorView
	Coverage        EvalCoverageView
}

// EvalSeasonOption is one pill in the season filter.
type EvalSeasonOption struct {
	Label   string
	Value   string
	Current bool
	URL     string
}

// EvalRubricOption is one pill in the rubric switcher.
type EvalRubricOption struct {
	Name    string
	Active  bool
	Current bool
	URL     string
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
	TotalLegs    int
	WithGameLogs int
	WithInjuries int
	WithNFLVerse int
	WithNfelo    int
	WithOdds     int
}

// evalResults serves GET /analyzer/results, showing group-scoped calibration,
// week-over-week progression, factor attribution, and data coverage.
func (h *slipHandlers) evalResults(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.user(r.Context()); !ok {
		h.redirect(w, r, "/login")
		return
	}

	backURL := safeReturnPath(r.URL.Query().Get("from"))
	page := h.page(r, "Grading Results")
	page.Tab = ""

	if h.deps.Store == nil {
		h.renderPage(w, "analyzer-results", EvalResultsPage{
			Page:     page,
			BackURL:  backURL,
			GuideURL: buildGuideURL(backURL),
		})
		return
	}

	group, ok := h.group(r.Context())
	if !ok {
		h.renderPage(w, "analyzer-results", EvalResultsPage{
			Page:     page,
			BackURL:  backURL,
			GuideURL: buildGuideURL(backURL),
		})
		return
	}

	ctx := r.Context()
	allSeasons, err := h.deps.Store.SeasonsForGroup(ctx, group.ID)
	if err != nil {
		h.serverError(w, err)
		return
	}
	gradedSeasons, err := h.deps.Store.GradedSeasonsForGroup(ctx, group.ID)
	if err != nil {
		h.serverError(w, err)
		return
	}

	selectedSeason, ok := parseEvalSeason(w, r, allSeasons, gradedSeasons)
	if !ok {
		return
	}

	var suite eval.SuiteReport
	if h.deps.EvalCache != nil {
		suite, err = h.deps.EvalCache.GetOrCompute(ctx, h.deps.Store, group.ID, selectedSeason)
	} else {
		var dataset eval.ReplayDataset
		dataset, err = eval.LoadDataset(ctx, h.deps.Store, eval.ReplayFilter{
			GroupID: group.ID,
			Season:  selectedSeason,
		})
		if err == nil {
			suite = eval.EvaluateAll(dataset, analyzer.BuiltInRubrics())
		}
	}
	if err != nil {
		h.serverError(w, err)
		return
	}

	selectedReport := selectEvalReport(suite, strings.TrimSpace(r.URL.Query().Get("rubric")))
	data := buildEvalResultsPage(
		page,
		backURL,
		group.Name,
		selectedSeason,
		allSeasons,
		suite,
		selectedReport,
	)
	h.renderPage(w, "analyzer-results", data)
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
		return eval.Report{RubricName: analyzer.ActiveRubric.Name, Coverage: suite.Coverage}
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
	groupName string,
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

	seasons := make([]EvalSeasonOption, 0, len(seasonYears)+1)
	seasons = append(seasons, EvalSeasonOption{
		Label:   "All seasons",
		Value:   "all",
		Current: selectedSeason == 0,
		URL:     buildResultsURL("all", report.RubricName, backURL),
	})
	for _, year := range seasonYears {
		val := strconv.Itoa(year)
		seasons = append(seasons, EvalSeasonOption{
			Label:   val,
			Value:   val,
			Current: year == selectedSeason,
			URL:     buildResultsURL(val, report.RubricName, backURL),
		})
	}

	seasonLabel := "All seasons"
	seasonParam := "all"
	if selectedSeason > 0 {
		seasonLabel = fmt.Sprintf("%d season", selectedSeason)
		seasonParam = strconv.Itoa(selectedSeason)
	}

	rubrics := make([]EvalRubricOption, 0, len(suite.Reports))
	for _, rep := range suite.Reports {
		rubrics = append(rubrics, EvalRubricOption{
			Name:    rep.RubricName,
			Active:  rep.RubricName == suite.ActiveRubric,
			Current: rep.RubricName == report.RubricName,
			URL:     buildResultsURL(seasonParam, rep.RubricName, backURL),
		})
	}

	legBands := buildEvalBands(report.LegBands)
	slipBands := buildEvalBands(report.SlipBands)
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
		Page:            page,
		BackURL:         backURL,
		GuideURL:        buildGuideURL(backURL),
		GroupName:       groupName,
		HasGroup:        true,
		HasData:         hasData,
		Season:          selectedSeason,
		SeasonLabel:     seasonLabel,
		Seasons:         seasons,
		SelectedRubric:  report.RubricName,
		Rubrics:         rubrics,
		ShowRubricDiffs: len(suite.Diffs) > 0,
		Monotonic:       report.LegMonotonic,
		MonotonicStatus: monotonicStatus,
		LegBands:        legBands,
		SlipBands:       slipBands,
		Diffs:           buildEvalDiffs(suite.Diffs),
		Weekly:          buildEvalWeekly(report.Weekly),
		Factors:         buildEvalFactors(report.Factors),
		Coverage: EvalCoverageView{
			TotalLegs:    suite.Coverage.TotalLegs,
			WithGameLogs: suite.Coverage.WithGameLogs,
			WithInjuries: suite.Coverage.WithInjuries,
			WithNFLVerse: suite.Coverage.WithNFLVerse,
			WithNfelo:    suite.Coverage.WithNfelo,
			WithOdds:     suite.Coverage.WithOdds,
		},
	}
}

func buildGuideURL(backURL string) string {
	if backURL == "" {
		return "/analyzer/guide"
	}
	return "/analyzer/guide?from=" + url.QueryEscape(backURL)
}

func buildResultsURL(season, rubric, backURL string) string {
	params := url.Values{}
	if season != "" {
		params.Set("season", season)
	}
	if rubric != "" {
		params.Set("rubric", rubric)
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
