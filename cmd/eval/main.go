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
	rubricName := fs.String("rubric", analyzer.RubricV1.Name, "built-in rubric name for single-rubric eval")
	baselineName := fs.String("baseline", "", "baseline built-in rubric name for comparison")
	candidateName := fs.String("candidate", "", "candidate built-in rubric name for comparison")
	candidateFile := fs.String("candidate-file", "", "path to candidate rubric JSON file for comparison")
	season := fs.Int("season", 0, "filter to season year (0 = all)")
	fromWeek := fs.Int("from-week", 0, "filter to week >= from-week (0 = unbounded)")
	toWeek := fs.Int("to-week", 0, "filter to week <= to-week (0 = unbounded)")
	format := fs.String("format", "text", "output format: text or json")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if strings.TrimSpace(*dbPath) == "" {
		return errors.New("--db is required")
	}
	if *format != "text" && *format != "json" {
		return fmt.Errorf("unsupported --format %q (want text or json)", *format)
	}

	database, err := db.OpenReadOnly(*dbPath)
	if err != nil {
		return err
	}
	defer func() { _ = database.Close() }()

	store := db.NewStore(database)
	defer func() { _ = store.Close() }()

	dataset, err := eval.LoadDataset(ctx, store, eval.ReplayFilter{
		Season:   *season,
		FromWeek: *fromWeek,
		ToWeek:   *toWeek,
	})
	if err != nil {
		return err
	}

	compareMode := *baselineName != "" || *candidateName != "" || *candidateFile != ""
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
		if *format == "json" {
			return writeJSON(stdout, report)
		}
		return writeReportText(stdout, report)
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

	baseReport := eval.Evaluate(dataset, baseRubric)
	candReport := eval.Evaluate(dataset, candRubric)
	diff := eval.Compare(baseReport, candReport)

	if *format == "json" {
		return writeJSON(stdout, diffJSONOutput{
			Baseline:  baseReport,
			Candidate: candReport,
			Diff:      diff,
		})
	}
	return writeDiffText(stdout, baseReport, candReport, diff)
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
	return writeBandsTable(w, r.SlipBands)
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
	return writeFactorsTable(w, cand.Factors)
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
