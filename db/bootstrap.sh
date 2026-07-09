#!/usr/bin/env bash
# Runs ONCE on a fresh Postgres volume (docker-entrypoint-initdb.d).
# Creates the two engine roles + schemas ONLY — never any data.
# All seeding happens later via `make seed` (Go seeder, batched).
#
# Isolation model (one Postgres server, two engines):
#   schema "cedar"   owned by role "cedar"   — the app data Cedar's PEP queries
#   schema "spicedb" owned by role "spicedb" — SpiceDB's datastore tables
# SpiceDB issues unqualified table names, so a per-role search_path routes its
# DDL/DML into its own schema (verified by the smoke test in the Makefile).
set -euo pipefail

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-SQL
	CREATE ROLE cedar   LOGIN PASSWORD '${CEDAR_DB_PASSWORD:-cedar}';
	CREATE ROLE spicedb LOGIN PASSWORD '${SPICEDB_DB_PASSWORD:-spicedb}';

	CREATE SCHEMA cedar   AUTHORIZATION cedar;
	CREATE SCHEMA spicedb AUTHORIZATION spicedb;

	-- Route each engine's unqualified table names into its own schema.
	ALTER ROLE cedar   IN DATABASE ${POSTGRES_DB} SET search_path = cedar;
	ALTER ROLE spicedb IN DATABASE ${POSTGRES_DB} SET search_path = spicedb;

	-- Nothing may leak into public.
	REVOKE CREATE ON SCHEMA public FROM PUBLIC;
SQL
