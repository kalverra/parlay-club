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
		fmt.Fprintf(os.Stderr, "eval: %v\n", err)
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
			return fmt.Errorf("unknown rubric %q (available: %s)", *rubricName, strings.Join(analyzer.RubricNames(), ", "))
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
		return fmt.Errorf("unknown baseline rubric %q (available: %s)", baseName, strings.Join(analyzer.RubricNames(), ", "))
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
		raw, err := os.ReadFile(file)
		if err != nil {
			return analyzer.Rubric{}, fmt.Errorf("read candidate rubric %s: %w", file, err)
		}
		var r analyzer.Rubric
		if err := json.Unmarshal(raw, &r); err != nil {
			return analyzer.Rubric{}, fmt.Errorf("parse candidate rubric %s: %w", file, err)
		}
		if strings.TrimSpace(r.Name) == "" {
			r.Name = filepath.Base(file)
		}
		return r, nil
	}
	if name == "" {
		return analyzer.Rubric{}, errors.New("--candidate or --candidate-file is required when --baseline is set")
	}
	r, ok := analyzer.RubricByName(name)
	if !ok {
		return analyzer.Rubric{}, fmt.Errorf("unknown candidate rubric %q (available: %s)", name, strings.Join(analyzer.RubricNames(), ", "))
	}
	return r, nil
}

func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func writeReportText(w io.Writer, r eval.Report) error {
	fmt.Fprintf(w, "Rubric: %s\n", r.RubricName)
	writeCoverage(w, r.Coverage)

	fmt.Fprintln(w, "\nLeg Calibration:")
	writeBandsTable(w, r.LegBands)
	fmt.Fprintf(w, "Monotonic: %v (win=%v, lift=%v)\n", r.LegMonotonic, r.LegMonotonicWin, r.LegMonotonicLift)

	fmt.Fprintln(w, "\nFactor Attribution:")
	writeFactorsTable(w, r.Factors)

	fmt.Fprintln(w, "\nSlip Calibration:")
	writeBandsTable(w, r.SlipBands)
	return nil
}

func writeDiffText(w io.Writer, base, cand eval.Report, diff eval.DiffReport) error {
	fmt.Fprintf(w, "Comparison: %s -> %s\n", diff.BaselineName, diff.CandidateName)
	writeCoverage(w, base.Coverage)

	fmt.Fprintln(w, "\nLeg Calibration Diff:")
	writeBandDiffsTable(w, diff.LegBandDiffs)

	fmt.Fprintln(w, "\nSlip Calibration Diff:")
	writeBandDiffsTable(w, diff.SlipBandDiffs)

	fmt.Fprintln(w, "\nLeg Band Migrations:")
	writeMigrationsTable(w, diff.Migrations)

	fmt.Fprintf(w, "\nCandidate Factor Attribution (%s):\n", cand.RubricName)
	writeFactorsTable(w, cand.Factors)
	return nil
}

func writeCoverage(w io.Writer, c eval.DataCoverage) {
	fmt.Fprintf(
		w,
		"Data Coverage: legs=%d gamelogs=%d injuries=%d nflverse=%d nfelo=%d odds=%d\n",
		c.TotalLegs, c.WithGameLogs, c.WithInjuries, c.WithNFLVerse, c.WithNfelo, c.WithOdds,
	)
}

func writeBandsTable(w io.Writer, bands []eval.BandStats) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "Band\tTotal\tDecided\tW-L-P\tWinRate\tImplied\tLift\tFlatROI\tBrier")
	for _, b := range bands {
		fmt.Fprintf(
			tw,
			"%s\t%d\t%d\t%d-%d-%d\t%.3f\t%.3f\t%+.3f\t%+.3f\t%.4f\n",
			b.Band, b.Total, b.Decided, b.Wins, b.Losses, b.Pushes,
			b.WinRate, b.AvgImpliedProb, b.Lift, b.FlatROI, b.BrierScore,
		)
	}
	_ = tw.Flush()
}

func writeFactorsTable(w io.Writer, factors []eval.FactorStats) {
	if len(factors) == 0 {
		fmt.Fprintln(w, "(none)")
		return
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "Rule\tLabel\tCount\tW-L\tWinRate\tImplied\tLift\tAvgDelta")
	for _, f := range factors {
		fmt.Fprintf(
			tw,
			"%s\t%s\t%d\t%d-%d\t%.3f\t%.3f\t%+.3f\t%+.2f\n",
			f.Rule, f.Label, f.Count, f.Wins, f.Losses,
			f.WinRate, f.AvgImpliedProb, f.Lift, f.AvgDelta,
		)
	}
	_ = tw.Flush()
}

func writeBandDiffsTable(w io.Writer, diffs []eval.BandDiff) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "Band\tBaseN\tCandN\tWinRateDelta\tLiftDelta\tFlatROIDelta")
	for _, d := range diffs {
		fmt.Fprintf(
			tw,
			"%s\t%d\t%d\t%+.3f\t%+.3f\t%+.3f\n",
			d.Band, d.BaselineDecided, d.CandidateDecided,
			d.WinRateDelta, d.LiftDelta, d.FlatROIDelta,
		)
	}
	_ = tw.Flush()
}

func writeMigrationsTable(w io.Writer, migrations []eval.MigrationCell) {
	if len(migrations) == 0 {
		fmt.Fprintln(w, "(none)")
		return
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "From\tTo\tTotal\tDecided\tW-L\tWinRate")
	for _, m := range migrations {
		fmt.Fprintf(
			tw,
			"%s\t%s\t%d\t%d\t%d-%d\t%.3f\n",
			m.FromBand, m.ToBand, m.Total, m.Decided, m.Wins, m.Losses, m.WinRate,
		)
	}
	_ = tw.Flush()
}
