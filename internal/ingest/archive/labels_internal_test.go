package archive

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSourceForHostMapsOddsProviders(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"parlay-api.com":       SourceOdds,
		"api.the-odds-api.com": SourceOdds,
		"site.api.espn.com":    SourceESPN,
		"api.open-meteo.com":   SourceWeather,
		"example.test":         "example.test",
	}

	for host, want := range tests {
		t.Run(host, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, want, sourceForHost(host))
		})
	}
}
