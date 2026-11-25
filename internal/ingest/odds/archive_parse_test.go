package odds_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/ingest/odds"
)

func TestParseLinesJSON(t *testing.T) {
	t.Parallel()

	t.Run("parses game lines fixture", func(t *testing.T) {
		t.Parallel()
		raw, err := os.ReadFile(filepath.Join("testdata", "odds_nfl.json"))
		require.NoError(t, err)

		lines, err := odds.ParseLinesJSON(raw)
		require.NoError(t, err)
		assert.NotEmpty(t, lines)
	})

	t.Run("rejects invalid JSON", func(t *testing.T) {
		t.Parallel()
		_, err := odds.ParseLinesJSON([]byte("{invalid"))
		require.Error(t, err)
	})
}

func TestParseLinesSnapshotJSON(t *testing.T) {
	t.Parallel()

	t.Run("parses events and lines from fixture", func(t *testing.T) {
		t.Parallel()
		raw, err := os.ReadFile(filepath.Join("testdata", "odds_nfl.json"))
		require.NoError(t, err)

		events, lines, err := odds.ParseLinesSnapshotJSON(raw)
		require.NoError(t, err)
		assert.NotEmpty(t, events)
		assert.NotEmpty(t, lines)
		assert.NotEmpty(t, events[0].ID)
		assert.NotEmpty(t, events[0].HomeTeam)
		assert.NotEmpty(t, events[0].AwayTeam)
		assert.False(t, events[0].CommenceTime.IsZero())
	})

	t.Run("rejects invalid JSON", func(t *testing.T) {
		t.Parallel()
		_, _, err := odds.ParseLinesSnapshotJSON([]byte("{invalid"))
		require.Error(t, err)
	})
}

func TestParsePropsJSON(t *testing.T) {
	t.Parallel()

	t.Run("parses parlay props array fixture", func(t *testing.T) {
		t.Parallel()
		raw, err := os.ReadFile(filepath.Join("testdata", "props_sample.json"))
		require.NoError(t, err)

		props, err := odds.ParsePropsJSON(raw)
		require.NoError(t, err)
		assert.NotEmpty(t, props)
	})

	t.Run("parses odds api event props object fixture", func(t *testing.T) {
		t.Parallel()
		raw, err := os.ReadFile(filepath.Join("testdata", "odds_prop_links.json"))
		require.NoError(t, err)

		props, err := odds.ParsePropsJSON(raw)
		require.NoError(t, err)
		assert.NotEmpty(t, props)
	})

	t.Run("parses empty props fixture", func(t *testing.T) {
		t.Parallel()
		raw, err := os.ReadFile(filepath.Join("testdata", "props_empty.json"))
		require.NoError(t, err)

		props, err := odds.ParsePropsJSON(raw)
		require.NoError(t, err)
		assert.Empty(t, props)
	})

	t.Run("rejects invalid JSON", func(t *testing.T) {
		t.Parallel()
		_, err := odds.ParsePropsJSON([]byte("not-json"))
		require.Error(t, err)
	})
}
