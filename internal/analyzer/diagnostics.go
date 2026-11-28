package analyzer

import (
	"cmp"
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/kalverra/parlay-club/internal/modelv2"
	"github.com/kalverra/parlay-club/internal/parlay"
)

// SignalPill keys, Layer 2 targets, and ContextNote priced-in tags.
const (
	PillConflictingLegs   = "conflicting_legs"
	PillHeavySGPTax       = "heavy_sgp_tax"
	PillSteepVig          = "steep_vig"
	PillLegBloat          = "leg_bloat"
	PillHeavyChalk        = "heavy_chalk"
	PillLongshotVariance  = "longshot_variance"
	PillPricedInNarrative = "priced_in_narrative"
	PillContextTrap       = "context_trap"
	PillCorrelatedStack   = "correlated_stack"
	PillSharpPrice        = "sharp_price"

	TargetSynergy = "synergy"
	TargetBookCut = "book_cut"
	TargetHitRate = "hit_rate"
	TargetContext = "context"

	TagAlreadyInLine  = "Already in the line"
	TagLikelyPricedIn = "Likely Priced In"

	maxSignalPills   = 3
	maxContextRadar  = 2
	nudgeChalkOdds   = -230
	longshotDescProb = 0.15
)

// SignalPill is a compact, clickable diagnostic badge for Layer 1/Layer 2 UI.
type SignalPill struct {
	Key    string // "conflicting_legs", "heavy_sgp_tax", "leg_bloat", "priced_in_narrative", ...
	Icon   string // compact symbol token
	Label  string // e.g. "Conflicting Legs", "Heavy SGP Tax", "Leg Bloat", "Priced-In Narrative"
	Tone   string // "warn" | "bad" | "info"
	Target string // Layer 2 diagnostic row anchor ("synergy", "book_cut", "hit_rate", "context")
	Detail string // 1-sentence plain-English summary for drawer focus
}

// ContextNote is a model/matchup context feature tagged as priced into the market line.
type ContextNote struct {
	Label    string // e.g. "Opponent allows top-5 TE yards"
	PricedIn string // e.g. "Vegas knows this; the line is already bumped up"
	Tag      string // "Already in the line" | "Likely Priced In"
}

// BuildDiagnostics holds structural build diagnostics and probability provenance
// for a leg or parlay slip.
type BuildDiagnostics struct {
	DisplayGrade     string        // e.g. "A-", "B", "C-", "D", "F"
	Descriptor       string        // 2-word reality check: "Tight Build", "Clean Build", "Longshot", "Vig Trap", "Self-Sabotage"
	VegasImpliedProb float64       // 1 / decimalOdds (or combined independent decimal odds)
	FairNoVigProb    float64       // Power-devigged fair prob when sharp/consensus exists (0 if unavailable)
	RawModelProb     float64       // standalone unblended model prob (0 when no model ran)
	HoldPct          float64       // estimated sportsbook vig/hold %
	CorrTaxPct       float64       // SGP payout shave vs. independent math (0 when not an SGP or no tax)
	SignalPills      []SignalPill  // capped at 3, ordered by severity
	ContextRadar     []ContextNote // capped at 2, sourced from top model/matchup features
	CleanupNudge     string        // optional 1-line concrete structural improvement
}

// ProcessOutcome classifies a settled slip or leg along the Process (A-B vs C-F)
// and Outcome (Won vs Lost) 2x2 matrix.
type ProcessOutcome struct {
	Key     string // "clean_hit", "tough_beat", "total_bailout", "book_donation", or ""
	Label   string // "Clean Hit", "Tough Beat", "Total Bailout", "Book Donation"
	Tone    string // "positive", "neutral", "warn", "bad"
	Summary string // plain-English takeaway
}

// ClassifyProcessOutcome maps a pre-kickoff grade letter and settled outcome
// onto the 2x2 Process vs. Outcome matrix. Returns the zero value for
// ungraded letters.
func ClassifyProcessOutcome(letter string, won bool) ProcessOutcome {
	norm := strings.ToUpper(strings.TrimSpace(letter))
	norm = strings.TrimRight(norm, "+-")
	switch norm {
	case "A", "B":
		if won {
			return ProcessOutcome{
				Key:     "clean_hit",
				Label:   "Clean Hit",
				Tone:    "positive",
				Summary: "Solid math, paid off.",
			}
		}
		return ProcessOutcome{
			Key:     "tough_beat",
			Label:   "Tough Beat",
			Tone:    "neutral",
			Summary: "Good process, bad variance. Don't tilt.",
		}
	case "C", "D", "F":
		if won {
			return ProcessOutcome{
				Key:     "total_bailout",
				Label:   "Total Bailout",
				Tone:    "warn",
				Summary: "Awful structure, got lucky. Enjoy the cash.",
			}
		}
		return ProcessOutcome{
			Key:     "book_donation",
			Label:   "Book Donation",
			Tone:    "bad",
			Summary: "The exact trap we flagged hit.",
		}
	default:
		return ProcessOutcome{}
	}
}

// ClassifyProcessResult maps a pre-kickoff grade letter and settled parlay.Result
// onto the 2x2 Process vs. Outcome matrix. Pushed, voided, pending, and
// ungraded slips or legs return the zero value.
func ClassifyProcessResult(letter string, result parlay.Result) ProcessOutcome {
	switch result {
	case parlay.Won:
		return ClassifyProcessOutcome(letter, true)
	case parlay.Lost:
		return ClassifyProcessOutcome(letter, false)
	default:
		return ProcessOutcome{}
	}
}

func displayGradeForScore(score int, cfg V2Config) string {
	letter := letterForScore(score, cfg)
	switch letter {
	case "A":
		return subdivideBand(score, cfg.BandA, 100, "A")
	case "B":
		return subdivideBand(score, cfg.BandB, cfg.BandA-1, "B")
	case "C":
		return subdivideBand(score, cfg.BandC, cfg.BandB-1, "C")
	case "D":
		return subdivideBand(score, cfg.BandD, cfg.BandC-1, "D")
	default:
		return "F"
	}
}

func subdivideBand(score, lo, hi int, base string) string {
	span := hi - lo
	if span < 3 {
		return base
	}
	third := float64(span+1) / 3.0
	if float64(score-lo) >= 2.0*third {
		return base + "+"
	}
	if float64(score-lo) >= third {
		return base
	}
	return base + "-"
}

func computeHoldPct(vegasImplied, fairRef float64, sharp SharpQuote, hasSharp bool) float64 {
	if vegasImplied > 0 && fairRef > 0 {
		hold := (1.0 - fairRef/vegasImplied) * 100.0
		if hold > 0 {
			return math.Round(hold*10) / 10
		}
		return 0
	}
	if hasSharp && sharp.VigPct > 0 {
		return math.Round(sharp.VigPct*1000) / 10
	}
	return 0
}

func buildDescriptor(
	letter string,
	score int,
	pricePillar, scriptPillar, stabPillar, ctxPillar PillarScore,
	hasDangerConflict, sgpTaxed bool,
	calibratedProb, vegasImpliedProb float64,
) string {
	switch {
	case hasDangerConflict:
		return "Self-Sabotage"
	case sgpTaxed && pricePillar.Score < 68, pricePillar.Score < 56:
		return "Vig Trap"
	case ctxPillar.Score < 56:
		return "Trap Risk"
	case isLongshotDescriptor(stabPillar.Score, calibratedProb, vegasImpliedProb):
		return "Longshot"
	}
	return buildBandDescriptor(letter, score, pricePillar, scriptPillar, stabPillar, ctxPillar)
}

func isLongshotDescriptor(stabScore int, calibratedProb, vegasImpliedProb float64) bool {
	if vegasImpliedProb > 0 && vegasImpliedProb < longshotDescProb {
		return true
	}
	if calibratedProb > 0 && calibratedProb < longshotDescProb {
		return true
	}
	return stabScore < 60 && calibratedProb > 0 && calibratedProb < v2LongshotProbMax
}

func buildBandDescriptor(
	letter string,
	score int,
	pricePillar, scriptPillar, stabPillar, ctxPillar PillarScore,
) string {
	switch letter {
	case "A":
		if score >= 90 {
			return "Clean Build"
		}
		return "Tight Build"
	case "B":
		if pricePillar.Score < 68 {
			return "Price Drag"
		}
		if stabPillar.Score < 68 {
			return "High Swing"
		}
		return "Solid Build"
	case "C":
		if pricePillar.Score <= min(scriptPillar.Score, min(stabPillar.Score, ctxPillar.Score)) &&
			pricePillar.Score < 68 {
			return "Vig Drag"
		}
		if stabPillar.Score < 68 {
			return "Fragile Build"
		}
		return "Mixed Build"
	case "D":
		if pricePillar.Score <= min(scriptPillar.Score, min(stabPillar.Score, ctxPillar.Score)) {
			return "Vig Trap"
		}
		if scriptPillar.Score <= min(stabPillar.Score, ctxPillar.Score) {
			return "Script Clash"
		}
		return "Fragile Build"
	default:
		if scriptPillar.Score < 50 {
			return "Self-Sabotage"
		}
		if pricePillar.Score < 50 {
			return "Vig Trap"
		}
		return "Broken Build"
	}
}

func pillToneRank(tone string) int {
	switch tone {
	case ToneBad:
		return 0
	case ToneWarn:
		return 1
	default:
		return 2
	}
}

func capSignalPills(pills []SignalPill) []SignalPill {
	if len(pills) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(pills))
	deduped := make([]SignalPill, 0, len(pills))
	for _, p := range pills {
		if _, ok := seen[p.Key]; ok {
			continue
		}
		seen[p.Key] = struct{}{}
		deduped = append(deduped, p)
	}
	slices.SortStableFunc(deduped, func(a, b SignalPill) int {
		return cmp.Compare(pillToneRank(a.Tone), pillToneRank(b.Tone))
	})
	if len(deduped) > maxSignalPills {
		deduped = deduped[:maxSignalPills]
	}
	return deduped
}

func buildLegDiagnostics(
	ev v2LegEval,
	rawModelProb, vegasImpliedProb, fairNoVigProb float64,
	cfg V2Config,
) BuildDiagnostics {
	g := ev.grade
	fairRef := fairNoVigProb
	if fairRef <= 0 {
		fairRef = g.CalibratedProb
	}
	holdPct := computeHoldPct(vegasImpliedProb, fairRef, ev.sharp, ev.hasSharp)

	hasDanger := false
	for _, f := range ev.findings {
		if f.Severity >= Danger {
			hasDanger = true
			break
		}
	}

	var pills []SignalPill
	if ev.pricePillar.Score < 56 {
		pills = append(pills, SignalPill{
			Key:    PillSteepVig,
			Icon:   "$",
			Label:  "Steep Vig",
			Tone:   ToneBad,
			Target: TargetBookCut,
			Detail: ev.pricePillar.Summary,
		})
	}
	if isChalkLeg(ev.leg) {
		pills = append(pills, SignalPill{
			Key:    PillHeavyChalk,
			Icon:   "!",
			Label:  "Heavy Chalk",
			Tone:   ToneWarn,
			Target: TargetHitRate,
			Detail: fmt.Sprintf("Heavy %d price adds little payout for the upset risk.", ev.leg.Odds),
		})
	}
	if g.CalibratedProb > 0 && g.CalibratedProb < v2LongshotProbMax {
		tone := ToneWarn
		if g.Score >= cfg.BandB {
			tone = "info"
		}
		pills = append(pills, SignalPill{
			Key:    PillLongshotVariance,
			Icon:   "~",
			Label:  "Longshot Odds",
			Tone:   tone,
			Target: TargetHitRate,
			Detail: fmt.Sprintf(
				"Low hit rate (~%d%%) swings widely week to week.",
				int(math.Round(g.CalibratedProb*100)),
			),
		})
	}
	pills = appendFindingPills(pills, ev.findings)
	if ev.pricePillar.Score >= 85 {
		pills = append(pills, SignalPill{
			Key:    PillSharpPrice,
			Icon:   "+",
			Label:  "Clean Price",
			Tone:   "info",
			Target: TargetBookCut,
			Detail: ev.pricePillar.Summary,
		})
	}

	radar := buildContextRadar([]v2LegEval{ev})

	var nudge string
	if ev.leg.Odds != 0 && ev.pricePillar.Score < 55 {
		nudge = "Shop this line for a better price—paying steep juice hurts long-term return."
	}

	return BuildDiagnostics{
		DisplayGrade: displayGradeForScore(g.Score, cfg),
		Descriptor: buildDescriptor(
			g.Letter,
			g.Score,
			ev.pricePillar,
			ev.scriptPillar,
			ev.stabPillar,
			ev.ctxPillar,
			hasDanger,
			false,
			g.CalibratedProb,
			vegasImpliedProb,
		),
		VegasImpliedProb: vegasImpliedProb,
		FairNoVigProb:    fairNoVigProb,
		RawModelProb:     rawModelProb,
		HoldPct:          holdPct,
		CorrTaxPct:       0,
		SignalPills:      capSignalPills(pills),
		ContextRadar:     radar,
		CleanupNudge:     nudge,
	}
}

func buildSingleLegSlipDiagnostics(only v2LegEval) BuildDiagnostics {
	diag := only.grade.Diagnostics
	diag.SignalPills = slices.Clone(diag.SignalPills)
	diag.ContextRadar = slices.Clone(diag.ContextRadar)
	return diag
}

func (e *V2Engine) buildMultiLegSlipDiagnostics(
	legs []Leg,
	evals []v2LegEval,
	gradedIdxs []int,
	warnings []Warning,
	jointEst modelv2.JointParlayEstimate,
	hasJoint bool,
	score int,
	letter string,
	jointProb, corrMult, vegasImpliedProb, fairNoVigProb, rawModelProb, combinedDec float64,
	hasAllOdds, sgpTaxed, hasDangerConflict, hasWarnConflict bool,
	chalkCount int,
	pricePillar, scriptPillar, stabPillar, ctxPillar PillarScore,
) BuildDiagnostics {
	fairRef := fairNoVigProb
	if fairRef <= 0 {
		fairRef = jointProb
	}
	holdPct := computeHoldPct(vegasImpliedProb, fairRef, SharpQuote{}, false)

	corrTaxPct := 0.0
	if sgpTaxed && corrMult > 1.0 {
		corrTaxPct = math.Round((1.0-1.0/corrMult)*1000) / 10
	}

	var pills []SignalPill
	if hasDangerConflict || corrMult < 0.85 {
		pills = append(pills, SignalPill{
			Key:    PillConflictingLegs,
			Icon:   "!",
			Label:  "Conflicting Legs",
			Tone:   ToneBad,
			Target: TargetSynergy,
			Detail: scriptPillar.Summary,
		})
	} else if hasWarnConflict || corrMult < 0.95 {
		pills = append(pills, SignalPill{
			Key:    PillConflictingLegs,
			Icon:   "!",
			Label:  "Conflicting Legs",
			Tone:   ToneWarn,
			Target: TargetSynergy,
			Detail: scriptPillar.Summary,
		})
	}

	if sgpTaxed {
		tone := ToneWarn
		if pricePillar.Score < 56 {
			tone = ToneBad
		}
		pills = append(pills, SignalPill{
			Key:    PillHeavySGPTax,
			Icon:   "%",
			Label:  "Heavy SGP Tax",
			Tone:   tone,
			Target: TargetBookCut,
			Detail: "Book prices already tax this same-game correlation.",
		})
	} else if pricePillar.Score < 56 {
		pills = append(pills, SignalPill{
			Key:    PillSteepVig,
			Icon:   "$",
			Label:  "Steep Vig",
			Tone:   ToneBad,
			Target: TargetBookCut,
			Detail: pricePillar.Summary,
		})
	}

	if len(gradedIdxs) > e.cfg.ParlayFreeLegs {
		pills = append(pills, SignalPill{
			Key:    PillLegBloat,
			Icon:   "#",
			Label:  "Leg Bloat",
			Tone:   ToneWarn,
			Target: TargetHitRate,
			Detail: fmt.Sprintf("%d legs compound book juice and miss risk.", len(gradedIdxs)),
		})
	}

	if chalkCount > 1 {
		pills = append(pills, SignalPill{
			Key:    PillHeavyChalk,
			Icon:   "!",
			Label:  "Heavy Chalk",
			Tone:   ToneWarn,
			Target: TargetHitRate,
			Detail: fmt.Sprintf("%d heavy-favorite legs compound upset risk for modest payout.", chalkCount),
		})
	}

	gradedEvals := make([]v2LegEval, 0, len(gradedIdxs))
	var allFindings []OverlayFinding
	for _, idx := range gradedIdxs {
		gradedEvals = append(gradedEvals, evals[idx])
		allFindings = append(allFindings, evals[idx].findings...)
	}
	pills = appendFindingPills(pills, allFindings)

	if jointProb > 0 && jointProb < longshotDescProb {
		tone := ToneWarn
		if score >= e.cfg.BandB {
			tone = "info"
		}
		pills = append(pills, SignalPill{
			Key:    PillLongshotVariance,
			Icon:   "~",
			Label:  "Longshot Odds",
			Tone:   tone,
			Target: TargetHitRate,
			Detail: fmt.Sprintf("Combined hit rate (~%d%%) is a longshot ticket.", int(math.Round(jointProb*100))),
		})
	}

	if corrMult >= 1.08 && !sgpTaxed && !hasDangerConflict && !hasWarnConflict {
		pills = append(pills, SignalPill{
			Key:    PillCorrelatedStack,
			Icon:   "+",
			Label:  "Correlated Stack",
			Tone:   "info",
			Target: TargetSynergy,
			Detail: scriptPillar.Summary,
		})
	}
	if pricePillar.Score >= 85 && !sgpTaxed {
		pills = append(pills, SignalPill{
			Key:    PillSharpPrice,
			Icon:   "+",
			Label:  "Clean Price",
			Tone:   "info",
			Target: TargetBookCut,
			Detail: pricePillar.Summary,
		})
	}

	radar := buildContextRadar(gradedEvals)
	nudge := buildSlipCleanupNudge(
		legs, evals, gradedIdxs, warnings, jointEst, hasJoint,
		combinedDec, hasAllOdds, sgpTaxed, hasDangerConflict, corrMult, pricePillar, e.cfg,
	)

	return BuildDiagnostics{
		DisplayGrade: displayGradeForScore(score, e.cfg),
		Descriptor: buildDescriptor(
			letter,
			score,
			pricePillar,
			scriptPillar,
			stabPillar,
			ctxPillar,
			hasDangerConflict || corrMult < 0.85,
			sgpTaxed,
			jointProb,
			vegasImpliedProb,
		),
		VegasImpliedProb: vegasImpliedProb,
		FairNoVigProb:    fairNoVigProb,
		RawModelProb:     rawModelProb,
		HoldPct:          holdPct,
		CorrTaxPct:       corrTaxPct,
		SignalPills:      capSignalPills(pills),
		ContextRadar:     radar,
		CleanupNudge:     nudge,
	}
}

func appendFindingPills(pills []SignalPill, findings []OverlayFinding) []SignalPill {
	for _, f := range findings {
		switch {
		case f.Rule == ruleNarrativeStreakTrap:
			pills = append(pills, SignalPill{
				Key:    PillPricedInNarrative,
				Icon:   "!",
				Label:  "Priced-In Narrative",
				Tone:   ToneWarn,
				Target: TargetContext,
				Detail: f.Detail,
			})
		case f.Severity >= Danger:
			pills = append(pills, SignalPill{
				Key:    PillContextTrap,
				Icon:   "!",
				Label:  f.Label,
				Tone:   ToneBad,
				Target: TargetContext,
				Detail: f.Detail,
			})
		case f.Severity == Warn && f.ScoreDelta < 0:
			pills = append(pills, SignalPill{
				Key:    PillContextTrap + "_" + f.Rule,
				Icon:   "!",
				Label:  f.Label,
				Tone:   ToneWarn,
				Target: TargetContext,
				Detail: f.Detail,
			})
		}
	}
	return pills
}

func buildContextRadar(evals []v2LegEval) []ContextNote {
	notes := make([]ContextNote, 0, maxContextRadar)
	seen := make(map[string]struct{}, maxContextRadar)

	addNote := func(n ContextNote) {
		if len(notes) >= maxContextRadar {
			return
		}
		if _, dup := seen[n.Label]; dup {
			return
		}
		seen[n.Label] = struct{}{}
		notes = append(notes, n)
	}

	// 1. Prioritize active overlay findings.
	var findings []OverlayFinding
	for _, ev := range evals {
		findings = append(findings, ev.findings...)
	}
	slices.SortStableFunc(findings, func(a, b OverlayFinding) int {
		return cmp.Compare(b.Severity, a.Severity)
	})
	for _, f := range findings {
		tag := TagAlreadyInLine
		if f.Rule == ruleNarrativeStreakTrap || f.Rule == ruleModelMarketDivergence {
			tag = TagLikelyPricedIn
		}
		addNote(ContextNote{
			Label:    f.Label,
			PricedIn: f.Detail,
			Tag:      tag,
		})
	}

	// 2. Fill remaining slots with top matchup/game features already priced into lines.
	for _, ev := range evals {
		if len(notes) >= maxContextRadar {
			break
		}
		if ev.game.Weather != nil && ev.game.Weather.WindMPH >= 15 && !ev.game.Indoor {
			addNote(ContextNote{
				Label:    fmt.Sprintf("%d mph wind forecast", int(math.Round(ev.game.Weather.WindMPH))),
				PricedIn: "Books shade passing totals and props when wind spikes.",
				Tag:      TagAlreadyInLine,
			})
		}
		if ev.game.MarketScript.HasTotal && ev.game.MarketScript.GameTotal >= 48.0 {
			addNote(ContextNote{
				Label:    fmt.Sprintf("High %s game total", number(ev.game.MarketScript.GameTotal)),
				PricedIn: "Shootout scoring expectations are already baked into the lines.",
				Tag:      TagAlreadyInLine,
			})
		}
		if roleDef := ev.game.OpponentRoleDefense(ev.leg.Team, ev.role); roleDef != nil && roleDef.Games >= 3 {
			opp := opponentTeamCode(ev.leg, ev.game)
			if roleDef.TotalEPAAllowed > 0.05 {
				addNote(ContextNote{
					Label:    fmt.Sprintf("Soft %s matchup vs. %s", ev.role, opp),
					PricedIn: "Vegas knows the defense rank; the prop line is already bumped up.",
					Tag:      TagAlreadyInLine,
				})
			} else if roleDef.TotalEPAAllowed < -0.05 {
				addNote(ContextNote{
					Label:    fmt.Sprintf("Tough %s matchup vs. %s", ev.role, opp),
					PricedIn: "Matchup difficulty is already reflected in the posted line.",
					Tag:      TagAlreadyInLine,
				})
			}
		}
		if ev.hasSharp && ev.sharp.Line != nil {
			addNote(ContextNote{
				Label: fmt.Sprintf(
					"%s benchmark at %s",
					sharpSourceLabel(ev.sharp),
					sharpLineDisplay(normalizeSharpMarket(ev.leg.Market), ev.sharp.Line),
				),
				PricedIn: "Sharp consensus already prices in public team and injury news.",
				Tag:      TagAlreadyInLine,
			})
		}
	}

	return notes
}

func opponentTeamCode(leg Leg, game Game) string {
	if strings.EqualFold(leg.Team, game.Home) {
		return game.Away
	}
	return game.Home
}

func buildSlipCleanupNudge(
	legs []Leg,
	evals []v2LegEval,
	gradedIdxs []int,
	warnings []Warning,
	jointEst modelv2.JointParlayEstimate,
	hasJoint bool,
	combinedDec float64,
	hasAllOdds, sgpTaxed, hasDangerConflict bool,
	corrMult float64,
	pricePillar PillarScore,
	cfg V2Config,
) string {
	if hasDangerConflict || corrMult < 0.85 {
		if hasJoint && len(jointEst.PairCorrelations) > 0 {
			for _, pair := range jointEst.PairCorrelations {
				if pair.Correlation <= -0.15 {
					legA, okA := resolveGradedLeg(legs, gradedIdxs, pair.LegAIndex)
					legB, okB := resolveGradedLeg(legs, gradedIdxs, pair.LegBIndex)
					if okA && okB {
						return fmt.Sprintf(
							"Split %s and %s—they pull in opposite game scripts.",
							shortLegLabel(legA),
							shortLegLabel(legB),
						)
					}
				}
			}
		}
		for _, w := range warnings {
			if w.Rule == ruleTrailingRB || w.Rule == ruleTotalContradiction {
				return "Drop one side of the conflicting script so the legs don't fight each other."
			}
		}
		return "Drop one conflicting leg so the ticket doesn't need opposite scripts to hit."
	}

	// Inspect heaviest chalk leg (<= -230) where dropping it removes a failure point.
	if hasAllOdds && combinedDec > 1 && len(gradedIdxs) >= 2 {
		worstChalkIdx := -1
		worstOdds := 0
		for _, idx := range gradedIdxs {
			odds := evals[idx].leg.Odds
			if odds != 0 && odds <= nudgeChalkOdds && (worstChalkIdx == -1 || odds < worstOdds) {
				worstChalkIdx = idx
				worstOdds = odds
			}
		}
		if worstChalkIdx >= 0 {
			dec := parlay.AmericanToDecimal(worstOdds)
			if dec > 1 {
				pctCut := max(1, int(math.Round((1.0-1.0/dec)*100)))
				return fmt.Sprintf(
					"Dropping the %d leg cuts your payout by %d%% but removes an extra way to lose.",
					worstOdds,
					pctCut,
				)
			}
		}
	}

	if sgpTaxed {
		return "Drop the correlated add-on leg or shop another book—this SGP price already taxes the stack."
	}
	if len(gradedIdxs) > cfg.ParlayFreeLegs+1 {
		return fmt.Sprintf(
			"Trimming this %d-leg ticket to 2-3 core legs cuts compounding vig and miss risk.",
			len(gradedIdxs),
		)
	}
	if pricePillar.Score < 55 {
		return "Shop for better odds on the steepest leg—paying heavy juice drags the whole ticket."
	}
	return ""
}
