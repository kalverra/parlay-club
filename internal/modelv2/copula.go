package modelv2

import (
	"cmp"
	"fmt"
	"math"
	"math/bits"
	"slices"
	"strings"
	"sync"
)

const (
	qmcSamples = 8192
	maxQMCDims = 32

	minCopulaProb = 1e-4
	maxCopulaProb = 1.0 - 1e-4

	heavyFavoriteSpreadThreshold = 7.0
	shootoutTotalThreshold       = 48.0

	cholZeroPivotTol   = -1e-12
	cholZeroOffDiagTol = 1e-9
	stackMatrixLimit   = 16

	distinctPlayerSameNodeCorr = 0.20
)

// CopulaLeg describes one parlay leg for deterministic Gaussian copula
// joint-probability estimation.
type CopulaLeg struct {
	LegID      int64
	GameID     string
	Team       string
	PlayerID   string
	PlayerName string
	Role       string
	Market     string
	Side       string
	Prob       float64
	HomeSpread float64
	GameTotal  float64
}

// CopulaGameContext carries optional per-game consensus spread and total used
// to select the active correlation regime matrix.
type CopulaGameContext struct {
	HomeTeam   string
	AwayTeam   string
	HomeSpread float64
	GameTotal  float64
	HasSpread  bool
	HasTotal   bool
}

// PairCorrelation describes the signed copula correlation between two legs in
// the same game after within-player overrides and directional sign flips.
type PairCorrelation struct {
	LegAIndex    int
	LegBIndex    int
	LegAID       int64
	LegBID       int64
	GameID       string
	NodeA        string
	NodeB        string
	Correlation  float64
	WithinPlayer bool
}

// JointParlayEstimate holds the deterministic QMC Gaussian copula output for a
// multi-leg parlay slip.
type JointParlayEstimate struct {
	JointProb        float64
	IndependentProb  float64
	CorrMultiplier   float64
	PairCorrelations []PairCorrelation
	RegimesByGame    map[string]string
	JitterApplied    float64
}

type resolvedCopulaLeg struct {
	origIdx   int
	leg       CopulaLeg
	market    string
	nodeIdx   int
	nodeName  string
	sign      float64
	loading   float64
	gameLevel bool
}

type sobolPoly struct {
	degree int
	poly   uint32
	initM  []uint32
}

// Joe & Kuo (2010) new-joe-kuo-6.21201 primitive polynomials and initial
// direction numbers for Sobol dimensions 2..32 (dimension 1 is Van der Corput).
var sobolDimParams = [maxQMCDims - 1]sobolPoly{
	{degree: 1, poly: 0, initM: []uint32{1}},
	{degree: 2, poly: 1, initM: []uint32{1, 3}},
	{degree: 3, poly: 1, initM: []uint32{1, 3, 1}},
	{degree: 3, poly: 2, initM: []uint32{1, 1, 1}},
	{degree: 4, poly: 1, initM: []uint32{1, 1, 3, 3}},
	{degree: 4, poly: 4, initM: []uint32{1, 3, 5, 13}},
	{degree: 5, poly: 2, initM: []uint32{1, 1, 5, 5, 17}},
	{degree: 5, poly: 4, initM: []uint32{1, 1, 5, 5, 5}},
	{degree: 5, poly: 7, initM: []uint32{1, 1, 7, 11, 19}},
	{degree: 5, poly: 11, initM: []uint32{1, 1, 5, 1, 1}},
	{degree: 5, poly: 13, initM: []uint32{1, 1, 1, 3, 11}},
	{degree: 5, poly: 14, initM: []uint32{1, 3, 5, 5, 31}},
	{degree: 6, poly: 1, initM: []uint32{1, 3, 3, 9, 7, 49}},
	{degree: 6, poly: 13, initM: []uint32{1, 1, 1, 15, 21, 21}},
	{degree: 6, poly: 16, initM: []uint32{1, 3, 1, 13, 27, 49}},
	{degree: 6, poly: 19, initM: []uint32{1, 1, 1, 15, 7, 5}},
	{degree: 6, poly: 22, initM: []uint32{1, 3, 1, 15, 13, 25}},
	{degree: 6, poly: 25, initM: []uint32{1, 1, 5, 5, 19, 61}},
	{degree: 7, poly: 1, initM: []uint32{1, 3, 7, 11, 23, 15, 103}},
	{degree: 7, poly: 4, initM: []uint32{1, 3, 7, 13, 13, 15, 69}},
	{degree: 7, poly: 7, initM: []uint32{1, 1, 3, 13, 7, 35, 63}},
	{degree: 7, poly: 8, initM: []uint32{1, 3, 5, 9, 1, 25, 53}},
	{degree: 7, poly: 14, initM: []uint32{1, 3, 1, 13, 9, 35, 107}},
	{degree: 7, poly: 19, initM: []uint32{1, 3, 1, 5, 27, 61, 31}},
	{degree: 7, poly: 21, initM: []uint32{1, 1, 5, 11, 19, 41, 61}},
	{degree: 7, poly: 28, initM: []uint32{1, 3, 5, 3, 3, 13, 69}},
	{degree: 7, poly: 31, initM: []uint32{1, 1, 7, 13, 1, 19, 1}},
	{degree: 7, poly: 32, initM: []uint32{1, 3, 7, 5, 13, 49, 47}},
	{degree: 7, poly: 37, initM: []uint32{1, 1, 5, 11, 21, 17, 19}},
	{degree: 7, poly: 41, initM: []uint32{1, 1, 1, 1, 5, 29, 89}},
	{degree: 7, poly: 42, initM: []uint32{1, 3, 3, 15, 15, 41, 95}},
}

var haltonExtraPrimes = [...]int{
	137, 139, 149, 151, 157, 163, 167, 173,
	179, 181, 191, 193, 197, 199, 211, 223,
}

var qmcNormalsOnce = sync.OnceValue(func() []float64 {
	return buildSobolNormals(qmcSamples, maxQMCDims)
})

// SelectRegime returns the correlation regime key for one game:
// heavy_favorite when |HomeSpread| >= 7, shootout when GameTotal >= 48,
// else neutral.
func SelectRegime(ctx CopulaGameContext) string {
	if ctx.HasSpread && math.Abs(ctx.HomeSpread) >= heavyFavoriteSpreadThreshold {
		return RegimeHeavyFavorite
	}
	if ctx.HasTotal && ctx.GameTotal >= shootoutTotalThreshold {
		return RegimeShootout
	}
	return RegimeNeutral
}

// EstimateJoint evaluates the deterministic QMC Gaussian copula joint
// probability across all legs, factoring across independent games, selecting
// each game's regime matrix, applying within-player overrides and Under sign
// flips, and falling back to unit-diagonal-preserving jitter if a submatrix is
// not positive semi-definite.
func (c CopulaArtifact) EstimateJoint(
	legs []CopulaLeg,
	gameCtx map[string]CopulaGameContext,
) (JointParlayEstimate, error) {
	if len(legs) == 0 {
		return JointParlayEstimate{CorrMultiplier: 1.0}, nil
	}
	for i, leg := range legs {
		if math.IsNaN(leg.Prob) || math.IsInf(leg.Prob, 0) || leg.Prob < 0 || leg.Prob > 1 {
			return JointParlayEstimate{}, fmt.Errorf("leg %d prob %v must be finite in [0, 1]", i, leg.Prob)
		}
	}

	if len(legs) == 1 {
		p := legs[0].Prob
		return JointParlayEstimate{
			JointProb:       p,
			IndependentProb: p,
			CorrMultiplier:  1.0,
		}, nil
	}

	if err := c.checkMatrixDimensions(); err != nil {
		return JointParlayEstimate{}, err
	}

	gameOrder, groups := groupLegsByGame(legs)
	indepProb := 1.0
	jointProb := 1.0
	var maxJitter float64
	var pairs []PairCorrelation
	var regimes map[string]string

	for _, groupKey := range gameOrder {
		idxs := groups[groupKey]
		if len(idxs) == 1 {
			p := legs[idxs[0]].Prob
			indepProb *= p
			jointProb *= p
			continue
		}

		if regimes == nil {
			regimes = make(map[string]string, len(gameOrder))
		}
		ctx := resolveGameContext(legs, idxs, gameCtx)
		regime, mat := c.selectRegimeMatrix(ctx)
		regimes[groupKey] = regime

		for _, idx := range idxs {
			indepProb *= legs[idx].Prob
		}

		gameJoint, gameJitter, gamePairs, err := c.estimateGameJoint(groupKey, legs, idxs, ctx, mat)
		if err != nil {
			return JointParlayEstimate{}, fmt.Errorf("game %q: %w", groupKey, err)
		}
		jointProb *= gameJoint
		if gameJitter > maxJitter {
			maxJitter = gameJitter
		}
		pairs = append(pairs, gamePairs...)
	}

	slices.SortFunc(pairs, comparePairCorrelations)

	corrMult := 1.0
	if indepProb > 0 {
		corrMult = jointProb / indepProb
	} else if jointProb == 0 {
		corrMult = 1.0
	}

	return JointParlayEstimate{
		JointProb:        clampUnit(jointProb),
		IndependentProb:  clampUnit(indepProb),
		CorrMultiplier:   corrMult,
		PairCorrelations: pairs,
		RegimesByGame:    regimes,
		JitterApplied:    maxJitter,
	}, nil
}

func (c CopulaArtifact) checkMatrixDimensions() error {
	n := len(c.Nodes)
	if n < 2 {
		return fmt.Errorf("copula nodes must have at least 2 entries, got %d", n)
	}
	if len(c.DefaultMatrix) != n {
		return fmt.Errorf("default_matrix rows %d != %d nodes", len(c.DefaultMatrix), n)
	}
	for i, row := range c.DefaultMatrix {
		if len(row) != n {
			return fmt.Errorf("default_matrix row %d cols %d != %d nodes", i, len(row), n)
		}
	}
	for regime, mat := range c.RegimeMatrices {
		if len(mat) != n {
			return fmt.Errorf("regime_matrices[%q] rows %d != %d nodes", regime, len(mat), n)
		}
		for i, row := range mat {
			if len(row) != n {
				return fmt.Errorf("regime_matrices[%q] row %d cols %d != %d nodes", regime, i, len(row), n)
			}
		}
	}
	return nil
}

func groupLegsByGame(legs []CopulaLeg) ([]string, map[string][]int) {
	order := make([]string, 0, len(legs))
	groups := make(map[string][]int, len(legs))
	anonCounter := 0
	for i, leg := range legs {
		key := strings.TrimSpace(leg.GameID)
		if key == "" {
			// Legs without a GameID cannot be matched to a shared game and remain independent.
			key = fmt.Sprintf("__independent_%d", anonCounter)
			anonCounter++
		}
		if _, exists := groups[key]; !exists {
			order = append(order, key)
		}
		groups[key] = append(groups[key], i)
	}
	return order, groups
}

func resolveGameContext(
	legs []CopulaLeg,
	idxs []int,
	gameCtx map[string]CopulaGameContext,
) CopulaGameContext {
	if len(idxs) == 0 {
		return CopulaGameContext{}
	}
	gameID := strings.TrimSpace(legs[idxs[0]].GameID)
	if ctx, ok := gameCtx[gameID]; ok {
		return ctx
	}
	var fallback CopulaGameContext
	for _, idx := range idxs {
		leg := legs[idx]
		if !fallback.HasSpread && leg.HomeSpread != 0 && !math.IsNaN(leg.HomeSpread) {
			fallback.HomeSpread = leg.HomeSpread
			fallback.HasSpread = true
		}
		if !fallback.HasTotal && leg.GameTotal > 0 && !math.IsNaN(leg.GameTotal) {
			fallback.GameTotal = leg.GameTotal
			fallback.HasTotal = true
		}
	}
	return fallback
}

func (c CopulaArtifact) selectRegimeMatrix(ctx CopulaGameContext) (string, [][]float64) {
	regime := SelectRegime(ctx)
	if mat, ok := c.RegimeMatrices[regime]; ok {
		return regime, mat
	}
	if regime != RegimeNeutral {
		if mat, ok := c.RegimeMatrices[RegimeNeutral]; ok {
			return RegimeNeutral, mat
		}
	}
	return RegimeNeutral, c.DefaultMatrix
}

func (c CopulaArtifact) estimateGameJoint(
	gameID string,
	legs []CopulaLeg,
	idxs []int,
	ctx CopulaGameContext,
	mat [][]float64,
) (jointProb, jitter float64, pairs []PairCorrelation, err error) {
	m := len(idxs)
	refTeam := findRefTeam(legs, idxs, ctx)

	var resolvedStack [stackMatrixLimit]resolvedCopulaLeg
	resolved := resolvedStack[:0]
	if m > stackMatrixLimit {
		resolved = make([]resolvedCopulaLeg, 0, m)
	}
	for _, idx := range idxs {
		resolved = append(resolved, c.resolveLeg(idx, legs[idx], refTeam))
	}

	var subStack [stackMatrixLimit * stackMatrixLimit]float64
	var cholStack [stackMatrixLimit * stackMatrixLimit]float64
	var zStack [stackMatrixLimit]float64

	var sub, chol []float64
	var z []float64
	if m <= stackMatrixLimit {
		sub = subStack[:m*m]
		chol = cholStack[:m*m]
		z = zStack[:m]
	} else {
		sub = make([]float64, m*m)
		chol = make([]float64, m*m)
		z = make([]float64, m)
	}

	pairs = make([]PairCorrelation, 0, m*(m-1)/2)
	for i := range m {
		sub[i*m+i] = 1.0
		pClamped := clampCopulaProb(resolved[i].leg.Prob)
		z[i] = normInv(pClamped)
		for j := i + 1; j < m; j++ {
			signedCorr, within := c.pairCorrelation(resolved[i], resolved[j], mat)
			sub[i*m+j] = signedCorr
			sub[j*m+i] = signedCorr
			pairs = append(pairs, PairCorrelation{
				LegAIndex:    resolved[i].origIdx,
				LegBIndex:    resolved[j].origIdx,
				LegAID:       resolved[i].leg.LegID,
				LegBID:       resolved[j].leg.LegID,
				GameID:       gameID,
				NodeA:        resolved[i].nodeName,
				NodeB:        resolved[j].nodeName,
				Correlation:  signedCorr,
				WithinPlayer: within,
			})
		}
	}

	jitter, err = choleskyWithJitter(chol, sub, m)
	if err != nil {
		return 0, 0, nil, err
	}

	jointProb = evalQMCMVN(chol, z, m)
	return jointProb, jitter, pairs, nil
}

func findRefTeam(legs []CopulaLeg, idxs []int, ctx CopulaGameContext) string {
	for _, idx := range idxs {
		if team := strings.TrimSpace(legs[idx].Team); team != "" {
			return team
		}
	}
	return strings.TrimSpace(ctx.HomeTeam)
}

func (c CopulaArtifact) resolveLeg(origIdx int, leg CopulaLeg, refTeam string) resolvedCopulaLeg {
	market := strings.ToUpper(strings.TrimSpace(leg.Market))
	side := strings.ToUpper(strings.TrimSpace(leg.Side))
	sign := 1.0
	if side == "UNDER" || side == "NO" {
		sign = -1.0
	}

	team := strings.TrimSpace(leg.Team)
	isOpp := refTeam != "" && team != "" && !strings.EqualFold(team, refTeam)

	switch market {
	case "SPREAD", "ML":
		if isOpp {
			sign = -sign
		}
		idx := c.findNodeIndex("TEAM_MARGIN")
		return resolvedCopulaLeg{
			origIdx:   origIdx,
			leg:       leg,
			market:    market,
			nodeIdx:   idx,
			nodeName:  "TEAM_MARGIN",
			sign:      sign,
			loading:   1.0,
			gameLevel: true,
		}
	case "TOTAL":
		idx := c.findNodeIndex("GAME_TOTAL")
		return resolvedCopulaLeg{
			origIdx:   origIdx,
			leg:       leg,
			market:    market,
			nodeIdx:   idx,
			nodeName:  "GAME_TOTAL",
			sign:      sign,
			loading:   1.0,
			gameLevel: true,
		}
	case "TEAM_TOTAL":
		prefix := ""
		if isOpp {
			prefix = "OPP_"
		}
		name := prefix + "TEAM_TOTAL"
		if idx := c.findNodeIndex(name); idx >= 0 {
			return resolvedCopulaLeg{
				origIdx:   origIdx,
				leg:       leg,
				market:    market,
				nodeIdx:   idx,
				nodeName:  name,
				sign:      sign,
				loading:   1.0,
				gameLevel: true,
			}
		}
		// Proxy TEAM_TOTAL through the team's passing-TD / game-scoring factor so it
		// correlates positively with both TEAM_MARGIN and GAME_TOTAL while remaining PSD.
		proxy := prefix + "QB_PASS_TDS"
		idx := c.findNodeIndex(proxy)
		if idx < 0 {
			proxy = "GAME_TOTAL"
			idx = c.findNodeIndex(proxy)
		}
		return resolvedCopulaLeg{
			origIdx:   origIdx,
			leg:       leg,
			market:    market,
			nodeIdx:   idx,
			nodeName:  name,
			sign:      sign,
			loading:   0.75,
			gameLevel: true,
		}
	}

	prefix := ""
	if isOpp {
		prefix = "OPP_"
	}
	role := normalizeCopulaRole(leg.Role, market)
	canonicalName := prefix + role + "_" + market
	if idx := c.findNodeIndex(canonicalName); idx >= 0 {
		return resolvedCopulaLeg{
			origIdx:  origIdx,
			leg:      leg,
			market:   market,
			nodeIdx:  idx,
			nodeName: canonicalName,
			sign:     sign,
			loading:  1.0,
		}
	}

	proxyName, loading := c.proxyPropNode(prefix, role, market)
	idx := -1
	if proxyName != "" {
		idx = c.findNodeIndex(proxyName)
	}
	return resolvedCopulaLeg{
		origIdx:  origIdx,
		leg:      leg,
		market:   market,
		nodeIdx:  idx,
		nodeName: canonicalName,
		sign:     sign,
		loading:  loading,
	}
}

func (c CopulaArtifact) findNodeIndex(name string) int {
	for i, node := range c.Nodes {
		if node == name {
			return i
		}
	}
	return -1
}

func normalizeCopulaRole(role, market string) string {
	r := strings.ToUpper(strings.TrimSpace(role))
	switch r {
	case "QB", "WR1", "WR2", "TE1", "RB1", "RB2":
		return r
	case "WR":
		return "WR2"
	case "RB":
		return "RB2"
	case "TE":
		return "TE1"
	}
	switch market {
	case MarketPassYds, MarketPassTDs:
		return "QB"
	case MarketRushYds, MarketRushAtt:
		return "RB2"
	default:
		return "WR2"
	}
}

func (c CopulaArtifact) proxyPropNode(prefix, role, market string) (string, float64) {
	candidates := proxyCandidates(prefix, role, market)
	for _, cand := range candidates {
		if c.findNodeIndex(cand.name) >= 0 {
			return cand.name, cand.loading
		}
	}
	return "", 1.0
}

type nodeProxy struct {
	name    string
	loading float64
}

func proxyCandidates(prefix, role, market string) []nodeProxy {
	switch market {
	case MarketAnytimeTD:
		return anytimeTDProxies(prefix, role)
	case MarketReceptions:
		if role == "RB1" || role == "RB2" {
			return []nodeProxy{
				{prefix + "RB1_REC_YDS", 0.85},
				{prefix + "QB_PASS_YDS", 0.30},
			}
		}
		return []nodeProxy{
			{prefix + role + "_REC_YDS", 0.85},
			{prefix + "WR2_REC_YDS", 0.80},
		}
	case MarketRushAtt:
		return []nodeProxy{
			{prefix + role + "_RUSH_YDS", 0.85},
			{prefix + "RB1_RUSH_YDS", 0.65},
		}
	case MarketRushYds:
		if role == "QB" {
			return []nodeProxy{{"GAME_TOTAL", 0.25}}
		}
		return []nodeProxy{{prefix + "RB1_RUSH_YDS", 0.50}}
	case MarketRecYds:
		return []nodeProxy{
			{prefix + "QB_PASS_YDS", 0.30},
			{"GAME_TOTAL", 0.25},
		}
	case MarketPassYds, MarketPassTDs:
		return []nodeProxy{{prefix + "QB_" + market, 0.75}}
	default:
		return nil
	}
}

func anytimeTDProxies(prefix, role string) []nodeProxy {
	switch role {
	case "QB":
		return []nodeProxy{
			{prefix + "QB_PASS_TDS", 0.45},
			{"TEAM_MARGIN", 0.30},
		}
	case "RB1", "RB2":
		return []nodeProxy{
			{prefix + role + "_RUSH_YDS", 0.65},
			{prefix + "RB1_RUSH_YDS", 0.55},
		}
	default:
		return []nodeProxy{
			{prefix + role + "_REC_YDS", 0.65},
			{prefix + "WR2_REC_YDS", 0.60},
		}
	}
}

func (c CopulaArtifact) pairCorrelation(
	a, b resolvedCopulaLeg,
	mat [][]float64,
) (signed float64, withinPlayer bool) {
	var rho float64
	if a.nodeIdx >= 0 && b.nodeIdx >= 0 {
		rho = 0.5 * (mat[a.nodeIdx][b.nodeIdx] + mat[b.nodeIdx][a.nodeIdx])
		rho *= a.loading * b.loading
	}

	if !a.gameLevel && !b.gameLevel {
		if isSamePlayer(a.leg, b.leg) {
			withinPlayer = true
			switch a.market {
			case b.market:
				rho = 1.0
			default:
				if wp, ok := lookupWithinPlayer(c.WithinPlayer, a.market, b.market); ok {
					rho = wp
				}
			}
		} else if a.nodeIdx >= 0 && a.nodeIdx == b.nodeIdx {
			// Two distinct players resolving to the same role node are not identical variables.
			rho *= distinctPlayerSameNodeCorr
		}
	}

	signed = clampCorr(a.sign * b.sign * rho)
	return signed, withinPlayer
}

func isSamePlayer(a, b CopulaLeg) bool {
	if a.PlayerID != "" && b.PlayerID != "" {
		return a.PlayerID == b.PlayerID
	}
	if a.PlayerName == "" || b.PlayerName == "" {
		return false
	}
	if a.Team != "" && b.Team != "" && !strings.EqualFold(a.Team, b.Team) {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(a.PlayerName), strings.TrimSpace(b.PlayerName))
}

func lookupWithinPlayer(within map[string]float64, marketA, marketB string) (float64, bool) {
	if len(within) == 0 {
		return 0, false
	}
	if v, ok := within[marketA+":"+marketB]; ok {
		return v, true
	}
	if v, ok := within[marketB+":"+marketA]; ok {
		return v, true
	}
	return 0, false
}

// choleskyWithJitter computes the lower-triangular Cholesky factor chol of the
// symmetric unit-diagonal matrix mat (n x n). If mat is slightly indefinite due
// to within-player overrides or rounding, it iteratively shrinks off-diagonal
// entries by 1/(1+lambda) (preserving unit diagonal and marginal laws) until
// Cholesky succeeds.
func choleskyWithJitter(chol, mat []float64, n int) (float64, error) {
	jitterSteps := [...]float64{
		0, 1e-9, 1e-7, 1e-5, 1e-4, 1e-3,
		0.005, 0.01, 0.025, 0.05, 0.1, 0.2, 0.4, 0.8, 1.6, 3.2, 6.4, 12.8, 25.6,
	}
	for _, lambda := range jitterSteps {
		if tryCholeskyScaled(chol, mat, n, lambda) {
			return lambda, nil
		}
	}
	return 0, fmt.Errorf("cholesky factorization failed for %dx%d correlation matrix", n, n)
}

func tryCholeskyScaled(chol, mat []float64, n int, lambda float64) bool {
	scale := 1.0 / (1.0 + lambda)
	for i := range n {
		rowOffset := i * n
		for j := range i + 1 {
			var sum float64
			if i == j {
				sum = 1.0
			} else {
				sum = mat[rowOffset+j] * scale
			}
			colOffset := j * n
			for k := range j {
				sum -= chol[rowOffset+k] * chol[colOffset+k]
			}
			if i == j {
				if sum < cholZeroPivotTol {
					return false
				}
				if sum < 0 {
					sum = 0
				}
				chol[rowOffset+j] = math.Sqrt(sum)
				continue
			}
			diag := chol[colOffset+j]
			if diag == 0 {
				if math.Abs(sum) > cholZeroOffDiagTol {
					return false
				}
				chol[rowOffset+j] = 0
				continue
			}
			chol[rowOffset+j] = sum / diag
		}
	}
	return true
}

func evalQMCMVN(chol, z []float64, m int) float64 {
	normals := qmcNormalsOnce()
	hits := 0
	if m <= maxQMCDims {
		for r := range qmcSamples {
			u := normals[r*maxQMCDims:]
			ok := true
			for i := range m {
				row := chol[i*m:]
				sum := 0.0
				for j := 0; j <= i; j++ {
					sum += row[j] * u[j]
				}
				if sum > z[i] {
					ok = false
					break
				}
			}
			if ok {
				hits++
			}
		}
		return float64(hits) / float64(qmcSamples)
	}

	extra := make([]float64, m-maxQMCDims)
	for r := range qmcSamples {
		u := normals[r*maxQMCDims:]
		for d := maxQMCDims; d < m; d++ {
			prime := haltonExtraPrimes[(d-maxQMCDims)%len(haltonExtraPrimes)]
			extra[d-maxQMCDims] = normInv(haltonPoint(r+1, prime))
		}
		ok := true
		for i := range m {
			row := chol[i*m:]
			sum := 0.0
			limit := min(i+1, maxQMCDims)
			for j := range limit {
				sum += row[j] * u[j]
			}
			for j := maxQMCDims; j <= i; j++ {
				sum += row[j] * extra[j-maxQMCDims]
			}
			if sum > z[i] {
				ok = false
				break
			}
		}
		if ok {
			hits++
		}
	}
	return float64(hits) / float64(qmcSamples)
}

func buildSobolNormals(nSamples, dims int) []float64 {
	const numBits = 32
	v := make([][]uint32, dims)
	for d := range dims {
		vd := make([]uint32, numBits)
		if d == 0 {
			for k := range numBits {
				vd[k] = uint32(1) << (31 - k)
			}
		} else {
			spec := sobolDimParams[d-1]
			s := spec.degree
			for k := range s {
				vd[k] = spec.initM[k] << (31 - k)
			}
			for k := s; k < numBits; k++ {
				val := vd[k-s] ^ (vd[k-s] >> s)
				for i := 1; i < s; i++ {
					if (spec.poly>>uint(s-1-i))&1 != 0 {
						val ^= vd[k-i]
					}
				}
				vd[k] = val
			}
		}
		v[d] = vd
	}

	out := make([]float64, nSamples*dims)
	x := make([]uint32, dims)
	const inv2Pow32 = 1.0 / 4294967296.0
	midpointShift := 0.5 / float64(nSamples)

	for r := range nSamples {
		base := r * dims
		for d := range dims {
			u := float64(x[d])*inv2Pow32 + midpointShift
			out[base+d] = normInv(u)
		}
		if r+1 < nSamples {
			c := bits.TrailingZeros32(^uint32(r))
			for d := range dims {
				x[d] ^= v[d][c]
			}
		}
	}
	return out
}

func haltonPoint(index, base int) float64 {
	f := 1.0
	r := 0.0
	i := index
	fb := float64(base)
	for i > 0 {
		f /= fb
		r += f * float64(i%base)
		i /= base
	}
	if r <= 0 {
		return 1e-12
	}
	if r >= 1 {
		return 1 - 1e-12
	}
	return r
}

func comparePairCorrelations(a, b PairCorrelation) int {
	absA := math.Abs(a.Correlation)
	absB := math.Abs(b.Correlation)
	if absA != absB {
		return cmp.Compare(absB, absA)
	}
	if a.Correlation != b.Correlation {
		return cmp.Compare(b.Correlation, a.Correlation)
	}
	if a.LegAIndex != b.LegAIndex {
		return cmp.Compare(a.LegAIndex, b.LegAIndex)
	}
	return cmp.Compare(a.LegBIndex, b.LegBIndex)
}

func clampCopulaProb(p float64) float64 {
	if p < minCopulaProb {
		return minCopulaProb
	}
	if p > maxCopulaProb {
		return maxCopulaProb
	}
	return p
}

func clampCorr(rho float64) float64 {
	if math.IsNaN(rho) {
		return 0
	}
	if rho < -1.0 {
		return -1.0
	}
	if rho > 1.0 {
		return 1.0
	}
	return rho
}

// normInv returns the inverse standard normal CDF Φ⁻¹(p) using Acklam's
// rational approximation (relative error < 1.15e-9 across (0, 1)).
func normInv(p float64) float64 {
	if p <= 0 {
		return math.Inf(-1)
	}
	if p >= 1 {
		return math.Inf(1)
	}

	const (
		a1 = -3.969683028665376e+01
		a2 = 2.209460984245205e+02
		a3 = -2.759285104469687e+02
		a4 = 1.383577518672690e+02
		a5 = -3.066479806614716e+01
		a6 = 2.506628277459239e+00

		b1 = -5.447609879822406e+01
		b2 = 1.615858368580409e+02
		b3 = -1.556989798598866e+02
		b4 = 6.680131188771972e+01
		b5 = -1.328068155288572e+01

		c1 = -7.784894002430293e-03
		c2 = -3.223964580411365e-01
		c3 = -2.400758277161838e+00
		c4 = -2.549732539343734e+00
		c5 = 4.374664141464968e+00
		c6 = 2.938163982698783e+00

		d1 = 7.784695709041462e-03
		d2 = 3.224671290700398e-01
		d3 = 2.445134137142996e+00
		d4 = 3.754408661907416e+00

		pLow  = 0.02425
		pHigh = 1 - pLow
	)

	if p < pLow {
		q := math.Sqrt(-2 * math.Log(p))
		return (((((c1*q+c2)*q+c3)*q+c4)*q+c5)*q + c6) / ((((d1*q+d2)*q+d3)*q+d4)*q + 1)
	}
	if p <= pHigh {
		q := p - 0.5
		r := q * q
		return (((((a1*r+a2)*r+a3)*r+a4)*r+a5)*r + a6) * q / (((((b1*r+b2)*r+b3)*r+b4)*r+b5)*r + 1)
	}
	q := math.Sqrt(-2 * math.Log(1-p))
	return -(((((c1*q+c2)*q+c3)*q+c4)*q+c5)*q + c6) / ((((d1*q+d2)*q+d3)*q+d4)*q + 1)
}
