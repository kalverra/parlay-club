package modelv2

import (
	"errors"
	"fmt"
	"maps"
	"math"
)

const (
	// ShrinkMaxGames defines the Weeks 1-5 early-season Empirical Bayes
	// shrinkage window. Once a player has played at least ShrinkMaxGames prior
	// games (Week 6+), observed EWMA metrics are used without prior shrinkage.
	ShrinkMaxGames = 5

	// DefaultPriorStrength is the default pseudo-game prior weight (tau) used
	// when a RolePrior does not specify a positive PriorStrength.
	DefaultPriorStrength = 3.0
)

// Canonical FeatureSchemaVersion = 1 feature names shared with the offline
// Python training pipeline.
const (
	FeatGamesPlayed              = "games_played"
	FeatEWMAOffensePct           = "ewma_offense_pct"
	FeatEWMATargetShare          = "ewma_target_share"
	FeatEWMAAirYardsShare        = "ewma_air_yards_share"
	FeatEWMAWOPR                 = "ewma_wopr"
	FeatEWMACarryShare           = "ewma_carry_share"
	FeatEWMAPassAtt              = "ewma_pass_att"
	FeatEWMACarries              = "ewma_carries"
	FeatEWMATargets              = "ewma_targets"
	FeatEWMAReceptions           = "ewma_receptions"
	FeatEWMAPassYds              = "ewma_pass_yds"
	FeatEWMARushYds              = "ewma_rush_yds"
	FeatEWMARecYds               = "ewma_rec_yds"
	FeatEWMAPassTDs              = "ewma_pass_tds"
	FeatEWMARushTDs              = "ewma_rush_tds"
	FeatEWMARecTDs               = "ewma_rec_tds"
	FeatEWMARZOpps               = "ewma_rz_opps"
	FeatEWMATotalEPA             = "ewma_total_epa"
	FeatEWMACPOE                 = "ewma_cpoe"
	FeatEWMAExpPassYds           = "ewma_exp_pass_yds"
	FeatEWMAExpRushYds           = "ewma_exp_rush_yds"
	FeatEWMAExpRecYds            = "ewma_exp_rec_yds"
	FeatEWMAExpReceptions        = "ewma_exp_receptions"
	FeatEWMAExpPassTDs           = "ewma_exp_pass_tds"
	FeatEWMAExpRushTDs           = "ewma_exp_rush_tds"
	FeatEWMAExpRecTDs            = "ewma_exp_rec_tds"
	FeatTeamSpread               = "team_spread"
	FeatAbsSpread                = "abs_spread"
	FeatGameTotal                = "game_total"
	FeatTeamImpliedTotal         = "team_implied_total"
	FeatOppImpliedTotal          = "opp_implied_total"
	FeatOppRoleYdsAllowed        = "opp_role_yds_allowed"
	FeatOppRoleReceptionsAllowed = "opp_role_receptions_allowed"
	FeatOppRoleTDsAllowed        = "opp_role_tds_allowed"
	FeatOppRoleEPAAllowed        = "opp_role_epa_allowed"
	FeatOppDefPassEPAAllowed     = "opp_def_pass_epa_allowed"
	FeatOppDefRushEPAAllowed     = "opp_def_rush_epa_allowed"
	FeatTeamPlaysPerGame         = "team_plays_per_game"
	FeatTeamPassRate             = "team_pass_rate"
	FeatIsIndoor                 = "is_indoor"
	FeatWindMPH                  = "wind_mph"
	FeatPrecipIn                 = "precip_in"
	FeatTempF                    = "temp_f"
	FeatLine                     = "line"
)

// RolePrior holds position/role-archetype baseline priors for Weeks 1-5
// Empirical Bayes shrinkage.
type RolePrior struct {
	Market        string             `json:"market"`
	Role          string             `json:"role"`
	PriorStrength float64            `json:"prior_strength"`
	Mean          float64            `json:"mean"`
	Std           float64            `json:"std,omitempty"`
	Features      map[string]float64 `json:"features,omitempty"`
}

// Validate verifies the RolePrior fields and map key consistency.
func (p RolePrior) Validate(mapKey string) error {
	if p.Market == "" {
		return errors.New("market must not be empty")
	}
	if p.Role == "" {
		return errors.New("role must not be empty")
	}
	if want := PriorKey(p.Market, p.Role); mapKey != want {
		return fmt.Errorf("map key %q does not match %q", mapKey, want)
	}
	if math.IsNaN(p.PriorStrength) || math.IsInf(p.PriorStrength, 0) || p.PriorStrength < 0 {
		return fmt.Errorf("prior_strength %v must be finite >= 0", p.PriorStrength)
	}
	if math.IsNaN(p.Mean) || math.IsInf(p.Mean, 0) {
		return fmt.Errorf("mean %v must be finite", p.Mean)
	}
	if math.IsNaN(p.Std) || math.IsInf(p.Std, 0) || p.Std < 0 {
		return fmt.Errorf("std %v must be finite >= 0", p.Std)
	}
	for feat, val := range p.Features {
		if feat == "" {
			return errors.New("features key must not be empty")
		}
		if math.IsNaN(val) || math.IsInf(val, 0) {
			return fmt.Errorf("features[%q]=%v must be finite", feat, val)
		}
	}
	return nil
}

// PriorKey formats the canonical map key for ArtifactBundle.Priors.
func PriorKey(market, role string) string {
	return market + ":" + role
}

// LookupPrior finds the most specific RolePrior for (market, role, position),
// falling back from exact MARKET:ROLE to a position-default role (e.g. WR ->
// WR2, RB -> RB2, TE -> TE1, QB -> QB) and finally MARKET:DEFAULT.
func (b *ArtifactBundle) LookupPrior(market, role, position string) (RolePrior, bool) {
	if b == nil || len(b.Priors) == 0 || market == "" {
		return RolePrior{}, false
	}
	if role != "" {
		if p, ok := b.Priors[PriorKey(market, role)]; ok {
			return p, true
		}
	}
	if fallbackRole := defaultRoleForPosition(position); fallbackRole != "" && fallbackRole != role {
		if p, ok := b.Priors[PriorKey(market, fallbackRole)]; ok {
			return p, true
		}
	}
	if p, ok := b.Priors[PriorKey(market, "DEFAULT")]; ok {
		return p, true
	}
	return RolePrior{}, false
}

func defaultRoleForPosition(position string) string {
	switch position {
	case "QB":
		return "QB"
	case "WR":
		return "WR2"
	case "RB":
		return "RB2"
	case "TE":
		return "TE1"
	default:
		return ""
	}
}

// EmpiricalBayesShrink shrinks an observed player metric toward priorMean when
// gamesPlayed is in the Weeks 1-5 sparse window (0 <= gamesPlayed < ShrinkMaxGames):
//
//	shrunk = (n * observed + tau * priorMean) / (n + tau)
//
// When gamesPlayed <= 0 or observed is NaN/Inf, priorMean is returned. When
// gamesPlayed >= ShrinkMaxGames (or priorStrength <= 0), observed is returned
// unshrunk.
func EmpiricalBayesShrink(observed, priorMean float64, gamesPlayed int, priorStrength float64) float64 {
	if gamesPlayed <= 0 || math.IsNaN(observed) || math.IsInf(observed, 0) {
		return priorMean
	}
	if gamesPlayed >= ShrinkMaxGames || priorStrength <= 0 {
		return observed
	}
	n := float64(gamesPlayed)
	return (n*observed + priorStrength*priorMean) / (n + priorStrength)
}

// ShrinkFeatureMap returns a copy of features with all prior-backed usage and
// opportunity features shrunk toward prior during Weeks 1-5. Missing or NaN
// prior-backed features receive prior values when gamesPlayed < ShrinkMaxGames.
func ShrinkFeatureMap(features map[string]float64, gamesPlayed int, prior RolePrior) map[string]float64 {
	out := make(map[string]float64, len(features)+len(prior.Features))
	maps.Copy(out, features)
	if gamesPlayed >= ShrinkMaxGames || len(prior.Features) == 0 {
		return out
	}
	strength := effectivePriorStrength(prior)
	for feat, priorVal := range prior.Features {
		obs, ok := out[feat]
		if !ok {
			obs = math.NaN()
		}
		out[feat] = EmpiricalBayesShrink(obs, priorVal, gamesPlayed, strength)
	}
	return out
}

// ShrinkFeatureSlice applies Weeks 1-5 Empirical Bayes shrinkage to an aligned
// feature slice without allocating when cap(dst) >= len(featureNames).
func ShrinkFeatureSlice(
	dst, observed []float64,
	featureNames []string,
	gamesPlayed int,
	prior RolePrior,
) []float64 {
	n := len(featureNames)
	if cap(dst) < n {
		dst = make([]float64, n)
	} else {
		dst = dst[:n]
	}
	for i := range n {
		if i < len(observed) {
			dst[i] = observed[i]
		} else {
			dst[i] = math.NaN()
		}
	}
	if gamesPlayed >= ShrinkMaxGames || len(prior.Features) == 0 {
		return dst
	}
	strength := effectivePriorStrength(prior)
	for i, name := range featureNames {
		if priorVal, ok := prior.Features[name]; ok {
			dst[i] = EmpiricalBayesShrink(dst[i], priorVal, gamesPlayed, strength)
		}
	}
	return dst
}

// AlignFeatures maps named features into an ordered float64 slice matching
// names, setting absent keys to math.NaN() for GBDT missing-branch routing.
func AlignFeatures(names []string, values map[string]float64) []float64 {
	return AlignFeaturesInto(nil, names, values)
}

// AlignFeaturesInto populates dst with values aligned to names, reusing dst's
// backing array when capacity permits.
func AlignFeaturesInto(dst []float64, names []string, values map[string]float64) []float64 {
	n := len(names)
	if cap(dst) < n {
		dst = make([]float64, n)
	} else {
		dst = dst[:n]
	}
	for i, name := range names {
		if v, ok := values[name]; ok {
			dst[i] = v
		} else {
			dst[i] = math.NaN()
		}
	}
	return dst
}

func effectivePriorStrength(prior RolePrior) float64 {
	if prior.PriorStrength > 0 {
		return prior.PriorStrength
	}
	return DefaultPriorStrength
}
