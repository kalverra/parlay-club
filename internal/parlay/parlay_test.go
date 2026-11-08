package parlay_test

import (
	"slices"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kalverra/parlay-club/internal/parlay"
)

func TestAmericanToDecimal(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		american int
		want     float64
	}{
		{name: "minus 110", american: -110, want: 1.9090909090909092},
		{name: "plus 150", american: 150, want: 2.5},
		{name: "minus 200", american: -200, want: 1.5},
		{name: "plus 100", american: 100, want: 2},
		{name: "minus 100", american: -100, want: 2},
		{name: "plus 250", american: 250, want: 3.5},
		{name: "minus 400", american: -400, want: 1.25},
		{name: "zero is not a price", american: 0, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.InDelta(t, tt.want, parlay.AmericanToDecimal(tt.american), 1e-12)
		})
	}
}

func TestDecimalToAmerican(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		decimal float64
		want    int
	}{
		{name: "minus 110", decimal: 1.9090909090909092, want: -110},
		{name: "plus 150", decimal: 2.5, want: 150},
		{name: "minus 200", decimal: 1.5, want: -200},
		{name: "even money", decimal: 2, want: 100},
		{name: "plus 250", decimal: 3.5, want: 250},
		{name: "minus 400", decimal: 1.25, want: -400},
		{name: "one is not a price", decimal: 1, want: 0},
		{name: "below one is not a price", decimal: 0.5, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, parlay.DecimalToAmerican(tt.decimal))
		})
	}
}

func TestConversionsRoundTrip(t *testing.T) {
	t.Parallel()

	for _, american := range []int{-110, -200, -400, 100, 150, 250} {
		t.Run(strconv.Itoa(american), func(t *testing.T) {
			t.Parallel()

			roundTripped := parlay.DecimalToAmerican(parlay.AmericanToDecimal(american))
			assert.Equal(t, american, roundTripped)
		})
	}
}

func TestCombinedOdds(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		legs []parlay.Leg
		want float64
	}{
		{
			name: "one leg",
			legs: []parlay.Leg{{Decimal: parlay.AmericanToDecimal(-110), Result: parlay.Pending}},
			want: 1.9090909090909092,
		},
		{
			name: "three leg combo",
			legs: []parlay.Leg{
				{Decimal: parlay.AmericanToDecimal(-110), Result: parlay.Pending},
				{Decimal: parlay.AmericanToDecimal(150), Result: parlay.Pending},
				{Decimal: parlay.AmericanToDecimal(-200), Result: parlay.Pending},
			},
			want: 7.159090909090909,
		},
		{
			name: "push and void legs drop out",
			legs: []parlay.Leg{
				{Decimal: parlay.AmericanToDecimal(-110), Result: parlay.Pending},
				{Decimal: parlay.AmericanToDecimal(150), Result: parlay.Push},
				{Decimal: parlay.AmericanToDecimal(-200), Result: parlay.Pending},
				{Decimal: parlay.AmericanToDecimal(250), Result: parlay.Void},
			},
			want: 2.8636363636363638,
		},
		{
			name: "every leg dropped is even money",
			legs: []parlay.Leg{
				{Decimal: parlay.AmericanToDecimal(150), Result: parlay.Push},
				{Decimal: parlay.AmericanToDecimal(-200), Result: parlay.Void},
			},
			want: 1,
		},
		{
			name: "no legs is even money",
			legs: nil,
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.InDelta(t, tt.want, parlay.CombinedOdds(tt.legs), 1e-12)
		})
	}
}

func TestPayoutWithBoost(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		stake    float64
		decimal  float64
		boostPct float64
		want     float64
	}{
		{name: "no boost", stake: 10, decimal: 3, boostPct: 0, want: 30},
		{name: "profit boost", stake: 10, decimal: 3, boostPct: 25, want: 35},
		{name: "half boost", stake: 50, decimal: 1.9, boostPct: 50, want: 117.5},
		{name: "full boost", stake: 100, decimal: 2, boostPct: 100, want: 300},
		{name: "winning stake only", stake: 10, decimal: 1, boostPct: 50, want: 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.InDelta(t, tt.want, parlay.Payout(tt.stake, tt.decimal, tt.boostPct), 1e-12)
		})
	}
}

func TestReferencePrice(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		candidates []float64
		own        float64
		want       float64
	}{
		{name: "no candidates falls back to own price", candidates: nil, own: 1.91, want: 1.91},
		{name: "odd count", candidates: []float64{2, 1.5, 3}, own: 1.91, want: 2},
		{name: "even count means the middle two", candidates: []float64{1.5, 2, 2.5, 3}, own: 1.91, want: 2.25},
		{name: "even count of two", candidates: []float64{1.9, 2.1}, own: 1.91, want: 2},
		{name: "unsorted input", candidates: []float64{3, 1.5, 2.5, 2}, own: 1.91, want: 2.25},
		{name: "one candidate wins over own", candidates: []float64{2.4}, own: 1.91, want: 2.4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			candidates := slices.Clone(tt.candidates)
			assert.InDelta(t, tt.want, parlay.ReferencePrice(tt.candidates, tt.own), 1e-12)
			assert.Equal(t, candidates, tt.candidates, "reference price must not reorder its input")
		})
	}
}

func TestUnits(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		result  parlay.Result
		decimal float64
		want    float64
	}{
		{name: "won pays decimal minus one", result: parlay.Won, decimal: 3, want: 2},
		{name: "lost costs one unit", result: parlay.Lost, decimal: 3, want: -1},
		{name: "push breaks even", result: parlay.Push, decimal: 3, want: 0},
		{name: "void breaks even", result: parlay.Void, decimal: 3, want: 0},
		{name: "pending has no units", result: parlay.Pending, decimal: 3, want: 0},
		{name: "needs grading has no units", result: parlay.NeedsGrading, decimal: 3, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.InDelta(t, tt.want, parlay.Units(tt.result, tt.decimal), 1e-12)
		})
	}
}

func TestResultCounts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		result parlay.Result
		want   bool
	}{
		{result: parlay.Pending, want: true},
		{result: parlay.NeedsGrading, want: true},
		{result: parlay.Won, want: true},
		{result: parlay.Lost, want: true},
		{result: parlay.Push, want: false},
		{result: parlay.Void, want: false},
	}

	for _, tt := range tests {
		t.Run(string(tt.result), func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, tt.result.Counts())
		})
	}
}

func TestResultValidity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		result parlay.Result
		leg    bool
		slip   bool
	}{
		{result: parlay.Pending, leg: true, slip: true},
		{result: parlay.NeedsGrading, leg: true, slip: false},
		{result: parlay.Won, leg: true, slip: true},
		{result: parlay.Lost, leg: true, slip: true},
		{result: parlay.Push, leg: true, slip: true},
		{result: parlay.Void, leg: true, slip: false},
		{result: parlay.Result("MAYBE"), leg: false, slip: false},
		{result: parlay.Result(""), leg: false, slip: false},
	}

	for _, tt := range tests {
		t.Run(string(tt.result), func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.leg, tt.result.IsLegResult())
			assert.Equal(t, tt.slip, tt.result.IsSlipResult())
		})
	}
}
