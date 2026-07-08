// Package config loads runtime configuration from the environment. Local dev
// loads .env via the Makefile; docker-compose injects the same vars via env_file.
package config

import (
	"fmt"
	"net/url"
	"os"
)

type Config struct {
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	Port       string
	PolicyFile string
}

// Load reads configuration, applying defaults that match .env.example and the
// compose network (DB_HOST defaults to the "postgres" service name).
func Load() Config {
	return Config{
		DBHost:     env("DB_HOST", "postgres"),
		DBPort:     env("DB_PORT", "5432"),
		DBUser:     env("DB_USER", "app"),
		DBPassword: env("DB_PASSWORD", "app"),
		DBName:     env("DB_NAME", "authz"),
		Port:       env("PORT", "8080"),
		PolicyFile: env("POLICY_FILE", "policies/policy.cedar"),
	}
}

// DatabaseURL builds a Postgres DSN, URL-encoding credentials so passwords with
// special characters do not break the connection string.
func (c Config) DatabaseURL() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		url.QueryEscape(c.DBUser),
		url.QueryEscape(c.DBPassword),
		c.DBHost,
		c.DBPort,
		url.PathEscape(c.DBName),
	)
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
