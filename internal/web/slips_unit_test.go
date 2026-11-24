package web

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kalverra/parlay-club/internal/parlay"
)

func TestFormatImpliedChance(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		prob float64
		want string
	}{
		{name: "zero is empty", prob: 0, want: ""},
		{name: "negative is empty", prob: -0.25, want: ""},
		{name: "one is empty", prob: 1, want: ""},
		{name: "above one is empty", prob: 1.2, want: ""},
		{name: "NaN is empty", prob: math.NaN(), want: ""},
		{name: "positive Inf is empty", prob: math.Inf(1), want: ""},
		{name: "negative Inf is empty", prob: math.Inf(-1), want: ""},
		{name: "below 0.1 percent", prob: parlay.ImpliedProb(200000), want: "<0.1%"},
		{name: "just below 0.1 percent", prob: 0.00099, want: "<0.1%"},
		{name: "at 0.1 percent", prob: 0.001, want: "0.1%"},
		{name: "plus 5000", prob: parlay.ImpliedProb(5000), want: "2.0%"},
		{name: "plus 1200", prob: parlay.ImpliedProb(1200), want: "7.7%"},
		{name: "9.94 percent stays one decimal", prob: 0.0994, want: "9.9%"},
		{name: "9.96 percent rounds to whole 10", prob: 0.0996, want: "10%"},
		{name: "10 percent", prob: 0.10, want: "10%"},
		{name: "plus 150", prob: parlay.ImpliedProb(150), want: "40%"},
		{name: "minus 110", prob: parlay.ImpliedProb(-110), want: "52%"},
		{name: "99.4 percent stays 99", prob: 0.994, want: "99%"},
		{name: "99.5 percent caps at >99%", prob: 0.995, want: ">99%"},
		{name: "minus 50000", prob: parlay.ImpliedProb(-50000), want: ">99%"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, formatImpliedChance(tt.prob))
		})
	}
}
