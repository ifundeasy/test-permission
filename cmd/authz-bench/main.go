// Command authz-bench runs the Cedar vs SpiceDB benchmark.
//
//	-mode verify : equivalence gate — every ground-truth tuple must get the
//	               SAME decision from Cedar, SpiceDB, and the generator's
//	               expectation. Exits non-zero on any mismatch.
//	-mode run    : timed cells per model × engine × concurrency:
//	               cedar (end-to-end: Postgres fetch + eval), cedar-eval
//	               (engine only, entities pre-fetched), spicedb-fully_consistent
//	               (cache bypassed), spicedb-minimize_latency (production posture).
//	               Results → console table + CSV + JSON under -out.
package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	cedaradapter "github.com/ifundeasy/test-permission/internal/adapter/outbound/cedar"
	pgadapter "github.com/ifundeasy/test-permission/internal/adapter/outbound/postgres"
	spicedbadapter "github.com/ifundeasy/test-permission/internal/adapter/outbound/spicedb"
	"github.com/ifundeasy/test-permission/internal/bench"
	"github.com/ifundeasy/test-permission/internal/config"
	"github.com/ifundeasy/test-permission/internal/core/port"
	"github.com/ifundeasy/test-permission/internal/seed"
)

var models = []string{"rbac", "rebac", "abac", "pbac", "acl"}

func main() {
	var (
		mode        = flag.String("mode", "verify", "verify | run")
		tuplesDir   = flag.String("tuples", "bench/tuples", "ground-truth tuple dir (from authz-seed)")
		policiesDir = flag.String("policies", "policies", "cedar policy dir")
		outDir      = flag.String("out", "bench/results", "results output dir (run mode)")
		warmup      = flag.Int("warmup", 5000, "warmup checks per cell")
		n           = flag.Int("n", 0, "checks per cell (0 = duration-based)")
		durationS   = flag.Duration("duration", 30*time.Second, "duration per cell when -n=0")
		concurrency = flag.String("concurrency", "1,8,32", "comma-separated concurrency levels")
		modelsFlag  = flag.String("models", strings.Join(models, ","), "models to run")
	)
	flag.Parse()

	cfg := config.Load()
	ctx := context.Background()

	// Cedar decider (embedded engine + Postgres loader).
	docs := map[string][]byte{}
	paths, _ := filepath.Glob(filepath.Join(*policiesDir, "*.cedar"))
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			log.Fatalf("read policy: %v", err)
		}
		docs[filepath.Base(p)] = b
	}
	pool, err := pgadapter.NewPool(ctx, cfg.CedarDatabaseURL())
	if err != nil {
		log.Fatalf("connect postgres: %v", err)
	}
	defer pool.Close()
	loader := pgadapter.NewLoader(pool)
	cedarDecider, err := cedaradapter.NewDecider(docs, loader)
	if err != nil {
		log.Fatalf("cedar decider: %v", err)
	}

	// SpiceDB deciders (both consistency modes).
	spicedbFull, err := spicedbadapter.NewDecider(cfg.SpiceDBEndpoint, cfg.SpiceDBKey, spicedbadapter.FullyConsistent)
	if err != nil {
		log.Fatalf("spicedb decider: %v", err)
	}
	spicedbMinLat, err := spicedbadapter.NewDecider(cfg.SpiceDBEndpoint, cfg.SpiceDBKey, spicedbadapter.MinimizeLatency)
	if err != nil {
		log.Fatalf("spicedb decider: %v", err)
	}

	runModels := strings.Split(*modelsFlag, ",")
	switch *mode {
	case "verify":
		verify(ctx, cedarDecider, spicedbFull, *tuplesDir, runModels)
	case "run":
		run(ctx, cedarDecider, loader, spicedbFull, spicedbMinLat,
			*tuplesDir, *outDir, runModels, *warmup, *n, *durationS, parseInts(*concurrency))
	default:
		log.Fatalf("unknown mode %q", *mode)
	}
}

// verify is the equivalence gate: Cedar == SpiceDB == expected, per tuple.
func verify(ctx context.Context, cedar, spicedb port.Decider, tuplesDir string, runModels []string) {
	failed := false
	fmt.Println("=== equivalence gate: Cedar == SpiceDB == expected ===")
	for _, model := range runModels {
		tuples, err := seed.LoadTuples(tuplesDir, model)
		if err != nil {
			log.Fatalf("load tuples %s: %v (run `make seed` first)", model, err)
		}
		for _, d := range []port.Decider{cedar, spicedb} {
			r := bench.Verify(ctx, d, model, tuples, 10)
			status := "OK"
			if r.Mismatch > 0 || r.Errors > 0 {
				status = "FAIL"
				failed = true
			}
			fmt.Printf("%-5s %-26s %5d tuples  %4d mismatch  %4d errors  %s\n",
				model, d.Name(), r.Total, r.Mismatch, r.Errors, status)
		}
	}
	if failed {
		fmt.Println("EQUIVALENCE GATE FAILED — the engines disagree; do not benchmark this dataset.")
		os.Exit(1)
	}
	fmt.Println("equivalence gate PASSED — engines agree on every tuple.")
}

func run(ctx context.Context, cedarDecider *cedaradapter.Decider, loader port.EntityLoader,
	spicedbFull, spicedbMinLat port.Decider, tuplesDir, outDir string,
	runModels []string, warmup, n int, duration time.Duration, levels []int) {

	var cells []bench.Cell
	for _, model := range runModels {
		tuples, err := seed.LoadTuples(tuplesDir, model)
		if err != nil {
			log.Fatalf("load tuples %s: %v", model, err)
		}
		evalOnly, err := bench.EvalOnlyFn(ctx, loader, cedarDecider, tuples)
		if err != nil {
			log.Fatalf("preload %s: %v", model, err)
		}
		engines := []struct {
			name string
			fn   func(context.Context, int, seed.Tuple) error
		}{
			{"cedar", bench.DeciderFn(cedarDecider)},
			{"cedar-eval", evalOnly},
			{spicedbFull.Name(), bench.DeciderFn(spicedbFull)},
			{spicedbMinLat.Name(), bench.DeciderFn(spicedbMinLat)},
		}
		for _, eng := range engines {
			for _, c := range levels {
				fmt.Printf("measuring %s × %s × c=%d …\n", model, eng.name, c)
				cell := bench.Measure(ctx, model, eng.name, tuples, eng.fn, warmup, n, duration, c)
				cells = append(cells, cell)
				fmt.Printf("  %d checks in %.1fs  %.0f/s  p50=%.0fµs p90=%.0fµs p99=%.0fµs errs=%d\n",
					cell.Checks, cell.Seconds, cell.Throughput, cell.P50US, cell.P90US, cell.P99US, cell.ErrorCount)
			}
		}
	}
	printTable(cells)
	if err := writeResults(outDir, cells); err != nil {
		log.Fatalf("write results: %v", err)
	}
}

func printTable(cells []bench.Cell) {
	fmt.Println("\n=== results ===")
	fmt.Printf("%-6s %-26s %4s %9s %10s %9s %9s %9s %9s %6s\n",
		"model", "engine", "conc", "checks", "thr(/s)", "avg(µs)", "p50(µs)", "p90(µs)", "p99(µs)", "errs")
	for _, c := range cells {
		fmt.Printf("%-6s %-26s %4d %9d %10.0f %9.0f %9.0f %9.0f %9.0f %6d\n",
			c.Model, c.Engine, c.Concurrency, c.Checks, c.Throughput, c.MeanUS, c.P50US, c.P90US, c.P99US, c.ErrorCount)
	}
}

func writeResults(dir string, cells []bench.Cell) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	stamp := time.Now().Format("20060102-150405")

	jf, err := os.Create(filepath.Join(dir, stamp+".json"))
	if err != nil {
		return err
	}
	defer jf.Close()
	enc := json.NewEncoder(jf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(cells); err != nil {
		return err
	}

	cf, err := os.Create(filepath.Join(dir, stamp+".csv"))
	if err != nil {
		return err
	}
	defer cf.Close()
	w := csv.NewWriter(cf)
	_ = w.Write([]string{"model", "engine", "concurrency", "checks", "errors", "seconds", "checks_per_sec", "mean_us", "p50_us", "p90_us", "p95_us", "p99_us", "max_us"})
	for _, c := range cells {
		_ = w.Write([]string{
			c.Model, c.Engine, strconv.Itoa(c.Concurrency), strconv.Itoa(c.Checks), strconv.Itoa(c.ErrorCount),
			fmt.Sprintf("%.2f", c.Seconds), fmt.Sprintf("%.1f", c.Throughput), fmt.Sprintf("%.1f", c.MeanUS),
			fmt.Sprintf("%.1f", c.P50US), fmt.Sprintf("%.1f", c.P90US), fmt.Sprintf("%.1f", c.P95US), fmt.Sprintf("%.1f", c.P99US), fmt.Sprintf("%.1f", c.MaxUS),
		})
	}
	w.Flush()
	fmt.Printf("results written: %s/%s.{json,csv}\n", dir, stamp)
	return w.Error()
}

func parseInts(s string) []int {
	var out []int
	for _, part := range strings.Split(s, ",") {
		v, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || v < 1 {
			log.Fatalf("bad concurrency %q", part)
		}
		out = append(out, v)
	}
	return out
}
