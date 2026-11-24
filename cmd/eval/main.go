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
	allRubrics := fs.Bool("all", false, "evaluate all built-in rubrics and emit suite calibration + diffs")
	rubricName := fs.String("rubric", analyzer.RubricV1.Name, "built-in rubric name for single-rubric eval")
	baselineName := fs.String("baseline", "", "baseline built-in rubric name for comparison")
	candidateName := fs.String("candidate", "", "candidate built-in rubric name for comparison")
	candidateFile := fs.String("candidate-file", "", "path to candidate rubric JSON file for comparison")
	season := fs.Int("season", 0, "filter to season year (0 = all)")
	fromWeek := fs.Int("from-week", 0, "filter to week >= from-week (0 = unbounded)")
	toWeek := fs.Int("to-week", 0, "filter to week <= to-week (0 = unbounded)")
	format := fs.String("format", "text", "output format: text, json, or markdown")

	if err := fs.Parse(args); err != nil {
		return err
	}

	hasDB := strings.TrimSpace(*dbPath) != ""
	hasDataset := strings.TrimSpace(*datasetPath) != ""
	if hasDB && hasDataset {
		return errors.New("--db and --dataset are mutually exclusive")
	}
	if !hasDB && !hasDataset {
		return errors.New("--db or --dataset is required")
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
		})
		if err != nil {
			return err
		}
	}

	if *allRubrics {
		suite := eval.EvaluateAll(dataset, analyzer.BuiltInRubrics())
		switch *format {
		case "json":
			return writeJSON(stdout, suite)
		case "markdown":
			return writeSuiteMarkdown(stdout, suite)
		default:
			return writeSuiteText(stdout, suite)
		}
	}

	if !compareMode {
		rubric, ok := analyzer.RubricByName(*rubricName)
		if !ok {
			return fmt.Errorf(
				"unknown rubric %q (available: %s)",
				*rubricName,
				strings.Join(analyzer.RubricNames(), ", "),
			)
		}
		report := eval.Evaluate(dataset, rubric)
		switch *format {
		case "json":
			return writeJSON(stdout, report)
		case "markdown":
			return writeReportMarkdown(stdout, report)
		default:
			return writeReportText(stdout, report)
		}
	}

	baseName := *baselineName
	if baseName == "" {
		baseName = analyzer.RubricV1.Name
	}
	baseRubric, ok := analyzer.RubricByName(baseName)
	if !ok {
		return fmt.Errorf(
			"unknown baseline rubric %q (available: %s)",
			baseName,
			strings.Join(analyzer.RubricNames(), ", "),
		)
	}

	candRubric, err := loadCandidateRubric(*candidateName, *candidateFile)
	if err != nil {
		return err
	}

	baseReport, candReport, diff := eval.CompareRubrics(dataset, baseRubric, candRubric)

	switch *format {
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

func loadCandidateRubric(name, file string) (analyzer.Rubric, error) {
	if file != "" {
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
	if name == "" {
		return analyzer.Rubric{}, errors.New("--candidate or --candidate-file is required when --baseline is set")
	}
	r, ok := analyzer.RubricByName(name)
	if !ok {
		return analyzer.Rubric{}, fmt.Errorf(
			"unknown candidate rubric %q (available: %s)",
			name,
			strings.Join(analyzer.RubricNames(), ", "),
		)
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

	ew.println("\nFactor Attribution:")
	if ew.err != nil {
		return ew.err
	}
	if err := writeFactorsTable(w, r.Factors); err != nil {
		return err
	}

	ew.println("\nSlip Calibration:")
	if ew.err != nil {
		return ew.err
	}
	if err := writeBandsTable(w, r.SlipBands); err != nil {
		return err
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

	ew.println("\nLeg Calibration Diff:")
	if ew.err != nil {
		return ew.err
	}
	if err := writeBandDiffsTable(w, diff.LegBandDiffs); err != nil {
		return err
	}

	ew.println("\nSlip Calibration Diff:")
	if ew.err != nil {
		return ew.err
	}
	if err := writeBandDiffsTable(w, diff.SlipBandDiffs); err != nil {
		return err
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
		"Data Coverage: legs=%d gamelogs=%d injuries=%d nflverse=%d nfelo=%d odds=%d\n",
		c.TotalLegs, c.WithGameLogs, c.WithInjuries, c.WithNFLVerse, c.WithNfelo, c.WithOdds,
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
	if ew.err != nil {
		return ew.err
	}
	for i, r := range suite.Reports {
		if i > 0 {
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
		"**Data Coverage:** legs=%d | gamelogs=%d | injuries=%d | nflverse=%d | nfelo=%d | odds=%d\n",
		c.TotalLegs, c.WithGameLogs, c.WithInjuries, c.WithNFLVerse, c.WithNfelo, c.WithOdds,
	)
}

func writeReportSectionsMarkdown(ew *errWriter, r eval.Report) {
	ew.printf("\n#### Leg Calibration\n\n")
	writeBandsMarkdown(ew, r.LegBands)
	ew.printf(
		"\nMonotonic: `%v` (win=`%v`, lift=`%v`)\n",
		r.LegMonotonic, r.LegMonotonicWin, r.LegMonotonicLift,
	)

	ew.printf("\n#### Slip Calibration\n\n")
	writeBandsMarkdown(ew, r.SlipBands)

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

	ew.printf("\n#### Slip Calibration Diff\n\n")
	writeBandDiffsMarkdown(ew, diff.SlipBandDiffs)

	ew.printf("\n#### Leg Band Migrations\n\n")
	writeMigrationsMarkdown(ew, diff.Migrations)
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
