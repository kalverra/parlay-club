package web

import (
	"cmp"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/kalverra/parlay-club/internal/analyzer"
	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/parlay"
)

// AnalyzerView is the sharp-friend banner under the slip's legs.
type AnalyzerView struct {
	Clean    bool
	Grade    GradeView
	Warnings []AnalyzerWarningView
}

// GradeView is a clickable Build Grade pill with 3-layer progressive disclosure.
type GradeView struct {
	Letter          string
	DisplayGrade    string // e.g. "A-", "B+", "C-", "D", "F" (falls back to Letter)
	Descriptor      string // 2-word structural reality check
	Score           int
	Class           string
	Summary         string
	GroundRule      string // pinned 1-sentence ground-rule banner for Layer 2
	HitChance       string // e.g. "~61%" or "<1%"; empty when CalibratedProb <= 0
	HitReality      string // e.g. "61% est. · ~2 in 3" (Model Context)
	VegasImplied    string // e.g. "13% implied · ~1 in 8" (Vegas Implied)
	VegasReality    string // e.g. "13% implied (hits ~1 in 8 times)"
	FairNoVig       string // e.g. "54% fair · ~1 in 2" (Fair No-Vig Est.)
	RawModel        string // e.g. "56% model · ~5 in 9" (Model Context)
	HoldText        string // e.g. "4.5% hold"
	CorrTaxText     string // e.g. "12.0% SGP tax"
	HasMarketBox    bool
	RealismLabel    string // "Likely Side" | "Coin Flip" | "Underdog" | "Longshot" | "Lottery Ticket"
	RealismPct      int    // 0..100 position on Coin Flip -> Lottery Ticket scale
	BookCutSummary  string
	SynergySummary  string
	ContextSummary  string
	PayoutLabel     string // "Clean price" | "Fair price" | "Slight juice" | "Steep vig"
	PayoutToneClass string // matches Price & Vig pillar tone
	Uncertain       bool
	CorrSummary     string
	Pillars         []PillarView
	Why             []string
	Factors         []GradeFactorView
	SignalPills     []SignalPillView  // max 3 clickable Layer 1/2 pills
	ContextRadar    []ContextNoteView // max 2 priced-in context notes
	CleanupNudge    string            // optional 1-line structural improvement
	WhyPopovers     []GradeWhyPopover // Layer 3 live-slip popovers
	ModelCard       []string          // 4-bullet Model Card
}

// SignalPillView is a compact clickable diagnostic badge on Layer 1 cards.
type SignalPillView struct {
	Key       string
	Icon      string
	Label     string
	Tone      string
	ToneClass string
	Target    string // "synergy", "book_cut", "hit_rate", "context"
	Detail    string
}

// ContextNoteView is one Context Radar feature tagged as already priced in.
type ContextNoteView struct {
	Label    string
	PricedIn string
	Tag      string // "Already in the line" | "Likely Priced In"
}

// GradeWhyPopover is one Layer 3 inline "Why does this matter?" micro-lesson
// interpolating the live slip or leg numbers.
type GradeWhyPopover struct {
	Key     string
	Title   string
	Summary string
	Body    string
}

// PillarView is one v2 scoring pillar inside a grade breakdown popover.
type PillarView struct {
	Key       string
	Label     string
	Score     int
	Tone      string
	ToneClass string
	BarClass  string
	Summary   string
}

// GradeFactorView is one scoring adjustment in a grade breakdown.
type GradeFactorView struct {
	Rule      string
	Label     string
	DeltaText string
	Positive  bool
	Negative  bool
	Detail    string
}

// AnalyzerWarningView is one banner card.
type AnalyzerWarningView struct {
	Rule     string
	Severity string
	Class    string
	Message  string
	Legs     []AnalyzerLegView
}

// AnalyzerLegView is one leg a warning names, with the person who owns it.
type AnalyzerLegView struct {
	OwnerID   int64
	Owner     string
	Selection string
}

// AnalyzerGuidePage is the in-app grading rubric and analyzer rule reference.
type AnalyzerGuidePage struct {
	Page
	BackURL      string
	Nav          []DocsNavView
	EngineName   string
	V2Active     bool
	ModelCard    []string
	Pillars      []GuidePillarView
	LegWeights   string
	LegScriptWts string
	SlipWeights  string
	Rubric       analyzer.RubricDoc
	Bands        []GuideBandView
	Categories   []GuideRuleCategoryView
}

// GuidePillarView documents one v2 grading pillar on the guide page.
type GuidePillarView struct {
	Key        string
	Label      string
	LegWeight  string
	SlipWeight string
	Summary    string
	Points     []string
}

// GuideBandView is one letter-grade score band on the guide page.
type GuideBandView struct {
	Letter      string
	Range       string
	Description string
	Class       string
}

// GuideRuleCategoryView groups analyzer rules under a category heading.
type GuideRuleCategoryView struct {
	Name  string
	Rules []GuideRuleView
}

// GuideRuleView is one analyzer rule card on the guide page.
type GuideRuleView struct {
	ID            string
	Title         string
	SeverityLabel string
	SeverityClass string
	Threshold     string
	Rationale     string
	Example       string
}

// analyzerGuide serves GET /analyzer/guide and GET /docs/grading, rendering the
// live analyzer rules and A-F grading rubric.
func (h *slipHandlers) analyzerGuide(w http.ResponseWriter, r *http.Request) {
	backURL := safeReturnPath(r.URL.Query().Get("from"))
	rubric := analyzer.GradeRubric()

	bands := make([]GuideBandView, 0, len(rubric.Bands))
	for _, band := range rubric.Bands {
		bands = append(bands, GuideBandView{
			Letter:      band.Letter,
			Range:       band.Range,
			Description: band.Description,
			Class:       gradeClass(band.Letter),
		})
	}

	docs := analyzer.RuleDocs()
	var categories []GuideRuleCategoryView
	catIndex := make(map[string]int)
	for _, doc := range docs {
		sevLabel, sevClass := analyzerSeverity(doc.Severity)
		ruleView := GuideRuleView{
			ID:            doc.ID,
			Title:         doc.Title,
			SeverityLabel: sevLabel,
			SeverityClass: sevClass,
			Threshold:     doc.Threshold,
			Rationale:     doc.Rationale,
			Example:       doc.Example,
		}
		idx, ok := catIndex[doc.Category]
		if !ok {
			idx = len(categories)
			catIndex[doc.Category] = idx
			categories = append(categories, GuideRuleCategoryView{Name: doc.Category})
		}
		categories[idx].Rules = append(categories[idx].Rules, ruleView)
	}

	engine := h.deps.GradingEngine
	if engine == nil {
		engine = analyzer.ActiveEngine
	}
	engineName := "v1"
	if engine != nil && engine.Name() != "" {
		engineName = engine.Name()
	}

	page := h.pageWithTab(r, "Grading & Analyzer Guide", TabDocs)
	h.renderPage(w, "analyzer-guide", AnalyzerGuidePage{
		Page:         page,
		BackURL:      backURL,
		Nav:          docsNav("grading", backURL),
		EngineName:   engineName,
		V2Active:     engineName == "v2",
		ModelCard:    buildModelCard(),
		Pillars:      guidePillars(analyzer.DefaultV2Config),
		LegWeights:   formatLegWeights(analyzer.DefaultV2Config),
		LegScriptWts: formatLegScriptWeights(analyzer.DefaultV2Config),
		SlipWeights:  formatSlipWeights(analyzer.DefaultV2Config),
		Rubric:       rubric,
		Bands:        bands,
		Categories:   categories,
	})
}

func safeReturnPath(raw string) string {
	if raw == "" || !strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "//") {
		return ""
	}
	if strings.ContainsRune(raw, '\\') {
		return ""
	}
	for _, r := range raw {
		if r < 0x20 || r == 0x7f {
			return ""
		}
	}
	if colon := strings.IndexByte(raw, ':'); colon != -1 {
		if slash := strings.IndexByte(raw[1:], '/'); slash == -1 || colon < slash+1 {
			return ""
		}
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.IsAbs() || parsed.Host != "" {
		return ""
	}
	return raw
}

// analyzerView shapes the analyzer's warnings and deterministic A-F grades for
// the slip banner and each leg row using engine (defaulting to ActiveEngine).
func analyzerView(
	legs []db.Leg,
	gamesByID map[string]analyzer.Game,
	stats map[string]analyzer.PlayerStats,
	names map[int64]string,
	engine analyzer.GradingEngine,
) (AnalyzerView, map[int64]GradeView) {
	input := make([]analyzer.Leg, 0, len(legs))
	legsByID := make(map[int64]db.Leg, len(legs))
	for _, leg := range legs {
		legsByID[leg.ID] = leg
		input = append(input, analyzer.LegFromDB(leg, names[leg.UserID]))
	}

	var eval analyzer.SlipEvaluation
	if engine != nil {
		eval = engine.EvaluateSlip(input, gamesByID, stats)
	} else {
		eval = analyzer.EvaluateSlip(input, gamesByID, stats)
	}
	warnings := eval.Warnings
	slipGrade := eval.Grade

	legGrades := make(map[int64]GradeView, len(slipGrade.LegGrades))
	for _, lg := range slipGrade.LegGrades {
		if lg.Letter == "" {
			continue
		}
		legGrades[lg.LegID] = LegGradeView(lg)
	}

	view := AnalyzerView{
		Clean: len(warnings) == 0,
		Grade: SlipGradeView(slipGrade),
	}
	for _, warning := range warnings {
		label, class := analyzerSeverity(warning.Severity)
		card := AnalyzerWarningView{
			Rule:     warning.Rule,
			Severity: label,
			Class:    class,
			Message:  warning.Message,
		}
		for _, id := range warning.LegIDs {
			leg, ok := legsByID[id]
			if !ok {
				continue
			}
			card.Legs = append(card.Legs, AnalyzerLegView{
				OwnerID:   leg.UserID,
				Owner:     names[leg.UserID],
				Selection: leg.Selection,
			})
		}
		view.Warnings = append(view.Warnings, card)
	}
	return view, legGrades
}

const groundRuleText = "Build grades check ticket structure and book vig—not whether a bet will win."

type gradeBuildInput struct {
	letter         string
	score          int
	summary        string
	calibratedProb float64
	rawModelProb   float64
	vegasProb      float64
	fairNoVigProb  float64
	uncertain      bool
	corrMult       float64
	multiLeg       bool
	pillars        []analyzer.PillarScore
	factors        []analyzer.GradeFactor
	diagnostics    analyzer.BuildDiagnostics
}

// LegGradeView shapes an analyzer.LegGrade into a template-ready GradeView.
func LegGradeView(lg analyzer.LegGrade) GradeView {
	return buildGradeView(gradeBuildInput{
		letter:         lg.Letter,
		score:          lg.Score,
		calibratedProb: lg.CalibratedProb,
		rawModelProb:   lg.RawModelProb,
		vegasProb:      lg.VegasImpliedProb,
		fairNoVigProb:  lg.FairNoVigProb,
		uncertain:      lg.Uncertain,
		pillars:        lg.Pillars,
		factors:        lg.Factors,
		diagnostics:    lg.Diagnostics,
	})
}

// SlipGradeView shapes an analyzer.SlipGrade into a template-ready GradeView.
func SlipGradeView(sg analyzer.SlipGrade) GradeView {
	return buildGradeView(gradeBuildInput{
		letter:         sg.Letter,
		score:          sg.Score,
		summary:        sg.Summary,
		calibratedProb: sg.CalibratedProb,
		rawModelProb:   sg.RawModelProb,
		vegasProb:      sg.VegasImpliedProb,
		fairNoVigProb:  sg.FairNoVigProb,
		uncertain:      sg.Uncertain,
		corrMult:       sg.CorrMultiplier,
		multiLeg:       len(sg.LegGrades) > 1,
		pillars:        sg.Pillars,
		factors:        sg.Factors,
		diagnostics:    sg.Diagnostics,
	})
}

func buildGradeView(in gradeBuildInput) GradeView {
	if in.letter == "" {
		return GradeView{}
	}
	factorViews := make([]GradeFactorView, 0, len(in.factors))
	for _, f := range in.factors {
		factorViews = append(factorViews, GradeFactorView{
			Rule:      f.Rule,
			Label:     f.Label,
			DeltaText: fmt.Sprintf("%+d", f.Delta),
			Positive:  f.Delta > 0,
			Negative:  f.Delta < 0,
			Detail:    f.Detail,
		})
	}
	pillarViews := make([]PillarView, 0, len(in.pillars))
	var bookCutSummary, synergySummary, contextSummary string
	for _, p := range in.pillars {
		toneClass, barClass := pillarToneClasses(p.Tone)
		pillarViews = append(pillarViews, PillarView{
			Key:       p.Key,
			Label:     p.Label,
			Score:     clampScore(p.Score),
			Tone:      p.Tone,
			ToneClass: toneClass,
			BarClass:  barClass,
			Summary:   p.Summary,
		})
		switch p.Key {
		case analyzer.PillarPriceVig:
			bookCutSummary = p.Summary
		case analyzer.PillarScriptFit:
			synergySummary = p.Summary
		case analyzer.PillarContextTraps:
			contextSummary = p.Summary
		}
	}

	var why []string
	var corrSummary string
	if len(in.pillars) > 0 {
		why = collectWhyBullets(in.pillars)
		if in.multiLeg {
			corrSummary = formatCorrSummary(in.corrMult)
		}
	}
	payoutLabel, payoutToneClass := payoutValueFromPillars(in.pillars)

	diag := in.diagnostics
	displayGrade := strings.TrimSpace(diag.DisplayGrade)
	if displayGrade == "" {
		displayGrade = in.letter
	}
	descriptor := strings.TrimSpace(diag.Descriptor)
	if descriptor == "" && len(in.pillars) > 0 {
		descriptor = fallbackBandDescriptor(in.letter)
	}

	vegasProb := firstPositiveProb(diag.VegasImpliedProb, in.vegasProb)
	fairProb := firstPositiveProb(diag.FairNoVigProb, in.fairNoVigProb)
	rawModelProb := firstPositiveProb(diag.RawModelProb, in.rawModelProb)

	vegasImplied := formatImpliedHuman(vegasProb)
	vegasReality := parlay.FormatImpliedReality(vegasProb)
	fairNoVig := formatFairHuman(fairProb)
	rawModel := formatModelHuman(rawModelProb)
	hitReality := formatModelHuman(in.calibratedProb)
	holdText := formatHoldText(diag.HoldPct)
	corrTaxText := formatCorrTaxText(diag.CorrTaxPct)

	hasMarketBox := len(in.pillars) > 0 &&
		(vegasImplied != "" || fairNoVig != "" || rawModel != "" || hitReality != "" || holdText != "")

	realismProb := firstPositiveProb(in.calibratedProb, vegasProb)
	realismLabel, realismPct := computeRealism(realismProb)

	signalPills := buildSignalPillViews(diag.SignalPills)
	contextRadar := buildContextNoteViews(diag.ContextRadar)

	var whyPopovers []GradeWhyPopover
	if len(in.pillars) > 0 {
		whyPopovers = buildWhyPopovers(
			vegasProb,
			in.calibratedProb,
			diag.HoldPct,
			diag.CorrTaxPct,
			in.corrMult,
			in.multiLeg,
			contextRadar,
		)
	}

	return GradeView{
		Letter:          in.letter,
		DisplayGrade:    displayGrade,
		Descriptor:      descriptor,
		Score:           in.score,
		Class:           gradeClass(in.letter),
		Summary:         in.summary,
		GroundRule:      groundRuleText,
		HitChance:       formatHitChance(in.calibratedProb),
		HitReality:      hitReality,
		VegasImplied:    vegasImplied,
		VegasReality:    vegasReality,
		FairNoVig:       fairNoVig,
		RawModel:        rawModel,
		HoldText:        holdText,
		CorrTaxText:     corrTaxText,
		HasMarketBox:    hasMarketBox,
		RealismLabel:    realismLabel,
		RealismPct:      realismPct,
		BookCutSummary:  bookCutSummary,
		SynergySummary:  synergySummary,
		ContextSummary:  contextSummary,
		PayoutLabel:     payoutLabel,
		PayoutToneClass: payoutToneClass,
		Uncertain:       in.uncertain,
		CorrSummary:     corrSummary,
		Pillars:         pillarViews,
		Why:             why,
		Factors:         factorViews,
		SignalPills:     signalPills,
		ContextRadar:    contextRadar,
		CleanupNudge:    strings.TrimSpace(diag.CleanupNudge),
		WhyPopovers:     whyPopovers,
		ModelCard:       buildModelCard(),
	}
}

func firstPositiveProb(primary, fallback float64) float64 {
	if primary > 0 && primary < 1 && !math.IsNaN(primary) {
		return primary
	}
	if fallback > 0 && fallback < 1 && !math.IsNaN(fallback) {
		return fallback
	}
	return 0
}

func formatImpliedHuman(prob float64) string {
	return formatProbWithVerb(prob, "implied")
}

func formatFairHuman(prob float64) string {
	return formatProbWithVerb(prob, "fair")
}

func formatModelHuman(prob float64) string {
	return formatProbWithVerb(prob, "est.")
}

func formatProbWithVerb(prob float64, verb string) string {
	pct := formatImpliedChance(prob)
	if pct == "" {
		return ""
	}
	oneIn := parlay.FormatOneIn(prob)
	if oneIn == "" {
		return pct + " " + verb
	}
	return fmt.Sprintf("%s %s · %s", pct, verb, oneIn)
}

func formatHoldText(holdPct float64) string {
	if holdPct <= 0 || math.IsNaN(holdPct) {
		return ""
	}
	return fmt.Sprintf("%.1f%% hold", holdPct)
}

func formatCorrTaxText(corrTaxPct float64) string {
	if corrTaxPct <= 0 || math.IsNaN(corrTaxPct) {
		return ""
	}
	return fmt.Sprintf("%.1f%% SGP tax", corrTaxPct)
}

func fallbackBandDescriptor(letter string) string {
	switch letter {
	case "A":
		return "Clean Build"
	case "B":
		return "Solid Build"
	case "C":
		return "Mixed Build"
	case "D":
		return "Fragile Build"
	default:
		return "Broken Build"
	}
}

func computeRealism(prob float64) (label string, barPct int) {
	if prob <= 0 || prob >= 1 || math.IsNaN(prob) {
		return "", 0
	}
	switch {
	case prob >= 0.56:
		label = "Likely Side"
	case prob >= 0.45:
		label = "Coin Flip"
	case prob >= 0.25:
		label = "Underdog"
	case prob >= 0.12:
		label = "Longshot"
	default:
		label = "Lottery Ticket"
	}
	// Map 55%+ -> 5% (Coin Flip end) down to 1% -> 98% (Lottery Ticket end).
	dist := max(0.0, (0.55-prob)/0.55)
	barPct = min(max(int(math.Round(dist*100)), 5), 98)
	return label, barPct
}

func buildSignalPillViews(pills []analyzer.SignalPill) []SignalPillView {
	if len(pills) == 0 {
		return nil
	}
	limit := min(len(pills), 3)
	out := make([]SignalPillView, 0, limit)
	for _, p := range pills[:limit] {
		out = append(out, SignalPillView{
			Key:       p.Key,
			Icon:      p.Icon,
			Label:     p.Label,
			Tone:      p.Tone,
			ToneClass: signalPillToneClass(p.Tone, p.Key),
			Target:    p.Target,
			Detail:    p.Detail,
		})
	}
	return out
}

func signalPillToneClass(tone, key string) string {
	switch tone {
	case analyzer.ToneBad:
		return "border-danger bg-danger-soft text-danger-ink"
	case analyzer.ToneWarn:
		return "border-warning bg-warning-soft text-warning-ink"
	default:
		if key == analyzer.PillCorrelatedStack || key == analyzer.PillSharpPrice {
			return "border-grade-clean bg-grade-clean-soft text-grade-clean-ink"
		}
		return "border-grade-solid bg-grade-solid-soft text-grade-solid-ink"
	}
}

func buildContextNoteViews(notes []analyzer.ContextNote) []ContextNoteView {
	if len(notes) == 0 {
		return nil
	}
	limit := min(len(notes), 2)
	out := make([]ContextNoteView, 0, limit)
	for _, n := range notes[:limit] {
		tag := strings.TrimSpace(n.Tag)
		if tag == "" {
			tag = analyzer.TagAlreadyInLine
		}
		out = append(out, ContextNoteView{
			Label:    n.Label,
			PricedIn: n.PricedIn,
			Tag:      tag,
		})
	}
	return out
}

func buildWhyPopovers(
	vegasProb, calibratedProb, holdPct, corrTaxPct, corrMult float64,
	multiLeg bool,
	contextRadar []ContextNoteView,
) []GradeWhyPopover {
	var impliedBody string
	switch {
	case vegasProb > 0:
		impliedBody = fmt.Sprintf(
			"At this ticket's price, the book implies %s. Odds aren't just a payout—they set the hit-rate hurdle you have to clear just to break even.",
			parlay.FormatImpliedReality(vegasProb),
		)
	case calibratedProb > 0:
		impliedBody = fmt.Sprintf(
			"Even without retail odds listed, this profile estimates %s. Turning prices into hit frequency keeps longshots in perspective.",
			parlay.FormatImpliedReality(calibratedProb),
		)
	default:
		impliedBody = "Odds aren't just a payout—they translate straight into the hit rate needed to break even. Bigger payouts mean a steeper climb to cash consistently."
	}

	var vigBody string
	if holdPct > 0 {
		vigBody = fmt.Sprintf(
			"The book is building roughly %.1f%% vig into this price compared to a fair no-vig benchmark. That house cut compounds across every leg you add to a parlay.",
			holdPct,
		)
	} else {
		vigBody = "Sportsbooks bake a 4–5% house cut into standard lines, and that cut compounds across parlay legs. Avoiding heavy juice keeps more of the return when the ticket hits."
	}

	var corrBody string
	switch {
	case corrTaxPct > 0:
		corrBody = fmt.Sprintf(
			"These legs rise together (%.2fx script synergy), yet the book is shaving about %.1f%% off the payout via SGP pricing. You carry the script risk without getting full parlay pay.",
			corrMult,
			corrTaxPct,
		)
	case multiLeg && corrMult >= 1.03:
		corrBody = fmt.Sprintf(
			"Your legs share %.2fx positive script synergy without a heavy SGP payout shave. Same-game stacks help only when the book doesn't tax away the correlation.",
			corrMult,
		)
	case multiLeg && corrMult > 0 && corrMult <= 0.97:
		corrBody = fmt.Sprintf(
			"These legs pull against each other (%.2fx script multiplier). When one leg cashing makes another harder, standard parlay math underpays the real risk.",
			corrMult,
		)
	default:
		corrBody = "Same-game parlays often carry a hidden correlation tax where the book shades combined odds below independent math. Checking both script fit and the book's cut shows whether a stack helps."
	}

	var contextBody string
	if len(contextRadar) > 0 {
		contextBody = fmt.Sprintf(
			"Signals like %q are public context that oddsmakers bake into the opening number. A strong matchup or hot streak only helps if the posted line hasn't already moved past it.",
			contextRadar[0].Label,
		)
	} else {
		contextBody = "Public trends, weather, and defense ranks are usually reflected in the line before kickoff. Look for fair pricing rather than chasing headlines the book already adjusted for."
	}

	return []GradeWhyPopover{
		{
			Key:     "implied_prob",
			Title:   "Implied Probability",
			Summary: "Why does this matter?",
			Body:    impliedBody,
		},
		{
			Key:     "vig_hold",
			Title:   "Vig / Hold",
			Summary: "Why does this matter?",
			Body:    vigBody,
		},
		{
			Key:     "corr_tax",
			Title:   "Correlation Tax",
			Summary: "Why does this matter?",
			Body:    corrBody,
		},
		{
			Key:     "already_priced_in",
			Title:   "Already in the line",
			Summary: "Why does this matter?",
			Body:    contextBody,
		},
	}
}

func buildModelCard() []string {
	return []string{
		"Structural grade, not a lock: scores ticket price, leg synergy, hit-rate stability, and context traps.",
		"Provenance first: separates Vegas Implied odds, Fair No-Vig Est. sharp lines, and Model Context projections.",
		"Same-game math: uses a Gaussian copula to spot positive stacks, script conflicts, and SGP payout taxes.",
		"100% deterministic: plain Go rules over cached odds, nflverse usage, nfelo, weather, and injury reports.",
	}
}

func formatHitChance(prob float64) string {
	if prob <= 0 || prob >= 1 || math.IsNaN(prob) {
		return ""
	}
	if prob < 0.005 {
		return "<1%"
	}
	if prob > 0.995 {
		return ">99%"
	}
	return fmt.Sprintf("~%d%%", int(math.Round(prob*100)))
}

func payoutValueFromPillars(pillars []analyzer.PillarScore) (label, toneClass string) {
	for _, p := range pillars {
		if p.Key != analyzer.PillarPriceVig {
			continue
		}
		score := clampScore(p.Score)
		switch {
		case score >= 85:
			label = "Clean price"
		case score >= 73:
			label = "Fair price"
		case score >= 58:
			label = "Slight juice"
		default:
			label = "Steep vig"
		}
		toneClass, _ = pillarToneClasses(p.Tone)
		return label, toneClass
	}
	return "", ""
}

func clampScore(score int) int {
	return min(max(score, 0), 100)
}

func pillarToneClasses(tone string) (badgeClass, barClass string) {
	switch tone {
	case analyzer.ToneGood:
		return "border-grade-clean bg-grade-clean-soft text-grade-clean-ink", "bg-grade-clean"
	case analyzer.ToneWarn:
		return "border-warning bg-warning-soft text-warning-ink", "bg-warning"
	case analyzer.ToneBad:
		return "border-danger bg-danger-soft text-danger-ink", "bg-danger"
	default:
		return "border-line-strong bg-surface-2 text-ink-muted", "bg-ink-subtle"
	}
}

func collectWhyBullets(pillars []analyzer.PillarScore) []string {
	const maxWhyBullets = 3

	ordered := slices.Clone(pillars)
	slices.SortStableFunc(ordered, func(a, b analyzer.PillarScore) int {
		aActive := hasSpecificReason(a)
		bActive := hasSpecificReason(b)
		if aActive != bActive {
			if aActive {
				return -1
			}
			return 1
		}
		return cmp.Compare(pillarImpact(b), pillarImpact(a))
	})

	specific := make([]string, 0, maxWhyBullets)
	fallback := make([]string, 0, maxWhyBullets)
	seen := make(map[string]struct{}, len(pillars)*2)

	for _, p := range ordered {
		for _, r := range p.Reasons {
			text := strings.TrimSpace(r)
			if text == "" {
				continue
			}
			if _, dup := seen[text]; dup {
				continue
			}
			seen[text] = struct{}{}
			if isNeutralBoilerplate(text) {
				if len(fallback) < maxWhyBullets {
					fallback = append(fallback, text)
				}
				continue
			}
			specific = append(specific, text)
			if len(specific) == maxWhyBullets {
				return specific
			}
		}
	}

	if len(specific) > 0 {
		return specific
	}
	return fallback
}

func pillarImpact(p analyzer.PillarScore) int {
	diff := p.Score - 75
	if diff < 0 {
		return -diff
	}
	return diff
}

func hasSpecificReason(p analyzer.PillarScore) bool {
	for _, r := range p.Reasons {
		text := strings.TrimSpace(r)
		if text != "" && !isNeutralBoilerplate(text) {
			return true
		}
	}
	return false
}

func isNeutralBoilerplate(reason string) bool {
	switch reason {
	case "Neutral single-leg game script.",
		"No injury, weather, or trap flags active.",
		"Clean context with no injury, weather, or trap flags.",
		"Clean context across all legs.",
		"Legs span separate games, so they settle independently.",
		"Normal week-to-week variance for this market.",
		"Balanced leg stability across the ticket.":
		return true
	default:
		return false
	}
}

func formatCorrSummary(corrMult float64) string {
	switch {
	case corrMult <= 0:
		return ""
	case corrMult >= 1.05:
		return fmt.Sprintf("Script multiplier: %.2fx (legs rise together)", corrMult)
	case corrMult >= 1.02:
		return fmt.Sprintf("Script multiplier: %.2fx (mild positive fit)", corrMult)
	case corrMult <= 0.90:
		return fmt.Sprintf("Script multiplier: %.2fx (legs pull apart)", corrMult)
	case corrMult <= 0.97:
		return fmt.Sprintf("Script multiplier: %.2fx (mild script drag)", corrMult)
	default:
		return ""
	}
}

func guidePillars(cfg analyzer.V2Config) []GuidePillarView {
	return []GuidePillarView{
		{
			Key:   analyzer.PillarPriceVig,
			Label: analyzer.PillarLabelPriceVig,
			LegWeight: fmt.Sprintf(
				"%d%% (%d%% w/ script)",
				pctInt(cfg.LegWeightPrice),
				pctInt(cfg.LegScriptWeightPrice),
			),
			SlipWeight: fmt.Sprintf("%d%%", pctInt(cfg.SlipWeightPrice)),
			Summary:    "Checks retail odds against Power-devigged sharp consensus and calibrated fair value.",
			Points: []string{
				"Power de-vigging accounts for favorite-longshot bias in player props and Anytime TD lines.",
				"Flags steep vig, line fades vs. sharp/consensus books, heavy chalk (-230 or shorter), and parlay juice drag.",
				"Detects SGP correlation tax when a stack fits the script but the book's leg prices already charge for it.",
			},
		},
		{
			Key:        analyzer.PillarScriptFit,
			Label:      analyzer.PillarLabelScriptFit,
			LegWeight:  fmt.Sprintf("%d%% (when active)", pctInt(cfg.LegScriptWeightScript)),
			SlipWeight: fmt.Sprintf("%d%%", pctInt(cfg.SlipWeightScript)),
			Summary:    "Scores same-game correlation with a deterministic Gaussian copula plus single-leg script alignment.",
			Points: []string{
				"Estimates joint slip probability across same-game legs and compares it against independent leg multiplication.",
				"Cross-game legs settle independently; single-leg props still check spread and total fit.",
				"Severe script contradictions (like trailing RB overs or total-Under conflicts) cap the overall slip grade.",
			},
		},
		{
			Key:        analyzer.PillarStability,
			Label:      analyzer.PillarLabelStability,
			LegWeight:  fmt.Sprintf("%d%%", pctInt(cfg.LegWeightStability)),
			SlipWeight: fmt.Sprintf("%d%%", pctInt(cfg.SlipWeightStability)),
			Summary:    "Measures week-to-week repeatability using calibrated hit probability and outcome distribution shape.",
			Points: []string{
				"Models yardage curves (Zero-Adjusted Gamma/Log-Normal) and count stats (Negative Binomial) for tail risk.",
				"Flags high-swing Anytime TDs, longshot alt lines, and early-season samples shrunk toward role priors.",
				"Captures compounding miss risk as parlay leg count rises or multiple chalk legs stack up.",
			},
		},
		{
			Key:   analyzer.PillarContextTraps,
			Label: analyzer.PillarLabelContextTraps,
			LegWeight: fmt.Sprintf(
				"%d%% (%d%% w/ script)",
				pctInt(cfg.LegWeightContext),
				pctInt(cfg.LegScriptWeightContext),
			),
			SlipWeight: fmt.Sprintf("%d%%", pctInt(cfg.SlipWeightContext)),
			Summary:    "Applies deterministic overlays for injuries, weather, key numbers, role shifts, and trap lines.",
			Points: []string{
				"Covers player injury status, teammate-out volume bumps, wind headwinds, hooks, nfelo QB drag, and shrinking roles.",
				"Catches narrative hot-streak traps where the line has moved past recent medians, plus wide model-vs-market gaps.",
				"Consumes structured external context signals (late inactives, news, line moves) without retraining the model.",
			},
		},
	}
}

func pctInt(weight float64) int {
	return int(math.Round(weight * 100))
}

func formatLegWeights(cfg analyzer.V2Config) string {
	return fmt.Sprintf(
		"Price & Vig %d%%, Stability & Tail Risk %d%%, Context & Traps %d%%",
		pctInt(cfg.LegWeightPrice),
		pctInt(cfg.LegWeightStability),
		pctInt(cfg.LegWeightContext),
	)
}

func formatLegScriptWeights(cfg analyzer.V2Config) string {
	return fmt.Sprintf(
		"Price & Vig %d%%, Script Fit %d%%, Stability & Tail Risk %d%%, Context & Traps %d%%",
		pctInt(cfg.LegScriptWeightPrice),
		pctInt(cfg.LegScriptWeightScript),
		pctInt(cfg.LegScriptWeightStability),
		pctInt(cfg.LegScriptWeightContext),
	)
}

func formatSlipWeights(cfg analyzer.V2Config) string {
	return fmt.Sprintf(
		"Price & Vig %d%%, Script Fit %d%%, Stability & Tail Risk %d%%, Context & Traps %d%%",
		pctInt(cfg.SlipWeightPrice),
		pctInt(cfg.SlipWeightScript),
		pctInt(cfg.SlipWeightStability),
		pctInt(cfg.SlipWeightContext),
	)
}

func gradeClass(letter string) string {
	switch letter {
	case "A":
		return "grade-a"
	case "B":
		return "grade-b"
	case "C":
		return "grade-c"
	case "D":
		return "grade-d"
	case "F":
		return "grade-f"
	default:
		return "grade-neutral"
	}
}

// legModel is the nfelo chip beside a leg's odds. A spread shows the model's
// number from the leg team's side; a moneyline shows the model's win
// probability. Anything else, or a game without model data, has no chip.
func legModel(leg db.Leg, games map[string]analyzer.Game) string {
	if leg.Team == nil || leg.GameID == nil {
		return ""
	}
	game, ok := games[*leg.GameID]
	if !ok || game.Nfelo == nil {
		return ""
	}
	switch leg.MarketType {
	case "SPREAD":
		spread, ok := game.ModelSpread(*leg.Team)
		if !ok {
			return ""
		}
		return "nfelo " + formatModelSpread(spread)
	case "ML":
		probability, ok := game.ModelWinProb(*leg.Team)
		if !ok {
			return ""
		}
		return fmt.Sprintf("nfelo %d%%", int(math.Round(probability*100)))
	default:
		return ""
	}
}

// formatModelSpread prints a model spread with one decimal and a plus on a
// positive number.
func formatModelSpread(spread float64) string {
	text := strconv.FormatFloat(spread, 'f', 1, 64)
	if spread > 0 {
		return "+" + text
	}
	return text
}

// analyzerSeverity names the card's heading and the classes that color it.
func analyzerSeverity(severity analyzer.Severity) (string, string) {
	switch severity {
	case analyzer.Danger:
		return "Danger", "sev-danger"
	case analyzer.Warn:
		return "Warn", "sev-warn"
	default:
		return "Info", "sev-info"
	}
}
