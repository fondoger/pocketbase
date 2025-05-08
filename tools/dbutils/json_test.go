package dbutils_test

import (
	"testing"

	"github.com/pocketbase/pocketbase/tools/dbutils"
)

func TestJSONEach(t *testing.T) {
	result := dbutils.JSONEach("a.b")

	/* SQLite:
	expected := "json_each(CASE WHEN json_valid([[a.b]]) THEN [[a.b]] ELSE json_array([[a.b]]) END)"
	*/
	// PostgreSQL:
	expected := "json_array_elements_text(CASE WHEN [[a.b]] IS JSON OR json_valid([[a.b]]::text) THEN [[a.b]]::json ELSE json_array([[a.b]]) END)"

	if result != expected {
		t.Fatalf("Expected\n%v\ngot\n%v", expected, result)
	}
}

func TestJSONArrayLength(t *testing.T) {
	result := dbutils.JSONArrayLength("a.b")

	/* SQLite:
	expected := "json_array_length(CASE WHEN json_valid([[a.b]]) THEN [[a.b]] ELSE (CASE WHEN [[a.b]] = '' OR [[a.b]] IS NULL THEN json_array() ELSE json_array([[a.b]]) END) END)"
	*/
	// PostgreSQL:
	expected := "(CASE WHEN ([[a.b]] IS JSON OR JSON_VALID([[a.b]]::text)) AND json_typeof([[a.b]]::json) = 'array' THEN JSON_ARRAY_LENGTH([[a.b]]::json) ELSE 0 END)"

	if result != expected {
		t.Fatalf("Expected\n%v\ngot\n%v", expected, result)
	}
}

func TestJSONExtract(t *testing.T) {
	scenarios := []struct {
		name     string
		column   string
		path     string
		expected string
	}{
		{
			"empty path",
			"a.b",
			"",
			/* SQLite:
			"(CASE WHEN json_valid([[a.b]]) THEN JSON_EXTRACT([[a.b]], '$') ELSE JSON_EXTRACT(json_object('pb', [[a.b]]), '$.pb') END)",
			*/
			// PostgreSQL:
			`((CASE WHEN [[a.b]] IS JSON OR json_valid([[a.b]]::text) THEN JSON_QUERY([[a.b]]::json, '$') ELSE NULL END) #>> '{}')::text`,
		},
		{
			"starting with array index",
			"a.b",
			"[1].a[2]",
			/* SQLite:
			"(CASE WHEN json_valid([[a.b]]) THEN JSON_EXTRACT([[a.b]], '$[1].a[2]') ELSE JSON_EXTRACT(json_object('pb', [[a.b]]), '$.pb[1].a[2]') END)",
			*/
			// PostgreSQL:
			`((CASE WHEN [[a.b]] IS JSON OR json_valid([[a.b]]::text) THEN JSON_QUERY([[a.b]]::json, '$[1].a[2]') ELSE NULL END) #>> '{}')::text`,
		},
		{
			"starting with key",
			"a.b",
			"a.b[2].c",
			/* SQLite:
			"(CASE WHEN json_valid([[a.b]]) THEN JSON_EXTRACT([[a.b]], '$.a.b[2].c') ELSE JSON_EXTRACT(json_object('pb', [[a.b]]), '$.pb.a.b[2].c') END)",
			*/
			`((CASE WHEN [[a.b]] IS JSON OR json_valid([[a.b]]::text) THEN JSON_QUERY([[a.b]]::json, '$.a.b[2].c') ELSE NULL END) #>> '{}')::text`,
		},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			result := dbutils.JSONExtract(s.column, s.path)

			if result != s.expected {
				t.Fatalf("Expected\n%v\ngot\n%v", s.expected, result)
			}
		})
	}
}
