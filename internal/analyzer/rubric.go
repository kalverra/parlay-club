package analyzer

// Rubric defines the versioned scoring weights, category bounds, and letter
// grade cutoffs used by ScoreLegWithRubric and ScoreSlipWithRubric.
type Rubric struct {
	Name string `json:"name"`

	LegBaseline   int `json:"leg_baseline"`
	FormUsageMin  int `json:"form_usage_min"`
	FormUsageMax  int `json:"form_usage_max"`
	MatchupEnvMin int `json:"matchup_env_min"`
	MatchupEnvMax int `json:"matchup_env_max"`
	LinePriceMin  int `json:"line_price_min"`
	LinePriceMax  int `json:"line_price_max"`

	// Form & Usage leg factor deltas.
	HitRate            int `json:"hit_rate"`
	RisingUsage        int `json:"rising_usage"`
	MedianTrap         int `json:"median_trap"`
	MedianTrapSoftened int `json:"median_trap_softened"`
	TeammateOutBump    int `json:"teammate_out_bump"`
	ShrinkingRole      int `json:"shrinking_role"`

	// Matchup & Environment leg factor deltas.
	FavorableMatchup int `json:"favorable_matchup"`
	ToughMatchup     int `json:"tough_matchup"`
	WindWarn         int `json:"wind_warn"`
	WindDanger       int `json:"wind_danger"`
	FakeWeather      int `json:"fake_weather"`
	QBDrag           int `json:"qb_drag"`

	// Line & Price leg factor deltas.
	NfeloCushion   int `json:"nfelo_cushion"`
	Hook           int `json:"hook"`
	HookSoftened   int `json:"hook_softened"`
	TotalKeyNumber int `json:"total_key_number"`
	ChalkMin       int `json:"chalk_min"`
	ChalkMax       int `json:"chalk_max"`

	// Slip-level adjustments.
	StackBonus           int `json:"stack_bonus"`
	WarnConflict         int `json:"warn_conflict"`
	DangerConflict       int `json:"danger_conflict"`
	ParlayDragPerLeg     int `json:"parlay_drag_per_leg"`
	ParlayDragFreeLegs   int `json:"parlay_drag_free_legs"`
	ChalkDragPerExtraLeg int `json:"chalk_drag_per_extra_leg"`

	// Letter grade minimum score cutoffs.
	BandA int `json:"band_a"`
	BandB int `json:"band_b"`
	BandC int `json:"band_c"`
	BandD int `json:"band_d"`
}

// RubricV1 holds the initial production grading weights.
var RubricV1 = Rubric{
	Name: "v1",

	LegBaseline:   75,
	FormUsageMin:  -20,
	FormUsageMax:  15,
	MatchupEnvMin: -20,
	MatchupEnvMax: 15,
	LinePriceMin:  -20,
	LinePriceMax:  15,

	HitRate:            10,
	RisingUsage:        5,
	MedianTrap:         -15,
	MedianTrapSoftened: -5,
	TeammateOutBump:    5,
	ShrinkingRole:      -15,

	FavorableMatchup: 10,
	ToughMatchup:     -12,
	WindWarn:         -8,
	WindDanger:       -18,
	FakeWeather:      -3,
	QBDrag:           -8,

	NfeloCushion:   8,
	Hook:           -10,
	HookSoftened:   -4,
	TotalKeyNumber: -5,
	ChalkMin:       -10,
	ChalkMax:       -18,

	StackBonus:           8,
	WarnConflict:         -12,
	DangerConflict:       -20,
	ParlayDragPerLeg:     -3,
	ParlayDragFreeLegs:   3,
	ChalkDragPerExtraLeg: -5,

	BandA: 85,
	BandB: 73,
	BandC: 60,
	BandD: 48,
}

// ActiveRubric is the rubric used by ScoreLeg, ScoreSlip, and GradeRubric.
var ActiveRubric = RubricV1

var builtinRubrics = []Rubric{
	RubricV1,
}

// RubricByName returns the built-in rubric matching name.
func RubricByName(name string) (Rubric, bool) {
	for _, r := range builtinRubrics {
		if r.Name == name {
			return r, true
		}
	}
	return Rubric{}, false
}

// RubricNames returns the names of all built-in rubrics in registry order.
func RubricNames() []string {
	names := make([]string, len(builtinRubrics))
	for i, r := range builtinRubrics {
		names[i] = r.Name
	}
	return names
}

// BuiltInRubrics returns a copy of all built-in rubrics in registry order.
func BuiltInRubrics() []Rubric {
	out := make([]Rubric, len(builtinRubrics))
	copy(out, builtinRubrics)
	return out
}
