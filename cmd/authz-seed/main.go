// Command authz-seed generates the deterministic ERP benchmark dataset and
// writes it to the engines: Postgres schema "cedar" (Cedar's app data), SpiceDB
// relationships (its own datastore), and the ground-truth tuple files used by
// the equivalence gate and the benchmark. One canonical generator + fixed seed
// ⇒ both engines hold the identical logical dataset.
//
// Batching is 1000 records per write on BOTH engines (multi-row INSERT ON
// CONFLICT DO NOTHING; RetryableBulkImportRelationships with Touch), so re-runs
// are idempotent and phase checkpoints make seeding resumable.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	pgadapter "github.com/ifundeasy/test-permission/internal/adapter/outbound/postgres"
	"github.com/ifundeasy/test-permission/internal/catalog"
	"github.com/ifundeasy/test-permission/internal/config"
	"github.com/ifundeasy/test-permission/internal/seed"
)

func main() {
	var (
		engine    = flag.String("engine", "all", "cedar | spicedb | tuples | all")
		seedVal   = flag.Uint64("seed", 42, "PRNG seed (same seed ⇒ identical dataset)")
		scaleName = flag.String("scale", "full", "full (≥1M rows/model) | test (miniature)")
		batch     = flag.Int("batch", 1000, "records per write batch (both engines)")
		catPath   = flag.String("catalog", "catalog/services.json", "service catalog JSON")
		zedPath   = flag.String("zed", "schema/spicedb/schema.zed", "SpiceDB schema file")
		tuplesOut = flag.String("tuples-out", "bench/tuples", "ground-truth tuple output dir")
		resume    = flag.Bool("resume", true, "skip phases already checkpointed")
		wipe      = flag.Bool("wipe", false, "DESTRUCTIVE: wipe both engines' benchmark data first (required when changing -scale — IDs overlap across scales)")
	)
	flag.Parse()

	cfg := config.Load()
	scale := seed.FullScale()
	if *scaleName == "test" {
		scale = seed.TestScale()
	}

	cat, err := catalog.Load(*catPath)
	if err != nil {
		log.Fatalf("catalog: %v", err)
	}
	eps, pgs, cmps := cat.Counts()

	start := time.Now()
	fmt.Printf("authz-seed: engine=%s seed=%d scale=%s batch=%d catalog=(%d endpoints, %d pages, %d components)\n",
		*engine, *seedVal, *scaleName, *batch, eps, pgs, cmps)

	gen := seed.NewGenerator(*seedVal, scale, cat)
	fmt.Println("building basis (orgs, divisions, roles, accounts, personas)…")
	gen.BuildBasis()
	fmt.Printf("basis ready: %d orgs, %d divisions, %d roles, %d accounts, %d personas, %d registry resources\n",
		len(gen.Orgs), len(gen.Divisions), len(gen.Roles), len(gen.Accounts), len(gen.Personas), len(gen.Resources))

	ctx := context.Background()
	engines := strings.Split(*engine, ",")
	runAll := *engine == "all"

	// Fingerprint stored with checkpoints: resuming under different seed/scale
	// would silently mix datasets (IDs overlap across scales) — hard-refused.
	params := fmt.Sprintf("seed=%d scale=%s", *seedVal, *scaleName)

	if *wipe {
		if err := wipeAll(ctx, cfg, *tuplesOut); err != nil {
			log.Fatalf("wipe: %v", err)
		}
	}

	if runAll || contains(engines, "cedar") {
		if err := seedCedar(ctx, cfg, gen, *batch, *resume, params); err != nil {
			log.Fatalf("seed cedar: %v", err)
		}
	}
	if runAll || contains(engines, "spicedb") {
		if err := seedSpiceDB(ctx, cfg, gen, *batch, *resume, *zedPath, params); err != nil {
			log.Fatalf("seed spicedb: %v", err)
		}
	}
	if runAll || contains(engines, "tuples") {
		if err := sampleTuples(gen, scale, *tuplesOut); err != nil {
			log.Fatalf("sample tuples: %v", err)
		}
	}
	fmt.Printf("authz-seed finished in %s\n", time.Since(start).Truncate(time.Second))
}

func seedCedar(ctx context.Context, cfg config.Config, gen *seed.Generator, batch int, resume bool, params string) error {
	pool, err := pgadapter.NewPool(ctx, cfg.CedarDatabaseURL())
	if err != nil {
		return fmt.Errorf("connect postgres (cedar role): %w", err)
	}
	defer pool.Close()

	writer, err := seed.NewPostgresWriter(ctx, pool, batch, seed.NewProgress("cedar", 25*batch), params)
	if err != nil {
		return err
	}
	phases, err := pendingPhases(ctx, pool, "cedar", resume, params)
	if err != nil {
		return err
	}
	if len(phases) == 0 {
		fmt.Println("[cedar] all phases already checkpointed — nothing to do (use -resume=false to force)")
		return nil
	}
	return gen.Run(writer, phases)
}

func seedSpiceDB(ctx context.Context, cfg config.Config, gen *seed.Generator, batch int, resume bool, zedPath, params string) error {
	zed, err := os.ReadFile(zedPath)
	if err != nil {
		return fmt.Errorf("read zed schema: %w", err)
	}
	if err := seed.WriteSchema(ctx, cfg.SpiceDBEndpoint, cfg.SpiceDBKey, zed); err != nil {
		return err
	}
	fmt.Println("[spicedb] schema written")

	// checkpoints live in the shared Postgres (cedar schema) for both engines
	pool, err := pgadapter.NewPool(ctx, cfg.CedarDatabaseURL())
	if err != nil {
		return fmt.Errorf("connect postgres (checkpoints): %w", err)
	}
	defer pool.Close()

	writer, err := seed.NewSpiceDBWriter(ctx, cfg.SpiceDBEndpoint, cfg.SpiceDBKey, batch, seed.NewProgress("spicedb", 25*batch))
	if err != nil {
		return err
	}
	phases, err := pendingPhases(ctx, pool, "spicedb", resume, params)
	if err != nil {
		return err
	}
	if len(phases) == 0 {
		fmt.Println("[spicedb] all phases already checkpointed — nothing to do (use -resume=false to force)")
		return nil
	}
	checkpointed := &checkpointingSink{Sink: writer, pool: pool, engine: "spicedb", ctx: ctx, params: params}
	return gen.Run(checkpointed, phases)
}

func sampleTuples(gen *seed.Generator, scale seed.Scale, outDir string) error {
	fmt.Println("[tuples] sampling ground-truth tuples…")
	sampler := seed.NewSampler(gen, scale.TuplesPerModel)
	if err := gen.Run(sampler, seed.AllPhases); err != nil {
		return err
	}
	if err := sampler.WriteFiles(outDir); err != nil {
		return err
	}
	for _, m := range []string{"rbac", "rebac", "abac", "pbac", "acl"} {
		var allow, deny int
		for _, t := range sampler.Tuples[m] {
			if t.Expected == "allow" {
				allow++
			} else {
				deny++
			}
		}
		fmt.Printf("[tuples] %-5s %d tuples (%d allow / %d deny)\n", m, allow+deny, allow, deny)
	}
	fmt.Printf("[tuples] written to %s\n", outDir)
	return nil
}

// pendingPhases filters AllPhases down to those not yet checkpointed under the
// same seed/scale; checkpoints from different params abort with a -wipe demand.
func pendingPhases(ctx context.Context, pool *pgxpool.Pool, engine string, resume bool, params string) ([]string, error) {
	if !resume {
		return seed.AllPhases, nil
	}
	done, conflict, err := seed.DonePhases(ctx, pool, engine, params)
	if err != nil {
		return nil, fmt.Errorf("read checkpoints: %w", err)
	}
	if conflict != "" {
		return nil, fmt.Errorf("existing checkpoints were written with different params (%s ≠ %s) — "+
			"the datasets would silently mix (IDs overlap); rerun with -wipe", conflict, params)
	}
	var out []string
	for _, ph := range seed.AllPhases {
		if done[ph] {
			fmt.Printf("[%s] phase=%s already checkpointed — skipping\n", engine, ph)
			continue
		}
		out = append(out, ph)
	}
	return out, nil
}

// checkpointingSink marks phases complete in Postgres for sinks that have no
// database of their own (the SpiceDB writer).
type checkpointingSink struct {
	seed.Sink
	pool   *pgxpool.Pool
	engine string
	ctx    context.Context
	params string
}

func (c *checkpointingSink) EndPhase(phase string) error {
	if err := c.Sink.EndPhase(phase); err != nil {
		return err
	}
	_, err := c.pool.Exec(c.ctx,
		`INSERT INTO seed_checkpoints (engine, phase, params) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`,
		c.engine, phase, c.params)
	return err
}

// wipeAll clears the benchmark data on BOTH engines, the seed checkpoints, AND
// the ground-truth tuple files, so a different -seed/-scale can reseed from a
// clean slate (IDs overlap across scales). Only "table does not exist" (fresh
// database) is tolerated — any other TRUNCATE failure aborts loudly: a
// silently-skipped table would leave stale rows and diverge the engines.
func wipeAll(ctx context.Context, cfg config.Config, tuplesOut string) error {
	fmt.Println("[wipe] truncating cedar schema tables…")
	pool, err := pgadapter.NewPool(ctx, cfg.CedarDatabaseURL())
	if err != nil {
		return fmt.Errorf("connect postgres: %w", err)
	}
	defer pool.Close()
	tables := []string{
		"organizations", "divisions", "roles", "accounts", "personas",
		"endpoints", "pages", "components", "persona_roles", "role_grants",
		"folders", "rebac_documents", "unit_memberships", "abac_documents",
		"pbac_policies", "pbac_assignments", "purchase_orders",
		"acl_documents", "acl_entries", "seed_checkpoints",
	}
	for _, t := range tables {
		if _, err := pool.Exec(ctx, "TRUNCATE "+t); err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "42P01" { // undefined_table: fresh DB
				continue
			}
			return fmt.Errorf("truncate %s: %w", t, err)
		}
	}
	fmt.Println("[wipe] deleting spicedb relationships…")
	if err := seed.WipeRelationships(ctx, cfg.SpiceDBEndpoint, cfg.SpiceDBKey); err != nil {
		return err
	}
	fmt.Println("[wipe] removing stale ground-truth tuple files…")
	for _, m := range []string{"rbac", "rebac", "abac", "pbac", "acl"} {
		p := filepath.Join(tuplesOut, m+".json")
		if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove %s: %w", p, err)
		}
	}
	fmt.Println("[wipe] done")
	return nil
}

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}
