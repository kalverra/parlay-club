package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/analyzer"
	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/eval"
	"github.com/kalverra/parlay-club/internal/ingest/archive"
	"github.com/kalverra/parlay-club/internal/ingest/nflverse"
	"github.com/kalverra/parlay-club/internal/parlay"
)

func seedEvalDB(t *testing.T) string {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "eval-test.db")
	database, err := db.Open(dbPath)
	require.NoError(t, err)
	defer func() { _ = database.Close() }()

	ctx := t.Context()
	require.NoError(t, db.Migrate(ctx, database))
	store := db.NewStore(database)
	defer func() { _ = store.Close() }()

	user, err := store.CreateUserWithIdentity(ctx, "Ada", "ada@example.com", "sub-1", "")
	require.NoError(t, err)
	group, err := store.Create(ctx, user.ID, "Sunday Club")
	require.NoError(t, err)

	week, err := store.UpsertWeek(ctx, 2025, nflverse.SeasonTypeRegular, 4)
	require.NoError(t, err)

	kickoff := time.Date(2025, 10, 5, 17, 0, 0, 0, time.UTC)
	_, err = store.UpsertGame(ctx, db.Game{
		ID: "g1", WeekID: week.ID, HomeTeam: "BUF", AwayTeam: "MIA",
		HomeName: "Buffalo Bills", AwayName: "Miami Dolphins",
		Kickoff: kickoff, Status: db.GameFinal, Indoor: true,
	})
	require.NoError(t, err)

	preGamelog := `{"names":["receivingYards"],"labels":["YDS"],"categories":[{"name":"receiving","count":1}],"seasonTypes":[{"displayName":"Regular Season","categories":[{"displayName":"Regular Season","splitType":"0","events":[{"eventId":"1","stats":["80"]},{"eventId":"2","stats":["75"]},{"eventId":"3","stats":["70"]},{"eventId":"4","stats":["65"]},{"eventId":"5","stats":["60"]}]}]}]}`
	require.NoError(t, store.SaveAPIResponse(ctx, archive.Record{
		Source:    "espn",
		Key:       "espn:gamelog:p1",
		Method:    "GET",
		URL:       "https://example.test/gamelog",
		Status:    200,
		FetchedAt: kickoff.Add(-1 * time.Hour),
		Body:      []byte(preGamelog),
	}))

	slip, err := store.CreateSlip(ctx, group.ID, week.ID, nil, "W4", db.SlateScopeFullWeek, 2, []int64{user.ID})
	require.NoError(t, err)
	leg, err := store.CreateLeg(ctx, db.Leg{
		SlipID:     slip.ID,
		UserID:     user.ID,
		GameID:     new("g1"),
		MarketType: "REC_YDS",
		Selection:  "Player Over 55.5 Rec Yds",
		Team:       new("BUF"),
		PlayerName: new("Player One"),
		PlayerID:   new("p1"),
		Side:       new("OVER"),
		Line:       new(55.5),
		Odds:       100,
	})
	require.NoError(t, err)
	require.NoError(t, store.SetLegResult(ctx, leg.ID, parlay.Won, "test"))
	require.NoError(t, store.SetSlipGrade(ctx, slip.ID, parlay.Won, db.SlipGraded))

	return dbPath
}

func TestEvalCLI(t *testing.T) {
	t.Parallel()
	dbPath := seedEvalDB(t)

	t.Run("single rubric text format", func(t *testing.T) {
		t.Parallel()
		var out, errBuf bytes.Buffer
		err := run(t.Context(), []string{"--db", dbPath, "--rubric", "v1", "--format", "text"}, &out, &errBuf)
		require.NoError(t, err)

		text := out.String()
		assert.Contains(t, text, "Rubric: v1")
		assert.Contains(t, text, "Data Coverage: legs=1 gamelogs=1")
		assert.Contains(t, text, "Leg Calibration:")
		assert.Contains(t, text, "Factor Attribution:")
		assert.Contains(t, text, "Slip Calibration:")
		assert.NotContains(t, text, "Slip Factor Attribution:")
	})

	t.Run("single rubric json format", func(t *testing.T) {
		t.Parallel()
		var out, errBuf bytes.Buffer
		err := run(
			t.Context(),
			[]string{"--db", dbPath, "--season", "2025", "--from-week", "1", "--to-week", "12", "--format", "json"},
			&out,
			&errBuf,
		)
		require.NoError(t, err)

		var report eval.Report
		require.NoError(t, json.Unmarshal(out.Bytes(), &report))
		assert.Equal(t, "v1", report.RubricName)
		assert.Equal(t, 1, report.Coverage.TotalLegs)
		assert.Equal(t, 1, report.Coverage.WithGameLogs)
	})

	t.Run("baseline vs candidate-file diff", func(t *testing.T) {
		t.Parallel()
		cand := analyzer.RubricV1
		cand.Name = "v2-candidate"
		cand.HitRate = 0 // drops leg from Band A (85) to Band B (75)
		candBytes, err := json.Marshal(cand)
		require.NoError(t, err)

		candPath := filepath.Join(t.TempDir(), "rubric-v2.json")
		require.NoError(t, os.WriteFile(candPath, candBytes, 0o600))

		var out, errBuf bytes.Buffer
		err = run(
			t.Context(),
			[]string{"--db", dbPath, "--baseline", "v1", "--candidate-file", candPath},
			&out,
			&errBuf,
		)
		require.NoError(t, err)

		text := out.String()
		assert.Contains(t, text, "Comparison: v1 -> v2-candidate")
		assert.Contains(t, text, "Leg Calibration Diff:")
		assert.Contains(t, text, "Slip Calibration Diff:")
		assert.Contains(t, text, "Leg Band Migrations:")
	})

	t.Run("missing db and dataset flags returns error", func(t *testing.T) {
		t.Parallel()
		var out, errBuf bytes.Buffer
		err := run(t.Context(), []string{"--rubric", "v1"}, &out, &errBuf)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--db or --dataset is required")
	})

	t.Run("db and dataset flags are mutually exclusive", func(t *testing.T) {
		t.Parallel()
		datasetPath := filepath.Join("..", "..", "internal", "eval", "testdata", "benchmark_dataset.json")
		var out, errBuf bytes.Buffer
		err := run(t.Context(), []string{"--db", dbPath, "--dataset", datasetPath}, &out, &errBuf)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "mutually exclusive")
	})

	t.Run("all cannot be combined with compare flags", func(t *testing.T) {
		t.Parallel()
		datasetPath := filepath.Join("..", "..", "internal", "eval", "testdata", "benchmark_dataset.json")
		var out, errBuf bytes.Buffer
		err := run(t.Context(), []string{"--dataset", datasetPath, "--all", "--baseline", "v1"}, &out, &errBuf)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--all cannot be combined")
	})

	t.Run("unsupported format returns error", func(t *testing.T) {
		t.Parallel()
		datasetPath := filepath.Join("..", "..", "internal", "eval", "testdata", "benchmark_dataset.json")
		var out, errBuf bytes.Buffer
		err := run(t.Context(), []string{"--dataset", datasetPath, "--format", "xml"}, &out, &errBuf)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported --format")
	})

	t.Run("missing dataset file returns error", func(t *testing.T) {
		t.Parallel()
		var out, errBuf bytes.Buffer
		err := run(t.Context(), []string{"--dataset", filepath.Join(t.TempDir(), "missing.json")}, &out, &errBuf)
		require.Error(t, err)
	})

	t.Run("dataset single rubric markdown format", func(t *testing.T) {
		t.Parallel()
		datasetPath := filepath.Join("..", "..", "internal", "eval", "testdata", "benchmark_dataset.json")
		var out, errBuf bytes.Buffer
		err := run(t.Context(), []string{"--dataset", datasetPath, "--format", "markdown"}, &out, &errBuf)
		require.NoError(t, err)

		md := out.String()
		assert.Contains(t, md, "## Grading Eval: v1")
		assert.Contains(t, md, "**Data Coverage:** legs=72")
		assert.Contains(t, md, "| Band | Total | Decided | W-L-P |")
		assert.Contains(t, md, "#### Slip Calibration")
		assert.Contains(t, md, "#### Top Factor Attribution")
		assert.Contains(t, md, "#### Slip Factor Attribution")
		assert.Contains(t, md, "blowout_passing")
	})

	t.Run("dataset single rubric text format", func(t *testing.T) {
		t.Parallel()
		datasetPath := filepath.Join("..", "..", "internal", "eval", "testdata", "benchmark_dataset.json")
		var out, errBuf bytes.Buffer
		err := run(t.Context(), []string{"--dataset", datasetPath, "--format", "text"}, &out, &errBuf)
		require.NoError(t, err)

		text := out.String()
		assert.Contains(t, text, "Slip Factor Attribution:")
		assert.Contains(t, text, "blowout_passing")
	})

	t.Run("dataset all rubrics json format", func(t *testing.T) {
		t.Parallel()
		datasetPath := filepath.Join("..", "..", "internal", "eval", "testdata", "benchmark_dataset.json")
		var out, errBuf bytes.Buffer
		err := run(t.Context(), []string{"--dataset", datasetPath, "--all", "--format", "json"}, &out, &errBuf)
		require.NoError(t, err)

		var suite eval.SuiteReport
		require.NoError(t, json.Unmarshal(out.Bytes(), &suite))
		assert.Equal(t, analyzer.ActiveRubric.Name, suite.ActiveRubric)
		assert.Equal(t, 72, suite.Coverage.TotalLegs)
		require.Len(t, suite.Reports, len(analyzer.BuiltInRubrics()))
		assert.Len(t, suite.Reports[0].Weekly, 4)
	})

	t.Run("dataset all rubrics text format", func(t *testing.T) {
		t.Parallel()
		datasetPath := filepath.Join("..", "..", "internal", "eval", "testdata", "benchmark_dataset.json")
		var out, errBuf bytes.Buffer
		err := run(t.Context(), []string{"--dataset", datasetPath, "--all", "--format", "text"}, &out, &errBuf)
		require.NoError(t, err)

		text := out.String()
		assert.Contains(t, text, "Suite (active: v1)")
		assert.Contains(t, text, "Rubric: v1")
		assert.Contains(t, text, "Data Coverage: legs=72")
	})

	t.Run("dataset all rubrics markdown format", func(t *testing.T) {
		t.Parallel()
		datasetPath := filepath.Join("..", "..", "internal", "eval", "testdata", "benchmark_dataset.json")
		var out, errBuf bytes.Buffer
		err := run(t.Context(), []string{"--dataset", datasetPath, "--all", "--format", "markdown"}, &out, &errBuf)
		require.NoError(t, err)

		md := out.String()
		assert.Contains(t, md, "## Grading Eval Suite (active: `v1`)")
		assert.Contains(t, md, "### Rubric: `v1`")
		assert.Contains(t, md, "#### Leg Calibration")
		assert.Contains(t, md, "#### Slip Calibration")
		assert.Contains(t, md, "#### Top Factor Attribution")
		assert.Contains(t, md, "#### Slip Factor Attribution")
		assert.Contains(t, md, "blowout_passing")
	})

	t.Run("dataset compare markdown format", func(t *testing.T) {
		t.Parallel()
		datasetPath := filepath.Join("..", "..", "internal", "eval", "testdata", "benchmark_dataset.json")
		cand := analyzer.RubricV1
		cand.Name = "v2-candidate"
		cand.HitRate = 0
		candBytes, err := json.Marshal(cand)
		require.NoError(t, err)

		candPath := filepath.Join(t.TempDir(), "rubric-v2.json")
		require.NoError(t, os.WriteFile(candPath, candBytes, 0o600))

		var out, errBuf bytes.Buffer
		err = run(
			t.Context(),
			[]string{
				"--dataset", datasetPath,
				"--baseline", "v1",
				"--candidate-file", candPath,
				"--format", "markdown",
			},
			&out,
			&errBuf,
		)
		require.NoError(t, err)

		md := out.String()
		assert.Contains(t, md, "## Grading Eval Comparison: `v1` -> `v2-candidate`")
		assert.Contains(t, md, "#### Leg Calibration Diff")
		assert.Contains(t, md, "#### Slip Calibration Diff")
		assert.Contains(t, md, "#### Leg Band Migrations")
		assert.Contains(t, md, "### Candidate Factor Attribution (`v2-candidate`)")
		assert.Contains(t, md, "### Candidate Slip Factor Attribution (`v2-candidate`)")
	})
}
