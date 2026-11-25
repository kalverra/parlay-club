// Package modelv2 implements the pure-Go, zero-I/O v2 inference contract:
// embedded JSON artifact loading, schema and PSD validation, flat GBDT tree
// evaluation, Weeks 1-5 Empirical Bayes shrinkage, and post-hoc calibration.
package modelv2

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"sync"
	"time"
)

const (
	// CurrentFeatureSchemaVersion is the feature contract version expected by
	// this binary. Artifacts built for another schema version are rejected.
	CurrentFeatureSchemaVersion = 1

	// DefaultArtifactPath is the embedded baseline artifact path.
	DefaultArtifactPath = "artifacts/v2.json"
)

// Supported distributional model families for player prop markets.
const (
	FamilyZeroAdjGamma     = "zero_adj_gamma"
	FamilyZeroAdjLogNormal = "zero_adj_lognormal"
	FamilyNegBin           = "negbin"
	FamilyPoisson          = "poisson"
	FamilyBernoulli        = "bernoulli"
)

// Canonical player prop market keys stored in ArtifactBundle.PropModels.
const (
	MarketPassYds    = "PASS_YDS"
	MarketRushYds    = "RUSH_YDS"
	MarketRecYds     = "REC_YDS"
	MarketReceptions = "RECEPTIONS"
	MarketRushAtt    = "RUSH_ATT"
	MarketPassTDs    = "PASS_TDS"
	MarketAnytimeTD  = "ANYTIME_TD"
)

// Canonical copula regime keys in CopulaArtifact.RegimeMatrices.
const (
	RegimeNeutral       = "neutral"
	RegimeHeavyFavorite = "heavy_favorite"
	RegimeShootout      = "shootout"
)

const (
	minSeason        = 1999
	maxSeason        = 2100
	corrMatrixTol    = 1e-6
	psdEigenvalueTol = -1e-9
)

//go:embed artifacts/*.json
var embeddedArtifacts embed.FS

var loadDefaultOnce = sync.OnceValues(func() (*ArtifactBundle, error) {
	return LoadEmbeddedPath(DefaultArtifactPath)
})

// ArtifactBundle is the versioned, self-contained model artifact loaded from
// embedded JSON at startup.
type ArtifactBundle struct {
	Version              string                       `json:"version"`
	FeatureSchemaVersion int                          `json:"feature_schema_version"`
	TrainedSeasons       []int                        `json:"trained_seasons"`
	BuiltAt              string                       `json:"built_at"`
	Validation           ValidationSummary            `json:"validation"`
	PropModels           map[string]DistModelArtifact `json:"prop_models"`
	Priors               map[string]RolePrior         `json:"priors"`
	Copula               CopulaArtifact               `json:"copula"`
}

// ValidationSummary records offline walk-forward validation metrics exported
// alongside the trained artifact.
type ValidationSummary struct {
	HeldOutSeasons    []int   `json:"held_out_seasons,omitempty"`
	LogLoss           float64 `json:"log_loss,omitempty"`
	BrierScore        float64 `json:"brier_score,omitempty"`
	MarketLogLoss     float64 `json:"market_log_loss,omitempty"`
	MarketBrierScore  float64 `json:"market_brier_score,omitempty"`
	SGPLogLoss        float64 `json:"sgp_log_loss,omitempty"`
	NaiveIndepLogLoss float64 `json:"naive_indep_log_loss,omitempty"`
	PopulatedBins     int     `json:"populated_bins,omitempty"`
	TotalBins         int     `json:"total_bins,omitempty"`
	Notes             string  `json:"notes,omitempty"`
}

// DistModelArtifact defines the distributional GBDT models, ordered feature
// names, market anchor blend weight, and post-hoc calibrator for one market.
type DistModelArtifact struct {
	Market            string               `json:"market"`
	Family            string               `json:"family"`
	FeatureNames      []string             `json:"feature_names"`
	ParamModels       map[string]GBDTModel `json:"param_models"`
	Calibrator        CalibratorArtifact   `json:"calibrator"`
	MarketBlendWeight float64              `json:"market_blend_weight"`
}

// CopulaArtifact stores canonical node names, regime correlation matrices, and
// within-player correlation overrides for same-game parlay dependency modeling.
type CopulaArtifact struct {
	Nodes          []string               `json:"nodes"`
	DefaultMatrix  [][]float64            `json:"default_matrix"`
	RegimeMatrices map[string][][]float64 `json:"regime_matrices,omitempty"`
	WithinPlayer   map[string]float64     `json:"within_player"`
}

// LoadEmbedded returns the validated default embedded ArtifactBundle.
func LoadEmbedded() (*ArtifactBundle, error) {
	return loadDefaultOnce()
}

// LoadEmbeddedPath reads and validates a named artifact JSON file from the
// embedded artifacts filesystem.
func LoadEmbeddedPath(name string) (*ArtifactBundle, error) {
	raw, err := embeddedArtifacts.ReadFile(name)
	if err != nil {
		return nil, fmt.Errorf("read embedded artifact %q: %w", name, err)
	}
	bundle, err := ParseArtifactBundle(raw)
	if err != nil {
		return nil, fmt.Errorf("load embedded artifact %q: %w", name, err)
	}
	return bundle, nil
}

// ParseArtifactBundle unmarshals JSON artifact bytes and validates schema,
// tree structure, calibrators, priors, and copula PSD invariants.
func ParseArtifactBundle(data []byte) (*ArtifactBundle, error) {
	if len(data) == 0 {
		return nil, errors.New("empty artifact payload")
	}
	var bundle ArtifactBundle
	if err := json.Unmarshal(data, &bundle); err != nil {
		return nil, fmt.Errorf("decode artifact JSON: %w", err)
	}
	if err := bundle.Validate(); err != nil {
		return nil, fmt.Errorf("validate artifact bundle: %w", err)
	}
	return &bundle, nil
}

// Validate checks every schema, model, prior, calibrator, and copula matrix
// invariant on the ArtifactBundle.
func (b *ArtifactBundle) Validate() error {
	if b == nil {
		return errors.New("artifact bundle is nil")
	}
	if b.Version == "" {
		return errors.New("version must not be empty")
	}
	if b.FeatureSchemaVersion != CurrentFeatureSchemaVersion {
		return fmt.Errorf(
			"unsupported feature_schema_version %d (want %d)",
			b.FeatureSchemaVersion,
			CurrentFeatureSchemaVersion,
		)
	}
	if len(b.TrainedSeasons) == 0 {
		return errors.New("trained_seasons must not be empty")
	}
	for i, s := range b.TrainedSeasons {
		if s < minSeason || s > maxSeason {
			return fmt.Errorf("trained_seasons[%d]=%d out of range [%d, %d]", i, s, minSeason, maxSeason)
		}
	}
	if b.BuiltAt == "" {
		return errors.New("built_at must not be empty")
	}
	if _, err := time.Parse(time.RFC3339, b.BuiltAt); err != nil {
		return fmt.Errorf("built_at must be RFC3339: %w", err)
	}
	if len(b.PropModels) == 0 {
		return errors.New("prop_models must not be empty")
	}
	for key, model := range b.PropModels {
		if model.Market == "" {
			return fmt.Errorf("prop_models[%q]: market must not be empty", key)
		}
		if model.Market != key {
			return fmt.Errorf("prop_models[%q]: market %q does not match map key", key, model.Market)
		}
		if err := model.Validate(); err != nil {
			return fmt.Errorf("prop_models[%q]: %w", key, err)
		}
	}
	if len(b.Priors) == 0 {
		return errors.New("priors must not be empty")
	}
	for key, prior := range b.Priors {
		if err := prior.Validate(key); err != nil {
			return fmt.Errorf("priors[%q]: %w", key, err)
		}
	}
	if err := b.Copula.Validate(); err != nil {
		return fmt.Errorf("copula: %w", err)
	}
	return nil
}

// Validate checks one DistModelArtifact's family, feature list, parameter
// GBDTs, blend weight, and calibrator.
func (d DistModelArtifact) Validate() error {
	reqParams, ok := requiredParamsForFamily(d.Family)
	if !ok {
		return fmt.Errorf("unsupported family %q", d.Family)
	}
	if len(d.FeatureNames) == 0 {
		return errors.New("feature_names must not be empty")
	}
	seenFeatures := make(map[string]struct{}, len(d.FeatureNames))
	for i, name := range d.FeatureNames {
		if name == "" {
			return fmt.Errorf("feature_names[%d] must not be empty", i)
		}
		if _, dup := seenFeatures[name]; dup {
			return fmt.Errorf("duplicate feature name %q", name)
		}
		seenFeatures[name] = struct{}{}
	}
	if math.IsNaN(d.MarketBlendWeight) || math.IsInf(d.MarketBlendWeight, 0) ||
		d.MarketBlendWeight < 0 || d.MarketBlendWeight > 1 {
		return fmt.Errorf("market_blend_weight %v must be finite in [0, 1]", d.MarketBlendWeight)
	}
	if len(d.ParamModels) == 0 {
		return errors.New("param_models must not be empty")
	}
	for _, req := range reqParams {
		if _, exists := d.ParamModels[req]; !exists {
			return fmt.Errorf("missing required param_model %q for family %q", req, d.Family)
		}
	}
	for paramName, gbdt := range d.ParamModels {
		if paramName == "" {
			return errors.New("param_models key must not be empty")
		}
		if err := gbdt.Validate(len(d.FeatureNames)); err != nil {
			return fmt.Errorf("param_models[%q]: %w", paramName, err)
		}
	}
	if err := d.Calibrator.Validate(); err != nil {
		return fmt.Errorf("calibrator: %w", err)
	}
	return nil
}

// Validate checks node names, within-player correlation bounds, and symmetry +
// PSD invariants on DefaultMatrix and every RegimeMatrices entry.
func (c CopulaArtifact) Validate() error {
	n := len(c.Nodes)
	if n < 2 {
		return fmt.Errorf("nodes must have at least 2 entries, got %d", n)
	}
	seen := make(map[string]struct{}, n)
	for i, node := range c.Nodes {
		if node == "" {
			return fmt.Errorf("nodes[%d] must not be empty", i)
		}
		if _, dup := seen[node]; dup {
			return fmt.Errorf("duplicate node name %q", node)
		}
		seen[node] = struct{}{}
	}
	if err := ValidateCorrMatrix(c.DefaultMatrix, n); err != nil {
		return fmt.Errorf("default_matrix: %w", err)
	}
	for regime, mat := range c.RegimeMatrices {
		if regime == "" {
			return errors.New("regime_matrices key must not be empty")
		}
		if err := ValidateCorrMatrix(mat, n); err != nil {
			return fmt.Errorf("regime_matrices[%q]: %w", regime, err)
		}
	}
	for pair, rho := range c.WithinPlayer {
		if pair == "" {
			return errors.New("within_player key must not be empty")
		}
		if math.IsNaN(rho) || math.IsInf(rho, 0) || rho < -1 || rho > 1 {
			return fmt.Errorf("within_player[%q]=%v must be finite in [-1, 1]", pair, rho)
		}
	}
	return nil
}

// ValidateCorrMatrix verifies that mat is an n x n symmetric, unit-diagonal,
// bounded [-1, 1], positive semi-definite correlation matrix.
func ValidateCorrMatrix(mat [][]float64, n int) error {
	if len(mat) != n {
		return fmt.Errorf("expected %d rows, got %d", n, len(mat))
	}
	for i := range n {
		if len(mat[i]) != n {
			return fmt.Errorf("row %d: expected %d cols, got %d", i, n, len(mat[i]))
		}
		for j := range n {
			v := mat[i][j]
			if math.IsNaN(v) || math.IsInf(v, 0) {
				return fmt.Errorf("entry (%d,%d) is non-finite", i, j)
			}
			if v < -1-corrMatrixTol || v > 1+corrMatrixTol {
				return fmt.Errorf("entry (%d,%d)=%v out of [-1, 1]", i, j, v)
			}
		}
		if math.Abs(mat[i][i]-1.0) > corrMatrixTol {
			return fmt.Errorf("diagonal (%d,%d)=%v must be 1.0", i, i, mat[i][i])
		}
		for j := i + 1; j < n; j++ {
			if math.Abs(mat[i][j]-mat[j][i]) > corrMatrixTol {
				return fmt.Errorf("matrix not symmetric at (%d,%d): %v != %v", i, j, mat[i][j], mat[j][i])
			}
		}
	}

	// Verify positive semi-definiteness via Cholesky factorization.
	chol := make([]float64, n*n)
	for i := range n {
		for j := range i + 1 {
			sum := mat[i][j]
			if i != j {
				// Symmetrize within tolerance so tiny JSON rounding asymmetry never trips Cholesky.
				sum = 0.5 * (mat[i][j] + mat[j][i])
			}
			for k := range j {
				sum -= chol[i*n+k] * chol[j*n+k]
			}
			if i == j {
				if sum < psdEigenvalueTol {
					return fmt.Errorf("matrix is not positive semi-definite (pivot %d = %g)", i, sum)
				}
				if sum < 0 {
					sum = 0
				}
				chol[i*n+j] = math.Sqrt(sum)
				continue
			}
			diag := chol[j*n+j]
			if diag == 0 {
				if math.Abs(sum) > corrMatrixTol {
					return fmt.Errorf("matrix is not positive semi-definite at (%d,%d)", i, j)
				}
				chol[i*n+j] = 0
				continue
			}
			chol[i*n+j] = sum / diag
		}
	}
	return nil
}

func requiredParamsForFamily(family string) ([]string, bool) {
	switch family {
	case FamilyZeroAdjGamma:
		return []string{"pi0", "mu", "k"}, true
	case FamilyZeroAdjLogNormal:
		return []string{"pi0", "mu", "sigma"}, true
	case FamilyNegBin:
		return []string{"mu", "r"}, true
	case FamilyPoisson:
		return []string{"lambda"}, true
	case FamilyBernoulli:
		return []string{"p"}, true
	default:
		return nil, false
	}
}

// PropModel returns the named DistModelArtifact if present in the bundle.
func (b *ArtifactBundle) PropModel(market string) (DistModelArtifact, bool) {
	if b == nil {
		return DistModelArtifact{}, false
	}
	m, ok := b.PropModels[market]
	return m, ok
}

// SortedPropMarkets returns the bundle's configured prop market keys in sorted order.
func (b *ArtifactBundle) SortedPropMarkets() []string {
	if b == nil || len(b.PropModels) == 0 {
		return nil
	}
	keys := make([]string, 0, len(b.PropModels))
	for k := range b.PropModels {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}
