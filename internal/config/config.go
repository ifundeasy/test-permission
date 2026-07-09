// Package config loads runtime configuration from the environment. Local runs
// load .env via the Makefile; docker-compose injects the same vars via env_file.
package config

import (
	"fmt"
	"net/url"
	"os"
)

type Config struct {
	DBHost          string
	DBPort          string
	DBName          string
	CedarDBPassword string // role "cedar" → schema "cedar" (app data)
	SpiceDBEndpoint string
	SpiceDBKey      string
	Port            string
	PoliciesDir     string
}

// Load reads configuration, applying local-dev defaults that match
// .env.example and the compose network.
func Load() Config {
	return Config{
		DBHost:          env("DB_HOST", "postgres"),
		DBPort:          env("DB_PORT", "5432"),
		DBName:          env("DB_NAME", "authz"),
		CedarDBPassword: env("CEDAR_DB_PASSWORD", "cedar"),
		SpiceDBEndpoint: env("SPICEDB_ENDPOINT", "spicedb:50051"),
		SpiceDBKey:      env("SPICEDB_PRESHARED_KEY", "benchkey"),
		Port:            env("PORT", "8080"),
		PoliciesDir:     env("POLICIES_DIR", "policies"),
	}
}

// CedarDatabaseURL builds the DSN for the "cedar" role (search_path = cedar).
// Credentials are URL-encoded so special characters don't break the DSN.
//
// Pool sizing is explicit for benchmark fairness: pgxpool's default MaxConns is
// NumCPU, which silently caps below the bench's highest concurrency (32) and
// makes Cedar's high-concurrency cells measure client-side pool queueing
// instead of engine+fetch cost. 48 covers c=32 with slack while staying well
// under Postgres' default max_connections=100 alongside SpiceDB's own pool.
func (c Config) CedarDatabaseURL() string {
	return fmt.Sprintf("postgres://cedar:%s@%s:%s/%s?sslmode=disable&pool_max_conns=%s&pool_min_conns=%s",
		url.QueryEscape(c.CedarDBPassword), c.DBHost, c.DBPort, url.PathEscape(c.DBName),
		env("CEDAR_POOL_MAX_CONNS", "48"), env("CEDAR_POOL_MIN_CONNS", "8"))
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
