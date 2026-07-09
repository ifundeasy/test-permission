// Command authz-service is the benchmark facade (composition root): it builds
// both engine deciders — Cedar (embedded, loads entities from Postgres) and
// SpiceDB (gRPC, both consistency modes) — injects them into the core Router,
// and exposes POST /v1/authorize over HTTP.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	cedaradapter "github.com/ifundeasy/test-permission/internal/adapter/outbound/cedar"
	pgadapter "github.com/ifundeasy/test-permission/internal/adapter/outbound/postgres"
	spicedbadapter "github.com/ifundeasy/test-permission/internal/adapter/outbound/spicedb"

	"github.com/ifundeasy/test-permission/internal/adapter/inbound/rest"
	"github.com/ifundeasy/test-permission/internal/config"
	"github.com/ifundeasy/test-permission/internal/core/service"
)

func main() {
	cfg := config.Load()

	// Cedar: parse every policy file once, wire the Postgres entity loader.
	docs, err := readPolicies(cfg.PoliciesDir)
	if err != nil {
		log.Fatalf("read policies: %v", err)
	}
	ctx := context.Background()
	pool, err := pgadapter.NewPool(ctx, cfg.CedarDatabaseURL())
	if err != nil {
		log.Fatalf("connect postgres (cedar role): %v", err)
	}
	defer pool.Close()
	cedarDecider, err := cedaradapter.NewDecider(docs, pgadapter.NewLoader(pool))
	if err != nil {
		log.Fatalf("init cedar decider: %v", err)
	}

	// SpiceDB: one decider per consistency mode.
	spicedbMinLat, err := spicedbadapter.NewDecider(cfg.SpiceDBEndpoint, cfg.SpiceDBKey, spicedbadapter.MinimizeLatency)
	if err != nil {
		log.Fatalf("init spicedb decider: %v", err)
	}
	spicedbFull, err := spicedbadapter.NewDecider(cfg.SpiceDBEndpoint, cfg.SpiceDBKey, spicedbadapter.FullyConsistent)
	if err != nil {
		log.Fatalf("init spicedb decider: %v", err)
	}

	router := service.NewRouter(cedarDecider, spicedbMinLat, spicedbFull)
	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           rest.NewRouter(rest.NewHandler(router)),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("authz-service (benchmark facade) listening on :%s — engines: %v", cfg.Port, router.Engines())
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()

	// Graceful shutdown on SIGINT/SIGTERM.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}
	log.Println("stopped")
}

// readPolicies loads every *.cedar file in dir.
func readPolicies(dir string) (map[string][]byte, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*.cedar"))
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return nil, errors.New("no *.cedar policy files found in " + dir)
	}
	docs := make(map[string][]byte, len(paths))
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		docs[filepath.Base(p)] = b
	}
	return docs, nil
}
