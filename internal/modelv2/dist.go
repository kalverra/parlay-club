package modelv2

import (
	"errors"
	"fmt"
	"math"
	"strings"
)

const (
	minLogEta = -30.0
	maxLogEta = 30.0

	maxGammaIter = 10000
	maxBetaIter  = 10000
	cfTiny       = 1e-30
	cfEps        = 1e-15

	negBinDirectSumLimit = 256
	maxUncertaintyBlend  = 0.95
)

// DistStats summarizes the location, spread, and tail asymmetry of a predicted
// prop outcome distribution.
type DistStats struct {
	Mean     float64
	Variance float64
	StdDev   float64
	CV       float64
	Skewness float64
}

// Distribution represents a parameterized outcome distribution for a prop
// market, supporting survival evaluation P(Y > t) and analytical moments.
type Distribution interface {
	Family() string
	Survival(threshold float64) float64
	Stats() DistStats
	Validate() error
}

// LegPrediction holds the distributional, blended, and post-hoc calibrated
// prediction for one prop leg.
type LegPrediction struct {
	Market         string
	Family         string
	Side           string
	Line           float64
	Stats          DistStats
	ModelProb      float64
	MarketProb     float64
	BlendWeight    float64
	BlendedProb    float64
	CalibratedProb float64
}

// ZeroAdjGamma models a continuous non-negative outcome with point mass Pi0 at
// zero and a Gamma(shape=K, rate=K/Mu) positive component.
type ZeroAdjGamma struct {
	Pi0 float64
	Mu  float64
	K   float64
}

// Family returns FamilyZeroAdjGamma.
func (ZeroAdjGamma) Family() string {
	return FamilyZeroAdjGamma
}

// Validate checks that Pi0 is in [0, 1) and Mu, K are finite and positive.
func (g ZeroAdjGamma) Validate() error {
	if !isFiniteInHalfOpenUnit(g.Pi0) {
		return fmt.Errorf("zero_adj_gamma pi0 %v must be finite in [0, 1)", g.Pi0)
	}
	if !isFinitePositive(g.Mu) {
		return fmt.Errorf("zero_adj_gamma mu %v must be finite > 0", g.Mu)
	}
	if !isFinitePositive(g.K) {
		return fmt.Errorf("zero_adj_gamma k %v must be finite > 0", g.K)
	}
	return nil
}

// Survival returns P(Y > threshold) = (1 - Pi0) * Q(K, threshold*K/Mu) for
// threshold > 0, (1 - Pi0) at threshold == 0, and 1.0 for threshold < 0.
func (g ZeroAdjGamma) Survival(threshold float64) float64 {
	if math.IsNaN(threshold) || math.IsInf(threshold, 1) {
		return 0
	}
	if math.IsInf(threshold, -1) || threshold < 0 {
		return 1
	}
	active := 1.0 - g.Pi0
	if threshold == 0 {
		return clampUnit(active)
	}
	if g.Mu <= 0 || g.K <= 0 {
		return 0
	}
	x := threshold * (g.K / g.Mu)
	_, q := regIncGamma(g.K, x)
	return clampUnit(active * q)
}

// Stats returns the exact mixture Mean, Variance, StdDev, CV, and Skewness.
func (g ZeroAdjGamma) Stats() DistStats {
	if g.Mu <= 0 || g.K <= 0 || g.Pi0 < 0 || g.Pi0 >= 1 {
		return DistStats{}
	}
	posCV2 := 1.0 / g.K
	posThirdCentralNorm := 2.0 / (g.K * g.K)
	return zeroAdjustedStats(g.Pi0, g.Mu, posCV2, posThirdCentralNorm)
}

// ZeroAdjLogNormal models a continuous non-negative outcome with point mass
// Pi0 at zero and a LogNormal(Mu, Sigma) positive component where ln(X) ~ N(Mu, Sigma^2).
type ZeroAdjLogNormal struct {
	Pi0   float64
	Mu    float64
	Sigma float64
}

// Family returns FamilyZeroAdjLogNormal.
func (ZeroAdjLogNormal) Family() string {
	return FamilyZeroAdjLogNormal
}

// Validate checks that Pi0 is in [0, 1), Mu is finite, and Sigma > 0.
func (l ZeroAdjLogNormal) Validate() error {
	if !isFiniteInHalfOpenUnit(l.Pi0) {
		return fmt.Errorf("zero_adj_lognormal pi0 %v must be finite in [0, 1)", l.Pi0)
	}
	if math.IsNaN(l.Mu) || math.IsInf(l.Mu, 0) {
		return fmt.Errorf("zero_adj_lognormal mu %v must be finite", l.Mu)
	}
	if !isFinitePositive(l.Sigma) {
		return fmt.Errorf("zero_adj_lognormal sigma %v must be finite > 0", l.Sigma)
	}
	return nil
}

// Survival returns P(Y > threshold) for the zero-adjusted log-normal mixture.
func (l ZeroAdjLogNormal) Survival(threshold float64) float64 {
	if math.IsNaN(threshold) || math.IsInf(threshold, 1) {
		return 0
	}
	if math.IsInf(threshold, -1) || threshold < 0 {
		return 1
	}
	active := 1.0 - l.Pi0
	if threshold == 0 {
		return clampUnit(active)
	}
	if l.Sigma <= 0 {
		return 0
	}
	z := (math.Log(threshold) - l.Mu) / l.Sigma
	survPos := 0.5 * math.Erfc(z/math.Sqrt2)
	return clampUnit(active * survPos)
}

// Stats returns the exact mixture Mean, Variance, StdDev, CV, and Skewness.
func (l ZeroAdjLogNormal) Stats() DistStats {
	if l.Sigma <= 0 || l.Pi0 < 0 || l.Pi0 >= 1 || math.IsNaN(l.Mu) || math.IsInf(l.Mu, 0) {
		return DistStats{}
	}
	s2 := l.Sigma * l.Sigma
	v := math.Expm1(s2)
	posMean := math.Exp(l.Mu + 0.5*s2)
	posThirdCentralNorm := (v + 3.0) * v * v
	return zeroAdjustedStats(l.Pi0, posMean, v, posThirdCentralNorm)
}

// NegBin models a non-negative integer count with mean Mu > 0 and dispersion
// (size) parameter R > 0 such that Var(Y) = Mu + Mu^2/R.
type NegBin struct {
	Mu float64
	R  float64
}

// Family returns FamilyNegBin.
func (NegBin) Family() string {
	return FamilyNegBin
}

// Validate checks that Mu and R are finite and positive.
func (nb NegBin) Validate() error {
	if !isFinitePositive(nb.Mu) {
		return fmt.Errorf("negbin mu %v must be finite > 0", nb.Mu)
	}
	if !isFinitePositive(nb.R) {
		return fmt.Errorf("negbin r %v must be finite > 0", nb.R)
	}
	return nil
}

// PMF returns P(Y = j) evaluated in log-space via math.Lgamma.
func (nb NegBin) PMF(j int) float64 {
	if j < 0 || nb.Mu <= 0 || nb.R <= 0 {
		return 0
	}
	fj := float64(j)
	lgJR, _ := math.Lgamma(fj + nb.R)
	lgR, _ := math.Lgamma(nb.R)
	lgJ1, _ := math.Lgamma(fj + 1.0)
	logP := -math.Log1p(nb.Mu / nb.R)
	logQ := math.Log(nb.Mu / (nb.R + nb.Mu))
	lp := lgJR - lgR - lgJ1 + nb.R*logP + fj*logQ
	return clampUnit(math.Exp(lp))
}

// Survival returns P(Y > threshold) = 1 - P(Y <= floor(threshold)).
// For small thresholds in the lower/mid body it sums log-space PMFs; in the
// upper tail or past negBinDirectSumLimit it evaluates I_{mu/(r+mu)}(k+1, r)
// directly to avoid subtraction cancellation and O(k) loops.
func (nb NegBin) Survival(threshold float64) float64 {
	if math.IsNaN(threshold) || math.IsInf(threshold, 1) {
		return 0
	}
	if math.IsInf(threshold, -1) || threshold < 0 {
		return 1
	}
	if nb.Mu <= 0 || nb.R <= 0 {
		return 0
	}
	kFloat := math.Floor(threshold)
	if kFloat <= negBinDirectSumLimit {
		k := int(kFloat)
		cdf := 0.0
		for j := 0; j <= k; j++ {
			cdf += nb.PMF(j)
		}
		if cdf <= 0.5 {
			return clampUnit(1.0 - cdf)
		}
	}
	q := nb.Mu / (nb.R + nb.Mu)
	return clampUnit(regIncBeta(q, kFloat+1.0, nb.R))
}

// Stats returns the exact Mean, Variance, StdDev, CV, and Skewness of NegBin.
func (nb NegBin) Stats() DistStats {
	if nb.Mu <= 0 || nb.R <= 0 {
		return DistStats{}
	}
	variance := nb.Mu * (1.0 + nb.Mu/nb.R)
	stdDev := math.Sqrt(variance)
	cv := math.Sqrt(1.0/nb.Mu + 1.0/nb.R)
	skew := (1.0 + 2.0*nb.Mu/nb.R) / stdDev
	return DistStats{
		Mean:     nb.Mu,
		Variance: variance,
		StdDev:   stdDev,
		CV:       cv,
		Skewness: skew,
	}
}

// Poisson models a non-negative integer count with rate Lambda > 0.
type Poisson struct {
	Lambda float64
}

// Family returns FamilyPoisson.
func (Poisson) Family() string {
	return FamilyPoisson
}

// Validate checks that Lambda is finite and positive.
func (p Poisson) Validate() error {
	if !isFinitePositive(p.Lambda) {
		return fmt.Errorf("poisson lambda %v must be finite > 0", p.Lambda)
	}
	return nil
}

// PMF returns P(Y = j) evaluated in log-space via math.Lgamma.
func (p Poisson) PMF(j int) float64 {
	if j < 0 || p.Lambda <= 0 {
		return 0
	}
	fj := float64(j)
	lgJ1, _ := math.Lgamma(fj + 1.0)
	lp := -p.Lambda + fj*math.Log(p.Lambda) - lgJ1
	return clampUnit(math.Exp(lp))
}

// Survival returns P(Y > threshold) = P(floor(threshold) + 1, Lambda), using
// the exact Poisson-Gamma identity P(Y >= k+1) = regIncGammaP(k+1, Lambda).
func (p Poisson) Survival(threshold float64) float64 {
	if math.IsNaN(threshold) || math.IsInf(threshold, 1) {
		return 0
	}
	if math.IsInf(threshold, -1) || threshold < 0 {
		return 1
	}
	if p.Lambda <= 0 {
		return 0
	}
	kFloat := math.Floor(threshold)
	if kFloat == 0 {
		return clampUnit(-math.Expm1(-p.Lambda))
	}
	surv, _ := regIncGamma(kFloat+1.0, p.Lambda)
	return clampUnit(surv)
}

// Stats returns the exact Mean, Variance, StdDev, CV, and Skewness of Poisson.
func (p Poisson) Stats() DistStats {
	if p.Lambda <= 0 {
		return DistStats{}
	}
	stdDev := math.Sqrt(p.Lambda)
	invSqrt := 1.0 / stdDev
	return DistStats{
		Mean:     p.Lambda,
		Variance: p.Lambda,
		StdDev:   stdDev,
		CV:       invSqrt,
		Skewness: invSqrt,
	}
}

// Bernoulli models a binary outcome in {0, 1} with event probability P in [0, 1].
type Bernoulli struct {
	P float64
}

// Family returns FamilyBernoulli.
func (Bernoulli) Family() string {
	return FamilyBernoulli
}

// Validate checks that P is finite and in [0, 1].
func (b Bernoulli) Validate() error {
	if math.IsNaN(b.P) || math.IsInf(b.P, 0) || b.P < 0 || b.P > 1 {
		return fmt.Errorf("bernoulli p %v must be finite in [0, 1]", b.P)
	}
	return nil
}

// Survival returns P(Y > threshold): 1.0 when threshold < 0, P when 0 <= threshold < 1,
// and 0.0 when threshold >= 1.
func (b Bernoulli) Survival(threshold float64) float64 {
	if math.IsNaN(threshold) || math.IsInf(threshold, 1) {
		return 0
	}
	if math.IsInf(threshold, -1) || threshold < 0 {
		return 1
	}
	if threshold < 1.0 {
		return clampUnit(b.P)
	}
	return 0
}

// Stats returns the exact Mean, Variance, StdDev, CV, and Skewness of Bernoulli.
func (b Bernoulli) Stats() DistStats {
	if b.P < 0 || b.P > 1 || math.IsNaN(b.P) {
		return DistStats{}
	}
	variance := b.P * (1.0 - b.P)
	stdDev := math.Sqrt(variance)
	var cv, skew float64
	if b.P > 0 {
		cv = math.Sqrt((1.0 - b.P) / b.P)
	}
	if stdDev > 0 {
		skew = (1.0 - 2.0*b.P) / stdDev
	}
	return DistStats{
		Mean:     b.P,
		Variance: variance,
		StdDev:   stdDev,
		CV:       cv,
		Skewness: skew,
	}
}

// EvalDistribution evaluates the GBDT parameter ensembles on an aligned
// feature vector and returns the corresponding validated Distribution.
func (d DistModelArtifact) EvalDistribution(features []float64) (Distribution, error) {
	switch d.Family {
	case FamilyZeroAdjGamma:
		dist, err := d.evalZeroAdjGamma(features)
		if err != nil {
			return nil, err
		}
		return dist, nil
	case FamilyZeroAdjLogNormal:
		dist, err := d.evalZeroAdjLogNormal(features)
		if err != nil {
			return nil, err
		}
		return dist, nil
	case FamilyNegBin:
		dist, err := d.evalNegBin(features)
		if err != nil {
			return nil, err
		}
		return dist, nil
	case FamilyPoisson:
		if _, hasR := d.ParamModels["r"]; hasR {
			dist, err := d.evalPoissonNegBin(features)
			if err != nil {
				return nil, err
			}
			return dist, nil
		}
		dist, err := d.evalPoisson(features)
		if err != nil {
			return nil, err
		}
		return dist, nil
	case FamilyBernoulli:
		dist, err := d.evalBernoulli(features)
		if err != nil {
			return nil, err
		}
		return dist, nil
	default:
		return nil, fmt.Errorf("unsupported family %q", d.Family)
	}
}

func (d DistModelArtifact) evalZeroAdjGamma(features []float64) (ZeroAdjGamma, error) {
	pi0Model, okPi := d.ParamModels["pi0"]
	muModel, okMu := d.ParamModels["mu"]
	kModel, okK := d.ParamModels["k"]
	if !okPi || !okMu || !okK {
		return ZeroAdjGamma{}, errors.New("zero_adj_gamma requires pi0, mu, k param models")
	}
	dist := ZeroAdjGamma{
		Pi0: sigmoid(pi0Model.Eval(features)),
		Mu:  math.Exp(clampLogEta(muModel.Eval(features))),
		K:   math.Exp(clampLogEta(kModel.Eval(features))),
	}
	return dist, dist.Validate()
}

func (d DistModelArtifact) evalZeroAdjLogNormal(features []float64) (ZeroAdjLogNormal, error) {
	pi0Model, okPi := d.ParamModels["pi0"]
	muModel, okMu := d.ParamModels["mu"]
	sigmaModel, okSig := d.ParamModels["sigma"]
	if !okPi || !okMu || !okSig {
		return ZeroAdjLogNormal{}, errors.New("zero_adj_lognormal requires pi0, mu, sigma param models")
	}
	dist := ZeroAdjLogNormal{
		Pi0:   sigmoid(pi0Model.Eval(features)),
		Mu:    muModel.Eval(features),
		Sigma: math.Exp(clampLogEta(sigmaModel.Eval(features))),
	}
	return dist, dist.Validate()
}

func (d DistModelArtifact) evalNegBin(features []float64) (NegBin, error) {
	muModel, okMu := d.ParamModels["mu"]
	rModel, okR := d.ParamModels["r"]
	if !okMu || !okR {
		return NegBin{}, errors.New("negbin requires mu, r param models")
	}
	dist := NegBin{
		Mu: math.Exp(clampLogEta(muModel.Eval(features))),
		R:  math.Exp(clampLogEta(rModel.Eval(features))),
	}
	return dist, dist.Validate()
}

func (d DistModelArtifact) evalPoisson(features []float64) (Poisson, error) {
	lambdaModel, okLam := d.ParamModels["lambda"]
	if !okLam {
		return Poisson{}, errors.New("poisson requires lambda param model")
	}
	dist := Poisson{Lambda: math.Exp(clampLogEta(lambdaModel.Eval(features)))}
	return dist, dist.Validate()
}

func (d DistModelArtifact) evalPoissonNegBin(features []float64) (NegBin, error) {
	lambdaModel, okLam := d.ParamModels["lambda"]
	rModel, okR := d.ParamModels["r"]
	if !okLam || !okR {
		return NegBin{}, errors.New("poisson requires lambda param model")
	}
	dist := NegBin{
		Mu: math.Exp(clampLogEta(lambdaModel.Eval(features))),
		R:  math.Exp(clampLogEta(rModel.Eval(features))),
	}
	return dist, dist.Validate()
}

func (d DistModelArtifact) evalBernoulli(features []float64) (Bernoulli, error) {
	pModel, okP := d.ParamModels["p"]
	if !okP {
		return Bernoulli{}, errors.New("bernoulli requires p param model")
	}
	dist := Bernoulli{P: sigmoid(pModel.Eval(features))}
	return dist, dist.Validate()
}

func (d DistModelArtifact) evalSurvivalAndStats(
	features []float64,
	line float64,
) (family string, pOver float64, stats DistStats, err error) {
	switch d.Family {
	case FamilyZeroAdjGamma:
		dist, err := d.evalZeroAdjGamma(features)
		if err != nil {
			return "", 0, DistStats{}, err
		}
		return dist.Family(), dist.Survival(line), dist.Stats(), nil
	case FamilyZeroAdjLogNormal:
		dist, err := d.evalZeroAdjLogNormal(features)
		if err != nil {
			return "", 0, DistStats{}, err
		}
		return dist.Family(), dist.Survival(line), dist.Stats(), nil
	case FamilyNegBin:
		dist, err := d.evalNegBin(features)
		if err != nil {
			return "", 0, DistStats{}, err
		}
		return dist.Family(), dist.Survival(line), dist.Stats(), nil
	case FamilyPoisson:
		if _, hasR := d.ParamModels["r"]; hasR {
			dist, err := d.evalPoissonNegBin(features)
			if err != nil {
				return "", 0, DistStats{}, err
			}
			return dist.Family(), dist.Survival(line), dist.Stats(), nil
		}
		dist, err := d.evalPoisson(features)
		if err != nil {
			return "", 0, DistStats{}, err
		}
		return dist.Family(), dist.Survival(line), dist.Stats(), nil
	case FamilyBernoulli:
		dist, err := d.evalBernoulli(features)
		if err != nil {
			return "", 0, DistStats{}, err
		}
		return dist.Family(), dist.Survival(line), dist.Stats(), nil
	default:
		return "", 0, DistStats{}, fmt.Errorf("unsupported family %q", d.Family)
	}
}

// PredictLeg evaluates the distributional model, computes survival P(Y > line)
// and shape moments, blends with an optional market anchor (shifting toward the
// market by uncertaintyBoost when context uncertainty is heightened), and
// applies the post-hoc calibrator on the Over/positive probability scale so
// Over and Under calibrated probabilities remain complementary.
func (d DistModelArtifact) PredictLeg(
	features []float64,
	side string,
	line, marketProb, uncertaintyBoost float64,
) (LegPrediction, error) {
	if math.IsNaN(line) || math.IsInf(line, 0) {
		return LegPrediction{}, fmt.Errorf("line %v must be finite", line)
	}
	outSide, isUnder, err := normalizePropSide(side)
	if err != nil {
		return LegPrediction{}, err
	}

	family, pOver, stats, err := d.evalSurvivalAndStats(features, line)
	if err != nil {
		return LegPrediction{}, err
	}

	hasMarket := !math.IsNaN(marketProb) && !math.IsInf(marketProb, 0) && marketProb > 0 && marketProb < 1
	var wEff float64
	blendedOver := pOver
	if hasMarket {
		wEff = effectiveBlendWeight(d.MarketBlendWeight, uncertaintyBoost)
		marketOver := marketProb
		if isUnder {
			marketOver = 1.0 - marketProb
		}
		blendedOver = wEff*marketOver + (1.0-wEff)*pOver
	}

	calOver := d.Calibrator.Calibrate(blendedOver)

	modelProb := pOver
	blendedProb := blendedOver
	calProb := calOver
	if isUnder {
		modelProb = clampUnit(1.0 - pOver)
		blendedProb = clampUnit(1.0 - blendedOver)
		calProb = clampCalibProb(1.0 - calOver)
	}

	return LegPrediction{
		Market:         d.Market,
		Family:         family,
		Side:           outSide,
		Line:           line,
		Stats:          stats,
		ModelProb:      modelProb,
		MarketProb:     marketProbIfValid(marketProb, hasMarket),
		BlendWeight:    wEff,
		BlendedProb:    blendedProb,
		CalibratedProb: calProb,
	}, nil
}

func normalizePropSide(side string) (canonical string, isUnder bool, err error) {
	trimmed := strings.TrimSpace(side)
	switch {
	case trimmed == "", strings.EqualFold(trimmed, "OVER"):
		return "OVER", false, nil
	case strings.EqualFold(trimmed, "YES"):
		return "YES", false, nil
	case strings.EqualFold(trimmed, "UNDER"):
		return "UNDER", true, nil
	case strings.EqualFold(trimmed, "NO"):
		return "NO", true, nil
	default:
		return "", false, fmt.Errorf("unsupported prop side %q", side)
	}
}

func effectiveBlendWeight(baseWeight, uncertaintyBoost float64) float64 {
	w := clampUnit(baseWeight)
	if math.IsNaN(uncertaintyBoost) || math.IsInf(uncertaintyBoost, 0) || uncertaintyBoost <= 0 {
		return w
	}
	boosted := math.Min(maxUncertaintyBlend, w+uncertaintyBoost)
	return math.Max(w, boosted)
}

func marketProbIfValid(marketProb float64, ok bool) float64 {
	if !ok {
		return 0
	}
	return marketProb
}

// zeroAdjustedStats computes exact moments for Y = B * X where P(B = 1) = 1 - pi0
// and X > 0 has mean posMean, squared CV posCV2 = Var(X)/posMean^2, and
// normalized third central moment posThirdCentralNorm = E[(X/posMean - 1)^3].
func zeroAdjustedStats(pi0, posMean, posCV2, posThirdCentralNorm float64) DistStats {
	active := 1.0 - pi0
	mean := active * posMean
	normVar := active * (posCV2 + pi0)
	variance := posMean * posMean * normVar
	stdDev := posMean * math.Sqrt(normVar)

	var cv, skew float64
	if active > 0 {
		cv = math.Sqrt((posCV2 + pi0) / active)
	}
	if normVar > 0 {
		normThird := active * (posThirdCentralNorm + 3.0*pi0*posCV2 + pi0*(2.0*pi0-1.0))
		skew = normThird / (normVar * math.Sqrt(normVar))
	}
	return DistStats{
		Mean:     mean,
		Variance: variance,
		StdDev:   stdDev,
		CV:       cv,
		Skewness: skew,
	}
}

// regIncGamma computes the regularized lower P(a, x) and upper Q(a, x)
// incomplete gamma functions for a > 0, x >= 0.
func regIncGamma(a, x float64) (p, q float64) {
	if a <= 0 || math.IsNaN(a) || math.IsNaN(x) {
		return 0, 0
	}
	if x <= 0 {
		return 0, 1
	}
	if math.IsInf(x, 1) {
		return 1, 0
	}
	lgA, _ := math.Lgamma(a)
	logPrefactor := -x + a*math.Log(x) - lgA

	if x < a+1.0 {
		ap := a
		term := 1.0 / a
		sum := term
		for range maxGammaIter {
			ap += 1.0
			term *= x / ap
			sum += term
			if math.Abs(term) <= math.Abs(sum)*cfEps {
				break
			}
		}
		p = clampUnit(sum * math.Exp(logPrefactor))
		return p, clampUnit(1.0 - p)
	}

	b := x + 1.0 - a
	c := 1.0 / cfTiny
	d := 1.0 / b
	if math.Abs(b) < cfTiny {
		d = 1.0 / cfTiny
	}
	h := d
	for i := 1; i <= maxGammaIter; i++ {
		fi := float64(i)
		an := -fi * (fi - a)
		b += 2.0
		d = an*d + b
		if math.Abs(d) < cfTiny {
			d = cfTiny
		}
		c = b + an/c
		if math.Abs(c) < cfTiny {
			c = cfTiny
		}
		d = 1.0 / d
		del := d * c
		h *= del
		if math.Abs(del-1.0) <= cfEps {
			break
		}
	}
	q = clampUnit(h * math.Exp(logPrefactor))
	return clampUnit(1.0 - q), q
}

// regIncBeta computes the regularized incomplete beta function I_x(a, b) for
// a > 0, b > 0, and x in [0, 1].
func regIncBeta(x, a, b float64) float64 {
	if math.IsNaN(x) || a <= 0 || b <= 0 {
		return 0
	}
	if x <= 0 {
		return 0
	}
	if x >= 1 {
		return 1
	}
	lgA, _ := math.Lgamma(a)
	lgB, _ := math.Lgamma(b)
	lgAB, _ := math.Lgamma(a + b)
	bt := math.Exp(lgAB - lgA - lgB + a*math.Log(x) + b*math.Log1p(-x))
	if x < (a+1.0)/(a+b+2.0) {
		return clampUnit(bt * betaCF(a, b, x) / a)
	}
	return clampUnit(1.0 - bt*betaCF(b, a, 1.0-x)/b)
}

func betaCF(a, b, x float64) float64 {
	qab := a + b
	qap := a + 1.0
	qam := a - 1.0
	c := 1.0
	d := 1.0 - qab*x/qap
	if math.Abs(d) < cfTiny {
		d = cfTiny
	}
	d = 1.0 / d
	h := d
	for m := 1; m <= maxBetaIter; m++ {
		fm := float64(m)
		m2 := 2.0 * fm
		aa := fm * (b - fm) * x / ((qam + m2) * (a + m2))
		d = 1.0 + aa*d
		if math.Abs(d) < cfTiny {
			d = cfTiny
		}
		c = 1.0 + aa/c
		if math.Abs(c) < cfTiny {
			c = cfTiny
		}
		d = 1.0 / d
		h *= d * c

		aa = -(a + fm) * (qab + fm) * x / ((a + m2) * (qap + m2))
		d = 1.0 + aa*d
		if math.Abs(d) < cfTiny {
			d = cfTiny
		}
		c = 1.0 + aa/c
		if math.Abs(c) < cfTiny {
			c = cfTiny
		}
		d = 1.0 / d
		del := d * c
		h *= del
		if math.Abs(del-1.0) <= cfEps {
			break
		}
	}
	return h
}

func clampLogEta(eta float64) float64 {
	if math.IsNaN(eta) {
		return 0
	}
	if eta < minLogEta {
		return minLogEta
	}
	if eta > maxLogEta {
		return maxLogEta
	}
	return eta
}

func clampUnit(p float64) float64 {
	if math.IsNaN(p) || p <= 0 {
		return 0
	}
	if p >= 1 {
		return 1
	}
	return p
}

func isFinitePositive(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0) && v > 0
}

func isFiniteInHalfOpenUnit(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0) && v >= 0 && v < 1
}
