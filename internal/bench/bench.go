// Package bench runs the two-engine benchmark: an equivalence gate first
// (Cedar == SpiceDB == expected on every ground-truth tuple — timing refuses
// to run on a dataset the engines disagree about), then timed measurement
// cells per model × engine × concurrency with percentile reporting.
package bench

import (
	"context"
	"fmt"
	"math/rand/v2"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	cedaradapter "github.com/ifundeasy/test-permission/internal/adapter/outbound/cedar"
	"github.com/ifundeasy/test-permission/internal/core/port"
	"github.com/ifundeasy/test-permission/internal/seed"
)

// VerifyResult summarizes the equivalence gate for one model × engine.
type VerifyResult struct {
	Model    string
	Engine   string
	Total    int
	Mismatch int
	Errors   int
}

// Verify pushes every tuple through the decider and compares to the expected
// ground-truth decision. Any error (including SpiceDB CONDITIONAL) is a failure.
func Verify(ctx context.Context, d port.Decider, model string, tuples []seed.Tuple, maxPrint int) VerifyResult {
	res := VerifyResult{Model: model, Engine: d.Name(), Total: len(tuples)}
	printed := 0
	for i, t := range tuples {
		out, err := d.Check(ctx, t.Request())
		if err != nil {
			res.Errors++
			if printed < maxPrint {
				fmt.Printf("  ERROR %s/%s tuple[%d] %s %s %s: %v\n", model, d.Name(), i, t.Principal, t.Action, t.Resource, err)
				printed++
			}
			continue
		}
		if string(out.Decision) != t.Expected {
			res.Mismatch++
			if printed < maxPrint {
				fmt.Printf("  MISMATCH %s/%s tuple[%d] %s %s %s: got %s want %s\n",
					model, d.Name(), i, t.Principal, t.Action, t.Resource, out.Decision, t.Expected)
				printed++
			}
		}
	}
	return res
}

// Cell is one measurement: model × engine × concurrency.
type Cell struct {
	Model       string  `json:"model"`
	Engine      string  `json:"engine"`
	Concurrency int     `json:"concurrency"`
	Checks      int     `json:"checks"`
	ErrorCount  int     `json:"errors"`
	Seconds     float64 `json:"seconds"`
	Throughput  float64 `json:"checks_per_sec"`
	MeanUS      float64 `json:"mean_us"`
	P50US       float64 `json:"p50_us"`
	P90US       float64 `json:"p90_us"`
	P95US       float64 `json:"p95_us"`
	P99US       float64 `json:"p99_us"`
	MaxUS       float64 `json:"max_us"`
}

// checkFn abstracts the thing being timed (full Check, or Cedar eval-only).
// The tuple index lets implementations use pre-computed per-tuple state without
// key lookups (and without ambiguity when tuples differ only in Context).
type checkFn func(ctx context.Context, i int, t seed.Tuple) error

// reservoirCap bounds per-worker latency samples (algorithm R). Percentiles
// come from the merged reservoirs; mean/max/count are exact. Preallocating the
// reservoir keeps slice growth and GC churn OUT of the timed window — on fast
// cells (cedar-eval runs >1M checks/s) unbounded sample slices would reallocate
// hundreds of MB mid-measurement and pollute tail latencies.
const reservoirCap = 1 << 16

// Measure warms up, then runs checks over the tuple set (round-robin) at the
// given concurrency for either n checks or the duration, whichever is set.
func Measure(ctx context.Context, model, engine string, tuples []seed.Tuple,
	fn checkFn, warmup, n int, duration time.Duration, concurrency int) Cell {

	// Warmup runs AT THE CELL'S CONCURRENCY and covers the FULL tuple set at
	// least once. Both matter for fairness: concurrent warmup actually opens
	// the connection-pool/stream capacity the timed section will use, and a
	// full pass touches every tuple's rows/relationships on BOTH engines so
	// no cell pays first-read cold costs inside its timed window.
	warmN := max(warmup, len(tuples))
	var wcounter atomic.Int64
	var wwg sync.WaitGroup
	for w := 0; w < concurrency; w++ {
		wwg.Add(1)
		go func() {
			defer wwg.Done()
			for {
				i := wcounter.Add(1) - 1
				if int(i) >= warmN {
					return
				}
				_ = fn(ctx, int(i)%len(tuples), tuples[int(i)%len(tuples)])
			}
		}()
	}
	wwg.Wait()

	type workerStats struct {
		samples []time.Duration // reservoir (preallocated)
		seen    int             // successful checks observed
		sum     time.Duration   // exact sum over ALL successful checks
		max     time.Duration
		errs    int
	}
	var (
		counter atomic.Int64
		wg      sync.WaitGroup
		stats   = make([]workerStats, concurrency)
	)
	deadline := time.Now().Add(duration)
	useCount := n > 0

	start := time.Now()
	for w := 0; w < concurrency; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			st := &stats[w]
			st.samples = make([]time.Duration, 0, reservoirCap)
			rng := rand.New(rand.NewPCG(uint64(w)+1, 0x9e3779b97f4a7c15))
			for {
				i := counter.Add(1) - 1
				if useCount {
					if int(i) >= n {
						return
					}
				} else if time.Now().After(deadline) {
					return
				}
				idx := int(i) % len(tuples)
				t0 := time.Now()
				err := fn(ctx, idx, tuples[idx])
				lat := time.Since(t0)
				if err != nil {
					// Errored checks are excluded from latency stats AND
					// throughput: fast failures must not flatter an engine.
					st.errs++
					continue
				}
				st.seen++
				st.sum += lat
				if lat > st.max {
					st.max = lat
				}
				if len(st.samples) < reservoirCap {
					st.samples = append(st.samples, lat)
				} else if j := rng.IntN(st.seen); j < reservoirCap {
					st.samples[j] = lat
				}
			}
		}(w)
	}
	wg.Wait()
	elapsed := time.Since(start)

	var (
		lats  []time.Duration
		total int
		errs  int
		sum   time.Duration
		maxL  time.Duration
	)
	for i := range stats {
		lats = append(lats, stats[i].samples...)
		total += stats[i].seen
		errs += stats[i].errs
		sum += stats[i].sum
		if stats[i].max > maxL {
			maxL = stats[i].max
		}
	}
	sort.Slice(lats, func(i, j int) bool { return lats[i] < lats[j] })

	cell := Cell{
		Model: model, Engine: engine, Concurrency: concurrency,
		Checks: total, ErrorCount: errs, Seconds: elapsed.Seconds(),
	}
	if total == 0 || len(lats) == 0 {
		return cell
	}
	us := func(d time.Duration) float64 { return float64(d.Nanoseconds()) / 1e3 }
	cell.Throughput = float64(total) / elapsed.Seconds()
	cell.MeanUS = us(sum / time.Duration(total))
	cell.P50US = us(pct(lats, 0.50))
	cell.P90US = us(pct(lats, 0.90))
	cell.P95US = us(pct(lats, 0.95))
	cell.P99US = us(pct(lats, 0.99))
	cell.MaxUS = us(maxL)
	return cell
}

func pct(sorted []time.Duration, q float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	i := int(q * float64(len(sorted)-1))
	return sorted[i]
}

// DeciderFn wraps a port.Decider as a checkFn.
func DeciderFn(d port.Decider) checkFn {
	return func(ctx context.Context, _ int, t seed.Tuple) error {
		_, err := d.Check(ctx, t.Request())
		return err
	}
}

// EvalOnlyFn pre-loads and pre-converts every tuple ONCE (index-aligned — no
// key ambiguity between tuples that differ only in Context), so the timed
// function is purely the engine's authorize call: no SQL, no map lookups, no
// domain→engine conversion inside the timed window.
func EvalOnlyFn(ctx context.Context, loader port.EntityLoader, d *cedaradapter.Decider, tuples []seed.Tuple) (checkFn, error) {
	prepared := make([]*cedaradapter.PreparedCheck, len(tuples))
	for i, t := range tuples {
		req := t.Request()
		ents, err := loader.LoadForCheck(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("preload tuple %d: %w", i, err)
		}
		p, err := d.Prepare(req, ents)
		if err != nil {
			return nil, fmt.Errorf("prepare tuple %d: %w", i, err)
		}
		prepared[i] = p
	}
	return func(_ context.Context, i int, _ seed.Tuple) error {
		_ = d.EvaluatePrepared(prepared[i])
		return nil
	}, nil
}
