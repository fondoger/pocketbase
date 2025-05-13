//go:build !no_default_driver

package core

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pocketbase/dbx"
	_ "modernc.org/sqlite"
)

func DefaultDBConnect(dbPath string) (*dbx.DB, error) {
	// Note: the busy_timeout pragma must be first because
	// the connection needs to be set to block on busy before WAL mode
	// is set in case it hasn't been already set by another connection.
	pragmas := "?_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)&_pragma=journal_size_limit(200000000)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(ON)&_pragma=temp_store(MEMORY)&_pragma=cache_size(-16000)"

	db, err := dbx.Open("sqlite", dbPath+pragmas)
	if err != nil {
		return nil, err
	}

	return db, nil
}

// Sample Connection String: "postgres://<username>:<password>@127.0.0.1:<port>"
func PostgresDBConnectFunc(connectionString string) DBConnectFunc {
	url, err := url.Parse(connectionString)
	if err != nil {
		panic(fmt.Errorf("invalid connection string: %s", err))
	}
	if url.Scheme != "postgres" {
		panic(fmt.Errorf("invalid connection string scheme: [%s], must be [postgres]", url.Scheme))
	}

	return func(dbPath string) (*dbx.DB, error) {
		// Case 1: Run in Unit Tests
		// - dbPath = "pb_test_randomstr_/data.db"
		// - dbFileName = "data.db"
		// - dbPrefix = "pb_test_randomstr_"
		// - dbName = "pb_test_randomstr_data"
		// Case 2: Run in Production
		// - dbPath = "data.db"
		// - dbFileName = "data.db"
		// - dbPrefix = ""
		// - dbName = "data"
		dbFileName := filepath.Base(dbPath)
		dbPrefix := strings.TrimSuffix(filepath.Dir(dbPath), ".") // for "example.db" it will be empty
		dbName := dbPrefix + strings.TrimSuffix(dbFileName, filepath.Ext(dbFileName))

		url.Path = "/" + dbName
		fmt.Println("dbPath:", dbPath, "dbName:", dbName)
		db, err := dbx.MustOpen("pgx", url.String())
		if err != nil {
			return nil, fmt.Errorf("failed to connect to Postgres: %s", err)
		}

		return db, nil
	}
}

func createGenerateUuidV7Function(db dbx.Builder) error {
	// Postgres:
	// 1. Create function
	funcDef := `
	-- Enable built-in pgcrypto extension to use gen_random_bytes function
	CREATE EXTENSION IF NOT EXISTS "pgcrypto";

	-- Adding "nocase" collation to be compatible with SQLite's built-in "nocase" collation
	CREATE COLLATION IF NOT EXISTS "nocase" (
		provider = icu,          -- Specify ICU as the provider
		locale = 'und-u-ks-level2', -- Undetermined locale, Unicode extension (-u-), collation strength (ks) level 2 (level2)
		deterministic = false    -- Case-insensitive collations are typically non-deterministic
	);

	-- Alias [hex] to encode(..., 'hex')
	CREATE OR REPLACE FUNCTION hex(data bytea)
	RETURNS text
	LANGUAGE SQL
	IMMUTABLE
	AS $$
	SELECT encode(data, 'hex')
	$$;

	-- Alias [randomblob] to gen_random_bytes(...)
	CREATE OR REPLACE FUNCTION randomblob(length integer)
	RETURNS bytea
	LANGUAGE SQL
	IMMUTABLE
	AS $$
	SELECT gen_random_bytes(length)
	$$;

	-- Create the uuid_generate_v7 function
	create or replace function uuid_generate_v7()
		returns uuid
		as $$
		begin
		-- use random v4 uuid as starting point (which has the same variant we need)
		-- then overlay timestamp
		-- then set version 7 by flipping the 2 and 1 bit in the version 4 string
		return encode(
			set_bit(
			set_bit(
				overlay(uuid_send(gen_random_uuid())
						placing substring(int8send(floor(extract(epoch from clock_timestamp()) * 1000)::bigint) from 3)
						from 1 for 6
				),
				52, 1
			),
			53, 1
			),
			'hex')::uuid;
		end
		$$
		language plpgsql
		volatile;
	
	-- Create json_valid function
	CREATE OR REPLACE FUNCTION json_valid(text) RETURNS boolean AS $$
	BEGIN
		PERFORM $1::jsonb;
		RETURN TRUE;
	EXCEPTION WHEN others THEN
		RETURN FALSE;
	END;
	$$ LANGUAGE plpgsql IMMUTABLE;
	`
	_, err := db.NewQuery(funcDef).Execute()
	return err
}
