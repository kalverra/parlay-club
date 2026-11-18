package web

import (
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/kalverra/parlay-club/internal/analyzer"
	"github.com/kalverra/parlay-club/internal/db"
)

// AnalyzerView is the sharp-friend banner under the slip's legs.
type AnalyzerView struct {
	Clean    bool
	Grade    GradeView
	Warnings []AnalyzerWarningView
}

// GradeView is a clickable A-F grade pill with an inline factor breakdown.
type GradeView struct {
	Letter  string
	Score   int
	Class   string
	Summary string
	Factors []GradeFactorView
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
	BackURL    string
	Rubric     analyzer.RubricDoc
	Bands      []GuideBandView
	Categories []GuideRuleCategoryView
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

// analyzerGuide serves GET /analyzer/guide, rendering the live analyzer rules
// and A-F grading rubric.
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

	page := h.page(r, "Grading & Analyzer Guide")
	page.Tab = ""
	h.renderPage(w, "analyzer-guide", AnalyzerGuidePage{
		Page:       page,
		BackURL:    backURL,
		Rubric:     rubric,
		Bands:      bands,
		Categories: categories,
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
// the slip banner and each leg row.
func analyzerView(
	legs []db.Leg,
	gamesByID map[string]analyzer.Game,
	stats map[string]analyzer.PlayerStats,
	names map[int64]string,
) (AnalyzerView, map[int64]GradeView) {
	input := make([]analyzer.Leg, 0, len(legs))
	legsByID := make(map[int64]db.Leg, len(legs))
	for _, leg := range legs {
		legsByID[leg.ID] = leg
		input = append(input, analyzer.LegFromDB(leg, names[leg.UserID]))
	}

	warnings := analyzer.AnalyzeSlip(input, gamesByID, stats)
	slipGrade := analyzer.ScoreSlip(input, gamesByID, stats)

	legGrades := make(map[int64]GradeView, len(slipGrade.LegGrades))
	for _, lg := range slipGrade.LegGrades {
		if lg.Letter == "" {
			continue
		}
		legGrades[lg.LegID] = gradeView(lg.Letter, lg.Score, "", lg.Factors)
	}

	view := AnalyzerView{
		Clean: len(warnings) == 0,
		Grade: gradeView(slipGrade.Letter, slipGrade.Score, slipGrade.Summary, slipGrade.Factors),
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

func gradeView(letter string, score int, summary string, factors []analyzer.GradeFactor) GradeView {
	if letter == "" {
		return GradeView{}
	}
	factorViews := make([]GradeFactorView, 0, len(factors))
	for _, f := range factors {
		factorViews = append(factorViews, GradeFactorView{
			Rule:      f.Rule,
			Label:     f.Label,
			DeltaText: fmt.Sprintf("%+d", f.Delta),
			Positive:  f.Delta > 0,
			Negative:  f.Delta < 0,
			Detail:    f.Detail,
		})
	}
	return GradeView{
		Letter:  letter,
		Score:   score,
		Class:   gradeClass(letter),
		Summary: summary,
		Factors: factorViews,
	}
}

func gradeClass(letter string) string {
	switch letter {
	case "A":
		return "border-emerald-700 bg-emerald-950 text-emerald-300"
	case "B":
		return "border-sky-700 bg-sky-950 text-sky-300"
	case "C":
		return "border-amber-700 bg-amber-950 text-amber-300"
	case "D":
		return "border-orange-700 bg-orange-950 text-orange-300"
	case "F":
		return "border-rose-800 bg-rose-950 text-rose-300"
	default:
		return "border-slate-700 bg-slate-950 text-slate-300"
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
		return "Danger", "border-rose-800 bg-rose-950 text-rose-200"
	case analyzer.Warn:
		return "Warn", "border-amber-700 bg-amber-950 text-amber-300"
	default:
		return "Info", "border-sky-700 bg-sky-950 text-sky-300"
	}
}
