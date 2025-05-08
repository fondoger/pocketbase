package dbutils

import (
	"fmt"
	"strings"
)

// TODO: replace json with `jsonb` everywhere in the codebase
// TODO: Use PostgreSQL's native JSON functions instead of manually simulate JSON functions like SQLite.

// JSONEach returns JSON_EACH SQLite string expression with
// some normalizations for non-json columns.
func JSONEach(column string) string {
	/* SQLite:
	return fmt.Sprintf(
		`json_each(CASE WHEN json_valid([[%s]]) THEN [[%s]] ELSE json_array([[%s]]) END)`,
		column, column, column,
	)
	*/
	// PostgreSQL:
	return fmt.Sprintf(
		// `json_array_elements_text(CASE WHEN [[%s]] IS JSON OR json_valid([[%s]]::text) THEN [[%s]]::json ELSE json_array([[%s]]) END)`,
		`json_array_elements_text(CASE WHEN [[%s]] IS JSON OR json_valid([[%s]]::text) THEN [[%s]]::json ELSE json_array([[%s]]) END)`,
		column, column, column, column,
	)
}

// JSONArrayLength returns JSON_ARRAY_LENGTH SQLite string expression
// with some normalizations for non-json columns.
//
// It works with both json and non-json column values.
//
// Returns 0 for empty string or NULL column values.
func JSONArrayLength(column string) string {
	/* SQLite:
	return fmt.Sprintf(
		`json_array_length(CASE WHEN json_valid([[%s]]) THEN [[%s]] ELSE (CASE WHEN [[%s]] = '' OR [[%s]] IS NULL THEN json_array() ELSE json_array([[%s]]) END) END)`,
		column, column, column, column, column,
	)
	*/
	// PostgreSQL:
	return fmt.Sprintf(
		`(CASE WHEN ([[%s]] IS JSON OR JSON_VALID([[%s]]::text)) AND json_typeof([[%s]]::json) = 'array' THEN JSON_ARRAY_LENGTH([[%s]]::json) ELSE 0 END)`,
		column, column, column, column,
	)
}

// JSONExtract returns a JSON_EXTRACT SQLite string expression with
// some normalizations for non-json columns.
func JSONExtract(column string, path string) string {
	// prefix the path with dot if it is not starting with array notation
	if path != "" && !strings.HasPrefix(path, "[") {
		path = "." + path
	}

	/* SQLite:
	return fmt.Sprintf(
		// note: the extra object wrapping is needed to workaround the cases where a json_extract is used with non-json columns.
		"(CASE WHEN json_valid([[%s]]) THEN JSON_EXTRACT([[%s]], '$%s') ELSE JSON_EXTRACT(json_object('pb', [[%s]]), '$.pb%s') END)",
		column,
		column,
		path,
		column,
		path,
	)
	*/

	// PostgreSQL:
	// Using `json_value::text` will get a string with double quotes. Using `json_value #>> '{}'` to get string content instead.
	// Adding `::text` at the end as a hint to `typeAwareJoin` to convert the other value to text while comparing the data (only if the other type is not determined).
	return fmt.Sprintf(
		`((CASE WHEN [[%s]] IS JSON OR json_valid([[%s]]::text) THEN JSON_QUERY([[%s]]::json, '$%s') ELSE NULL END) #>> '{}')::text`,
		column,
		column,
		column,
		path,
	)
}
