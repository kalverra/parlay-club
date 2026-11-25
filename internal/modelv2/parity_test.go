package modelv2_test

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/modelv2"
)

type parityFixture struct {
	FeatureSchemaVersion int                  `json:"feature_schema_version"`
	LegVectors           []legParityVector    `json:"leg_vectors"`
	CopulaVectors        []copulaParityVector `json:"copula_vectors"`
}

type legParityVector struct {
	ID                    string              `json:"id"`
	Market                string              `json:"market"`
	Role                  string              `json:"role"`
	Position              string              `json:"position"`
	GamesPlayed           int                 `json:"games_played"`
	RawFeatures           map[string]*float64 `json:"raw_features"`
	AlignedShrunkFeatures []*float64          `json:"aligned_shrunk_features"`
	Side                  string              `json:"side"`
	Line                  float64             `json:"line"`
	MarketProb            float64             `json:"market_prob"`
	UncertaintyBoost      float64             `json:"uncertainty_boost"`
	Expected              legParityExpected   `json:"expected"`
}

type legParityExpected struct {
	Market         string             `json:"market"`
	Family         string             `json:"family"`
	Side           string             `json:"side"`
	Line           float64            `json:"line"`
	Stats          statsParityPayload `json:"stats"`
	ModelProb      float64            `json:"model_prob"`
	MarketProb     float64            `json:"market_prob"`
	BlendWeight    float64            `json:"blend_weight"`
	BlendedProb    float64            `json:"blended_prob"`
	CalibratedProb float64            `json:"calibrated_prob"`
}

type statsParityPayload struct {
	Mean     float64 `json:"mean"`
	Variance float64 `json:"variance"`
	StdDev   float64 `json:"std_dev"`
	CV       float64 `json:"cv"`
	Skewness float64 `json:"skewness"`
}

type copulaParityVector struct {
	ID          string                          `json:"id"`
	Legs        []copulaLegPayload              `json:"legs"`
	GameContext map[string]copulaGameCtxPayload `json:"game_context"`
	Expected    copulaExpectedPayload           `json:"expected"`
}

type copulaLegPayload struct {
	LegID      int64   `json:"leg_id"`
	GameID     string  `json:"game_id"`
	Team       string  `json:"team"`
	PlayerID   string  `json:"player_id"`
	PlayerName string  `json:"player_name"`
	Role       string  `json:"role"`
	Market     string  `json:"market"`
	Side       string  `json:"side"`
	Prob       float64 `json:"prob"`
}

type copulaGameCtxPayload struct {
	HomeTeam   string  `json:"home_team"`
	AwayTeam   string  `json:"away_team"`
	HomeSpread float64 `json:"home_spread"`
	GameTotal  float64 `json:"game_total"`
	HasSpread  bool    `json:"has_spread"`
	HasTotal   bool    `json:"has_total"`
}

type copulaExpectedPayload struct {
	JointProb        float64                        `json:"joint_prob"`
	IndependentProb  float64                        `json:"independent_prob"`
	CorrMultiplier   float64                        `json:"corr_multiplier"`
	JitterApplied    float64                        `json:"jitter_applied"`
	RegimesByGame    map[string]string              `json:"regimes_by_game"`
	PairCorrelations []copulaPairCorrelationPayload `json:"pair_correlations"`
}

type copulaPairCorrelationPayload struct {
	LegAIndex    int     `json:"leg_a_index"`
	LegBIndex    int     `json:"leg_b_index"`
	LegAID       int64   `json:"leg_a_id"`
	LegBID       int64   `json:"leg_b_id"`
	GameID       string  `json:"game_id"`
	NodeA        string  `json:"node_a"`
	NodeB        string  `json:"node_b"`
	Correlation  float64 `json:"correlation"`
	WithinPlayer bool    `json:"within_player"`
}

const parityTol = 1e-6

func loadParityFixture(t *testing.T) parityFixture {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "parity_vectors.json"))
	require.NoError(t, err)

	var fixture parityFixture
	require.NoError(t, json.Unmarshal(raw, &fixture))
	return fixture
}

func TestEmbeddedTrainedArtifactAndValidationGates(t *testing.T) {
	t.Parallel()

	bundle, err := modelv2.LoadEmbedded()
	require.NoError(t, err)

	assert.NotEqual(t, "v2.0.0-baseline", bundle.Version, "expected real walk-forward trained artifact")
	assert.GreaterOrEqual(t, len(bundle.TrainedSeasons), 4)
	assert.GreaterOrEqual(t, len(bundle.Validation.HeldOutSeasons), 3)
	assert.Positive(t, bundle.Validation.TotalBins)
	assert.GreaterOrEqual(
		t,
		bundle.Validation.PopulatedBins,
		int(math.Ceil(0.80*float64(bundle.Validation.TotalBins))),
	)
	assert.LessOrEqual(t, bundle.Validation.LogLoss, bundle.Validation.MarketLogLoss+1e-4)
	assert.LessOrEqual(t, bundle.Validation.BrierScore, bundle.Validation.MarketBrierScore+1e-4)
	assert.Less(t, bundle.Validation.SGPLogLoss, bundle.Validation.NaiveIndepLogLoss)
}

func TestPythonGoLegInferenceParity(t *testing.T) {
	t.Parallel()

	bundle, err := modelv2.LoadEmbedded()
	require.NoError(t, err)

	fixture := loadParityFixture(t)
	assert.Equal(t, modelv2.CurrentFeatureSchemaVersion, fixture.FeatureSchemaVersion)
	require.GreaterOrEqual(t, len(fixture.LegVectors), 100)

	for _, vec := range fixture.LegVectors {
		t.Run(vec.ID, func(t *testing.T) {
			t.Parallel()

			pm, ok := bundle.PropModel(vec.Market)
			require.True(t, ok, "missing prop model for %s", vec.Market)
			require.Len(t, vec.AlignedShrunkFeatures, len(pm.FeatureNames))

			aligned := make([]float64, len(vec.AlignedShrunkFeatures))
			for i, ptr := range vec.AlignedShrunkFeatures {
				if ptr == nil {
					aligned[i] = math.NaN()
				} else {
					aligned[i] = *ptr
				}
			}

			got, err := pm.PredictLeg(aligned, vec.Side, vec.Line, vec.MarketProb, vec.UncertaintyBoost)
			require.NoError(t, err)
			assertLegPredictionParity(t, vec.Expected, got)

			// Verify end-to-end raw feature -> EB shrinkage -> alignment -> inference parity.
			rawMap := make(map[string]float64, len(vec.RawFeatures))
			for k, ptr := range vec.RawFeatures {
				if ptr == nil {
					rawMap[k] = math.NaN()
				} else {
					rawMap[k] = *ptr
				}
			}
			prior, hasPrior := bundle.LookupPrior(vec.Market, vec.Role, vec.Position)
			require.True(t, hasPrior, "missing prior for (%s, %s, %s)", vec.Market, vec.Role, vec.Position)

			shrunk := modelv2.ShrinkFeatureMap(rawMap, vec.GamesPlayed, prior)
			alignedFromRaw := modelv2.AlignFeatures(pm.FeatureNames, shrunk)
			gotFromRaw, err := pm.PredictLeg(alignedFromRaw, vec.Side, vec.Line, vec.MarketProb, vec.UncertaintyBoost)
			require.NoError(t, err)
			assertLegPredictionParity(t, vec.Expected, gotFromRaw)
		})
	}
}

func assertLegPredictionParity(t *testing.T, want legParityExpected, got modelv2.LegPrediction) {
	t.Helper()

	assert.Equal(t, want.Market, got.Market)
	assert.Equal(t, want.Family, got.Family)
	assert.Equal(t, want.Side, got.Side)
	assert.InDelta(t, want.Line, got.Line, parityTol)

	assert.InDelta(t, want.ModelProb, got.ModelProb, parityTol)
	assert.InDelta(t, want.MarketProb, got.MarketProb, parityTol)
	assert.InDelta(t, want.BlendWeight, got.BlendWeight, parityTol)
	assert.InDelta(t, want.BlendedProb, got.BlendedProb, parityTol)
	assert.InDelta(t, want.CalibratedProb, got.CalibratedProb, parityTol)

	assert.InDelta(t, want.Stats.Mean, got.Stats.Mean, parityTol)
	assert.InDelta(
		t,
		want.Stats.Variance,
		got.Stats.Variance,
		math.Max(parityTol, parityTol*math.Abs(want.Stats.Variance)),
	)
	assert.InDelta(t, want.Stats.StdDev, got.Stats.StdDev, parityTol)
	assert.InDelta(t, want.Stats.CV, got.Stats.CV, parityTol)
	assert.InDelta(t, want.Stats.Skewness, got.Stats.Skewness, parityTol)
}

func TestPythonGoCopulaParity(t *testing.T) {
	t.Parallel()

	bundle, err := modelv2.LoadEmbedded()
	require.NoError(t, err)

	fixture := loadParityFixture(t)
	require.NotEmpty(t, fixture.CopulaVectors)

	for _, vec := range fixture.CopulaVectors {
		t.Run(vec.ID, func(t *testing.T) {
			t.Parallel()

			legs := make([]modelv2.CopulaLeg, len(vec.Legs))
			for i, l := range vec.Legs {
				legs[i] = modelv2.CopulaLeg{
					LegID:      l.LegID,
					GameID:     l.GameID,
					Team:       l.Team,
					PlayerID:   l.PlayerID,
					PlayerName: l.PlayerName,
					Role:       l.Role,
					Market:     l.Market,
					Side:       l.Side,
					Prob:       l.Prob,
				}
			}

			ctxMap := make(map[string]modelv2.CopulaGameContext, len(vec.GameContext))
			for gid, c := range vec.GameContext {
				ctxMap[gid] = modelv2.CopulaGameContext{
					HomeTeam:   c.HomeTeam,
					AwayTeam:   c.AwayTeam,
					HomeSpread: c.HomeSpread,
					GameTotal:  c.GameTotal,
					HasSpread:  c.HasSpread,
					HasTotal:   c.HasTotal,
				}
			}

			got, err := bundle.Copula.EstimateJoint(legs, ctxMap)
			require.NoError(t, err)

			assert.InDelta(t, vec.Expected.JointProb, got.JointProb, parityTol)
			assert.InDelta(t, vec.Expected.IndependentProb, got.IndependentProb, parityTol)
			assert.InDelta(t, vec.Expected.CorrMultiplier, got.CorrMultiplier, parityTol)
			assert.InDelta(t, vec.Expected.JitterApplied, got.JitterApplied, parityTol)
			if len(vec.Expected.RegimesByGame) == 0 {
				assert.Empty(t, got.RegimesByGame)
			} else {
				assert.Equal(t, vec.Expected.RegimesByGame, got.RegimesByGame)
			}

			require.Len(t, got.PairCorrelations, len(vec.Expected.PairCorrelations))
			for i, wantPair := range vec.Expected.PairCorrelations {
				gotPair := got.PairCorrelations[i]
				assert.Equal(t, wantPair.LegAIndex, gotPair.LegAIndex)
				assert.Equal(t, wantPair.LegBIndex, gotPair.LegBIndex)
				assert.Equal(t, wantPair.LegAID, gotPair.LegAID)
				assert.Equal(t, wantPair.LegBID, gotPair.LegBID)
				assert.Equal(t, wantPair.GameID, gotPair.GameID)
				assert.Equal(t, wantPair.NodeA, gotPair.NodeA)
				assert.Equal(t, wantPair.NodeB, gotPair.NodeB)
				assert.Equal(t, wantPair.WithinPlayer, gotPair.WithinPlayer)
				assert.InDelta(t, wantPair.Correlation, gotPair.Correlation, parityTol)
			}
		})
	}
}
