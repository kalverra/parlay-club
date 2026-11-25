package analyzer_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/analyzer"
	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/ingest/odds"
	"github.com/kalverra/parlay-club/internal/parlay"
)

func TestBuildSharpBenchmarksTierPrecedence(t *testing.T) {
	t.Parallel()

	t.Run("tier 1 beats tier 2 and tier 3", func(t *testing.T) {
		t.Parallel()
		rows := []db.Odds{
			// Tier 1: Pinnacle (-110 / -110 -> 0.50 / 0.50, +100)
			{
				GameID:     "g1",
				Book:       "pinnacle",
				MarketType: "SPREAD",
				Team:       "BUF",
				Side:       "TEAM",
				Line:       new(-3.5),
				Price:      -110,
			},
			{
				GameID:     "g1",
				Book:       "pinnacle",
				MarketType: "SPREAD",
				Team:       "MIA",
				Side:       "TEAM",
				Line:       new(3.5),
				Price:      -110,
			},
			// Tier 2: LowVig at different line (-3.0)
			{
				GameID:     "g1",
				Book:       "lowvig",
				MarketType: "SPREAD",
				Team:       "BUF",
				Side:       "TEAM",
				Line:       new(-3.0),
				Price:      -105,
			},
			{
				GameID:     "g1",
				Book:       "lowvig",
				MarketType: "SPREAD",
				Team:       "MIA",
				Side:       "TEAM",
				Line:       new(3.0),
				Price:      -115,
			},
			// Tier 3: 3 retail books
			{
				GameID:     "g1",
				Book:       "draftkings",
				MarketType: "SPREAD",
				Team:       "BUF",
				Side:       "TEAM",
				Line:       new(-2.5),
				Price:      -110,
			},
			{
				GameID:     "g1",
				Book:       "draftkings",
				MarketType: "SPREAD",
				Team:       "MIA",
				Side:       "TEAM",
				Line:       new(2.5),
				Price:      -110,
			},
			{
				GameID:     "g1",
				Book:       "fanduel",
				MarketType: "SPREAD",
				Team:       "BUF",
				Side:       "TEAM",
				Line:       new(-2.5),
				Price:      -110,
			},
			{
				GameID:     "g1",
				Book:       "fanduel",
				MarketType: "SPREAD",
				Team:       "MIA",
				Side:       "TEAM",
				Line:       new(2.5),
				Price:      -110,
			},
			{
				GameID:     "g1",
				Book:       "betmgm",
				MarketType: "SPREAD",
				Team:       "BUF",
				Side:       "TEAM",
				Line:       new(-2.5),
				Price:      -110,
			},
			{
				GameID:     "g1",
				Book:       "betmgm",
				MarketType: "SPREAD",
				Team:       "MIA",
				Side:       "TEAM",
				Line:       new(2.5),
				Price:      -110,
			},
		}

		got := analyzer.BuildSharpBenchmarks(rows)
		q, ok := got["g1"][analyzer.SharpKey{MarketType: "SPREAD", Team: "BUF", Side: "TEAM"}]
		require.True(t, ok)
		assert.Equal(t, 1, q.Tier)
		assert.Equal(t, "Pinnacle", q.Source)
		require.NotNil(t, q.Line)
		assert.InDelta(t, -3.5, *q.Line, 1e-9)
		assert.InDelta(t, 0.5, q.FairProb, 1e-6)
		assert.InDelta(t, 0.5, q.PowerFairProb, 1e-6)
		wantVig, ok := parlay.VigPct(-110, -110)
		require.True(t, ok)
		assert.InDelta(t, wantVig, q.VigPct, 1e-9)
		assert.Equal(t, 100, q.FairAmerican)
		assert.Equal(t, 1, q.BookCount)
	})

	t.Run("tier 2 median across lowvig and betonlineag beats tier 3", func(t *testing.T) {
		t.Parallel()
		rows := []db.Odds{
			// LowVig: -150 / +130 -> 0.60 / 0.43478 -> devigged 0.57983 / 0.42017
			{GameID: "g1", Book: "lowvig", MarketType: "ML", Team: "BUF", Side: "TEAM", Price: -150},
			{GameID: "g1", Book: "lowvig", MarketType: "ML", Team: "MIA", Side: "TEAM", Price: 130},
			// BetOnline: -140 / +120 -> 0.58333 / 0.45455 -> devigged 0.56204 / 0.43796
			{GameID: "g1", Book: "betonlineag", MarketType: "ML", Team: "BUF", Side: "TEAM", Price: -140},
			{GameID: "g1", Book: "betonlineag", MarketType: "ML", Team: "MIA", Side: "TEAM", Price: 120},
			// Tier 3 books
			{GameID: "g1", Book: "draftkings", MarketType: "ML", Team: "BUF", Side: "TEAM", Price: -160},
			{GameID: "g1", Book: "draftkings", MarketType: "ML", Team: "MIA", Side: "TEAM", Price: 140},
			{GameID: "g1", Book: "fanduel", MarketType: "ML", Team: "BUF", Side: "TEAM", Price: -160},
			{GameID: "g1", Book: "fanduel", MarketType: "ML", Team: "MIA", Side: "TEAM", Price: 140},
			{GameID: "g1", Book: "betmgm", MarketType: "ML", Team: "BUF", Side: "TEAM", Price: -160},
			{GameID: "g1", Book: "betmgm", MarketType: "ML", Team: "MIA", Side: "TEAM", Price: 140},
		}

		got := analyzer.BuildSharpBenchmarks(rows)
		bufQ, ok := got["g1"][analyzer.SharpKey{MarketType: "ML", Team: "BUF", Side: "TEAM"}]
		require.True(t, ok)
		assert.Equal(t, 2, bufQ.Tier)
		assert.Equal(t, "LowVig", bufQ.Source)
		assert.Nil(t, bufQ.Line)
		// Median of ~0.57983 and ~0.56204 is ~0.57094
		assert.InDelta(t, 0.57094, bufQ.FairProb, 1e-4)
		assert.Negative(t, bufQ.FairAmerican)
	})

	t.Run("tier 3 requires at least 3 non-DFS books on the same line", func(t *testing.T) {
		t.Parallel()
		rows := []db.Odds{
			// Only 2 traditional books on -3.5, plus DFS/exchanges that must be excluded.
			{
				GameID:     "g1",
				Book:       "draftkings",
				MarketType: "SPREAD",
				Team:       "BUF",
				Side:       "TEAM",
				Line:       new(-3.5),
				Price:      -110,
			},
			{
				GameID:     "g1",
				Book:       "draftkings",
				MarketType: "SPREAD",
				Team:       "MIA",
				Side:       "TEAM",
				Line:       new(3.5),
				Price:      -110,
			},
			{
				GameID:     "g1",
				Book:       "fanduel",
				MarketType: "SPREAD",
				Team:       "BUF",
				Side:       "TEAM",
				Line:       new(-3.5),
				Price:      -110,
			},
			{
				GameID:     "g1",
				Book:       "fanduel",
				MarketType: "SPREAD",
				Team:       "MIA",
				Side:       "TEAM",
				Line:       new(3.5),
				Price:      -110,
			},
			{
				GameID:     "g1",
				Book:       "prizepicks",
				MarketType: "SPREAD",
				Team:       "BUF",
				Side:       "TEAM",
				Line:       new(-3.5),
				Price:      -119,
			},
			{
				GameID:     "g1",
				Book:       "prizepicks",
				MarketType: "SPREAD",
				Team:       "MIA",
				Side:       "TEAM",
				Line:       new(3.5),
				Price:      -119,
			},
			{
				GameID:     "g1",
				Book:       "novig",
				MarketType: "SPREAD",
				Team:       "BUF",
				Side:       "TEAM",
				Line:       new(-3.5),
				Price:      -102,
			},
			{GameID: "g1", Book: "novig", MarketType: "SPREAD", Team: "MIA", Side: "TEAM", Line: new(3.5), Price: -102},
		}

		got := analyzer.BuildSharpBenchmarks(rows)
		assert.Nil(t, got)

		// Adding a 3rd traditional book on -3.5 unlocks Tier 3 consensus.
		rows = append(
			rows,
			db.Odds{
				GameID:     "g1",
				Book:       "betmgm",
				MarketType: "SPREAD",
				Team:       "BUF",
				Side:       "TEAM",
				Line:       new(-3.5),
				Price:      -112,
			},
			db.Odds{
				GameID:     "g1",
				Book:       "betmgm",
				MarketType: "SPREAD",
				Team:       "MIA",
				Side:       "TEAM",
				Line:       new(3.5),
				Price:      -108,
			},
		)
		got = analyzer.BuildSharpBenchmarks(rows)
		q, ok := got["g1"][analyzer.SharpKey{MarketType: "SPREAD", Team: "BUF", Side: "TEAM"}]
		require.True(t, ok)
		assert.Equal(t, 3, q.Tier)
		assert.Equal(t, "market consensus", q.Source)
		require.NotNil(t, q.Line)
		assert.InDelta(t, -3.5, *q.Line, 1e-9)
		assert.InDelta(t, 0.5, q.FairProb, 1e-6)
	})
}

func TestBuildSharpBenchmarksAllMarketsAndPlayerNormalization(t *testing.T) {
	t.Parallel()

	rows := []db.Odds{
		// ML
		{GameID: "g1", Book: "pinnacle", MarketType: "ML", Team: "buf", Side: "TEAM", Price: -130},
		{GameID: "g1", Book: "pinnacle", MarketType: "ML", Team: "mia", Side: "TEAM", Price: 110},
		// SPREAD
		{GameID: "g1", Book: "pinnacle", MarketType: "SPREAD", Team: "BUF", Side: "TEAM", Line: new(-2.5), Price: -115},
		{GameID: "g1", Book: "pinnacle", MarketType: "SPREAD", Team: "MIA", Side: "TEAM", Line: new(2.5), Price: -105},
		// TOTAL
		{GameID: "g1", Book: "pinnacle", MarketType: "TOTAL", Team: "", Side: "OVER", Line: new(48.5), Price: -108},
		{GameID: "g1", Book: "pinnacle", MarketType: "TOTAL", Team: "", Side: "UNDER", Line: new(48.5), Price: -112},
		// TEAM_TOTAL (two separate teams)
		{
			GameID:     "g1",
			Book:       "pinnacle",
			MarketType: "TEAM_TOTAL",
			Team:       "BUF",
			Side:       "OVER",
			Line:       new(25.5),
			Price:      -110,
		},
		{
			GameID:     "g1",
			Book:       "pinnacle",
			MarketType: "TEAM_TOTAL",
			Team:       "BUF",
			Side:       "UNDER",
			Line:       new(25.5),
			Price:      -110,
		},
		{
			GameID:     "g1",
			Book:       "pinnacle",
			MarketType: "TEAM_TOTAL",
			Team:       "MIA",
			Side:       "OVER",
			Line:       new(22.5),
			Price:      -115,
		},
		{
			GameID:     "g1",
			Book:       "pinnacle",
			MarketType: "TEAM_TOTAL",
			Team:       "MIA",
			Side:       "UNDER",
			Line:       new(22.5),
			Price:      -105,
		},
		// Player props with punctuation/suffix variants
		{
			GameID:     "g1",
			Book:       "pinnacle",
			MarketType: "PASS_YDS",
			Team:       "BUF",
			PlayerName: "Josh Allen",
			Side:       "OVER",
			Line:       new(265.5),
			Price:      -112,
		},
		{
			GameID:     "g1",
			Book:       "pinnacle",
			MarketType: "PASS_YDS",
			Team:       "BUF",
			PlayerName: "Josh Allen",
			Side:       "UNDER",
			Line:       new(265.5),
			Price:      -108,
		},
		{
			GameID:     "g1",
			Book:       "pinnacle",
			MarketType: "REC_YDS",
			Team:       "MIA",
			PlayerName: "Marvin Harrison Jr.",
			Side:       "OVER",
			Line:       new(64.5),
			Price:      -115,
		},
		{
			GameID:     "g1",
			Book:       "pinnacle",
			MarketType: "REC_YDS",
			Team:       "MIA",
			PlayerName: "Marvin Harrison, Jr",
			Side:       "UNDER",
			Line:       new(64.5),
			Price:      -105,
		},
		{
			GameID:     "g1",
			Book:       "circa",
			MarketType: "RUSH_YDS",
			Team:       "MIA",
			PlayerName: "Ollie Gordon II",
			Side:       "OVER",
			Line:       new(52.5),
			Price:      -110,
		},
		{
			GameID:     "g1",
			Book:       "circa",
			MarketType: "RUSH_YDS",
			Team:       "MIA",
			PlayerName: "Ollie Gordon",
			Side:       "UNDER",
			Line:       new(52.5),
			Price:      -110,
		},
	}

	got := analyzer.BuildSharpBenchmarks(rows)
	g1 := got["g1"]
	require.NotNil(t, g1)

	mlBuf, ok := g1[analyzer.SharpKey{MarketType: "ML", Team: "BUF", Side: "TEAM"}]
	require.True(t, ok)
	mlMia, ok := g1[analyzer.SharpKey{MarketType: "ML", Team: "MIA", Side: "TEAM"}]
	require.True(t, ok)
	assert.InDelta(t, 1.0, mlBuf.FairProb+mlMia.FairProb, 1e-9)

	spBuf, ok := g1[analyzer.SharpKey{MarketType: "SPREAD", Team: "BUF", Side: "TEAM"}]
	require.True(t, ok)
	require.NotNil(t, spBuf.Line)
	assert.InDelta(t, -2.5, *spBuf.Line, 1e-9)

	totOver, ok := g1[analyzer.SharpKey{MarketType: "TOTAL", Side: "OVER"}]
	require.True(t, ok)
	require.NotNil(t, totOver.Line)
	assert.InDelta(t, 48.5, *totOver.Line, 1e-9)

	ttBuf, ok := g1[analyzer.SharpKey{MarketType: "TEAM_TOTAL", Team: "BUF", Side: "OVER"}]
	require.True(t, ok)
	assert.InDelta(t, 0.5, ttBuf.FairProb, 1e-6)
	ttMia, ok := g1[analyzer.SharpKey{MarketType: "TEAM_TOTAL", Team: "MIA", Side: "UNDER"}]
	require.True(t, ok)
	assert.Less(t, ttMia.FairProb, 0.5)

	passAllen, ok := g1[analyzer.SharpKey{MarketType: "PASS_YDS", Team: "BUF", PlayerName: "josh allen", Side: "OVER"}]
	require.True(t, ok)
	assert.Greater(t, passAllen.FairProb, 0.5)

	recHarrison, ok := g1[analyzer.SharpKey{MarketType: "REC_YDS", Team: "MIA", PlayerName: "marvin harrison", Side: "OVER"}]
	require.True(t, ok)
	assert.Greater(t, recHarrison.FairProb, 0.5)

	rushGordon, ok := g1[analyzer.SharpKey{MarketType: "RUSH_YDS", Team: "MIA", PlayerName: "ollie gordon", Side: "OVER"}]
	require.True(t, ok)
	assert.Equal(t, "Circa", rushGordon.Source)
	assert.InDelta(t, 0.5, rushGordon.FairProb, 1e-6)
}

func TestBuildSharpBenchmarksOneSidedAndInvalidFailSoft(t *testing.T) {
	t.Parallel()

	assert.Nil(t, analyzer.BuildSharpBenchmarks(nil))

	rows := []db.Odds{
		// ANYTIME_TD is one-sided YES -> must be skipped.
		{
			GameID:     "g1",
			Book:       "pinnacle",
			MarketType: "ANYTIME_TD",
			Team:       "BUF",
			PlayerName: "Josh Allen",
			Side:       "YES",
			Price:      120,
		},
		// One-sided ML.
		{GameID: "g1", Book: "pinnacle", MarketType: "ML", Team: "BUF", Side: "TEAM", Price: -140},
		// Mismatched spread lines (-3.5 vs +3.0).
		{GameID: "g1", Book: "pinnacle", MarketType: "SPREAD", Team: "BUF", Side: "TEAM", Line: new(-3.5), Price: -110},
		{GameID: "g1", Book: "pinnacle", MarketType: "SPREAD", Team: "MIA", Side: "TEAM", Line: new(3.0), Price: -110},
		// One-sided total.
		{GameID: "g1", Book: "pinnacle", MarketType: "TOTAL", Side: "OVER", Line: new(44.5), Price: -110},
		// Mismatched prop lines (55.5 vs 56.5).
		{
			GameID:     "g1",
			Book:       "pinnacle",
			MarketType: "RUSH_YDS",
			Team:       "BUF",
			PlayerName: "James Cook",
			Side:       "OVER",
			Line:       new(55.5),
			Price:      -110,
		},
		{
			GameID:     "g1",
			Book:       "pinnacle",
			MarketType: "RUSH_YDS",
			Team:       "BUF",
			PlayerName: "James Cook",
			Side:       "UNDER",
			Line:       new(56.5),
			Price:      -110,
		},
		// Corrupt overround (> 1.25).
		{GameID: "g1", Book: "lowvig", MarketType: "TOTAL", Side: "OVER", Line: new(44.5), Price: -300},
		{GameID: "g1", Book: "lowvig", MarketType: "TOTAL", Side: "UNDER", Line: new(44.5), Price: -300},
		// Zero price.
		{GameID: "g1", Book: "circa", MarketType: "ML", Team: "BUF", Side: "TEAM", Price: 0},
		{GameID: "g1", Book: "circa", MarketType: "ML", Team: "MIA", Side: "TEAM", Price: 110},
	}

	got := analyzer.BuildSharpBenchmarks(rows)
	assert.Nil(t, got)
}

func TestBuildSharpBenchmarksCapturedUSFixture(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile(filepath.Join("..", "ingest", "odds", "testdata", "odds_nfl.json"))
	require.NoError(t, err)

	lines, err := odds.ParseLinesJSON(raw)
	require.NoError(t, err)
	require.NotEmpty(t, lines)

	rows := make([]db.Odds, 0, len(lines))
	for _, l := range lines {
		rows = append(rows, db.Odds{
			GameID:     l.EventID,
			Book:       l.Book,
			MarketType: l.MarketType,
			Team:       strings.ToUpper(l.Team),
			Side:       l.Side,
			Line:       l.Line,
			Price:      l.Price,
		})
	}

	got := analyzer.BuildSharpBenchmarks(rows)
	require.NotEmpty(t, got)

	// Event dc56d980f527ffeef89940133b211b13 (Atlanta Falcons vs Cincinnati Bengals)
	// includes both lowvig and betonlineag in regions=us.
	const falconsBengalsID = "dc56d980f527ffeef89940133b211b13"
	fb := got[falconsBengalsID]
	require.NotEmpty(t, fb)

	atlML, ok := fb[analyzer.SharpKey{MarketType: "ML", Team: "ATLANTA FALCONS", Side: "TEAM"}]
	require.True(t, ok)
	cinML, ok := fb[analyzer.SharpKey{MarketType: "ML", Team: "CINCINNATI BENGALS", Side: "TEAM"}]
	require.True(t, ok)
	assert.Equal(t, 2, atlML.Tier)
	assert.Equal(t, "LowVig", atlML.Source)
	assert.InDelta(t, 1.0, atlML.FairProb+cinML.FairProb, 1e-6)

	totOver, ok := fb[analyzer.SharpKey{MarketType: "TOTAL", Side: "OVER"}]
	require.True(t, ok)
	totUnder, ok := fb[analyzer.SharpKey{MarketType: "TOTAL", Side: "UNDER"}]
	require.True(t, ok)
	assert.Equal(t, 2, totOver.Tier)
	assert.Equal(t, "LowVig", totOver.Source)
	require.NotNil(t, totOver.Line)
	assert.InDelta(t, 52.5, *totOver.Line, 1e-9)
	assert.InDelta(t, 1.0, totOver.FairProb+totUnder.FairProb, 1e-6)

	// Event 35e3bdff558b5d1506251e814986f164 (Baltimore Ravens vs Los Angeles Chargers)
	// has only DraftKings, FanDuel, BetRivers: all 3 agree on SPREAD -1.5/+1.5 and ML,
	// while TOTAL has 3 different lines (37.5, 38.5, 38.0) so TOTAL fails soft.
	const ravensChargersID = "35e3bdff558b5d1506251e814986f164"
	rc := got[ravensChargersID]
	require.NotEmpty(t, rc)

	ravensSpread, ok := rc[analyzer.SharpKey{MarketType: "SPREAD", Team: "BALTIMORE RAVENS", Side: "TEAM"}]
	require.True(t, ok)
	assert.Equal(t, 3, ravensSpread.Tier)
	assert.Equal(t, "market consensus", ravensSpread.Source)
	require.NotNil(t, ravensSpread.Line)
	assert.InDelta(t, -1.5, *ravensSpread.Line, 1e-9)

	_, hasTotal := rc[analyzer.SharpKey{MarketType: "TOTAL", Side: "OVER"}]
	assert.False(t, hasTotal, "total with 3 distinct lines across 3 books must not form Tier 3 consensus")
}

func TestHasSharpBenchmark(t *testing.T) {
	t.Parallel()

	game := analyzer.Game{
		ID: "g1",
		Sharp: map[analyzer.SharpKey]analyzer.SharpQuote{
			{MarketType: "SPREAD", Team: "BUF", Side: "TEAM"}:                  {FairProb: 0.5},
			{MarketType: "TOTAL", Side: "OVER"}:                                {FairProb: 0.5},
			{MarketType: "REC_YDS", PlayerName: "khalil shakir", Side: "OVER"}: {FairProb: 0.5},
		},
	}

	assert.False(t, analyzer.HasSharpBenchmark(analyzer.Game{ID: "empty"}, analyzer.Leg{Market: "SPREAD", Team: "BUF"}))
	assert.True(t, analyzer.HasSharpBenchmark(game, analyzer.Leg{Market: "SPREAD", Team: "buf"}))
	assert.True(t, analyzer.HasSharpBenchmark(game, analyzer.Leg{Market: "TOTAL", Team: "BUF", Side: "OVER"}))
	assert.True(t, analyzer.HasSharpBenchmark(game, analyzer.Leg{
		Market:     "REC_YDS",
		PlayerName: "Khalil Shakir",
		Side:       "OVER",
	}))
	assert.False(t, analyzer.HasSharpBenchmark(game, analyzer.Leg{Market: "ML", Team: "BUF"}))
}

func TestBuildSharpBenchmarksPowerVigAndBookCount(t *testing.T) {
	t.Parallel()

	t.Run("tier 1 multi-book aggregates PowerFairProb, VigPct, and BookCount", func(t *testing.T) {
		t.Parallel()
		rows := []db.Odds{
			// Pinnacle: -400 / +300
			{GameID: "g1", Book: "pinnacle", MarketType: "ML", Team: "BUF", Side: "TEAM", Price: -400},
			{GameID: "g1", Book: "pinnacle", MarketType: "ML", Team: "MIA", Side: "TEAM", Price: 300},
			// Circa: -380 / +290
			{GameID: "g1", Book: "circa", MarketType: "ML", Team: "BUF", Side: "TEAM", Price: -380},
			{GameID: "g1", Book: "circa", MarketType: "ML", Team: "MIA", Side: "TEAM", Price: 290},
			// Bookmaker: -420 / +310
			{GameID: "g1", Book: "bookmaker", MarketType: "ML", Team: "BUF", Side: "TEAM", Price: -420},
			{GameID: "g1", Book: "bookmaker", MarketType: "ML", Team: "MIA", Side: "TEAM", Price: 310},
		}

		got := analyzer.BuildSharpBenchmarks(rows)
		bufQ, ok := got["g1"][analyzer.SharpKey{MarketType: "ML", Team: "BUF", Side: "TEAM"}]
		require.True(t, ok)
		miaQ, ok := got["g1"][analyzer.SharpKey{MarketType: "ML", Team: "MIA", Side: "TEAM"}]
		require.True(t, ok)

		assert.Equal(t, 1, bufQ.Tier)
		assert.Equal(t, "Pinnacle", bufQ.Source)
		assert.Equal(t, 3, bufQ.BookCount)
		assert.Equal(t, 3, miaQ.BookCount)

		// Power de-vig allocates proportionally more vig to the longshot (+300),
		// so favorite PowerFairProb > multiplicative FairProb and probabilities sum to 1.
		assert.Greater(t, bufQ.PowerFairProb, bufQ.FairProb)
		assert.Less(t, miaQ.PowerFairProb, miaQ.FairProb)
		assert.InDelta(t, 1.0, bufQ.PowerFairProb+miaQ.PowerFairProb, 1e-9)

		// Median vig of Pinnacle (0.05), Circa (~0.0480), Bookmaker (~0.0517) is 0.05.
		wantVig, ok := parlay.VigPct(-400, 300)
		require.True(t, ok)
		assert.InDelta(t, wantVig, bufQ.VigPct, 1e-9)
		assert.InDelta(t, bufQ.VigPct, miaQ.VigPct, 1e-9)
	})

	t.Run("tier 3 consensus BookCount counts matching-line books only", func(t *testing.T) {
		t.Parallel()
		rows := []db.Odds{
			// 3 books on 47.5
			{GameID: "g1", Book: "draftkings", MarketType: "TOTAL", Side: "OVER", Line: new(47.5), Price: -110},
			{GameID: "g1", Book: "draftkings", MarketType: "TOTAL", Side: "UNDER", Line: new(47.5), Price: -110},
			{GameID: "g1", Book: "fanduel", MarketType: "TOTAL", Side: "OVER", Line: new(47.5), Price: -112},
			{GameID: "g1", Book: "fanduel", MarketType: "TOTAL", Side: "UNDER", Line: new(47.5), Price: -108},
			{GameID: "g1", Book: "betmgm", MarketType: "TOTAL", Side: "OVER", Line: new(47.5), Price: -115},
			{GameID: "g1", Book: "betmgm", MarketType: "TOTAL", Side: "UNDER", Line: new(47.5), Price: -105},
			// 1 book on 48.0 (must not count toward the 47.5 bucket)
			{GameID: "g1", Book: "caesars", MarketType: "TOTAL", Side: "OVER", Line: new(48.0), Price: -110},
			{GameID: "g1", Book: "caesars", MarketType: "TOTAL", Side: "UNDER", Line: new(48.0), Price: -110},
		}

		got := analyzer.BuildSharpBenchmarks(rows)
		overQ, ok := got["g1"][analyzer.SharpKey{MarketType: "TOTAL", Side: "OVER"}]
		require.True(t, ok)
		assert.Equal(t, 3, overQ.Tier)
		assert.Equal(t, 3, overQ.BookCount)
		wantPower, _, wantVig, ok := parlay.DevigTwoWayPower(-112, -108)
		require.True(t, ok)
		assert.InDelta(t, wantPower, overQ.PowerFairProb, 1e-9)
		assert.InDelta(t, wantVig, overQ.VigPct, 1e-9)
	})
}
