package migrations

import "github.com/pocketbase/dbx"

func createSQLiteEquivalentFunctions(db dbx.Builder) error {
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

	-- Create a json_query_or_null function that handles any types.
	CREATE OR REPLACE FUNCTION json_query_or_null(p_input jsonb, p_query text) RETURNS jsonb AS $$
		SELECT jsonb_path_query_first(p_input, p_query::jsonpath)
	$$ LANGUAGE sql IMMUTABLE;

	CREATE OR REPLACE FUNCTION json_query_or_null(p_input json, p_query text) RETURNS jsonb AS $$
		SELECT jsonb_path_query_first(p_input::jsonb, p_query::jsonpath)
	$$ LANGUAGE sql IMMUTABLE;

	-- Create a json_query_or_null function that handles any types.
	CREATE OR REPLACE FUNCTION json_query_or_null(p_input anyelement, p_query text) RETURNS jsonb AS $$
	BEGIN
		RETURN jsonb_path_query_first(p_input::text::jsonb, p_query::jsonpath);
	EXCEPTION WHEN others THEN
		RETURN NULL;
	END;
	$$ LANGUAGE plpgsql STABLE;

	-- SQLite-compatible date formatting for PocketBase filters and log queries.
	CREATE OR REPLACE FUNCTION strftime(p_format text, p_time_value text, VARIADIC p_modifiers text[])
	RETURNS text AS $$
	DECLARE
		v_time timestamptz;
		v_modifier text;
		v_format text;
	BEGIN
		v_time := CASE
			WHEN p_time_value IS NULL OR p_time_value = '' OR lower(p_time_value) = 'now' THEN clock_timestamp()
			ELSE p_time_value::timestamptz
		END;

		FOREACH v_modifier IN ARRAY p_modifiers LOOP
			IF lower(v_modifier) = 'utc' THEN
				CONTINUE;
			ELSIF lower(v_modifier) = 'localtime' THEN
				v_time := v_time AT TIME ZONE current_setting('TIMEZONE');
			ELSE
				v_time := v_time + v_modifier::interval;
			END IF;
		END LOOP;

		v_format := replace(p_format, '%%', '__PB_PERCENT__');
		v_format := replace(v_format, '%Y', 'YYYY');
		v_format := replace(v_format, '%m', 'MM');
		v_format := replace(v_format, '%d', 'DD');
		v_format := replace(v_format, '%H', 'HH24');
		v_format := replace(v_format, '%M', 'MI');
		v_format := replace(v_format, '%S', 'SS');
		v_format := replace(v_format, '%f', 'MS');
		v_format := replace(v_format, '%j', 'DDD');
		v_format := replace(v_format, '%w', 'D');
		v_format := replace(v_format, '__PB_PERCENT__', '%');

		RETURN to_char(v_time AT TIME ZONE 'UTC', v_format);
	EXCEPTION WHEN others THEN
		RETURN NULL;
	END;
	$$ LANGUAGE plpgsql VOLATILE;

	CREATE OR REPLACE FUNCTION strftime(p_format text, p_time_value text)
	RETURNS text AS $$
		SELECT strftime(p_format, p_time_value, VARIADIC ARRAY[]::text[])
	$$ LANGUAGE sql VOLATILE;

	CREATE OR REPLACE FUNCTION strftime(p_format text)
	RETURNS text AS $$
		SELECT strftime(p_format, 'now', VARIADIC ARRAY[]::text[])
	$$ LANGUAGE sql VOLATILE;
	`
	_, err := db.NewQuery(funcDef).Execute()
	return err
}
