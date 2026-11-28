// Command eval runs offline grading calibration and rubric comparison reports.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/kalverra/parlay-club/internal/analyzer"
	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/eval"
)

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdout, os.Stderr); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "eval: %v\n", err)
		os.Exit(1)
	}
}

type diffJSONOutput struct {
	Baseline  eval.Report     `json:"baseline"`
	Candidate eval.Report     `json:"candidate"`
	Diff      eval.DiffReport `json:"diff"`
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("eval", flag.ContinueOnError)
	fs.SetOutput(stderr)

	dbPath := fs.String("db", "", "path to SQLite database (opened read-only)")
	datasetPath := fs.String("dataset", "", "path to replay dataset JSON file (mutually exclusive with --db)")
	allRubrics := fs.Bool(
		"all",
		false,
		"evaluate all built-in grading engines side-by-side and emit suite calibration + diffs",
	)
	rubricName := fs.String("rubric", analyzer.RubricV1.Name, "built-in grading engine or rubric name for single eval")
	baselineName := fs.String("baseline", "", "baseline built-in grading engine or rubric name for comparison")
	candidateName := fs.String("candidate", "", "candidate built-in grading engine or rubric name for comparison")
	candidateFile := fs.String("candidate-file", "", "path to candidate rubric JSON file for comparison")
	season := fs.Int("season", 0, "filter to season year (0 = all)")
	fromWeek := fs.Int("from-week", 0, "filter to week >= from-week (0 = unbounded)")
	toWeek := fs.Int("to-week", 0, "filter to week <= to-week (0 = unbounded)")
	mode := fs.String("mode", string(eval.ModeSlips), "replay mode: slips or market")
	format := fs.String("format", "text", "output format: text, json, or markdown")

	if err := fs.Parse(args); err != nil {
		return err
	}

	modeSet := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "mode" {
			modeSet = true
		}
	})

	hasDB := strings.TrimSpace(*dbPath) != ""
	hasDataset := strings.TrimSpace(*datasetPath) != ""
	if hasDB && hasDataset {
		return errors.New("--db and --dataset are mutually exclusive")
	}
	if !hasDB && !hasDataset {
		return errors.New("--db or --dataset is required")
	}
	if hasDataset && modeSet {
		return errors.New("--mode cannot be combined with --dataset")
	}
	replayMode := eval.ReplayMode(strings.TrimSpace(*mode))
	if replayMode != eval.ModeSlips && replayMode != eval.ModeMarket {
		return fmt.Errorf("unsupported --mode %q (want slips or market)", *mode)
	}
	if *format != "text" && *format != "json" && *format != "markdown" {
		return fmt.Errorf("unsupported --format %q (want text, json, or markdown)", *format)
	}

	compareMode := *baselineName != "" || *candidateName != "" || *candidateFile != ""
	if *allRubrics && compareMode {
		return errors.New("--all cannot be combined with --baseline, --candidate, or --candidate-file")
	}

	var (
		dataset eval.ReplayDataset
		err     error
	)
	if hasDataset {
		dataset, err = eval.LoadDatasetJSON(*datasetPath)
		if err != nil {
			return err
		}
	} else {
		dataset, err = loadDatasetFromDB(ctx, *dbPath, eval.ReplayFilter{
			Season:   *season,
			FromWeek: *fromWeek,
			ToWeek:   *toWeek,
			Mode:     replayMode,
		})
		if err != nil {
			return err
		}
	}

	if *allRubrics {
		return runAll(stdout, dataset, *format)
	}
	if !compareMode {
		return runSingle(stdout, dataset, *rubricName, *format)
	}
	return runCompare(stdout, dataset, *baselineName, *candidateName, *candidateFile, *format)
}

func runAll(stdout io.Writer, dataset eval.ReplayDataset, format string) error {
	suite := eval.EvaluateAllEngines(dataset, analyzer.BuiltInEngines())
	switch format {
	case "json":
		return writeJSON(stdout, suite)
	case "markdown":
		return writeSuiteMarkdown(stdout, suite)
	default:
		return writeSuiteText(stdout, suite)
	}
}

func runSingle(stdout io.Writer, dataset eval.ReplayDataset, targetName, format string) error {
	report, ok := evaluateBuiltInTarget(dataset, targetName)
	if !ok {
		return fmt.Errorf(
			"unknown rubric or engine %q (available: %s)",
			targetName,
			strings.Join(availableTargetNames(), ", "),
		)
	}
	switch format {
	case "json":
		return writeJSON(stdout, report)
	case "markdown":
		return writeReportMarkdown(stdout, report)
	default:
		return writeReportText(stdout, report)
	}
}

func runCompare(
	stdout io.Writer,
	dataset eval.ReplayDataset,
	baselineName, candidateName, candidateFile, format string,
) error {
	baseName := baselineName
	if baseName == "" {
		baseName = analyzer.RubricV1.Name
	}
	baseReport, ok := evaluateBuiltInTarget(dataset, baseName)
	if !ok {
		return fmt.Errorf(
			"unknown baseline rubric or engine %q (available: %s)",
			baseName,
			strings.Join(availableTargetNames(), ", "),
		)
	}

	candReport, err := evaluateCandidateTarget(dataset, candidateName, candidateFile)
	if err != nil {
		return err
	}

	diff := eval.Compare(baseReport, candReport)

	switch format {
	case "json":
		return writeJSON(stdout, diffJSONOutput{
			Baseline:  baseReport,
			Candidate: candReport,
			Diff:      diff,
		})
	case "markdown":
		return writeDiffMarkdown(stdout, baseReport, candReport, diff)
	default:
		return writeDiffText(stdout, baseReport, candReport, diff)
	}
}

func evaluateBuiltInTarget(dataset eval.ReplayDataset, name string) (eval.Report, bool) {
	if engine, ok := analyzer.EngineByName(name); ok {
		return eval.EvaluateEngine(dataset, engine), true
	}
	if rubric, ok := analyzer.RubricByName(name); ok {
		return eval.Evaluate(dataset, rubric), true
	}
	return eval.Report{}, false
}

func evaluateCandidateTarget(dataset eval.ReplayDataset, name, file string) (eval.Report, error) {
	if file != "" {
		candRubric, err := loadCandidateRubricFile(file)
		if err != nil {
			return eval.Report{}, err
		}
		return eval.Evaluate(dataset, candRubric), nil
	}
	if name == "" {
		return eval.Report{}, errors.New("--candidate or --candidate-file is required when --baseline is set")
	}
	report, ok := evaluateBuiltInTarget(dataset, name)
	if !ok {
		return eval.Report{}, fmt.Errorf(
			"unknown candidate rubric or engine %q (available: %s)",
			name,
			strings.Join(availableTargetNames(), ", "),
		)
	}
	return report, nil
}

func availableTargetNames() []string {
	seen := make(map[string]bool)
	var out []string
	for _, n := range analyzer.EngineNames() {
		if !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	for _, n := range analyzer.RubricNames() {
		if !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	return out
}

func loadDatasetFromDB(ctx context.Context, dbPath string, filter eval.ReplayFilter) (eval.ReplayDataset, error) {
	database, err := db.OpenReadOnly(dbPath)
	if err != nil {
		return eval.ReplayDataset{}, err
	}
	defer func() { _ = database.Close() }()

	store := db.NewStore(database)
	defer func() { _ = store.Close() }()

	return eval.LoadDataset(ctx, store, filter)
}

func loadCandidateRubricFile(file string) (analyzer.Rubric, error) {
	cleanPath := filepath.Clean(file)
	// #nosec G304 -- CLI flag intentionally reads a user-specified rubric JSON file.
	raw, err := os.ReadFile(cleanPath)
	if err != nil {
		return analyzer.Rubric{}, fmt.Errorf("read candidate rubric %s: %w", cleanPath, err)
	}
	var r analyzer.Rubric
	if err := json.Unmarshal(raw, &r); err != nil {
		return analyzer.Rubric{}, fmt.Errorf("parse candidate rubric %s: %w", cleanPath, err)
	}
	if strings.TrimSpace(r.Name) == "" {
		r.Name = filepath.Base(cleanPath)
	}
	return r, nil
}

func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

type errWriter struct {
	w   io.Writer
	err error
}

func (ew *errWriter) printf(format string, args ...any) {
	if ew.err != nil {
		return
	}
	_, ew.err = fmt.Fprintf(ew.w, format, args...)
}

func (ew *errWriter) println(args ...any) {
	if ew.err != nil {
		return
	}
	_, ew.err = fmt.Fprintln(ew.w, args...)
}

func writeReportText(w io.Writer, r eval.Report) error {
	ew := &errWriter{w: w}
	ew.printf("Rubric: %s\n", r.RubricName)
	writeCoverage(ew, r.Coverage)

	ew.println("\nLeg Calibration:")
	if ew.err != nil {
		return ew.err
	}
	if err := writeBandsTable(w, r.LegBands); err != nil {
		return err
	}
	ew.printf("Monotonic: %v (win=%v, lift=%v)\n", r.LegMonotonic, r.LegMonotonicWin, r.LegMonotonicLift)

	if hasCalibrationData(r.Calibration) {
		ew.println("\nProbability Calibration:")
		if ew.err != nil {
			return ew.err
		}
		if err := writeCalibrationText(w, r.Calibration); err != nil {
			return err
		}
	}

	if r.MarketExplanation.SampleCount > 0 {
		ew.println("\n1. Explaining Vegas (Match First):")
		if ew.err != nil {
			return ew.err
		}
		if err := writeMarketExplanationText(w, r.MarketExplanation); err != nil {
			return err
		}
	}

	if r.MarketBeating.SampleCount > 0 {
		ew.println("\n2. Beating Vegas (Alpha Gate):")
		if ew.err != nil {
			return ew.err
		}
		if err := writeMarketBeatingText(w, r.MarketBeating); err != nil {
			return err
		}
	}

	ew.println("\nFactor Attribution:")
	if ew.err != nil {
		return ew.err
	}
	if err := writeFactorsTable(w, r.Factors); err != nil {
		return err
	}

	if hasBandTotals(r.SlipBands) {
		ew.println("\nSlip Calibration:")
		if ew.err != nil {
			return ew.err
		}
		if err := writeBandsTable(w, r.SlipBands); err != nil {
			return err
		}
	}
	if len(r.SlipFactors) > 0 {
		ew.println("\nSlip Factor Attribution:")
		if ew.err != nil {
			return ew.err
		}
		return writeFactorsTable(w, r.SlipFactors)
	}
	return nil
}

func writeDiffText(w io.Writer, base, cand eval.Report, diff eval.DiffReport) error {
	ew := &errWriter{w: w}
	ew.printf("Comparison: %s -> %s\n", diff.BaselineName, diff.CandidateName)
	writeCoverage(ew, base.Coverage)

	if hasCalibrationData(base.Calibration) || hasCalibrationData(cand.Calibration) {
		ew.println("\nEngine Calibration Comparison:")
		if ew.err != nil {
			return ew.err
		}
		if err := writeEngineComparisonTable(w, []eval.Report{base, cand}); err != nil {
			return err
		}
	}

	ew.println("\nLeg Calibration Diff:")
	if ew.err != nil {
		return ew.err
	}
	if err := writeBandDiffsTable(w, diff.LegBandDiffs); err != nil {
		return err
	}

	if hasBandTotals(base.SlipBands) || hasBandTotals(cand.SlipBands) {
		ew.println("\nSlip Calibration Diff:")
		if ew.err != nil {
			return ew.err
		}
		if err := writeBandDiffsTable(w, diff.SlipBandDiffs); err != nil {
			return err
		}
	}

	ew.println("\nLeg Band Migrations:")
	if ew.err != nil {
		return ew.err
	}
	if err := writeMigrationsTable(w, diff.Migrations); err != nil {
		return err
	}

	ew.printf("\nCandidate Factor Attribution (%s):\n", cand.RubricName)
	if ew.err != nil {
		return ew.err
	}
	if err := writeFactorsTable(w, cand.Factors); err != nil {
		return err
	}
	if len(cand.SlipFactors) > 0 {
		ew.printf("\nCandidate Slip Factor Attribution (%s):\n", cand.RubricName)
		if ew.err != nil {
			return ew.err
		}
		return writeFactorsTable(w, cand.SlipFactors)
	}
	return nil
}

func writeCoverage(ew *errWriter, c eval.DataCoverage) {
	ew.printf(
		"Data Coverage: legs=%d gamelogs=%d injuries=%d nflverse=%d nfelo=%d odds=%d sharp=%d l5_fallback=%d\n",
		c.TotalLegs, c.WithGameLogs, c.WithInjuries, c.WithNFLVerse, c.WithNfelo, c.WithOdds,
		c.WithSharp, c.WithL5Fallback,
	)
}

func writeBandsTable(w io.Writer, bands []eval.BandStats) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	ew := &errWriter{w: tw}
	ew.println("Band\tTotal\tDecided\tW-L-P\tWinRate\tImplied\tLift\tFlatROI\tBrier")
	for _, b := range bands {
		ew.printf(
			"%s\t%d\t%d\t%d-%d-%d\t%.3f\t%.3f\t%+.3f\t%+.3f\t%.4f\n",
			b.Band, b.Total, b.Decided, b.Wins, b.Losses, b.Pushes,
			b.WinRate, b.AvgImpliedProb, b.Lift, b.FlatROI, b.BrierScore,
		)
	}
	if ew.err != nil {
		return ew.err
	}
	return tw.Flush()
}

func writeFactorsTable(w io.Writer, factors []eval.FactorStats) error {
	if len(factors) == 0 {
		_, err := fmt.Fprintln(w, "(none)")
		return err
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	ew := &errWriter{w: tw}
	ew.println("Rule\tLabel\tCount\tW-L\tWinRate\tImplied\tLift\tAvgDelta")
	for _, f := range factors {
		ew.printf(
			"%s\t%s\t%d\t%d-%d\t%.3f\t%.3f\t%+.3f\t%+.2f\n",
			f.Rule, f.Label, f.Count, f.Wins, f.Losses,
			f.WinRate, f.AvgImpliedProb, f.Lift, f.AvgDelta,
		)
	}
	if ew.err != nil {
		return ew.err
	}
	return tw.Flush()
}

func writeBandDiffsTable(w io.Writer, diffs []eval.BandDiff) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	ew := &errWriter{w: tw}
	ew.println("Band\tBaseN\tCandN\tWinRateDelta\tLiftDelta\tFlatROIDelta")
	for _, d := range diffs {
		ew.printf(
			"%s\t%d\t%d\t%+.3f\t%+.3f\t%+.3f\n",
			d.Band, d.BaselineDecided, d.CandidateDecided,
			d.WinRateDelta, d.LiftDelta, d.FlatROIDelta,
		)
	}
	if ew.err != nil {
		return ew.err
	}
	return tw.Flush()
}

func writeMigrationsTable(w io.Writer, migrations []eval.MigrationCell) error {
	if len(migrations) == 0 {
		_, err := fmt.Fprintln(w, "(none)")
		return err
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	ew := &errWriter{w: tw}
	ew.println("From\tTo\tTotal\tDecided\tW-L\tWinRate")
	for _, m := range migrations {
		ew.printf(
			"%s\t%s\t%d\t%d\t%d-%d\t%.3f\n",
			m.FromBand, m.ToBand, m.Total, m.Decided, m.Wins, m.Losses, m.WinRate,
		)
	}
	if ew.err != nil {
		return ew.err
	}
	return tw.Flush()
}

func writeSuiteText(w io.Writer, suite eval.SuiteReport) error {
	ew := &errWriter{w: w}
	ew.printf("Suite (active: %s)\n", suite.ActiveRubric)
	if len(suite.Reports) > 1 {
		ew.println("\nEngine Calibration Comparison:")
		if ew.err != nil {
			return ew.err
		}
		if err := writeEngineComparisonTable(w, suite.Reports); err != nil {
			return err
		}
	}
	if ew.err != nil {
		return ew.err
	}
	for i, r := range suite.Reports {
		if i > 0 || len(suite.Reports) > 1 {
			ew.println()
			if ew.err != nil {
				return ew.err
			}
		}
		if err := writeReportText(w, r); err != nil {
			return err
		}
	}
	for _, d := range suite.Diffs {
		ew.println()
		if ew.err != nil {
			return ew.err
		}
		cand := findReportByName(suite.Reports, d.CandidateName)
		base := findReportByName(suite.Reports, d.BaselineName)
		if err := writeDiffText(w, base, cand, d); err != nil {
			return err
		}
	}
	return nil
}

func findReportByName(reports []eval.Report, name string) eval.Report {
	for _, r := range reports {
		if r.RubricName == name {
			return r
		}
	}
	return eval.Report{}
}

func writeReportMarkdown(w io.Writer, r eval.Report) error {
	ew := &errWriter{w: w}
	ew.printf("## Grading Eval: %s\n\n", r.RubricName)
	writeCoverageMarkdown(ew, r.Coverage)
	writeReportSectionsMarkdown(ew, r)
	return ew.err
}

func writeSuiteMarkdown(w io.Writer, suite eval.SuiteReport) error {
	ew := &errWriter{w: w}
	ew.printf("## Grading Eval Suite (active: `%s`)\n\n", suite.ActiveRubric)
	writeCoverageMarkdown(ew, suite.Coverage)

	if len(suite.Reports) > 0 {
		ew.printf("\n### Engine Calibration Comparison\n\n")
		writeEngineComparisonMarkdown(ew, suite.Reports)
	}

	for _, r := range suite.Reports {
		ew.printf("\n### Rubric: `%s`\n\n", r.RubricName)
		writeReportSectionsMarkdown(ew, r)
	}

	for _, d := range suite.Diffs {
		ew.printf("\n### Diff: `%s` -> `%s`\n\n", d.BaselineName, d.CandidateName)
		writeDiffSectionsMarkdown(ew, d)
	}
	return ew.err
}

func writeDiffMarkdown(w io.Writer, base, cand eval.Report, diff eval.DiffReport) error {
	ew := &errWriter{w: w}
	ew.printf("## Grading Eval Comparison: `%s` -> `%s`\n\n", diff.BaselineName, diff.CandidateName)
	writeCoverageMarkdown(ew, base.Coverage)
	if hasCalibrationData(base.Calibration) || hasCalibrationData(cand.Calibration) {
		ew.printf("\n#### Engine Calibration Comparison\n\n")
		writeEngineComparisonMarkdown(ew, []eval.Report{base, cand})
	}
	ew.println()
	writeDiffSectionsMarkdown(ew, diff)
	ew.printf("\n### Candidate Factor Attribution (`%s`)\n\n", cand.RubricName)
	writeFactorsMarkdown(ew, cand.Factors)
	if len(cand.SlipFactors) > 0 {
		ew.printf("\n### Candidate Slip Factor Attribution (`%s`)\n\n", cand.RubricName)
		writeFactorsMarkdown(ew, cand.SlipFactors)
	}
	return ew.err
}

func writeCoverageMarkdown(ew *errWriter, c eval.DataCoverage) {
	ew.printf(
		"**Data Coverage:** legs=%d | gamelogs=%d | injuries=%d | nflverse=%d | nfelo=%d | odds=%d | "+
			"sharp=%d | l5_fallback=%d\n",
		c.TotalLegs,
		c.WithGameLogs,
		c.WithInjuries,
		c.WithNFLVerse,
		c.WithNfelo,
		c.WithOdds,
		c.WithSharp,
		c.WithL5Fallback,
	)
}

func writeReportSectionsMarkdown(ew *errWriter, r eval.Report) {
	ew.printf("\n#### Leg Calibration\n\n")
	writeBandsMarkdown(ew, r.LegBands)
	ew.printf(
		"\nMonotonic: `%v` (win=`%v`, lift=`%v`)\n",
		r.LegMonotonic, r.LegMonotonicWin, r.LegMonotonicLift,
	)

	if hasCalibrationData(r.Calibration) {
		ew.printf("\n#### Probability Calibration\n\n")
		writeCalibrationMarkdown(ew, r.Calibration)
	}

	if r.MarketExplanation.SampleCount > 0 {
		ew.printf("\n#### 1. Explaining Vegas (Match First)\n\n")
		writeMarketExplanationMarkdown(ew, r.MarketExplanation)
	}

	if r.MarketBeating.SampleCount > 0 {
		ew.printf("\n#### 2. Beating Vegas (Alpha Gate)\n\n")
		writeMarketBeatingMarkdown(ew, r.MarketBeating)
	}

	if hasBandTotals(r.SlipBands) {
		ew.printf("\n#### Slip Calibration\n\n")
		writeBandsMarkdown(ew, r.SlipBands)
	}

	ew.printf("\n#### Top Factor Attribution\n\n")
	writeFactorsMarkdown(ew, r.Factors)

	if len(r.SlipFactors) > 0 {
		ew.printf("\n#### Slip Factor Attribution\n\n")
		writeFactorsMarkdown(ew, r.SlipFactors)
	}
}

func writeDiffSectionsMarkdown(ew *errWriter, diff eval.DiffReport) {
	ew.printf("#### Leg Calibration Diff\n\n")
	writeBandDiffsMarkdown(ew, diff.LegBandDiffs)

	if hasBandDiffTotals(diff.SlipBandDiffs) {
		ew.printf("\n#### Slip Calibration Diff\n\n")
		writeBandDiffsMarkdown(ew, diff.SlipBandDiffs)
	}

	ew.printf("\n#### Leg Band Migrations\n\n")
	writeMigrationsMarkdown(ew, diff.Migrations)
}

func hasBandTotals(bands []eval.BandStats) bool {
	for _, b := range bands {
		if b.Total > 0 {
			return true
		}
	}
	return false
}

func hasBandDiffTotals(diffs []eval.BandDiff) bool {
	for _, d := range diffs {
		if d.BaselineDecided > 0 || d.CandidateDecided > 0 {
			return true
		}
	}
	return false
}

func writeBandsMarkdown(ew *errWriter, bands []eval.BandStats) {
	ew.println("| Band | Total | Decided | W-L-P | Win Rate | Implied | Lift | Flat ROI | Brier |")
	ew.println("| --- | ---: | ---: | --- | ---: | ---: | ---: | ---: | ---: |")
	for _, b := range bands {
		ew.printf(
			"| %s | %d | %d | %d-%d-%d | %.3f | %.3f | %+.3f | %+.3f | %.4f |\n",
			b.Band, b.Total, b.Decided, b.Wins, b.Losses, b.Pushes,
			b.WinRate, b.AvgImpliedProb, b.Lift, b.FlatROI, b.BrierScore,
		)
	}
}

func writeFactorsMarkdown(ew *errWriter, factors []eval.FactorStats) {
	if len(factors) == 0 {
		ew.println("_None_")
		return
	}
	const maxFactors = 15
	limit := min(len(factors), maxFactors)
	ew.println("| Rule | Label | Count | W-L | Win Rate | Implied | Lift | Avg Delta |")
	ew.println("| --- | --- | ---: | --- | ---: | ---: | ---: | ---: |")
	for _, f := range factors[:limit] {
		ew.printf(
			"| `%s` | %s | %d | %d-%d | %.3f | %.3f | %+.3f | %+.2f |\n",
			f.Rule, f.Label, f.Count, f.Wins, f.Losses,
			f.WinRate, f.AvgImpliedProb, f.Lift, f.AvgDelta,
		)
	}
}

func writeBandDiffsMarkdown(ew *errWriter, diffs []eval.BandDiff) {
	ew.println("| Band | Base N | Cand N | Win Rate Delta | Lift Delta | Flat ROI Delta |")
	ew.println("| --- | ---: | ---: | ---: | ---: | ---: |")
	for _, d := range diffs {
		ew.printf(
			"| %s | %d | %d | %+.3f | %+.3f | %+.3f |\n",
			d.Band, d.BaselineDecided, d.CandidateDecided,
			d.WinRateDelta, d.LiftDelta, d.FlatROIDelta,
		)
	}
}

func writeMigrationsMarkdown(ew *errWriter, migrations []eval.MigrationCell) {
	if len(migrations) == 0 {
		ew.println("_None_")
		return
	}
	ew.println("| From | To | Total | Decided | W-L | Win Rate |")
	ew.println("| --- | --- | ---: | ---: | --- | ---: |")
	for _, m := range migrations {
		ew.printf(
			"| %s | %s | %d | %d | %d-%d | %.3f |\n",
			m.FromBand, m.ToBand, m.Total, m.Decided, m.Wins, m.Losses, m.WinRate,
		)
	}
}

func hasCalibrationData(c eval.CalibrationMetrics) bool {
	return len(c.Reliability) > 0 || c.MarketLogLoss > 0 || c.MarketBrier > 0 || c.SGPCount > 0
}

func hasModelCalibration(c eval.CalibrationMetrics) bool {
	return len(c.Reliability) > 0
}

func writeCalibrationText(w io.Writer, c eval.CalibrationMetrics) error {
	ew := &errWriter{w: w}
	if hasModelCalibration(c) {
		ew.printf(
			"  LogLoss=%.4f  Brier=%.4f  ClasswiseECE=%.4f  BinSpread=%d/%d (ok=%v)\n",
			c.LogLoss, c.BrierScore, c.ClasswiseECE, c.PopulatedBins, c.TotalBins, c.BinSpreadOK,
		)
	} else {
		ew.println("  ModelProbabilities: n/a (no calibrated probabilities emitted)")
	}
	if c.MarketLogLoss > 0 || c.MarketBrier > 0 {
		ew.printf("  MarketBaseline: LogLoss=%.4f  Brier=%.4f\n", c.MarketLogLoss, c.MarketBrier)
	}
	if c.SGPCount > 0 {
		ew.printf(
			"  SGP Joint (n=%d): CopulaLogLoss=%.4f  NaiveIndepLogLoss=%.4f\n",
			c.SGPCount, c.SGPLogLoss, c.NaiveIndepLogLoss,
		)
	}
	if ew.err != nil {
		return ew.err
	}
	if len(c.Reliability) == 0 {
		return nil
	}

	ew.println("\nReliability Bins:")
	if ew.err != nil {
		return ew.err
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	tew := &errWriter{w: tw}
	tew.println("Bin\tCount\tMeanPred\tEmpiricalHit")
	for _, b := range c.Reliability {
		tew.printf("%.2f-%.2f\t%d\t%.3f\t%.3f\n", b.BinLow, b.BinHigh, b.Count, b.MeanPredProb, b.EmpiricalHit)
	}
	if tew.err != nil {
		return tew.err
	}
	return tw.Flush()
}

func writeMarketExplanationText(w io.Writer, m eval.MarketExplanationMetrics) error {
	ew := &errWriter{w: w}
	ew.printf(
		"  Samples=%d  MAE=%.4f  RMSE=%.4f  KL=%.4f  PearsonR=%.4f  Within5pp=%.1f%%\n",
		m.SampleCount,
		m.MeanAbsError,
		m.RMSE,
		m.MeanKLDivergence,
		m.CorrelationR,
		m.Within5ppRate*100,
	)
	return ew.err
}

func writeMarketBeatingText(w io.Writer, m eval.MarketBeatingMetrics) error {
	ew := &errWriter{w: w}
	gateStatus := "FAIL"
	if m.AlphaGatePassed {
		gateStatus = "PASS"
	}
	ew.printf(
		"  Samples=%d  RawLL=%.4f  BlendedLL=%.4f  MarketLL=%.4f  LogLossSkill=%+.4f  BrierSkill=%+.4f  FlatROI=%+.2f%%  AlphaGate=%s\n",
		m.SampleCount,
		m.RawModelLogLoss,
		m.BlendedLogLoss,
		m.MarketLogLoss,
		m.LogLossSkill,
		m.BrierSkill,
		m.FlatROI*100,
		gateStatus,
	)
	return ew.err
}

func writeEngineComparisonTable(w io.Writer, reports []eval.Report) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	ew := &errWriter{w: tw}
	ew.println(
		"Engine\tLogLoss\tBrier\tClasswiseECE\tBinSpread\t" +
			"MarketLL\tMarketBrier\tSGPLogLoss\tNaiveSGPLL\tSGPCount",
	)
	for _, r := range reports {
		c := r.Calibration
		ll := "-"
		brier := "-"
		ece := "-"
		if hasModelCalibration(c) {
			ll = fmt.Sprintf("%.4f", c.LogLoss)
			brier = fmt.Sprintf("%.4f", c.BrierScore)
			ece = fmt.Sprintf("%.4f", c.ClasswiseECE)
		}
		spread := fmt.Sprintf("%d/%d (%v)", c.PopulatedBins, c.TotalBins, c.BinSpreadOK)
		mktLL := "-"
		mktBrier := "-"
		if c.MarketLogLoss > 0 || c.MarketBrier > 0 {
			mktLL = fmt.Sprintf("%.4f", c.MarketLogLoss)
			mktBrier = fmt.Sprintf("%.4f", c.MarketBrier)
		}
		sgpLL := "-"
		naiveLL := "-"
		if c.SGPCount > 0 {
			sgpLL = fmt.Sprintf("%.4f", c.SGPLogLoss)
			naiveLL = fmt.Sprintf("%.4f", c.NaiveIndepLogLoss)
		}
		ew.printf(
			"%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%d\n",
			r.RubricName, ll, brier, ece, spread, mktLL, mktBrier, sgpLL, naiveLL, c.SGPCount,
		)
	}
	if ew.err != nil {
		return ew.err
	}
	return tw.Flush()
}

func writeCalibrationMarkdown(ew *errWriter, c eval.CalibrationMetrics) {
	ew.println("| Metric | Value |")
	ew.println("| --- | ---: |")
	if hasModelCalibration(c) {
		ew.printf("| Log Loss | %.4f |\n", c.LogLoss)
		ew.printf("| Brier Score | %.4f |\n", c.BrierScore)
		ew.printf("| Classwise ECE | %.4f |\n", c.ClasswiseECE)
		ew.printf("| Bin Spread (>=80%%) | %d/%d (`%v`) |\n", c.PopulatedBins, c.TotalBins, c.BinSpreadOK)
	} else {
		ew.println("| Model Probabilities | _n/a_ |")
	}
	if c.MarketLogLoss > 0 || c.MarketBrier > 0 {
		ew.printf("| Market Log Loss | %.4f |\n", c.MarketLogLoss)
		ew.printf("| Market Brier | %.4f |\n", c.MarketBrier)
	}
	if c.SGPCount > 0 {
		ew.printf("| SGP Joint Log Loss (n=%d) | %.4f |\n", c.SGPCount, c.SGPLogLoss)
		ew.printf("| SGP Naive Indep Log Loss | %.4f |\n", c.NaiveIndepLogLoss)
	}
	if len(c.Reliability) > 0 {
		ew.printf("\n##### Reliability Bins\n\n")
		ew.println("| Bin | Count | Mean Pred | Empirical Hit |")
		ew.println("| --- | ---: | ---: | ---: |")
		for _, b := range c.Reliability {
			ew.printf(
				"| %.2f-%.2f | %d | %.3f | %.3f |\n",
				b.BinLow,
				b.BinHigh,
				b.Count,
				b.MeanPredProb,
				b.EmpiricalHit,
			)
		}
	}
}

func writeMarketExplanationMarkdown(ew *errWriter, m eval.MarketExplanationMetrics) {
	ew.println("| Metric | Value |")
	ew.println("| --- | ---: |")
	ew.printf("| Samples | %d |\n", m.SampleCount)
	ew.printf("| MAE (Raw vs. Market) | %.4f |\n", m.MeanAbsError)
	ew.printf("| RMSE | %.4f |\n", m.RMSE)
	ew.printf("| KL Divergence | %.4f |\n", m.MeanKLDivergence)
	ew.printf("| Pearson r | %.4f |\n", m.CorrelationR)
	ew.printf("| Within 5pp | %.1f%% |\n", m.Within5ppRate*100)
}

func writeMarketBeatingMarkdown(ew *errWriter, m eval.MarketBeatingMetrics) {
	gateStatus := "FAIL"
	if m.AlphaGatePassed {
		gateStatus = "PASS"
	}
	ew.println("| Metric | Value |")
	ew.println("| --- | ---: |")
	ew.printf("| Samples | %d |\n", m.SampleCount)
	ew.printf("| Raw Model Log Loss | %.4f |\n", m.RawModelLogLoss)
	ew.printf("| Blended Log Loss | %.4f |\n", m.BlendedLogLoss)
	ew.printf("| Market Log Loss | %.4f |\n", m.MarketLogLoss)
	ew.printf("| Log Loss Skill | %+.4f |\n", m.LogLossSkill)
	ew.printf("| Brier Skill | %+.4f |\n", m.BrierSkill)
	ew.printf("| Flat ROI | %+.2f%% |\n", m.FlatROI*100)
	ew.printf("| Alpha Gate (n>=250, LL Skill>0, ROI>0) | %s |\n", gateStatus)
}

func writeEngineComparisonMarkdown(ew *errWriter, reports []eval.Report) {
	ew.println(
		"| Engine | Log Loss | Brier | Classwise ECE | Bin Spread | Market LL | Market Brier | SGP LL | Naive SGP LL | SGPs |",
	)
	ew.println("| --- | ---: | ---: | ---: | --- | ---: | ---: | ---: | ---: | ---: |")
	for _, r := range reports {
		c := r.Calibration
		ll := "–"
		brier := "–"
		ece := "–"
		if hasModelCalibration(c) {
			ll = fmt.Sprintf("%.4f", c.LogLoss)
			brier = fmt.Sprintf("%.4f", c.BrierScore)
			ece = fmt.Sprintf("%.4f", c.ClasswiseECE)
		}
		spread := fmt.Sprintf("%d/%d (`%v`)", c.PopulatedBins, c.TotalBins, c.BinSpreadOK)
		mktLL := "–"
		mktBrier := "–"
		if c.MarketLogLoss > 0 || c.MarketBrier > 0 {
			mktLL = fmt.Sprintf("%.4f", c.MarketLogLoss)
			mktBrier = fmt.Sprintf("%.4f", c.MarketBrier)
		}
		sgpLL := "–"
		naiveLL := "–"
		if c.SGPCount > 0 {
			sgpLL = fmt.Sprintf("%.4f", c.SGPLogLoss)
			naiveLL = fmt.Sprintf("%.4f", c.NaiveIndepLogLoss)
		}
		ew.printf(
			"| `%s` | %s | %s | %s | %s | %s | %s | %s | %s | %d |\n",
			r.RubricName, ll, brier, ece, spread, mktLL, mktBrier, sgpLL, naiveLL, c.SGPCount,
		)
	}
}
