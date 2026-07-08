// Command authz-service is the composition root (the "wiring" of the hexagon):
// it loads config, constructs the driven adapters (Cedar engine + Postgres
// repository), injects them into the core use case, and exposes it over HTTP.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	cedaradapter "github.com/ifundeasy/test-permission/internal/adapter/outbound/cedar"
	pgadapter "github.com/ifundeasy/test-permission/internal/adapter/outbound/postgres"

	"github.com/ifundeasy/test-permission/internal/adapter/inbound/rest"
	"github.com/ifundeasy/test-permission/internal/config"
	"github.com/ifundeasy/test-permission/internal/core/service"
)

func main() {
	cfg := config.Load()

	// Driven adapter: Cedar policy engine (parse the policy document once).
	policyDoc, err := os.ReadFile(cfg.PolicyFile)
	if err != nil {
		log.Fatalf("read policy file %q: %v", cfg.PolicyFile, err)
	}
	engine, err := cedaradapter.NewEngine(cfg.PolicyFile, policyDoc)
	if err != nil {
		log.Fatalf("init cedar engine: %v", err)
	}

	// Driven adapter: Postgres entity repository.
	ctx := context.Background()
	pool, err := pgadapter.NewPool(ctx, cfg.DatabaseURL())
	if err != nil {
		log.Fatalf("connect postgres: %v", err)
	}
	defer pool.Close()
	repo := pgadapter.NewRepository(pool)

	// Core use case + driving adapter.
	authz := service.NewAuthorizer(repo, engine)
	router := rest.NewRouter(rest.NewHandler(authz))

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("authz-service listening on :%s", cfg.Port)
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
