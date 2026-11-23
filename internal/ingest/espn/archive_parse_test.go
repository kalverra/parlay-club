package espn_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/ingest/espn"
)

func TestParseGameLogJSON(t *testing.T) {
	t.Parallel()

	t.Run("parses gamelog fixture", func(t *testing.T) {
		t.Parallel()
		raw, err := os.ReadFile(filepath.Join("testdata", "gamelog_burrow.json"))
		require.NoError(t, err)

		markets, err := espn.ParseGameLogJSON(raw)
		require.NoError(t, err)
		assert.NotEmpty(t, markets["PASS_YDS"])
		assert.NotEmpty(t, markets["PASS_TDS"])
	})

	t.Run("parses empty gamelog fixture", func(t *testing.T) {
		t.Parallel()
		raw, err := os.ReadFile(filepath.Join("testdata", "gamelog_empty.json"))
		require.NoError(t, err)

		markets, err := espn.ParseGameLogJSON(raw)
		require.NoError(t, err)
		assert.Empty(t, markets)
	})

	t.Run("rejects invalid JSON", func(t *testing.T) {
		t.Parallel()
		_, err := espn.ParseGameLogJSON([]byte("{invalid"))
		require.Error(t, err)
	})
}

func TestParseRosterJSON(t *testing.T) {
	t.Parallel()

	t.Run("parses roster fixture", func(t *testing.T) {
		t.Parallel()
		raw, err := os.ReadFile(filepath.Join("testdata", "roster_bengals.json"))
		require.NoError(t, err)

		players, err := espn.ParseRosterJSON(raw)
		require.NoError(t, err)
		assert.NotEmpty(t, players)
		for _, p := range players {
			assert.NotEmpty(t, p.ID)
			assert.NotEmpty(t, p.FullName)
		}
	})

	t.Run("parses empty roster fixture", func(t *testing.T) {
		t.Parallel()
		raw, err := os.ReadFile(filepath.Join("testdata", "roster_empty.json"))
		require.NoError(t, err)

		players, err := espn.ParseRosterJSON(raw)
		require.NoError(t, err)
		assert.Empty(t, players)
	})

	t.Run("rejects invalid JSON", func(t *testing.T) {
		t.Parallel()
		_, err := espn.ParseRosterJSON([]byte("not-json"))
		require.Error(t, err)
	})
}
