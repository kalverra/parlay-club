package analyzer

import (
	"cmp"
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/modelv2"
	"github.com/kalverra/parlay-club/internal/parlay"
)

// Canonical v2 pillar keys, labels, and tone classifications.
const (
	PillarPriceVig     = "price_vig"
	PillarScriptFit    = "script_fit"
	PillarStability    = "stability_risk"
	PillarContextTraps = "context_traps"

	PillarLabelPriceVig     = "Price & Vig"
	PillarLabelScriptFit    = "Script Fit"
	PillarLabelStability    = "Stability & Tail Risk"
	PillarLabelContextTraps = "Context & Traps"

	ToneGood    = "good"
	ToneNeutral = "neutral"
	ToneWarn    = "warn"
	ToneBad     = "bad"
)

const (
	ruleScriptFit = "script_fit"
	ruleStability = "stability_risk"
	rulePriceVig  = "price_vig"
	ruleSGPTax    = "sgp_tax"

	v2StackFeatureCap = 64

	v2SpreadStdDev    = 13.45
	v2GameTotalStdDev = 13.80
	v2TeamTotalStdDev = 9.80

	v2NeutralScore    = 75
	v2StabilityBase   = 82
	v2CapWarnMax      = 84
	v2CapDangerMax    = 72
	v2SevereVigFloor  = 45
	v2LongshotProbMax = 0.25
	v2LowProbMax      = 0.38
	v2HighProbMin     = 0.56
	v2AssumedVig      = 0.045

	v2MinClampedProb = 0.01
	v2MaxClampedProb = 0.99
	v2FallbackProb   = 0.50
)

// V2Config holds tunable weights, thresholds, and band cutoffs for EngineV2.
type V2Config struct {
	Name string

	LegWeightPrice     float64
	LegWeightStability float64
	LegWeightContext   float64

	LegScriptWeightPrice     float64
	LegScriptWeightScript    float64
	LegScriptWeightStability float64
	LegScriptWeightContext   float64

	SlipWeightPrice     float64
	SlipWeightScript    float64
	SlipWeightStability float64
	SlipWeightContext   float64

	GameLineMarketBlend float64
	SpreadMarginStdDev  float64
	GameTotalStdDev     float64
	TeamTotalStdDev     float64

	ParlayFreeLegs    int
	ParlayVigDragStep int
	SGPCorrTaxMinMult float64
	SGPCorrTaxPenalty int

	BandA int
	BandB int
	BandC int
	BandD int
}

// V2Engine implements GradingEngine using calibrated v2 distributional prop
// models, Power-devigged market anchors, QMC Gaussian copula correlation, and
// pluggable ContextOverlay checks.
type V2Engine struct {
	cfg      V2Config
	bundle   *modelv2.ArtifactBundle
	overlays []ContextOverlay
}

type v2LegEval struct {
	grade        LegGrade
	leg          Leg
	game         Game
	role         RoleArchetype
	pred         *modelv2.LegPrediction
	findings     []OverlayFinding
	sharp        SharpQuote
	hasSharp     bool
	linesMatch   bool
	lineEdge     float64
	hasLineEdge  bool
	hasScriptSig bool
	pricePillar  PillarScore
	scriptPillar PillarScore
	stabPillar   PillarScore
	ctxPillar    PillarScore
}

// DefaultV2Config holds the default production tuning for EngineV2.
var DefaultV2Config = V2Config{
	Name: "v2",

	LegWeightPrice:     0.40,
	LegWeightStability: 0.25,
	LegWeightContext:   0.35,

	LegScriptWeightPrice:     0.30,
	LegScriptWeightScript:    0.15,
	LegScriptWeightStability: 0.25,
	LegScriptWeightContext:   0.30,

	SlipWeightPrice:     0.35,
	SlipWeightScript:    0.25,
	SlipWeightStability: 0.20,
	SlipWeightContext:   0.20,

	GameLineMarketBlend: 0.70,
	SpreadMarginStdDev:  v2SpreadStdDev,
	GameTotalStdDev:     v2GameTotalStdDev,
	TeamTotalStdDev:     v2TeamTotalStdDev,

	ParlayFreeLegs:    3,
	ParlayVigDragStep: 2,
	SGPCorrTaxMinMult: 1.05,
	SGPCorrTaxPenalty: 10,

	BandA: RubricV1.BandA,
	BandB: RubricV1.BandB,
	BandC: RubricV1.BandC,
	BandD: RubricV1.BandD,
}

// NewEngineV2 returns a V2Engine configured with cfg and optional custom
// ContextOverlay functions (defaulting to DefaultContextOverlays when empty),
// loading the embedded model artifact lazily on first evaluation.
func NewEngineV2(cfg V2Config, overlays ...ContextOverlay) *V2Engine {
	return &V2Engine{
		cfg:      cfg,
		overlays: slices.Clone(overlays),
	}
}

// NewEngineV2WithBundle returns a V2Engine bound to an explicit ArtifactBundle,
// useful for deterministic unit tests and ablation runs.
func NewEngineV2WithBundle(cfg V2Config, bundle *modelv2.ArtifactBundle, overlays ...ContextOverlay) *V2Engine {
	return &V2Engine{
		cfg:      cfg,
		bundle:   bundle,
		overlays: slices.Clone(overlays),
	}
}

// Name returns the engine identifier (default "v2").
func (e *V2Engine) Name() string {
	if e != nil && e.cfg.Name != "" {
		return e.cfg.Name
	}
	return DefaultV2Config.Name
}

// ScoreLeg computes the deterministic four-pillar v2 grade, calibrated
// probability, and backwards-compatible factors for a single leg.
func (e *V2Engine) ScoreLeg(leg Leg, games map[string]Game, stats map[string]PlayerStats) LegGrade {
	bundle := e.resolveBundle()
	return e.scoreLegInternal(leg, games, stats, bundle).grade
}

// EvaluateSlip runs v1-compatible slip warnings alongside the four-pillar v2
// per-leg and copula-adjusted slip grade in a single pass.
func (e *V2Engine) EvaluateSlip(legs []Leg, games map[string]Game, stats map[string]PlayerStats) SlipEvaluation {
	warnings := AnalyzeSlip(legs, games, stats)
	bundle := e.resolveBundle()

	legGrades := make([]LegGrade, len(legs))
	evals := make([]v2LegEval, len(legs))
	gradedIdxs := make([]int, 0, len(legs))

	for i, leg := range legs {
		ev := e.scoreLegInternal(leg, games, stats, bundle)
		evals[i] = ev
		legGrades[i] = ev.grade
		if ev.grade.Letter != "" {
			gradedIdxs = append(gradedIdxs, i)
		}
	}

	if len(gradedIdxs) == 0 {
		return SlipEvaluation{
			Warnings: warnings,
			Grade:    SlipGrade{LegGrades: legGrades},
		}
	}

	if len(gradedIdxs) == 1 {
		only := evals[gradedIdxs[0]]
		singleGrade := buildSingleLegSlipGrade(only, legGrades, warnings)
		return SlipEvaluation{
			Warnings: warnings,
			Grade:    singleGrade,
		}
	}

	jointEst, hasJoint := estimateSlipJoint(legs, evals, gradedIdxs, games, bundle)
	slipGrade := e.buildMultiLegSlipGrade(legs, evals, gradedIdxs, legGrades, warnings, jointEst, hasJoint)
	return SlipEvaluation{
		Warnings: warnings,
		Grade:    slipGrade,
	}
}

func (e *V2Engine) resolveBundle() *modelv2.ArtifactBundle {
	if e != nil && e.bundle != nil {
		return e.bundle
	}
	bundle, err := modelv2.LoadEmbedded()
	if err != nil {
		return nil
	}
	return bundle
}

func (e *V2Engine) scoreLegInternal(
	leg Leg,
	games map[string]Game,
	stats map[string]PlayerStats,
	bundle *modelv2.ArtifactBundle,
) v2LegEval {
	if leg.Market == marketOther || leg.GameID == "" {
		return v2LegEval{grade: LegGrade{LegID: leg.ID}, leg: leg}
	}
	game, ok := games[leg.GameID]
	if !ok {
		return v2LegEval{grade: LegGrade{LegID: leg.ID}, leg: leg}
	}

	ps, hasPS := stats[leg.PlayerID]
	role := resolveLegRole(leg, ps, hasPS)
	sharp, hasSharp := game.Sharp[sharpKeyForLeg(leg)]
	exactMarketProb, linesMatch, lineEdge, hasLineEdge := resolveSharpMarketAnchor(leg, sharp, hasSharp, e.cfg)

	var (
		pred     *modelv2.LegPrediction
		findings []OverlayFinding
	)
	market := normalizeSharpMarket(leg.Market)

	if bundle != nil {
		if pm, hasPropModel := bundle.PropModel(market); hasPropModel {
			p, f, okPred := e.predictPropLeg(bundle, pm, leg, game, stats, ps, hasPS, role, exactMarketProb, linesMatch)
			if okPred {
				pred = &p
				findings = f
			}
		}
	}
	if pred == nil {
		findings = EvaluateOverlays(leg, game, stats, nil, e.overlays...)
	}

	uncertain := HasHeightenedUncertainty(findings)
	calibratedProb, marketProb := resolveLegProbs(
		leg,
		game,
		pred,
		exactMarketProb,
		sharp,
		hasSharp,
		linesMatch,
		uncertain,
		e.cfg,
	)

	pricePillar := scoreLegPriceVigPillar(
		leg,
		calibratedProb,
		sharp,
		hasSharp,
		lineEdge,
		hasLineEdge,
	)
	scriptPillar, hasScriptSig := scoreLegScriptFitPillar(leg, game)
	stabPillar := scoreLegStabilityPillar(leg, pred, calibratedProb, ps, hasPS)
	ctxPillar, ctxFactors := scoreLegContextTrapsPillar(findings)

	score := e.rollupLegScore(pricePillar, scriptPillar, stabPillar, ctxPillar, hasScriptSig, findings)
	factors := buildLegGradeFactors(
		leg,
		calibratedProb,
		sharp,
		hasSharp,
		linesMatch,
		lineEdge,
		hasLineEdge,
		pricePillar,
		scriptPillar,
		stabPillar,
		hasScriptSig,
		ctxFactors,
		score,
	)

	vegasImpliedProb := parlay.ImpliedProb(leg.Odds)
	fairNoVigProb := marketProb
	rawModelProb := resolveRawModelProb(leg, game, pred, e.cfg)

	ev := v2LegEval{
		leg:          leg,
		game:         game,
		role:         role,
		pred:         pred,
		findings:     findings,
		sharp:        sharp,
		hasSharp:     hasSharp,
		linesMatch:   linesMatch,
		lineEdge:     lineEdge,
		hasLineEdge:  hasLineEdge,
		hasScriptSig: hasScriptSig,
		pricePillar:  pricePillar,
		scriptPillar: scriptPillar,
		stabPillar:   stabPillar,
		ctxPillar:    ctxPillar,
		grade: LegGrade{
			LegID:            leg.ID,
			Letter:           letterForScore(score, e.cfg),
			Score:            score,
			Factors:          factors,
			Pillars:          []PillarScore{pricePillar, scriptPillar, stabPillar, ctxPillar},
			CalibratedProb:   calibratedProb,
			MarketProb:       marketProb,
			RawModelProb:     rawModelProb,
			VegasImpliedProb: vegasImpliedProb,
			FairNoVigProb:    fairNoVigProb,
			Uncertain:        uncertain,
		},
	}
	ev.grade.Diagnostics = buildLegDiagnostics(ev, rawModelProb, vegasImpliedProb, fairNoVigProb, e.cfg)
	return ev
}

func (e *V2Engine) predictPropLeg(
	bundle *modelv2.ArtifactBundle,
	pm modelv2.DistModelArtifact,
	leg Leg,
	game Game,
	stats map[string]PlayerStats,
	ps PlayerStats,
	hasPS bool,
	role RoleArchetype,
	exactMarketProb float64,
	linesMatch bool,
) (modelv2.LegPrediction, []OverlayFinding, bool) {
	if _, err := propPredictionLine(leg); err != nil {
		return modelv2.LegPrediction{}, nil, false
	}

	var rawStack [v2StackFeatureCap]float64
	var shrunkStack [v2StackFeatureCap]float64
	var rawBuf, shrunkBuf []float64
	n := len(pm.FeatureNames)
	if n <= v2StackFeatureCap {
		rawBuf = rawStack[:n]
		shrunkBuf = shrunkStack[:n]
	} else {
		rawBuf = make([]float64, n)
		shrunkBuf = make([]float64, n)
	}

	gamesPlayed := populateFeatureSlice(rawBuf, pm.FeatureNames, leg, game, ps, hasPS, role)
	var shrunk []float64
	if prior, hasPrior := bundle.LookupPrior(pm.Market, string(role), ps.Position); hasPrior {
		shrunk = modelv2.ShrinkFeatureSlice(shrunkBuf, rawBuf, pm.FeatureNames, gamesPlayed, prior)
	} else {
		shrunk = rawBuf
	}

	marketInput := 0.0
	if linesMatch {
		marketInput = exactMarketProb
	}

	predLeg := leg
	if predLeg.Market == marketAnytimeTD && strings.TrimSpace(predLeg.Side) == "" {
		predLeg.Side = sideYes
	}

	pred, findings, err := PredictLegWithOverlays(
		pm,
		shrunk,
		predLeg,
		game,
		stats,
		marketInput,
		e.overlays...,
	)
	if err != nil {
		return modelv2.LegPrediction{}, nil, false
	}
	return pred, findings, true
}

func resolveLegRole(leg Leg, ps PlayerStats, hasPS bool) RoleArchetype {
	if hasPS && ps.Role != RoleUnknown {
		return ps.Role
	}
	if hasPS {
		switch db.NormalizePosition(ps.Position) {
		case "QB":
			return RoleQB
		case "WR":
			return RoleWR2
		case "RB":
			return RoleRB2
		case "TE":
			return RoleTE1
		}
	}
	switch normalizeSharpMarket(leg.Market) {
	case marketPassYds, marketPassTDs:
		return RoleQB
	case marketRushYds, marketRushAtt:
		return RoleRB2
	case marketRecYds, marketReceptions:
		return RoleWR2
	default:
		return RoleUnknown
	}
}

func populateFeatureSlice(
	dst []float64,
	names []string,
	leg Leg,
	game Game,
	ps PlayerStats,
	hasPS bool,
	role RoleArchetype,
) int {
	gamesPlayed := 0
	var ewma *PlayerEWMAUsage
	if hasPS && ps.EWMA != nil {
		ewma = ps.EWMA
		gamesPlayed = ewma.GamesPlayed
	} else if hasPS {
		if g := ps.Games[leg.Market]; len(g) > 0 {
			gamesPlayed = len(g)
		}
		if ps.Usage != nil && len(ps.Usage.Weeks) > gamesPlayed {
			gamesPlayed = len(ps.Usage.Weeks)
		}
	}

	teamSpread, hasSpread := resolveTeamSpread(leg, game)
	gameTotal, hasTotal := resolveGameTotal(game)
	teamImplied, oppImplied, hasImplied := resolveImpliedTotals(leg, game, teamSpread, hasSpread, gameTotal, hasTotal)
	teamEPA := game.TeamEPA(leg.Team)
	oppEPA := game.OpponentEPA(leg.Team)
	roleDef := game.OpponentRoleDefense(leg.Team, role)

	for i, name := range names {
		dst[i] = evalFeatureValue(
			name,
			leg,
			game,
			ps,
			hasPS,
			ewma,
			gamesPlayed,
			role,
			roleDef,
			teamEPA,
			oppEPA,
			teamSpread,
			hasSpread,
			gameTotal,
			hasTotal,
			teamImplied,
			oppImplied,
			hasImplied,
		)
	}
	return gamesPlayed
}

func evalFeatureValue(
	name string,
	leg Leg,
	game Game,
	ps PlayerStats,
	hasPS bool,
	ewma *PlayerEWMAUsage,
	gamesPlayed int,
	role RoleArchetype,
	roleDef *RoleDefenseProfile,
	teamEPA, oppEPA *TeamDefenseProfile,
	teamSpread float64,
	hasSpread bool,
	gameTotal float64,
	hasTotal bool,
	teamImplied, oppImplied float64,
	hasImplied bool,
) float64 {
	if v, ok := evalScriptLineFeature(
		name,
		leg,
		gamesPlayed,
		teamSpread,
		hasSpread,
		gameTotal,
		hasTotal,
		teamImplied,
		oppImplied,
		hasImplied,
	); ok {
		return v
	}
	if v, ok := evalDefenseFeature(name, role, roleDef, teamEPA, oppEPA); ok {
		return v
	}
	if v, ok := evalWeatherFeature(name, game); ok {
		return v
	}
	return evalEWMAFeature(name, leg.Market, ps, hasPS, ewma)
}

func evalScriptLineFeature(
	name string,
	leg Leg,
	gamesPlayed int,
	teamSpread float64,
	hasSpread bool,
	gameTotal float64,
	hasTotal bool,
	teamImplied, oppImplied float64,
	hasImplied bool,
) (float64, bool) {
	switch name {
	case modelv2.FeatGamesPlayed:
		return float64(gamesPlayed), true
	case modelv2.FeatTeamSpread:
		if hasSpread {
			return teamSpread, true
		}
		return math.NaN(), true
	case modelv2.FeatAbsSpread:
		if hasSpread {
			return math.Abs(teamSpread), true
		}
		return math.NaN(), true
	case modelv2.FeatGameTotal:
		if hasTotal {
			return gameTotal, true
		}
		return math.NaN(), true
	case modelv2.FeatTeamImpliedTotal:
		if hasImplied {
			return teamImplied, true
		}
		return math.NaN(), true
	case modelv2.FeatOppImpliedTotal:
		if hasImplied {
			return oppImplied, true
		}
		return math.NaN(), true
	case modelv2.FeatLine:
		if leg.Line != nil {
			return *leg.Line, true
		}
		return math.NaN(), true
	default:
		return 0, false
	}
}

func evalDefenseFeature(
	name string,
	role RoleArchetype,
	roleDef *RoleDefenseProfile,
	teamEPA, oppEPA *TeamDefenseProfile,
) (float64, bool) {
	switch name {
	case modelv2.FeatOppRoleYdsAllowed:
		if roleDef != nil && roleDef.Games > 0 {
			return rolePrimaryYdsAllowed(role, roleDef), true
		}
		return math.NaN(), true
	case modelv2.FeatOppRoleReceptionsAllowed:
		if roleDef != nil && roleDef.Games > 0 {
			return roleDef.ReceptionsAllowed, true
		}
		return math.NaN(), true
	case modelv2.FeatOppRoleTDsAllowed:
		if roleDef != nil && roleDef.Games > 0 {
			return rolePrimaryTDsAllowed(role, roleDef), true
		}
		return math.NaN(), true
	case modelv2.FeatOppRoleEPAAllowed:
		if roleDef != nil && roleDef.Games > 0 {
			return roleDef.TotalEPAAllowed, true
		}
		return math.NaN(), true
	case modelv2.FeatOppDefPassEPAAllowed:
		if isTeamDefensePopulated(oppEPA) {
			return oppEPA.DefPassEPAAllowed, true
		}
		return math.NaN(), true
	case modelv2.FeatOppDefRushEPAAllowed:
		if isTeamDefensePopulated(oppEPA) {
			return oppEPA.DefRushEPAAllowed, true
		}
		return math.NaN(), true
	case modelv2.FeatTeamPlaysPerGame:
		if teamEPA != nil && teamEPA.PlaysPerGame > 0 {
			return teamEPA.PlaysPerGame, true
		}
		return math.NaN(), true
	case modelv2.FeatTeamPassRate:
		if teamEPA != nil && teamEPA.PlaysPerGame > 0 {
			return teamEPA.PassRate, true
		}
		return math.NaN(), true
	default:
		return 0, false
	}
}

func evalWeatherFeature(name string, game Game) (float64, bool) {
	switch name {
	case modelv2.FeatIsIndoor:
		if game.Indoor {
			return 1.0, true
		}
		return 0.0, true
	case modelv2.FeatWindMPH:
		if game.Indoor {
			return 0.0, true
		}
		if game.Weather != nil {
			return game.Weather.WindMPH, true
		}
		return math.NaN(), true
	case modelv2.FeatPrecipIn:
		if game.Indoor {
			return 0.0, true
		}
		if game.Weather != nil {
			return game.Weather.PrecipIn, true
		}
		return math.NaN(), true
	case modelv2.FeatTempF:
		if game.Indoor {
			return 70.0, true
		}
		if game.Weather != nil && game.Weather.TempF != 0 {
			return game.Weather.TempF, true
		}
		return math.NaN(), true
	default:
		return 0, false
	}
}

func evalEWMAFeature(
	name, market string,
	ps PlayerStats,
	hasPS bool,
	ewma *PlayerEWMAUsage,
) float64 {
	if ewma != nil {
		if v, ok := evalEWMAUsageField(name, ewma); ok {
			return v
		}
		if v, ok := evalEWMAProdField(name, ewma); ok {
			return v
		}
	}
	if !hasPS {
		return math.NaN()
	}
	return evalFallbackEWMAFeature(name, market, ps)
}

func evalEWMAUsageField(name string, ewma *PlayerEWMAUsage) (float64, bool) {
	switch name {
	case modelv2.FeatEWMAOffensePct:
		return ewma.OffensePct, true
	case modelv2.FeatEWMATargetShare:
		return ewma.TargetShare, true
	case modelv2.FeatEWMAAirYardsShare:
		return ewma.AirYardsShare, true
	case modelv2.FeatEWMAWOPR:
		return ewma.WOPR, true
	case modelv2.FeatEWMACarryShare:
		return ewma.CarryShare, true
	case modelv2.FeatEWMAPassAtt:
		return ewma.PassAtt, true
	case modelv2.FeatEWMACarries:
		return ewma.Carries, true
	case modelv2.FeatEWMATargets:
		return ewma.Targets, true
	case modelv2.FeatEWMAReceptions:
		return ewma.Receptions, true
	case modelv2.FeatEWMARZOpps:
		return ewma.RZOpps, true
	case modelv2.FeatEWMATotalEPA:
		return ewma.TotalEPA, true
	case modelv2.FeatEWMACPOE:
		return ewma.CPOE, true
	default:
		return 0, false
	}
}

func evalEWMAProdField(name string, ewma *PlayerEWMAUsage) (float64, bool) {
	switch name {
	case modelv2.FeatEWMAPassYds:
		return ewma.PassYds, true
	case modelv2.FeatEWMARushYds:
		return ewma.RushYds, true
	case modelv2.FeatEWMARecYds:
		return ewma.RecYds, true
	case modelv2.FeatEWMAPassTDs:
		return ewma.PassTDs, true
	case modelv2.FeatEWMARushTDs:
		return ewma.RushTDs, true
	case modelv2.FeatEWMARecTDs:
		return ewma.RecTDs, true
	case modelv2.FeatEWMAExpPassYds:
		return ewma.ExpPassYds, true
	case modelv2.FeatEWMAExpRushYds:
		return ewma.ExpRushYds, true
	case modelv2.FeatEWMAExpRecYds:
		return ewma.ExpRecYds, true
	case modelv2.FeatEWMAExpReceptions:
		return ewma.ExpReceptions, true
	case modelv2.FeatEWMAExpPassTDs:
		return ewma.ExpPassTDs, true
	case modelv2.FeatEWMAExpRushTDs:
		return ewma.ExpRushTDs, true
	case modelv2.FeatEWMAExpRecTDs:
		return ewma.ExpRecTDs, true
	default:
		return 0, false
	}
}

func evalFallbackEWMAFeature(name, market string, ps PlayerStats) float64 {
	if ps.Usage != nil {
		switch name {
		case modelv2.FeatEWMAOffensePct:
			if v := ps.Usage.EffectiveSnapAvg(); v > 0 {
				return v
			}
		case modelv2.FeatEWMATargetShare:
			if v := ps.Usage.EffectiveTargetShare(); v > 0 {
				return v
			}
		case modelv2.FeatEWMAWOPR:
			if v := ps.Usage.RecentWOPR(); v > 0 {
				return v
			}
		case modelv2.FeatEWMACarries:
			if v := ps.Usage.AverageCarries(); v > 0 {
				return v
			}
		}
	}

	if gameVals := ps.Games[market]; len(gameVals) > 0 {
		switch {
		case name == modelv2.FeatEWMAPassYds && market == marketPassYds,
			name == modelv2.FeatEWMARushYds && market == marketRushYds,
			name == modelv2.FeatEWMARecYds && market == marketRecYds,
			name == modelv2.FeatEWMAReceptions && market == marketReceptions,
			name == modelv2.FeatEWMAPassTDs && market == marketPassTDs,
			name == modelv2.FeatEWMACarries && market == marketRushAtt:
			return ComputeEWMA(gameVals, EWMAHalfLifeWeeks)
		}
	}

	return math.NaN()
}

func rolePrimaryTDsAllowed(role RoleArchetype, prof *RoleDefenseProfile) float64 {
	if prof == nil {
		return 0
	}
	switch role {
	case RoleQB:
		return prof.PassTDsAllowed
	case RoleRB1, RoleRB2:
		return prof.RushTDsAllowed + prof.RecTDsAllowed
	case RoleWR1, RoleWR2, RoleTE1:
		return prof.RecTDsAllowed
	default:
		return 0
	}
}

func isTeamDefensePopulated(prof *TeamDefenseProfile) bool {
	if prof == nil {
		return false
	}
	return prof.PlaysPerGame > 0 || prof.DefPassEPAAllowed != 0 || prof.DefRushEPAAllowed != 0
}

func resolveTeamSpread(leg Leg, game Game) (float64, bool) {
	if game.MarketScript.HasSpread {
		switch {
		case strings.EqualFold(leg.Team, game.Home):
			return game.MarketScript.HomeSpread, true
		case strings.EqualFold(leg.Team, game.Away):
			return -game.MarketScript.HomeSpread, true
		}
	}
	return game.ModelSpread(leg.Team)
}

func resolveGameTotal(game Game) (float64, bool) {
	if game.MarketScript.HasTotal && game.MarketScript.GameTotal > 0 {
		return game.MarketScript.GameTotal, true
	}
	return 0, false
}

func resolveImpliedTotals(
	leg Leg,
	game Game,
	teamSpread float64,
	hasSpread bool,
	gameTotal float64,
	hasTotal bool,
) (teamImplied, oppImplied float64, ok bool) {
	if !hasSpread || !hasTotal {
		return 0, 0, false
	}
	if game.MarketScript.HasSpread && game.MarketScript.HasTotal {
		switch {
		case strings.EqualFold(leg.Team, game.Home):
			return game.MarketScript.HomeImpliedTotal, game.MarketScript.AwayImpliedTotal, true
		case strings.EqualFold(leg.Team, game.Away):
			return game.MarketScript.AwayImpliedTotal, game.MarketScript.HomeImpliedTotal, true
		}
	}
	return (gameTotal - teamSpread) / 2, (gameTotal + teamSpread) / 2, true
}

func resolveSharpMarketAnchor(
	leg Leg,
	sharp SharpQuote,
	hasSharp bool,
	cfg V2Config,
) (marketProb float64, linesMatch bool, lineEdge float64, hasLineEdge bool) {
	if !hasSharp {
		return 0, false, 0, false
	}
	fairProb := sharp.PowerFairProb
	if !(fairProb > 0 && fairProb < 1) {
		fairProb = sharp.FairProb
	}
	if !(fairProb > 0 && fairProb < 1) {
		return 0, false, 0, false
	}

	market := normalizeSharpMarket(leg.Market)
	if leg.Line == nil && sharp.Line == nil {
		return fairProb, true, 0, false
	}
	if leg.Line != nil && sharp.Line != nil {
		diff := math.Abs(*leg.Line - *sharp.Line)
		if diff <= lineMatchEpsilon {
			return fairProb, true, 0, false
		}
		edge := computeSharpLineEdge(market, leg.Side, *leg.Line, *sharp.Line)
		adjProb := adjustGameLineProb(market, fairProb, edge, cfg)
		return adjProb, false, edge, true
	}
	return 0, false, 0, false
}

func computeSharpLineEdge(market, side string, legLine, sharpLine float64) float64 {
	switch {
	case market == marketSpread:
		return legLine - sharpLine
	case strings.EqualFold(strings.TrimSpace(side), sideOver):
		return sharpLine - legLine
	case strings.EqualFold(strings.TrimSpace(side), sideUnder):
		return legLine - sharpLine
	default:
		return 0
	}
}

func adjustGameLineProb(market string, sharpProb, lineEdge float64, cfg V2Config) float64 {
	var stdDev float64
	switch market {
	case marketSpread:
		stdDev = cfg.SpreadMarginStdDev
	case marketTotal:
		stdDev = cfg.GameTotalStdDev
	case marketTeamTotal:
		stdDev = cfg.TeamTotalStdDev
	default:
		return 0
	}
	if stdDev <= 0 {
		return 0
	}
	z := modelv2.NormInv(clampProb(sharpProb, v2MinClampedProb, v2MaxClampedProb))
	return clampProb(modelv2.NormCDF(z+lineEdge/stdDev), v2MinClampedProb, v2MaxClampedProb)
}

func resolveLegProbs(
	leg Leg,
	game Game,
	pred *modelv2.LegPrediction,
	sharpAnchor float64,
	sharp SharpQuote,
	hasSharp, linesMatch, uncertain bool,
	cfg V2Config,
) (calibratedProb, marketProb float64) {
	if linesMatch && sharpAnchor > 0 && sharpAnchor < 1 {
		marketProb = sharpAnchor
	} else if !linesMatch && sharpAnchor > 0 && sharpAnchor < 1 && !db.IsPlayerMarket(normalizeSharpMarket(leg.Market)) {
		// Line-adjusted sharp benchmark for non-prop game lines.
		marketProb = sharpAnchor
	}

	if pred != nil && pred.CalibratedProb > 0 && pred.CalibratedProb < 1 {
		return clampProb(pred.CalibratedProb, v2MinClampedProb, v2MaxClampedProb), marketProb
	}

	modelProb := resolveGameLineModelProb(leg, game, cfg)
	switch {
	case marketProb > 0 && modelProb > 0:
		w := clampProb(cfg.GameLineMarketBlend, 0, 1)
		if uncertain {
			w = min(0.95, w+HeightenedUncertaintyBlendBoost)
		}
		return clampProb(w*marketProb+(1-w)*modelProb, v2MinClampedProb, v2MaxClampedProb), marketProb
	case marketProb > 0:
		return clampProb(marketProb, v2MinClampedProb, v2MaxClampedProb), marketProb
	case modelProb > 0:
		return clampProb(modelProb, v2MinClampedProb, v2MaxClampedProb), 0
	case hasSharp && sharp.PowerFairProb > 0 && sharp.PowerFairProb < 1:
		return clampProb(sharp.PowerFairProb, v2MinClampedProb, v2MaxClampedProb), 0
	default:
		return fallbackRetailFairProb(leg.Odds), 0
	}
}

func resolveRawModelProb(leg Leg, game Game, pred *modelv2.LegPrediction, cfg V2Config) float64 {
	if pred != nil && pred.ModelProb > 0 && pred.ModelProb < 1 {
		return clampProb(pred.ModelProb, v2MinClampedProb, v2MaxClampedProb)
	}
	if p := resolveGameLineModelProb(leg, game, cfg); p > 0 && p < 1 {
		return clampProb(p, v2MinClampedProb, v2MaxClampedProb)
	}
	return 0
}

func resolveGameLineModelProb(leg Leg, game Game, cfg V2Config) float64 {
	market := normalizeSharpMarket(leg.Market)
	switch market {
	case marketML:
		if p, ok := game.ModelWinProb(leg.Team); ok && p > 0 && p < 1 {
			return p
		}
	case marketSpread:
		if leg.Line != nil {
			if modelSpread, ok := game.ModelSpread(leg.Team); ok {
				cushion := *leg.Line - modelSpread
				return clampProb(modelv2.NormCDF(cushion/cfg.SpreadMarginStdDev), v2MinClampedProb, v2MaxClampedProb)
			}
		}
	case marketTotal:
		if leg.Line != nil && game.MarketScript.HasTotal && game.MarketScript.GameTotal > 0 {
			diff := game.MarketScript.GameTotal - *leg.Line
			if isNegativeSide(leg.Side) {
				diff = -diff
			}
			return clampProb(modelv2.NormCDF(diff/cfg.GameTotalStdDev), v2MinClampedProb, v2MaxClampedProb)
		}
	case marketTeamTotal:
		if leg.Line != nil && game.MarketScript.HasSpread && game.MarketScript.HasTotal {
			teamImp, _, ok := resolveImpliedTotals(
				leg,
				game,
				game.MarketScript.HomeSpread,
				true,
				game.MarketScript.GameTotal,
				true,
			)
			if ok {
				diff := teamImp - *leg.Line
				if isNegativeSide(leg.Side) {
					diff = -diff
				}
				return clampProb(modelv2.NormCDF(diff/cfg.TeamTotalStdDev), v2MinClampedProb, v2MaxClampedProb)
			}
		}
	}
	return 0
}

func fallbackRetailFairProb(odds int) float64 {
	raw := parlay.ImpliedProb(odds)
	if raw <= 0 || raw >= 1 {
		return v2FallbackProb
	}
	// Strip standard two-way book overround (~4.5%) so fallback legs match
	// typical vigged pricing instead of assuming 0% vig.
	return clampProb(raw/(1+v2AssumedVig), v2MinClampedProb, v2MaxClampedProb)
}

func scoreLegPriceVigPillar(
	leg Leg,
	calibratedProb float64,
	sharp SharpQuote,
	hasSharp bool,
	lineEdge float64,
	hasLineEdge bool,
) PillarScore {
	if leg.Odds == 0 {
		return PillarScore{
			Key:     PillarPriceVig,
			Label:   PillarLabelPriceVig,
			Score:   v2NeutralScore,
			Tone:    ToneNeutral,
			Summary: "No retail odds listed to compare against fair value.",
			Reasons: []string{"No retail odds listed to check price against."},
		}
	}

	ev := parlay.FairEV(calibratedProb, leg.Odds)
	score := evToPriceScore(ev)

	fairAmerican := parlay.DecimalToAmerican(1 / clampProb(calibratedProb, v2MinClampedProb, v2MaxClampedProb))
	reasons := make([]string, 0, 2)

	switch {
	case ev >= 0.025:
		reasons = append(reasons, fmt.Sprintf(
			"Getting %s vs. fair price near %s.",
			americanOddsText(leg.Odds),
			americanOddsText(fairAmerican),
		))
	case ev >= -0.02:
		reasons = append(reasons, fmt.Sprintf(
			"Fairly priced at %s near fair value (%s).",
			americanOddsText(leg.Odds),
			americanOddsText(fairAmerican),
		))
	case ev >= -0.055:
		if leg.Odds == -110 {
			reasons = append(reasons, "Standard -110 vig near fair market price.")
		} else {
			reasons = append(reasons, fmt.Sprintf(
				"Paying %s vs. fair price near %s.",
				americanOddsText(leg.Odds),
				americanOddsText(fairAmerican),
			))
		}
	default:
		reasons = append(reasons, fmt.Sprintf(
			"Paying %s vs. fair price near %s — steep juice.",
			americanOddsText(leg.Odds),
			americanOddsText(fairAmerican),
		))
	}

	if hasSharp && hasLineEdge && sharp.Line != nil && leg.Line != nil {
		market := normalizeSharpMarket(leg.Market)
		threshold := sharpLineThreshold(market)
		source := sharpSourceLabel(sharp)
		sharpLine := sharpLineDisplay(market, sharp.Line)
		if lineEdge >= threshold-lineMatchEpsilon {
			reasons = append(reasons, fmt.Sprintf("Getting %s while %s sits at %s.", selection(leg), source, sharpLine))
		} else if lineEdge <= -threshold+lineMatchEpsilon {
			reasons = append(reasons, fmt.Sprintf(
				"%s sits at %s, giving up %s.",
				source,
				sharpLine,
				sharpDiffText(market, math.Abs(lineEdge)),
			))
		}
	}

	if isChalkLeg(leg) && len(reasons) < 2 {
		reasons = append(reasons, fmt.Sprintf(
			"Heavy favorite price (%d) adds little payout for the upset risk.",
			leg.Odds,
		))
	}

	return PillarScore{
		Key:     PillarPriceVig,
		Label:   PillarLabelPriceVig,
		Score:   score,
		Tone:    toneForScore(score),
		Summary: reasons[0],
		Reasons: reasons,
	}
}

func evToPriceScore(ev float64) int {
	// Piecewise-linear monotonic curve anchored to design thresholds:
	// +8% -> 100, +5% -> 98, 0% -> 85, -4.5% -> 74, -10% -> 48, -15% -> 33, -25% -> 10.
	var score float64
	switch {
	case ev >= 0.08:
		score = 100
	case ev >= 0.05:
		score = 98 + (ev-0.05)/0.03*2
	case ev >= 0.0:
		score = 85 + ev/0.05*13
	case ev >= -0.045:
		score = 74 + (ev+0.045)/0.045*11
	case ev >= -0.10:
		score = 48 + (ev+0.10)/0.055*26
	case ev >= -0.15:
		score = 33 + (ev+0.15)/0.05*15
	case ev >= -0.25:
		score = 10 + (ev+0.25)/0.10*23
	default:
		score = 10 + (ev+0.25)*40
	}
	return clampInt(int(math.Round(score)), 0, 100)
}

func scoreLegScriptFitPillar(leg Leg, game Game) (PillarScore, bool) {
	market := normalizeSharpMarket(leg.Market)
	teamSpread, hasSpread := resolveTeamSpread(leg, game)
	gameTotal, hasTotal := resolveGameTotal(game)

	isOver := strings.EqualFold(strings.TrimSpace(leg.Side), sideOver) ||
		strings.EqualFold(strings.TrimSpace(leg.Side), sideYes)
	isUnder := isNegativeSide(leg.Side)

	delta := 0
	var reason string
	hasSignal := false

	switch {
	case market == marketRushYds || market == marketRushAtt:
		delta, reason, hasSignal = evalRushScriptFit(leg.Team, isOver, isUnder, teamSpread, hasSpread)
	case isPassOrRecMarket(market):
		delta, reason, hasSignal = evalPassScriptFit(isOver, isUnder, teamSpread, hasSpread, gameTotal, hasTotal)
	}

	score := clampInt(v2NeutralScore+delta, 0, 100)
	if !hasSignal {
		reason = "Neutral single-leg game script."
	}
	return PillarScore{
		Key:     PillarScriptFit,
		Label:   PillarLabelScriptFit,
		Score:   score,
		Tone:    toneForScore(score),
		Summary: reason,
		Reasons: []string{reason},
	}, hasSignal
}

func isPassOrRecMarket(market string) bool {
	switch market {
	case marketPassYds, marketPassTDs, marketRecYds, marketReceptions:
		return true
	default:
		return false
	}
}

func evalRushScriptFit(
	team string,
	isOver, isUnder bool,
	teamSpread float64,
	hasSpread bool,
) (delta int, reason string, ok bool) {
	if !hasSpread {
		return 0, "", false
	}
	if isOver {
		switch {
		case teamSpread <= -7.0:
			return 12, fmt.Sprintf("%s favored by %s fits a run-heavy closing script.", team, number(-teamSpread)), true
		case teamSpread <= -3.5:
			return 6, fmt.Sprintf("Favored game script (%s) supports rushing volume.", number(teamSpread)), true
		case teamSpread >= 7.0:
			return -18, fmt.Sprintf(
				"Big underdog (+%s) faces trailing script risk for rushing volume.",
				number(teamSpread),
			), true
		case teamSpread >= 4.5:
			return -12, fmt.Sprintf(
				"Underdog (+%s) risks falling behind and abandoning the run.",
				number(teamSpread),
			), true
		}
		return 0, "", false
	}
	if isUnder {
		switch {
		case teamSpread >= 4.5:
			return 8, fmt.Sprintf(
				"Underdog (+%s) script leans away from heavy rushing volume.",
				number(teamSpread),
			), true
		case teamSpread <= -7.0:
			return -10, fmt.Sprintf(
				"Heavy favorite (%s) script can feed late clock-killing carries.",
				number(teamSpread),
			), true
		}
	}
	return 0, "", false
}

func evalPassScriptFit(
	isOver, isUnder bool,
	teamSpread float64,
	hasSpread bool,
	gameTotal float64,
	hasTotal bool,
) (delta int, reason string, ok bool) {
	if isOver {
		switch {
		case hasSpread && teamSpread <= -10.0:
			return -8, fmt.Sprintf(
				"Double-digit favorite (%s) risk: late clock-drain can cap passing volume.",
				number(teamSpread),
			), true
		case hasTotal && gameTotal >= 48.0:
			return 8, fmt.Sprintf("High %s game total points to shootout passing volume.", number(gameTotal)), true
		case hasTotal && gameTotal <= 39.5:
			return -8, fmt.Sprintf("Low %s game total points to fewer passing drives.", number(gameTotal)), true
		}
		return 0, "", false
	}
	if isUnder && hasTotal {
		switch {
		case gameTotal <= 39.5:
			return 7, fmt.Sprintf("Low %s game total supports a quieter passing script.", number(gameTotal)), true
		case gameTotal >= 48.0:
			return -8, fmt.Sprintf(
				"High %s game total brings extra shootout pass volume risk.",
				number(gameTotal),
			), true
		}
	}
	return 0, "", false
}

func scoreLegStabilityPillar(
	leg Leg,
	pred *modelv2.LegPrediction,
	calibratedProb float64,
	ps PlayerStats,
	hasPS bool,
) PillarScore {
	score := v2StabilityBase
	reasons := make([]string, 0, 3)

	if isChalkLeg(leg) {
		score += chalkPenalty(leg.Odds, RubricV1)
		reasons = append(reasons, fmt.Sprintf(
			"Heavy %d price adds minimal payout while still carrying upset risk.",
			leg.Odds,
		))
	}

	switch {
	case calibratedProb < v2LongshotProbMax:
		penalty := 18 + int(math.Round((v2LongshotProbMax-calibratedProb)*40))
		score -= clampInt(penalty, 18, 28)
		reasons = append(reasons, fmt.Sprintf(
			"Longshot hit rate (~%d%%) swings widely week to week.",
			int(math.Round(calibratedProb*100)),
		))
	case calibratedProb < v2LowProbMax:
		score -= 8
		reasons = append(reasons, fmt.Sprintf(
			"Lower hit rate (~%d%%) adds week-to-week variance.",
			int(math.Round(calibratedProb*100)),
		))
	case calibratedProb >= v2HighProbMin && !isChalkLeg(leg):
		score += 6
		reasons = append(reasons, fmt.Sprintf(
			"Solid ~%d%% fair hit rate keeps variance in check.",
			int(math.Round(calibratedProb*100)),
		))
	}

	market := normalizeSharpMarket(leg.Market)
	switch market {
	case marketAnytimeTD:
		score -= 10
		reasons = append(reasons, "Touchdowns are high-swing binary outcomes.")
	case marketPassTDs:
		score -= 5
		reasons = append(reasons, "Passing TDs move in discrete jumps with extra variance.")
	}

	if pred != nil && market != marketAnytimeTD {
		cv := pred.Stats.CV
		skew := pred.Stats.Skewness
		switch {
		case cv >= 0.85 || skew >= 1.5:
			score -= 8
			reasons = append(reasons, "Wide outcome spread raises boom-or-bust miss risk.")
		case cv >= 0.55 || skew >= 1.0:
			score -= 4
			reasons = append(reasons, "Moderate week-to-week stat volatility.")
		case cv > 0 && cv <= 0.32 && skew < 0.7:
			score += 5
			reasons = append(reasons, "Tight, steady volume distribution.")
		}
	}

	if db.IsPlayerMarket(market) && hasPS && ps.EWMA != nil &&
		ps.EWMA.GamesPlayed > 0 && ps.EWMA.GamesPlayed <= 2 && len(reasons) < 3 {
		score -= 4
		reasons = append(reasons, fmt.Sprintf(
			"Early-season sample (%d game%s) leans partly on role priors.",
			ps.EWMA.GamesPlayed,
			pluralS(ps.EWMA.GamesPlayed),
		))
	}

	score = clampInt(score, 0, 100)
	if len(reasons) == 0 {
		reasons = append(reasons, "Normal week-to-week variance for this market.")
	}
	if len(reasons) > 3 {
		reasons = reasons[:3]
	}

	return PillarScore{
		Key:     PillarStability,
		Label:   PillarLabelStability,
		Score:   score,
		Tone:    toneForScore(score),
		Summary: reasons[0],
		Reasons: reasons,
	}
}

func scoreLegContextTrapsPillar(findings []OverlayFinding) (PillarScore, []GradeFactor) {
	if len(findings) == 0 {
		msg := "Clean context with no injury, weather, or trap flags."
		return PillarScore{
			Key:     PillarContextTraps,
			Label:   PillarLabelContextTraps,
			Score:   v2NeutralScore,
			Tone:    ToneNeutral,
			Summary: "No injury, weather, or trap flags active.",
			Reasons: []string{msg},
		}, nil
	}

	sorted := slices.Clone(findings)
	slices.SortStableFunc(sorted, func(a, b OverlayFinding) int {
		if c := cmp.Compare(b.Severity, a.Severity); c != 0 {
			return c
		}
		absA := a.ScoreDelta
		if absA < 0 {
			absA = -absA
		}
		absB := b.ScoreDelta
		if absB < 0 {
			absB = -absB
		}
		return cmp.Compare(absB, absA)
	})

	deltaSum := 0
	factors := make([]GradeFactor, 0, len(sorted))
	reasons := make([]string, 0, min(3, len(sorted)))
	for _, f := range sorted {
		deltaSum += f.ScoreDelta
		if f.ScoreDelta != 0 {
			factors = append(factors, GradeFactor{
				Rule:   f.Rule,
				Label:  f.Label,
				Delta:  f.ScoreDelta,
				Detail: f.Detail,
			})
		}
		if len(reasons) < 3 && strings.TrimSpace(f.Detail) != "" {
			reasons = append(reasons, f.Detail)
		}
	}

	clampedDelta := clampInt(deltaSum, -55, 20)
	score := clampInt(v2NeutralScore+clampedDelta, 0, 100)
	if len(reasons) == 0 {
		reasons = append(reasons, "Context flags active.")
	}

	return PillarScore{
		Key:     PillarContextTraps,
		Label:   PillarLabelContextTraps,
		Score:   score,
		Tone:    toneForScore(score),
		Summary: reasons[0],
		Reasons: reasons,
	}, factors
}

func (e *V2Engine) rollupLegScore(
	pricePillar, scriptPillar, stabPillar, ctxPillar PillarScore,
	hasScriptSig bool,
	findings []OverlayFinding,
) int {
	var raw float64
	if hasScriptSig {
		raw = e.cfg.LegScriptWeightPrice*float64(pricePillar.Score) +
			e.cfg.LegScriptWeightScript*float64(scriptPillar.Score) +
			e.cfg.LegScriptWeightStability*float64(stabPillar.Score) +
			e.cfg.LegScriptWeightContext*float64(ctxPillar.Score)
	} else {
		raw = e.cfg.LegWeightPrice*float64(pricePillar.Score) +
			e.cfg.LegWeightStability*float64(stabPillar.Score) +
			e.cfg.LegWeightContext*float64(ctxPillar.Score)
	}
	score := clampInt(int(math.Round(raw)), 0, 100)

	hasDangerOverlay := false
	for _, f := range findings {
		if f.Severity >= Danger {
			hasDangerOverlay = true
			break
		}
	}
	if hasDangerOverlay || pricePillar.Score < v2SevereVigFloor {
		score = min(score, v2CapDangerMax)
	}
	return score
}

func buildLegGradeFactors(
	leg Leg,
	calibratedProb float64,
	sharp SharpQuote,
	hasSharp, linesMatch bool,
	lineEdge float64,
	hasLineEdge bool,
	pricePillar, scriptPillar, stabPillar PillarScore,
	hasScriptSig bool,
	ctxFactors []GradeFactor,
	finalScore int,
) []GradeFactor {
	factors := make([]GradeFactor, 0, len(ctxFactors)+4)

	if hasSharp && hasLineEdge && sharp.Line != nil && leg.Line != nil {
		market := normalizeSharpMarket(leg.Market)
		threshold := sharpLineThreshold(market)
		source := sharpSourceLabel(sharp)
		sharpLine := sharpLineDisplay(market, sharp.Line)
		if lineEdge >= threshold-lineMatchEpsilon {
			factors = append(factors, GradeFactor{
				Rule:   ruleSharpLineEdge,
				Label:  "Sharp line edge",
				Delta:  sharpTierDelta(sharp.Tier, RubricV1.SharpLineEdge, RubricV1.SharpLineEdgeConsensus),
				Detail: fmt.Sprintf("Getting %s while %s sits at %s.", selection(leg), source, sharpLine),
			})
		} else if lineEdge <= -threshold+lineMatchEpsilon {
			factors = append(factors, GradeFactor{
				Rule:  ruleSharpLineFade,
				Label: "Sharp line fade",
				Delta: sharpTierDelta(sharp.Tier, RubricV1.SharpLineFade, RubricV1.SharpLineFadeConsensus),
				Detail: fmt.Sprintf(
					"%s sits at %s, giving up %s.",
					source,
					sharpLine,
					sharpDiffText(market, math.Abs(lineEdge)),
				),
			})
		}
	} else if leg.Odds != 0 {
		ev := parlay.FairEV(calibratedProb, leg.Odds)
		if ev >= sharpPriceEdgeThreshold-lineMatchEpsilon {
			delta := clampInt(int(math.Round(ev*150)), 4, 12)
			factors = append(factors, GradeFactor{
				Rule:   ruleSharpPriceEdge,
				Label:  priceFactorLabel(hasSharp && linesMatch),
				Delta:  delta,
				Detail: pricePillar.Summary,
			})
		} else if ev <= sharpPriceFadeThreshold+lineMatchEpsilon {
			delta := -clampInt(int(math.Round((-ev)*100)), 4, 15)
			factors = append(factors, GradeFactor{
				Rule:   ruleSharpPriceFade,
				Label:  "Steep price vs. fair value",
				Delta:  delta,
				Detail: pricePillar.Summary,
			})
		}
	}

	if isChalkLeg(leg) {
		factors = append(factors, GradeFactor{
			Rule:   ruleChalk,
			Label:  fmt.Sprintf("Heavy chalk (%d)", leg.Odds),
			Delta:  chalkPenalty(leg.Odds, RubricV1),
			Detail: fmt.Sprintf("Long odds of %d add little payout for the bust risk.", leg.Odds),
		})
	}

	if hasScriptSig && scriptPillar.Score != v2NeutralScore {
		factors = append(factors, GradeFactor{
			Rule:   ruleScriptFit,
			Label:  "Game script fit",
			Delta:  scriptPillar.Score - v2NeutralScore,
			Detail: scriptPillar.Summary,
		})
	}

	if stabDelta := stabPillar.Score - v2StabilityBase; stabDelta <= -8 || stabDelta >= 6 {
		label := "Tail risk"
		if stabDelta > 0 {
			label = "Steady hit profile"
		}
		factors = append(factors, GradeFactor{
			Rule:   ruleStability,
			Label:  label,
			Delta:  stabDelta,
			Detail: stabPillar.Summary,
		})
	}

	factors = append(factors, ctxFactors...)

	// Ensure top-band legs always surface at least one positive GradeFactor for
	// highlight callers like board.legHighlightReason.
	if finalScore >= RubricV1.BandA && !hasPositiveFactor(factors) {
		factors = append(factors, GradeFactor{
			Rule:   rulePriceVig,
			Label:  "Strong overall value",
			Delta:  max(2, pricePillar.Score-v2NeutralScore),
			Detail: pricePillar.Summary,
		})
	}

	return factors
}

func priceFactorLabel(hasSharp bool) string {
	if hasSharp {
		return "Sharp price edge"
	}
	return "Strong price edge"
}

func hasPositiveFactor(factors []GradeFactor) bool {
	for _, f := range factors {
		if f.Delta > 0 {
			return true
		}
	}
	return false
}

func estimateSlipJoint(
	legs []Leg,
	evals []v2LegEval,
	gradedIdxs []int,
	games map[string]Game,
	bundle *modelv2.ArtifactBundle,
) (modelv2.JointParlayEstimate, bool) {
	if bundle == nil || len(gradedIdxs) < 2 {
		return modelv2.JointParlayEstimate{}, false
	}

	copulaLegs := make([]modelv2.CopulaLeg, 0, len(gradedIdxs))
	gameCtx := make(map[string]modelv2.CopulaGameContext, len(gradedIdxs))

	for _, idx := range gradedIdxs {
		ev := evals[idx]
		leg := legs[idx]
		game := games[leg.GameID]

		homeSpread := game.MarketScript.HomeSpread
		hasSpread := game.MarketScript.HasSpread
		if !hasSpread && game.Nfelo != nil {
			homeSpread = game.Nfelo.HomeSpread
			hasSpread = true
		}
		if _, exists := gameCtx[leg.GameID]; !exists {
			gameCtx[leg.GameID] = modelv2.CopulaGameContext{
				HomeTeam:   game.Home,
				AwayTeam:   game.Away,
				HomeSpread: homeSpread,
				GameTotal:  game.MarketScript.GameTotal,
				HasSpread:  hasSpread,
				HasTotal:   game.MarketScript.HasTotal,
			}
		}

		copulaLegs = append(copulaLegs, modelv2.CopulaLeg{
			LegID:      leg.ID,
			GameID:     leg.GameID,
			Team:       leg.Team,
			PlayerID:   leg.PlayerID,
			PlayerName: leg.PlayerName,
			Role:       string(ev.role),
			Market:     normalizeSharpMarket(leg.Market),
			Side:       leg.Side,
			Prob:       ev.grade.CalibratedProb,
			HomeSpread: homeSpread,
			GameTotal:  game.MarketScript.GameTotal,
		})
	}

	est, err := bundle.Copula.EstimateJoint(copulaLegs, gameCtx)
	if err != nil {
		return modelv2.JointParlayEstimate{}, false
	}
	return est, true
}

func buildSingleLegSlipGrade(
	only v2LegEval,
	legGrades []LegGrade,
	warnings []Warning,
) SlipGrade {
	pillars := []PillarScore{
		only.pricePillar,
		only.scriptPillar,
		only.stabPillar,
		only.ctxPillar,
	}
	factors := slices.Clone(only.grade.Factors)
	summary := singleLegSlipSummary(only, warnings)
	return SlipGrade{
		Letter:           only.grade.Letter,
		Score:            only.grade.Score,
		Summary:          summary,
		Factors:          factors,
		LegGrades:        legGrades,
		Pillars:          pillars,
		CalibratedProb:   only.grade.CalibratedProb,
		MarketProb:       only.grade.MarketProb,
		RawModelProb:     only.grade.RawModelProb,
		VegasImpliedProb: only.grade.VegasImpliedProb,
		FairNoVigProb:    only.grade.FairNoVigProb,
		CorrMultiplier:   1.0,
		Diagnostics:      buildSingleLegSlipDiagnostics(only),
		Uncertain:        only.grade.Uncertain,
	}
}

func singleLegSlipSummary(only v2LegEval, warnings []Warning) string {
	if only.grade.Uncertain {
		return selection(only.leg) + " carries heightened injury or context uncertainty."
	}
	if len(warnings) > 0 {
		return fmt.Sprintf("1-leg ticket with %d heads-up note.", len(warnings))
	}
	if only.grade.Letter == "A" {
		return "Clean 1-leg play: " + only.pricePillar.Summary
	}
	return fmt.Sprintf("Clean 1-leg ticket on %s.", selection(only.leg))
}

func (e *V2Engine) buildMultiLegSlipGrade(
	legs []Leg,
	evals []v2LegEval,
	gradedIdxs []int,
	legGrades []LegGrade,
	warnings []Warning,
	jointEst modelv2.JointParlayEstimate,
	hasJoint bool,
) SlipGrade {
	indepProb := 1.0
	marketIndepProb := 1.0
	hasAllMarket := true
	rawModelIndepProb := 1.0
	hasAllRawModel := true
	combinedDec := 1.0
	hasAllOdds := true
	anyUncertain := false
	chalkCount := 0

	for _, idx := range gradedIdxs {
		ev := evals[idx]
		indepProb *= clampProb(ev.grade.CalibratedProb, v2MinClampedProb, v2MaxClampedProb)
		if ev.grade.MarketProb > 0 && ev.grade.MarketProb < 1 {
			marketIndepProb *= ev.grade.MarketProb
		} else {
			hasAllMarket = false
		}
		if ev.grade.RawModelProb > 0 && ev.grade.RawModelProb < 1 {
			rawModelIndepProb *= ev.grade.RawModelProb
		} else {
			hasAllRawModel = false
		}
		if d := parlay.AmericanToDecimal(ev.leg.Odds); d > 1 {
			combinedDec *= d
		} else {
			hasAllOdds = false
		}
		if ev.grade.Uncertain {
			anyUncertain = true
		}
		if isChalkLeg(ev.leg) {
			chalkCount++
		}
	}

	jointProb := indepProb
	corrMult := 1.0
	if hasJoint {
		jointProb = jointEst.JointProb
		corrMult = jointEst.CorrMultiplier
	}
	slipMarketProb := 0.0
	if hasAllMarket {
		slipMarketProb = marketIndepProb
	}
	slipRawModelProb := 0.0
	if hasAllRawModel {
		slipRawModelProb = rawModelIndepProb
	}
	vegasImpliedProb := 0.0
	if hasAllOdds && combinedDec > 1 {
		vegasImpliedProb = 1.0 / combinedDec
	}

	pricePillar, sgpTaxed := e.scoreSlipPriceVigPillar(
		len(gradedIdxs),
		jointProb,
		corrMult,
		combinedDec,
		hasAllOdds,
	)
	scriptPillar, hasDangerConflict, hasWarnConflict := scoreSlipScriptFitPillar(
		legs,
		evals,
		gradedIdxs,
		warnings,
		jointEst,
		hasJoint,
		corrMult,
	)
	stabPillar := scoreSlipStabilityPillar(evals, gradedIdxs, chalkCount)
	ctxPillar := scoreSlipContextTrapsPillar(evals, gradedIdxs)

	rawScore := e.cfg.SlipWeightPrice*float64(pricePillar.Score) +
		e.cfg.SlipWeightScript*float64(scriptPillar.Score) +
		e.cfg.SlipWeightStability*float64(stabPillar.Score) +
		e.cfg.SlipWeightContext*float64(ctxPillar.Score)
	score := clampInt(int(math.Round(rawScore)), 0, 100)

	if hasDangerConflict || corrMult < 0.85 || pricePillar.Score < v2SevereVigFloor {
		score = min(score, v2CapDangerMax)
	} else if hasWarnConflict || anyUncertain {
		score = min(score, v2CapWarnMax)
	}

	letter := letterForScore(score, e.cfg)
	factors := e.buildSlipGradeFactors(
		len(gradedIdxs),
		chalkCount,
		corrMult,
		sgpTaxed,
		warnings,
		pricePillar,
		scriptPillar,
	)
	summary := buildSlipSummary(
		len(gradedIdxs),
		letter,
		pricePillar,
		scriptPillar,
		ctxPillar,
		anyUncertain,
		corrMult,
		sgpTaxed,
	)

	diag := e.buildMultiLegSlipDiagnostics(
		legs,
		evals,
		gradedIdxs,
		warnings,
		jointEst,
		hasJoint,
		score,
		letter,
		jointProb,
		corrMult,
		vegasImpliedProb,
		slipMarketProb,
		slipRawModelProb,
		combinedDec,
		hasAllOdds,
		sgpTaxed,
		hasDangerConflict,
		hasWarnConflict,
		chalkCount,
		pricePillar,
		scriptPillar,
		stabPillar,
		ctxPillar,
	)

	return SlipGrade{
		Letter:           letter,
		Score:            score,
		Summary:          summary,
		Factors:          factors,
		LegGrades:        legGrades,
		Pillars:          []PillarScore{pricePillar, scriptPillar, stabPillar, ctxPillar},
		CalibratedProb:   jointProb,
		MarketProb:       slipMarketProb,
		RawModelProb:     slipRawModelProb,
		VegasImpliedProb: vegasImpliedProb,
		FairNoVigProb:    slipMarketProb,
		CorrMultiplier:   corrMult,
		Diagnostics:      diag,
		Uncertain:        anyUncertain,
	}
}

func (e *V2Engine) scoreSlipPriceVigPillar(
	gradedCount int,
	jointProb, corrMult, combinedDec float64,
	hasAllOdds bool,
) (PillarScore, bool) {
	if !hasAllOdds || combinedDec <= 1 {
		return PillarScore{
			Key:     PillarPriceVig,
			Label:   PillarLabelPriceVig,
			Score:   v2NeutralScore,
			Tone:    ToneNeutral,
			Summary: "Missing retail odds on one or more legs.",
			Reasons: []string{"Missing retail odds on one or more legs."},
		}, false
	}

	jointEV := jointProb*combinedDec - 1.0
	perLegEquivEV := math.Pow(max(1e-6, 1.0+jointEV), 1.0/float64(gradedCount)) - 1.0
	score := evToPriceScore(perLegEquivEV)

	reasons := make([]string, 0, 3)
	combinedAmerican := parlay.DecimalToAmerican(combinedDec)
	fairCombinedAmerican := parlay.DecimalToAmerican(1.0 / clampProb(jointProb, 1e-6, 0.999))

	switch {
	case perLegEquivEV >= 0.02:
		reasons = append(reasons, fmt.Sprintf(
			"Combined payout %s beats fair ticket odds near %s.",
			americanOddsText(combinedAmerican),
			americanOddsText(fairCombinedAmerican),
		))
	case perLegEquivEV >= -0.055:
		reasons = append(reasons, fmt.Sprintf(
			"Combined payout %s sits close to fair ticket odds near %s.",
			americanOddsText(combinedAmerican),
			americanOddsText(fairCombinedAmerican),
		))
	default:
		reasons = append(reasons, fmt.Sprintf(
			"Paying %s vs. fair ticket odds near %s — steep combined juice.",
			americanOddsText(combinedAmerican),
			americanOddsText(fairCombinedAmerican),
		))
	}

	sgpTaxed := false
	impliedSlipProb := 1.0 / combinedDec
	if corrMult > e.cfg.SGPCorrTaxMinMult && impliedSlipProb > jointProb {
		sgpTaxed = true
		score -= e.cfg.SGPCorrTaxPenalty
		reasons = append(reasons, "This stack fits the script, but the steep leg prices already tax the correlation.")
	}

	if gradedCount > e.cfg.ParlayFreeLegs {
		extra := gradedCount - e.cfg.ParlayFreeLegs
		score -= e.cfg.ParlayVigDragStep * extra
		reasons = append(reasons, fmt.Sprintf("%d legs of book juice shrink the payout.", gradedCount))
	}

	score = clampInt(score, 0, 100)
	return PillarScore{
		Key:     PillarPriceVig,
		Label:   PillarLabelPriceVig,
		Score:   score,
		Tone:    toneForScore(score),
		Summary: reasons[0],
		Reasons: reasons,
	}, sgpTaxed
}

func scoreSlipScriptFitPillar(
	legs []Leg,
	evals []v2LegEval,
	gradedIdxs []int,
	warnings []Warning,
	jointEst modelv2.JointParlayEstimate,
	hasJoint bool,
	corrMult float64,
) (pillar PillarScore, hasDangerConflict, hasWarnConflict bool) {
	score := corrMultToScriptScore(corrMult)

	hasStack := false
	for _, w := range warnings {
		switch w.Rule {
		case ruleTrailingRB, ruleTotalContradiction:
			hasDangerConflict = true
		case ruleBlowoutPassing:
			hasWarnConflict = true
		case ruleStackShootout, ruleStackClockDrain:
			hasStack = true
		}
	}

	switch {
	case hasDangerConflict:
		score = min(score, 44)
	case hasWarnConflict:
		score = min(score, 62)
	case hasStack && corrMult >= 1.0:
		score = max(score, 86)
	}

	reasons := make([]string, 0, 2)
	if hasJoint && len(jointEst.PairCorrelations) > 0 {
		top := jointEst.PairCorrelations[0]
		legA, okA := resolveGradedLeg(legs, gradedIdxs, top.LegAIndex)
		legB, okB := resolveGradedLeg(legs, gradedIdxs, top.LegBIndex)
		switch {
		case okA && okB && top.Correlation >= 0.15:
			reasons = append(reasons, fmt.Sprintf(
				"%s and %s rise together in the same game script.",
				shortLegLabel(legA),
				shortLegLabel(legB),
			))
		case okA && okB && top.Correlation <= -0.15:
			reasons = append(reasons, fmt.Sprintf(
				"%s and %s pull in opposite directions.",
				shortLegLabel(legA),
				shortLegLabel(legB),
			))
		default:
			reasons = append(reasons, "Same-game legs sit near neutral correlation.")
		}
	} else {
		reasons = append(reasons, "Legs span separate games, so they settle independently.")
	}

	for _, w := range warnings {
		if w.Rule == ruleTrailingRB || w.Rule == ruleTotalContradiction ||
			w.Rule == ruleBlowoutPassing || w.Rule == ruleStackShootout || w.Rule == ruleStackClockDrain {
			if len(reasons) < 2 {
				reasons = append(reasons, w.Message)
			}
			break
		}
	}

	// If cross-game legs carry single-leg script notes, surface the strongest non-neutral note.
	if len(reasons) == 1 && (!hasJoint || len(jointEst.PairCorrelations) == 0) {
		for _, idx := range gradedIdxs {
			if evals[idx].hasScriptSig {
				reasons = append(reasons, evals[idx].scriptPillar.Summary)
				break
			}
		}
	}

	score = clampInt(score, 0, 100)
	return PillarScore{
		Key:     PillarScriptFit,
		Label:   PillarLabelScriptFit,
		Score:   score,
		Tone:    toneForScore(score),
		Summary: reasons[0],
		Reasons: reasons,
	}, hasDangerConflict, hasWarnConflict
}

func corrMultToScriptScore(m float64) int {
	switch {
	case m >= 1.10:
		extra := int(math.Round((m - 1.10) * 40))
		return clampInt(90+extra, 90, 98)
	case m >= 1.05:
		return clampInt(82+int(math.Round((m-1.05)/0.05*7)), 82, 89)
	case m >= 0.95:
		return v2NeutralScore
	case m >= 0.85:
		return clampInt(45+int(math.Round((m-0.85)/0.10*17)), 45, 62)
	default:
		return clampInt(44-int(math.Round((0.85-m)*145)), 15, 44)
	}
}

func scoreSlipStabilityPillar(evals []v2LegEval, gradedIdxs []int, chalkCount int) PillarScore {
	sum := 0
	minStab := 100
	minProb := 1.0
	var weakestLeg Leg
	hasAnytimeTD := false

	for _, idx := range gradedIdxs {
		ev := evals[idx]
		sum += ev.stabPillar.Score
		if ev.stabPillar.Score < minStab {
			minStab = ev.stabPillar.Score
		}
		if ev.grade.CalibratedProb < minProb {
			minProb = ev.grade.CalibratedProb
			weakestLeg = ev.leg
		}
		if normalizeSharpMarket(ev.leg.Market) == marketAnytimeTD {
			hasAnytimeTD = true
		}
	}

	avgStab := float64(sum) / float64(len(gradedIdxs))
	score := int(math.Round(0.6*avgStab + 0.4*float64(minStab)))

	reasons := make([]string, 0, 3)
	if len(gradedIdxs) > 2 {
		score -= 3 * (len(gradedIdxs) - 2)
		if len(gradedIdxs) >= 4 {
			reasons = append(reasons, fmt.Sprintf(
				"Compounding %d legs shrinks the margin for error.",
				len(gradedIdxs),
			))
		}
	}

	switch {
	case minProb < v2LongshotProbMax:
		score -= 10
		reasons = append(reasons, fmt.Sprintf(
			"Weakest link sits near %d%% (%s).",
			int(math.Round(minProb*100)),
			shortLegLabel(weakestLeg),
		))
	case minProb < 0.35 || hasAnytimeTD:
		if minProb < 0.35 {
			score -= 5
		}
		reasons = append(reasons, fmt.Sprintf(
			"Higher swing leg near %d%% (%s).",
			int(math.Round(minProb*100)),
			shortLegLabel(weakestLeg),
		))
	}

	if chalkCount > 1 {
		score -= 5 * (chalkCount - 1)
		reasons = append(reasons, fmt.Sprintf(
			"%d heavy-favorite legs compound upset risk for modest payout.",
			chalkCount,
		))
	}

	score = clampInt(score, 0, 100)
	if len(reasons) == 0 {
		reasons = append(reasons, "Balanced leg stability across the ticket.")
	}
	if len(reasons) > 3 {
		reasons = reasons[:3]
	}

	return PillarScore{
		Key:     PillarStability,
		Label:   PillarLabelStability,
		Score:   score,
		Tone:    toneForScore(score),
		Summary: reasons[0],
		Reasons: reasons,
	}
}

func scoreSlipContextTrapsPillar(evals []v2LegEval, gradedIdxs []int) PillarScore {
	sum := 0
	worst := 100
	var allFindings []OverlayFinding

	for _, idx := range gradedIdxs {
		ev := evals[idx]
		sum += ev.ctxPillar.Score
		if ev.ctxPillar.Score < worst {
			worst = ev.ctxPillar.Score
		}
		allFindings = append(allFindings, ev.findings...)
	}

	avg := float64(sum) / float64(len(gradedIdxs))
	score := int(math.Round(avg))
	if worst < v2NeutralScore {
		score = int(math.Round(0.4*avg + 0.6*float64(worst)))
	}
	score = clampInt(score, 0, 100)

	if len(allFindings) == 0 {
		msg := "Clean context across all legs."
		return PillarScore{
			Key:     PillarContextTraps,
			Label:   PillarLabelContextTraps,
			Score:   score,
			Tone:    toneForScore(score),
			Summary: msg,
			Reasons: []string{msg},
		}
	}

	slices.SortStableFunc(allFindings, func(a, b OverlayFinding) int {
		if c := cmp.Compare(b.Severity, a.Severity); c != 0 {
			return c
		}
		absA := a.ScoreDelta
		if absA < 0 {
			absA = -absA
		}
		absB := b.ScoreDelta
		if absB < 0 {
			absB = -absB
		}
		return cmp.Compare(absB, absA)
	})

	reasons := make([]string, 0, 3)
	seenDetail := make(map[string]struct{}, 3)
	for _, f := range allFindings {
		d := strings.TrimSpace(f.Detail)
		if d == "" {
			continue
		}
		if _, dup := seenDetail[d]; dup {
			continue
		}
		seenDetail[d] = struct{}{}
		reasons = append(reasons, d)
		if len(reasons) == 3 {
			break
		}
	}
	if len(reasons) == 0 {
		reasons = append(reasons, "Context flags active on one or more legs.")
	}

	return PillarScore{
		Key:     PillarContextTraps,
		Label:   PillarLabelContextTraps,
		Score:   score,
		Tone:    toneForScore(score),
		Summary: reasons[0],
		Reasons: reasons,
	}
}

func (e *V2Engine) buildSlipGradeFactors(
	gradedCount, chalkCount int,
	corrMult float64,
	sgpTaxed bool,
	warnings []Warning,
	pricePillar, scriptPillar PillarScore,
) []GradeFactor {
	factors := make([]GradeFactor, 0, len(warnings)+4)

	for _, w := range warnings {
		switch w.Rule {
		case ruleStackShootout, ruleStackClockDrain:
			factors = append(factors, GradeFactor{
				Rule:   w.Rule,
				Label:  "Positive stack",
				Delta:  RubricV1.StackBonus,
				Detail: w.Message,
			})
		case ruleBlowoutPassing:
			factors = append(factors, GradeFactor{
				Rule:   w.Rule,
				Label:  "Script conflict",
				Delta:  RubricV1.WarnConflict,
				Detail: w.Message,
			})
		case ruleTrailingRB, ruleTotalContradiction:
			factors = append(factors, GradeFactor{
				Rule:   w.Rule,
				Label:  "Danger conflict",
				Delta:  RubricV1.DangerConflict,
				Detail: w.Message,
			})
		}
	}

	if !hasRuleFactor(factors, ruleStackShootout) && !hasRuleFactor(factors, ruleStackClockDrain) &&
		!hasRuleFactor(factors, ruleBlowoutPassing) && !hasRuleFactor(factors, ruleTrailingRB) &&
		!hasRuleFactor(factors, ruleTotalContradiction) {
		if corrMult >= 1.05 {
			factors = append(factors, GradeFactor{
				Rule:   ruleScriptFit,
				Label:  "Positive correlation",
				Delta:  max(4, scriptPillar.Score-v2NeutralScore),
				Detail: scriptPillar.Summary,
			})
		} else if corrMult < 0.95 {
			factors = append(factors, GradeFactor{
				Rule:   ruleScriptFit,
				Label:  "Correlation drag",
				Delta:  min(-4, scriptPillar.Score-v2NeutralScore),
				Detail: scriptPillar.Summary,
			})
		}
	}

	switch {
	case sgpTaxed:
		factors = append(factors, GradeFactor{
			Rule:   ruleSGPTax,
			Label:  "SGP correlation tax",
			Delta:  -e.cfg.SGPCorrTaxPenalty,
			Detail: "This stack fits the script, but the steep leg prices already tax the correlation.",
		})
	case pricePillar.Score >= 85:
		factors = append(factors, GradeFactor{
			Rule:   rulePriceVig,
			Label:  "Sharp ticket price",
			Delta:  pricePillar.Score - v2NeutralScore,
			Detail: pricePillar.Summary,
		})
	case pricePillar.Score < 60:
		factors = append(factors, GradeFactor{
			Rule:   rulePriceVig,
			Label:  "Steep ticket juice",
			Delta:  pricePillar.Score - v2NeutralScore,
			Detail: pricePillar.Summary,
		})
	}

	if gradedCount > e.cfg.ParlayFreeLegs {
		extra := gradedCount - e.cfg.ParlayFreeLegs
		factors = append(factors, GradeFactor{
			Rule:   ruleParlayDrag,
			Label:  fmt.Sprintf("%d-leg parlay drag", gradedCount),
			Delta:  -(e.cfg.ParlayVigDragStep * extra),
			Detail: fmt.Sprintf("%d legs of book juice shrink the payout.", gradedCount),
		})
	}
	if chalkCount > 1 {
		extra := chalkCount - 1
		factors = append(factors, GradeFactor{
			Rule:   ruleChalkDrag,
			Label:  fmt.Sprintf("%d chalk legs", chalkCount),
			Delta:  RubricV1.ChalkDragPerExtraLeg * extra,
			Detail: fmt.Sprintf("%d heavy-favorite legs compound upset risk for modest payout.", chalkCount),
		})
	}

	return factors
}

func hasRuleFactor(factors []GradeFactor, rule string) bool {
	for _, f := range factors {
		if f.Rule == rule {
			return true
		}
	}
	return false
}

func buildSlipSummary(
	gradedCount int,
	letter string,
	pricePillar, scriptPillar, ctxPillar PillarScore,
	uncertain bool,
	corrMult float64,
	sgpTaxed bool,
) string {
	switch {
	case sgpTaxed:
		return "Coherent script stack, though the leg prices already tax the correlation"
	case uncertain && ctxPillar.Score < v2NeutralScore:
		return ctxPillar.Summary
	case corrMult < 0.85:
		return scriptPillar.Summary
	case corrMult >= 1.08 && pricePillar.Score >= 80:
		return fmt.Sprintf("Strong %d-leg correlated ticket: %s", gradedCount, scriptPillar.Summary)
	case pricePillar.Score < 55:
		return pricePillar.Summary
	case letter == "A":
		return fmt.Sprintf("Sharp %d-leg ticket with strong price and clean script fit", gradedCount)
	case ctxPillar.Score < 65:
		return ctxPillar.Summary
	default:
		return fmt.Sprintf("Balanced %d-leg ticket (%s)", gradedCount, scriptPillar.Summary)
	}
}

func resolveGradedLeg(legs []Leg, gradedIdxs []int, copulaIdx int) (Leg, bool) {
	if copulaIdx < 0 || copulaIdx >= len(gradedIdxs) {
		return Leg{}, false
	}
	legIdx := gradedIdxs[copulaIdx]
	if legIdx < 0 || legIdx >= len(legs) {
		return Leg{}, false
	}
	return legs[legIdx], true
}

func shortLegLabel(leg Leg) string {
	if leg.PlayerName != "" {
		return leg.PlayerName + " " + marketLabel(leg.Market)
	}
	return selection(leg)
}

func toneForScore(score int) string {
	switch {
	case score >= 80:
		return ToneGood
	case score >= 68:
		return ToneNeutral
	case score >= 56:
		return ToneWarn
	default:
		return ToneBad
	}
}

func letterForScore(score int, cfg V2Config) string {
	switch {
	case score >= cfg.BandA:
		return "A"
	case score >= cfg.BandB:
		return "B"
	case score >= cfg.BandC:
		return "C"
	case score >= cfg.BandD:
		return "D"
	default:
		return "F"
	}
}

func clampProb(p, lo, hi float64) float64 {
	if math.IsNaN(p) || p < lo {
		return lo
	}
	if p > hi {
		return hi
	}
	return p
}

func pluralS(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
