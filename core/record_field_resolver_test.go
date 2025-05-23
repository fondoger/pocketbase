package core_test

import (
	"encoding/json"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
	"github.com/pocketbase/pocketbase/tools/list"
	"github.com/pocketbase/pocketbase/tools/search"
	"github.com/pocketbase/pocketbase/tools/types"
)

func TestRecordFieldResolverAllowedFields(t *testing.T) {
	t.Parallel()

	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	collection, err := app.FindCollectionByNameOrId("demo1")
	if err != nil {
		t.Fatal(err)
	}

	r := core.NewRecordFieldResolver(app, collection, nil, false)

	fields := r.AllowedFields()
	if len(fields) != 8 {
		t.Fatalf("Expected %d original allowed fields, got %d", 8, len(fields))
	}

	// change the allowed fields
	newFields := []string{"a", "b", "c"}
	expected := slices.Clone(newFields)
	r.SetAllowedFields(newFields)

	// change the new fields to ensure that the slice was cloned
	newFields[2] = "d"

	fields = r.AllowedFields()
	if len(fields) != len(expected) {
		t.Fatalf("Expected %d changed allowed fields, got %d", len(expected), len(fields))
	}

	for i, v := range expected {
		if fields[i] != v {
			t.Errorf("[%d] Expected field %q", i, v)
		}
	}
}

func TestRecordFieldResolverAllowHiddenFields(t *testing.T) {
	t.Parallel()

	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	collection, err := app.FindCollectionByNameOrId("demo1")
	if err != nil {
		t.Fatal(err)
	}

	r := core.NewRecordFieldResolver(app, collection, nil, false)

	allowHiddenFields := r.AllowHiddenFields()
	if allowHiddenFields {
		t.Fatalf("Expected original allowHiddenFields %v, got %v", allowHiddenFields, !allowHiddenFields)
	}

	// change the flag
	expected := !allowHiddenFields
	r.SetAllowHiddenFields(expected)

	allowHiddenFields = r.AllowHiddenFields()
	if allowHiddenFields != expected {
		t.Fatalf("Expected changed allowHiddenFields %v, got %v", expected, allowHiddenFields)
	}
}

/*
Note:
Sample Regex to generate test data:
- (\{:|dataEach|mmdataEach|__sm|__mr|__ml)(\w{9}|\w{6}|\w{8})\b
- $1TEST
*/
func TestRecordFieldResolverUpdateQuery(t *testing.T) {
	t.Parallel()

	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	authRecord, err := app.FindRecordById("users", "4q1xlclmfloku33")
	if err != nil {
		t.Fatal(err)
	}

	requestInfo := &core.RequestInfo{
		Context: "ctx",
		Headers: map[string]string{
			"a": "123",
			"b": "456",
		},
		Query: map[string]string{
			"a": "", // to ensure that :isset returns true because the key exists
			"b": "123",
		},
		Body: map[string]any{
			"a":                  nil, // to ensure that :isset returns true because the key exists
			"b":                  123,
			"number":             10,
			"select_many":        []string{"optionA", "optionC"},
			"rel_one":            "test",
			"rel_many":           []string{"test1", "test2"},
			"file_one":           "test",
			"file_many":          []string{"test1", "test2", "test3"},
			"self_rel_one":       "test",
			"self_rel_many":      []string{"test1"},
			"rel_many_cascade":   []string{"test1", "test2"},
			"rel_one_cascade":    "test1",
			"rel_one_no_cascade": "test1",
		},
		Auth: authRecord,
	}

	scenarios := []struct {
		name               string
		collectionIdOrName string
		rule               string
		allowHiddenFields  bool
		expectQuery        string
	}{
		{
			"non relation field (with all default operators)",
			"demo4",
			"title = true || title != 'test' || title ~ 'test1' || title !~ '%test2' || title > 1 || title >= 2 || title < 3 || title <= 4",
			false,
			/* SQLite:
			"SELECT `demo4`.* FROM `demo4` WHERE ([[demo4.title]] = 1 OR [[demo4.title]] IS NOT {:TEST} OR [[demo4.title]] LIKE {:TEST} ESCAPE '\\' OR [[demo4.title]] NOT LIKE {:TEST} ESCAPE '\\' OR [[demo4.title]] > {:TEST} OR [[demo4.title]] >= {:TEST} OR [[demo4.title]] < {:TEST} OR [[demo4.title]] <= {:TEST})",
			*/
			// PostgreSQL:
			`SELECT "demo4".* FROM "demo4" WHERE ([[demo4.title]] = TRUE OR [[demo4.title]] != {:TEST} OR [[demo4.title]] LIKE {:TEST} ESCAPE '\' OR [[demo4.title]] NOT LIKE {:TEST} ESCAPE '\' OR [[demo4.title]]::numeric > {:TEST}::numeric OR [[demo4.title]]::numeric >= {:TEST}::numeric OR [[demo4.title]]::numeric < {:TEST}::numeric OR [[demo4.title]]::numeric <= {:TEST}::numeric)`,
		},
		{
			"non relation field (with all opt/any operators)",
			"demo4",
			"title ?= true || title ?!= 'test' || title ?~ 'test1' || title ?!~ '%test2' || title ?> 1 || title ?>= 2 || title ?< 3 || title ?<= 4",
			false,
			/* SQLite:
			"SELECT `demo4`.* FROM `demo4` WHERE ([[demo4.title]] = 1 OR [[demo4.title]] IS NOT {:TEST} OR [[demo4.title]] LIKE {:TEST} ESCAPE '\\' OR [[demo4.title]] NOT LIKE {:TEST} ESCAPE '\\' OR [[demo4.title]] > {:TEST} OR [[demo4.title]] >= {:TEST} OR [[demo4.title]] < {:TEST} OR [[demo4.title]] <= {:TEST})",
			*/
			// PostgreSQL:
			`SELECT "demo4".* FROM "demo4" WHERE ([[demo4.title]] = TRUE OR [[demo4.title]] != {:TEST} OR [[demo4.title]] LIKE {:TEST} ESCAPE '\' OR [[demo4.title]] NOT LIKE {:TEST} ESCAPE '\' OR [[demo4.title]]::numeric > {:TEST}::numeric OR [[demo4.title]]::numeric >= {:TEST}::numeric OR [[demo4.title]]::numeric < {:TEST}::numeric OR [[demo4.title]]::numeric <= {:TEST}::numeric)`,
		},
		{
			"single direct rel",
			"demo4",
			"self_rel_one > true",
			false,
			/* SQLite:
			"SELECT `demo4`.* FROM `demo4` WHERE [[demo4.self_rel_one]] > 1",
			*/
			// PostgreSQL:
			`SELECT "demo4".* FROM "demo4" WHERE [[demo4.self_rel_one]]::numeric > TRUE::numeric`,
		},
		{
			"single direct rel (with id)",
			"demo4",
			"self_rel_one.id > true", // shouldn't have join
			false,
			/* SQLite:
			"SELECT `demo4`.* FROM `demo4` WHERE [[demo4.self_rel_one]] > 1",
			*/
			// PostgreSQL:
			`SELECT "demo4".* FROM "demo4" WHERE [[demo4.self_rel_one]]::numeric > TRUE::numeric`,
		},
		{
			"single direct rel (with non-id field)",
			"demo4",
			"self_rel_one.created > true", // should have join
			false,
			/* SQLite:
			"SELECT DISTINCT `demo4`.* FROM `demo4` LEFT JOIN `demo4` `demo4_self_rel_one` ON [[demo4_self_rel_one.id]] = [[demo4.self_rel_one]] WHERE [[demo4_self_rel_one.created]] > 1",
			*/
			// PostgreSQL:
			`SELECT DISTINCT "demo4".* FROM "demo4" LEFT JOIN "demo4" "demo4_self_rel_one" ON [[demo4_self_rel_one.id]] = [[demo4.self_rel_one]] WHERE [[demo4_self_rel_one.created]]::numeric > TRUE::numeric`,
		},
		{
			"multiple direct rel",
			"demo4",
			"self_rel_many ?> true",
			false,
			/* SQLite:
			"SELECT `demo4`.* FROM `demo4` WHERE [[demo4.self_rel_many]] > 1",
			*/
			// PostgreSQL:
			`SELECT "demo4".* FROM "demo4" WHERE [[demo4.self_rel_many]]::numeric > TRUE::numeric`,
		},
		{
			"multiple direct rel (with id)",
			"demo4",
			"self_rel_many.id ?> true", // should have join
			false,
			/* SQLite:
			"SELECT DISTINCT `demo4`.* FROM `demo4` LEFT JOIN json_each(CASE WHEN iif(json_valid([[demo4.self_rel_many]]), json_type([[demo4.self_rel_many]])='array', FALSE) THEN [[demo4.self_rel_many]] ELSE json_array([[demo4.self_rel_many]]) END) `demo4_self_rel_many_je` LEFT JOIN `demo4` `demo4_self_rel_many` ON [[demo4_self_rel_many.id]] = [[demo4_self_rel_many_je.value]] WHERE [[demo4_self_rel_many.id]] > 1",
			*/
			// PostgreSQL:
			`SELECT DISTINCT "demo4".* FROM "demo4" LEFT JOIN jsonb_array_elements_text(CASE WHEN ([[demo4.self_rel_many]] IS JSON OR json_valid([[demo4.self_rel_many]]::text)) AND jsonb_typeof([[demo4.self_rel_many]]::jsonb) = 'array' THEN [[demo4.self_rel_many]]::jsonb ELSE jsonb_build_array([[demo4.self_rel_many]]) END) "demo4_self_rel_many_je" ON 1=1 LEFT JOIN "demo4" "demo4_self_rel_many" ON [[demo4_self_rel_many.id]] = [[demo4_self_rel_many_je.value]] WHERE [[demo4_self_rel_many.id]]::numeric > TRUE::numeric`,
		},
		{
			"nested single rel (self rel)",
			"demo4",
			"self_rel_one.title > true",
			false,
			/* SQLite:
			"SELECT DISTINCT `demo4`.* FROM `demo4` LEFT JOIN `demo4` `demo4_self_rel_one` ON [[demo4_self_rel_one.id]] = [[demo4.self_rel_one]] WHERE [[demo4_self_rel_one.title]] > 1",
			*/
			// PostgreSQL:
			`SELECT DISTINCT "demo4".* FROM "demo4" LEFT JOIN "demo4" "demo4_self_rel_one" ON [[demo4_self_rel_one.id]] = [[demo4.self_rel_one]] WHERE [[demo4_self_rel_one.title]]::numeric > TRUE::numeric`,
		},
		{
			"nested single rel (other collection)",
			"demo4",
			"rel_one_cascade.title > true",
			false,
			/* SQLite:
			"SELECT DISTINCT `demo4`.* FROM `demo4` LEFT JOIN `demo3` `demo4_rel_one_cascade` ON [[demo4_rel_one_cascade.id]] = [[demo4.rel_one_cascade]] WHERE [[demo4_rel_one_cascade.title]] > 1",
			*/
			// PostgreSQL:
			`SELECT DISTINCT "demo4".* FROM "demo4" LEFT JOIN "demo3" "demo4_rel_one_cascade" ON [[demo4_rel_one_cascade.id]] = [[demo4.rel_one_cascade]] WHERE [[demo4_rel_one_cascade.title]]::numeric > TRUE::numeric`,
		},
		{
			"non-relation field + single rel",
			"demo4",
			"title > true || self_rel_one.title > true",
			false,
			/* SQLite:
			"SELECT DISTINCT `demo4`.* FROM `demo4` LEFT JOIN `demo4` `demo4_self_rel_one` ON [[demo4_self_rel_one.id]] = [[demo4.self_rel_one]] WHERE ([[demo4.title]] > 1 OR [[demo4_self_rel_one.title]] > 1)",
			*/
			// PostgreSQL:
			`SELECT DISTINCT "demo4".* FROM "demo4" LEFT JOIN "demo4" "demo4_self_rel_one" ON [[demo4_self_rel_one.id]] = [[demo4.self_rel_one]] WHERE ([[demo4.title]]::numeric > TRUE::numeric OR [[demo4_self_rel_one.title]]::numeric > TRUE::numeric)`,
		},
		{
			"nested incomplete relations (opt/any operator)",
			"demo4",
			"self_rel_many.self_rel_one ?> true",
			false,
			/* SQLite:
			"SELECT DISTINCT `demo4`.* FROM `demo4` LEFT JOIN json_each(CASE WHEN json_valid([[demo4.self_rel_many]]) THEN [[demo4.self_rel_many]] ELSE json_array([[demo4.self_rel_many]]) END) `demo4_self_rel_many_je` LEFT JOIN `demo4` `demo4_self_rel_many` ON [[demo4_self_rel_many.id]] = [[demo4_self_rel_many_je.value]] WHERE [[demo4_self_rel_many.self_rel_one]] > 1",
			*/
			// PostgreSQL:
			`SELECT DISTINCT "demo4".* FROM "demo4" LEFT JOIN jsonb_array_elements_text(CASE WHEN ([[demo4.self_rel_many]] IS JSON OR json_valid([[demo4.self_rel_many]]::text)) AND jsonb_typeof([[demo4.self_rel_many]]::jsonb) = 'array' THEN [[demo4.self_rel_many]]::jsonb ELSE jsonb_build_array([[demo4.self_rel_many]]) END) "demo4_self_rel_many_je" ON 1=1 LEFT JOIN "demo4" "demo4_self_rel_many" ON [[demo4_self_rel_many.id]] = [[demo4_self_rel_many_je.value]] WHERE [[demo4_self_rel_many.self_rel_one]]::numeric > TRUE::numeric`,
		},
		{
			"nested incomplete relations (multi-match operator)",
			"demo4",
			"self_rel_many.self_rel_one > true",
			false,
			/* SQLite:
			"SELECT DISTINCT `demo4`.* FROM `demo4` LEFT JOIN json_each(CASE WHEN json_valid([[demo4.self_rel_many]]) THEN [[demo4.self_rel_many]] ELSE json_array([[demo4.self_rel_many]]) END) `demo4_self_rel_many_je` LEFT JOIN `demo4` `demo4_self_rel_many` ON [[demo4_self_rel_many.id]] = [[demo4_self_rel_many_je.value]] WHERE ((([[demo4_self_rel_many.self_rel_one]] > 1) AND (NOT EXISTS (SELECT 1 FROM (SELECT [[__mm_demo4_self_rel_many.self_rel_one]] as [[multiMatchValue]] FROM `demo4` `__mm_demo4` LEFT JOIN json_each(CASE WHEN json_valid([[__mm_demo4.self_rel_many]]) THEN [[__mm_demo4.self_rel_many]] ELSE json_array([[__mm_demo4.self_rel_many]]) END) `__mm_demo4_self_rel_many_je` LEFT JOIN `demo4` `__mm_demo4_self_rel_many` ON [[__mm_demo4_self_rel_many.id]] = [[__mm_demo4_self_rel_many_je.value]] WHERE `__mm_demo4`.`id` = `demo4`.`id`) {{__smTEST}} WHERE NOT ([[__smTEST.multiMatchValue]] > 1)))))",
			*/
			// PostgreSQL:
			`SELECT DISTINCT "demo4".* FROM "demo4" LEFT JOIN jsonb_array_elements_text(CASE WHEN ([[demo4.self_rel_many]] IS JSON OR json_valid([[demo4.self_rel_many]]::text)) AND jsonb_typeof([[demo4.self_rel_many]]::jsonb) = 'array' THEN [[demo4.self_rel_many]]::jsonb ELSE jsonb_build_array([[demo4.self_rel_many]]) END) "demo4_self_rel_many_je" ON 1=1 LEFT JOIN "demo4" "demo4_self_rel_many" ON [[demo4_self_rel_many.id]] = [[demo4_self_rel_many_je.value]] WHERE ((([[demo4_self_rel_many.self_rel_one]]::numeric > TRUE::numeric) AND (NOT EXISTS (SELECT 1 FROM (SELECT [[__mm_demo4_self_rel_many.self_rel_one]] as [[multiMatchValue]] FROM "demo4" "__mm_demo4" LEFT JOIN jsonb_array_elements_text(CASE WHEN ([[__mm_demo4.self_rel_many]] IS JSON OR json_valid([[__mm_demo4.self_rel_many]]::text)) AND jsonb_typeof([[__mm_demo4.self_rel_many]]::jsonb) = 'array' THEN [[__mm_demo4.self_rel_many]]::jsonb ELSE jsonb_build_array([[__mm_demo4.self_rel_many]]) END) "__mm_demo4_self_rel_many_je" ON 1=1 LEFT JOIN "demo4" "__mm_demo4_self_rel_many" ON [[__mm_demo4_self_rel_many.id]] = [[__mm_demo4_self_rel_many_je.value]] WHERE "__mm_demo4"."id" = "demo4"."id") {{__smTEST}} WHERE NOT ([[__smTEST.multiMatchValue]]::numeric > TRUE::numeric)))))`,
		},
		{
			"nested complete relations (opt/any operator)",
			"demo4",
			"self_rel_many.self_rel_one.title ?> true",
			false,
			/* SQLite:
			"SELECT DISTINCT `demo4`.* FROM `demo4` LEFT JOIN json_each(CASE WHEN json_valid([[demo4.self_rel_many]]) THEN [[demo4.self_rel_many]] ELSE json_array([[demo4.self_rel_many]]) END) `demo4_self_rel_many_je` LEFT JOIN `demo4` `demo4_self_rel_many` ON [[demo4_self_rel_many.id]] = [[demo4_self_rel_many_je.value]] LEFT JOIN `demo4` `demo4_self_rel_many_self_rel_one` ON [[demo4_self_rel_many_self_rel_one.id]] = [[demo4_self_rel_many.self_rel_one]] WHERE [[demo4_self_rel_many_self_rel_one.title]] > 1",
			*/
			// PostgreSQL:
			`SELECT DISTINCT "demo4".* FROM "demo4" LEFT JOIN jsonb_array_elements_text(CASE WHEN ([[demo4.self_rel_many]] IS JSON OR json_valid([[demo4.self_rel_many]]::text)) AND jsonb_typeof([[demo4.self_rel_many]]::jsonb) = 'array' THEN [[demo4.self_rel_many]]::jsonb ELSE jsonb_build_array([[demo4.self_rel_many]]) END) "demo4_self_rel_many_je" ON 1=1 LEFT JOIN "demo4" "demo4_self_rel_many" ON [[demo4_self_rel_many.id]] = [[demo4_self_rel_many_je.value]] LEFT JOIN "demo4" "demo4_self_rel_many_self_rel_one" ON [[demo4_self_rel_many_self_rel_one.id]] = [[demo4_self_rel_many.self_rel_one]] WHERE [[demo4_self_rel_many_self_rel_one.title]]::numeric > TRUE::numeric`,
		},
		{
			"nested complete relations (multi-match operator)",
			"demo4",
			"self_rel_many.self_rel_one.title > true",
			false,
			/* SQLite:
			"SELECT DISTINCT `demo4`.* FROM `demo4` LEFT JOIN json_each(CASE WHEN json_valid([[demo4.self_rel_many]]) THEN [[demo4.self_rel_many]] ELSE json_array([[demo4.self_rel_many]]) END) `demo4_self_rel_many_je` LEFT JOIN `demo4` `demo4_self_rel_many` ON [[demo4_self_rel_many.id]] = [[demo4_self_rel_many_je.value]] LEFT JOIN `demo4` `demo4_self_rel_many_self_rel_one` ON [[demo4_self_rel_many_self_rel_one.id]] = [[demo4_self_rel_many.self_rel_one]] WHERE ((([[demo4_self_rel_many_self_rel_one.title]] > 1) AND (NOT EXISTS (SELECT 1 FROM (SELECT [[__mm_demo4_self_rel_many_self_rel_one.title]] as [[multiMatchValue]] FROM `demo4` `__mm_demo4` LEFT JOIN json_each(CASE WHEN json_valid([[__mm_demo4.self_rel_many]]) THEN [[__mm_demo4.self_rel_many]] ELSE json_array([[__mm_demo4.self_rel_many]]) END) `__mm_demo4_self_rel_many_je` LEFT JOIN `demo4` `__mm_demo4_self_rel_many` ON [[__mm_demo4_self_rel_many.id]] = [[__mm_demo4_self_rel_many_je.value]] LEFT JOIN `demo4` `__mm_demo4_self_rel_many_self_rel_one` ON [[__mm_demo4_self_rel_many_self_rel_one.id]] = [[__mm_demo4_self_rel_many.self_rel_one]] WHERE `__mm_demo4`.`id` = `demo4`.`id`) {{__smTEST}} WHERE NOT ([[__smTEST.multiMatchValue]] > 1)))))",
			*/
			// PostgreSQL:
			`SELECT DISTINCT "demo4".* FROM "demo4" LEFT JOIN jsonb_array_elements_text(CASE WHEN ([[demo4.self_rel_many]] IS JSON OR json_valid([[demo4.self_rel_many]]::text)) AND jsonb_typeof([[demo4.self_rel_many]]::jsonb) = 'array' THEN [[demo4.self_rel_many]]::jsonb ELSE jsonb_build_array([[demo4.self_rel_many]]) END) "demo4_self_rel_many_je" ON 1=1 LEFT JOIN "demo4" "demo4_self_rel_many" ON [[demo4_self_rel_many.id]] = [[demo4_self_rel_many_je.value]] LEFT JOIN "demo4" "demo4_self_rel_many_self_rel_one" ON [[demo4_self_rel_many_self_rel_one.id]] = [[demo4_self_rel_many.self_rel_one]] WHERE ((([[demo4_self_rel_many_self_rel_one.title]]::numeric > TRUE::numeric) AND (NOT EXISTS (SELECT 1 FROM (SELECT [[__mm_demo4_self_rel_many_self_rel_one.title]] as [[multiMatchValue]] FROM "demo4" "__mm_demo4" LEFT JOIN jsonb_array_elements_text(CASE WHEN ([[__mm_demo4.self_rel_many]] IS JSON OR json_valid([[__mm_demo4.self_rel_many]]::text)) AND jsonb_typeof([[__mm_demo4.self_rel_many]]::jsonb) = 'array' THEN [[__mm_demo4.self_rel_many]]::jsonb ELSE jsonb_build_array([[__mm_demo4.self_rel_many]]) END) "__mm_demo4_self_rel_many_je" ON 1=1 LEFT JOIN "demo4" "__mm_demo4_self_rel_many" ON [[__mm_demo4_self_rel_many.id]] = [[__mm_demo4_self_rel_many_je.value]] LEFT JOIN "demo4" "__mm_demo4_self_rel_many_self_rel_one" ON [[__mm_demo4_self_rel_many_self_rel_one.id]] = [[__mm_demo4_self_rel_many.self_rel_one]] WHERE "__mm_demo4"."id" = "demo4"."id") {{__smTEST}} WHERE NOT ([[__smTEST.multiMatchValue]]::numeric > TRUE::numeric)))))`,
		},
		{
			"repeated nested relations (opt/any operator)",
			"demo4",
			"self_rel_many.self_rel_one.self_rel_many.self_rel_one.title ?> true",
			false,
			/* SQLite:
			"SELECT DISTINCT `demo4`.* FROM `demo4` LEFT JOIN json_each(CASE WHEN json_valid([[demo4.self_rel_many]]) THEN [[demo4.self_rel_many]] ELSE json_array([[demo4.self_rel_many]]) END) `demo4_self_rel_many_je` LEFT JOIN `demo4` `demo4_self_rel_many` ON [[demo4_self_rel_many.id]] = [[demo4_self_rel_many_je.value]] LEFT JOIN `demo4` `demo4_self_rel_many_self_rel_one` ON [[demo4_self_rel_many_self_rel_one.id]] = [[demo4_self_rel_many.self_rel_one]] LEFT JOIN json_each(CASE WHEN json_valid([[demo4_self_rel_many_self_rel_one.self_rel_many]]) THEN [[demo4_self_rel_many_self_rel_one.self_rel_many]] ELSE json_array([[demo4_self_rel_many_self_rel_one.self_rel_many]]) END) `demo4_self_rel_many_self_rel_one_self_rel_many_je` LEFT JOIN `demo4` `demo4_self_rel_many_self_rel_one_self_rel_many` ON [[demo4_self_rel_many_self_rel_one_self_rel_many.id]] = [[demo4_self_rel_many_self_rel_one_self_rel_many_je.value]] LEFT JOIN `demo4` `demo4_self_rel_many_self_rel_one_self_rel_many_self_rel_one` ON [[demo4_self_rel_many_self_rel_one_self_rel_many_self_rel_one.id]] = [[demo4_self_rel_many_self_rel_one_self_rel_many.self_rel_one]] WHERE [[demo4_self_rel_many_self_rel_one_self_rel_many_self_rel_one.title]] > 1",
			*/
			// PostgreSQL:
			`SELECT DISTINCT "demo4".* FROM "demo4" LEFT JOIN jsonb_array_elements_text(CASE WHEN ([[demo4.self_rel_many]] IS JSON OR json_valid([[demo4.self_rel_many]]::text)) AND jsonb_typeof([[demo4.self_rel_many]]::jsonb) = 'array' THEN [[demo4.self_rel_many]]::jsonb ELSE jsonb_build_array([[demo4.self_rel_many]]) END) "demo4_self_rel_many_je" ON 1=1 LEFT JOIN "demo4" "demo4_self_rel_many" ON [[demo4_self_rel_many.id]] = [[demo4_self_rel_many_je.value]] LEFT JOIN "demo4" "demo4_self_rel_many_self_rel_one" ON [[demo4_self_rel_many_self_rel_one.id]] = [[demo4_self_rel_many.self_rel_one]] LEFT JOIN jsonb_array_elements_text(CASE WHEN ([[demo4_self_rel_many_self_rel_one.self_rel_many]] IS JSON OR json_valid([[demo4_self_rel_many_self_rel_one.self_rel_many]]::text)) AND jsonb_typeof([[demo4_self_rel_many_self_rel_one.self_rel_many]]::jsonb) = 'array' THEN [[demo4_self_rel_many_self_rel_one.self_rel_many]]::jsonb ELSE jsonb_build_array([[demo4_self_rel_many_self_rel_one.self_rel_many]]) END) "demo4_self_rel_many_self_rel_one_self_rel_many_je" ON 1=1 LEFT JOIN "demo4" "demo4_self_rel_many_self_rel_one_self_rel_many" ON [[demo4_self_rel_many_self_rel_one_self_rel_many.id]] = [[demo4_self_rel_many_self_rel_one_self_rel_many_je.value]] LEFT JOIN "demo4" "demo4_self_rel_many_self_rel_one_self_rel_many_self_rel_one" ON [[demo4_self_rel_many_self_rel_one_self_rel_many_self_rel_one.id]] = [[demo4_self_rel_many_self_rel_one_self_rel_many.self_rel_one]] WHERE [[demo4_self_rel_many_self_rel_one_self_rel_many_self_rel_one.title]]::numeric > TRUE::numeric`,
		},
		{
			"repeated nested relations (multi-match operator)",
			"demo4",
			"self_rel_many.self_rel_one.self_rel_many.self_rel_one.title > true",
			false,
			/* SQLite:
			"SELECT DISTINCT `demo4`.* FROM `demo4` LEFT JOIN json_each(CASE WHEN json_valid([[demo4.self_rel_many]]) THEN [[demo4.self_rel_many]] ELSE json_array([[demo4.self_rel_many]]) END) `demo4_self_rel_many_je` LEFT JOIN `demo4` `demo4_self_rel_many` ON [[demo4_self_rel_many.id]] = [[demo4_self_rel_many_je.value]] LEFT JOIN `demo4` `demo4_self_rel_many_self_rel_one` ON [[demo4_self_rel_many_self_rel_one.id]] = [[demo4_self_rel_many.self_rel_one]] LEFT JOIN json_each(CASE WHEN json_valid([[demo4_self_rel_many_self_rel_one.self_rel_many]]) THEN [[demo4_self_rel_many_self_rel_one.self_rel_many]] ELSE json_array([[demo4_self_rel_many_self_rel_one.self_rel_many]]) END) `demo4_self_rel_many_self_rel_one_self_rel_many_je` LEFT JOIN `demo4` `demo4_self_rel_many_self_rel_one_self_rel_many` ON [[demo4_self_rel_many_self_rel_one_self_rel_many.id]] = [[demo4_self_rel_many_self_rel_one_self_rel_many_je.value]] LEFT JOIN `demo4` `demo4_self_rel_many_self_rel_one_self_rel_many_self_rel_one` ON [[demo4_self_rel_many_self_rel_one_self_rel_many_self_rel_one.id]] = [[demo4_self_rel_many_self_rel_one_self_rel_many.self_rel_one]] WHERE ((([[demo4_self_rel_many_self_rel_one_self_rel_many_self_rel_one.title]] > 1) AND (NOT EXISTS (SELECT 1 FROM (SELECT [[__mm_demo4_self_rel_many_self_rel_one_self_rel_many_self_rel_one.title]] as [[multiMatchValue]] FROM `demo4` `__mm_demo4` LEFT JOIN json_each(CASE WHEN json_valid([[__mm_demo4.self_rel_many]]) THEN [[__mm_demo4.self_rel_many]] ELSE json_array([[__mm_demo4.self_rel_many]]) END) `__mm_demo4_self_rel_many_je` LEFT JOIN `demo4` `__mm_demo4_self_rel_many` ON [[__mm_demo4_self_rel_many.id]] = [[__mm_demo4_self_rel_many_je.value]] LEFT JOIN `demo4` `__mm_demo4_self_rel_many_self_rel_one` ON [[__mm_demo4_self_rel_many_self_rel_one.id]] = [[__mm_demo4_self_rel_many.self_rel_one]] LEFT JOIN json_each(CASE WHEN json_valid([[__mm_demo4_self_rel_many_self_rel_one.self_rel_many]]) THEN [[__mm_demo4_self_rel_many_self_rel_one.self_rel_many]] ELSE json_array([[__mm_demo4_self_rel_many_self_rel_one.self_rel_many]]) END) `__mm_demo4_self_rel_many_self_rel_one_self_rel_many_je` LEFT JOIN `demo4` `__mm_demo4_self_rel_many_self_rel_one_self_rel_many` ON [[__mm_demo4_self_rel_many_self_rel_one_self_rel_many.id]] = [[__mm_demo4_self_rel_many_self_rel_one_self_rel_many_je.value]] LEFT JOIN `demo4` `__mm_demo4_self_rel_many_self_rel_one_self_rel_many_self_rel_one` ON [[__mm_demo4_self_rel_many_self_rel_one_self_rel_many_self_rel_one.id]] = [[__mm_demo4_self_rel_many_self_rel_one_self_rel_many.self_rel_one]] WHERE `__mm_demo4`.`id` = `demo4`.`id`) {{__smTEST}} WHERE NOT ([[__smTEST.multiMatchValue]] > 1)))))",
			*/
			// PostgreSQL:
			`SELECT DISTINCT "demo4".* FROM "demo4" LEFT JOIN jsonb_array_elements_text(CASE WHEN ([[demo4.self_rel_many]] IS JSON OR json_valid([[demo4.self_rel_many]]::text)) AND jsonb_typeof([[demo4.self_rel_many]]::jsonb) = 'array' THEN [[demo4.self_rel_many]]::jsonb ELSE jsonb_build_array([[demo4.self_rel_many]]) END) "demo4_self_rel_many_je" ON 1=1 LEFT JOIN "demo4" "demo4_self_rel_many" ON [[demo4_self_rel_many.id]] = [[demo4_self_rel_many_je.value]] LEFT JOIN "demo4" "demo4_self_rel_many_self_rel_one" ON [[demo4_self_rel_many_self_rel_one.id]] = [[demo4_self_rel_many.self_rel_one]] LEFT JOIN jsonb_array_elements_text(CASE WHEN ([[demo4_self_rel_many_self_rel_one.self_rel_many]] IS JSON OR json_valid([[demo4_self_rel_many_self_rel_one.self_rel_many]]::text)) AND jsonb_typeof([[demo4_self_rel_many_self_rel_one.self_rel_many]]::jsonb) = 'array' THEN [[demo4_self_rel_many_self_rel_one.self_rel_many]]::jsonb ELSE jsonb_build_array([[demo4_self_rel_many_self_rel_one.self_rel_many]]) END) "demo4_self_rel_many_self_rel_one_self_rel_many_je" ON 1=1 LEFT JOIN "demo4" "demo4_self_rel_many_self_rel_one_self_rel_many" ON [[demo4_self_rel_many_self_rel_one_self_rel_many.id]] = [[demo4_self_rel_many_self_rel_one_self_rel_many_je.value]] LEFT JOIN "demo4" "demo4_self_rel_many_self_rel_one_self_rel_many_self_rel_one" ON [[demo4_self_rel_many_self_rel_one_self_rel_many_self_rel_one.id]] = [[demo4_self_rel_many_self_rel_one_self_rel_many.self_rel_one]] WHERE ((([[demo4_self_rel_many_self_rel_one_self_rel_many_self_rel_one.title]]::numeric > TRUE::numeric) AND (NOT EXISTS (SELECT 1 FROM (SELECT [[__mm_demo4_self_rel_many_self_rel_one_self_rel_many_self_rel_one.title]] as [[multiMatchValue]] FROM "demo4" "__mm_demo4" LEFT JOIN jsonb_array_elements_text(CASE WHEN ([[__mm_demo4.self_rel_many]] IS JSON OR json_valid([[__mm_demo4.self_rel_many]]::text)) AND jsonb_typeof([[__mm_demo4.self_rel_many]]::jsonb) = 'array' THEN [[__mm_demo4.self_rel_many]]::jsonb ELSE jsonb_build_array([[__mm_demo4.self_rel_many]]) END) "__mm_demo4_self_rel_many_je" ON 1=1 LEFT JOIN "demo4" "__mm_demo4_self_rel_many" ON [[__mm_demo4_self_rel_many.id]] = [[__mm_demo4_self_rel_many_je.value]] LEFT JOIN "demo4" "__mm_demo4_self_rel_many_self_rel_one" ON [[__mm_demo4_self_rel_many_self_rel_one.id]] = [[__mm_demo4_self_rel_many.self_rel_one]] LEFT JOIN jsonb_array_elements_text(CASE WHEN ([[__mm_demo4_self_rel_many_self_rel_one.self_rel_many]] IS JSON OR json_valid([[__mm_demo4_self_rel_many_self_rel_one.self_rel_many]]::text)) AND jsonb_typeof([[__mm_demo4_self_rel_many_self_rel_one.self_rel_many]]::jsonb) = 'array' THEN [[__mm_demo4_self_rel_many_self_rel_one.self_rel_many]]::jsonb ELSE jsonb_build_array([[__mm_demo4_self_rel_many_self_rel_one.self_rel_many]]) END) "__mm_demo4_self_rel_many_self_rel_one_self_rel_many_je" ON 1=1 LEFT JOIN "demo4" "__mm_demo4_self_rel_many_self_rel_one_self_rel_many" ON [[__mm_demo4_self_rel_many_self_rel_one_self_rel_many.id]] = [[__mm_demo4_self_rel_many_self_rel_one_self_rel_many_je.value]] LEFT JOIN "demo4" "__mm_demo4_self_rel_many_self_rel_one_self_rel_many_self_rel_one" ON [[__mm_demo4_self_rel_many_self_rel_one_self_rel_many_self_rel_one.id]] = [[__mm_demo4_self_rel_many_self_rel_one_self_rel_many.self_rel_one]] WHERE "__mm_demo4"."id" = "demo4"."id") {{__smTEST}} WHERE NOT ([[__smTEST.multiMatchValue]]::numeric > TRUE::numeric)))))`,
		},
		{
			"multiple relations (opt/any operators)",
			"demo4",
			"self_rel_many.title ?= 'test' || self_rel_one.json_object.a ?> true",
			false,
			/* SQLite:
			"SELECT DISTINCT `demo4`.* FROM `demo4` LEFT JOIN json_each(CASE WHEN json_valid([[demo4.self_rel_many]]) THEN [[demo4.self_rel_many]] ELSE json_array([[demo4.self_rel_many]]) END) `demo4_self_rel_many_je` LEFT JOIN `demo4` `demo4_self_rel_many` ON [[demo4_self_rel_many.id]] = [[demo4_self_rel_many_je.value]] LEFT JOIN `demo4` `demo4_self_rel_one` ON [[demo4_self_rel_one.id]] = [[demo4.self_rel_one]] WHERE ([[demo4_self_rel_many.title]] = {:TEST} OR (CASE WHEN json_valid([[demo4_self_rel_one.json_object]]) THEN JSON_EXTRACT([[demo4_self_rel_one.json_object]], '$.a') ELSE JSON_EXTRACT(json_object('pb', [[demo4_self_rel_one.json_object]]), '$.pb.a') END) > 1)",
			*/
			// PostgreSQL:
			`SELECT DISTINCT "demo4".* FROM "demo4" LEFT JOIN jsonb_array_elements_text(CASE WHEN ([[demo4.self_rel_many]] IS JSON OR json_valid([[demo4.self_rel_many]]::text)) AND jsonb_typeof([[demo4.self_rel_many]]::jsonb) = 'array' THEN [[demo4.self_rel_many]]::jsonb ELSE jsonb_build_array([[demo4.self_rel_many]]) END) "demo4_self_rel_many_je" ON 1=1 LEFT JOIN "demo4" "demo4_self_rel_many" ON [[demo4_self_rel_many.id]] = [[demo4_self_rel_many_je.value]] LEFT JOIN "demo4" "demo4_self_rel_one" ON [[demo4_self_rel_one.id]] = [[demo4.self_rel_one]] WHERE ([[demo4_self_rel_many.title]] = {:TEST} OR ((CASE WHEN [[demo4_self_rel_one.json_object]] IS JSON OR json_valid([[demo4_self_rel_one.json_object]]::text) THEN JSON_QUERY([[demo4_self_rel_one.json_object]]::jsonb, '$.a') ELSE NULL END) #>> '{}')::text::numeric > TRUE::numeric)`,
		},
		{
			"multiple relations (multi-match operators)",
			"demo4",
			"self_rel_many.title = 'test' || self_rel_one.json_object.a > true",
			false,
			/* SQLite:
			"SELECT DISTINCT `demo4`.* FROM `demo4` LEFT JOIN json_each(CASE WHEN json_valid([[demo4.self_rel_many]]) THEN [[demo4.self_rel_many]] ELSE json_array([[demo4.self_rel_many]]) END) `demo4_self_rel_many_je` LEFT JOIN `demo4` `demo4_self_rel_many` ON [[demo4_self_rel_many.id]] = [[demo4_self_rel_many_je.value]] LEFT JOIN `demo4` `demo4_self_rel_one` ON [[demo4_self_rel_one.id]] = [[demo4.self_rel_one]] WHERE ((([[demo4_self_rel_many.title]] = {:TEST}) AND (NOT EXISTS (SELECT 1 FROM (SELECT [[__mm_demo4_self_rel_many.title]] as [[multiMatchValue]] FROM `demo4` `__mm_demo4` LEFT JOIN json_each(CASE WHEN json_valid([[__mm_demo4.self_rel_many]]) THEN [[__mm_demo4.self_rel_many]] ELSE json_array([[__mm_demo4.self_rel_many]]) END) `__mm_demo4_self_rel_many_je` LEFT JOIN `demo4` `__mm_demo4_self_rel_many` ON [[__mm_demo4_self_rel_many.id]] = [[__mm_demo4_self_rel_many_je.value]] WHERE `__mm_demo4`.`id` = `demo4`.`id`) {{__smTEST}} WHERE NOT ([[__smTEST.multiMatchValue]] = {:TEST})))) OR (CASE WHEN json_valid([[demo4_self_rel_one.json_object]]) THEN JSON_EXTRACT([[demo4_self_rel_one.json_object]], '$.a') ELSE JSON_EXTRACT(json_object('pb', [[demo4_self_rel_one.json_object]]), '$.pb.a') END) > 1)",
			*/
			// PostgreSQL:
			`SELECT DISTINCT "demo4".* FROM "demo4" LEFT JOIN jsonb_array_elements_text(CASE WHEN ([[demo4.self_rel_many]] IS JSON OR json_valid([[demo4.self_rel_many]]::text)) AND jsonb_typeof([[demo4.self_rel_many]]::jsonb) = 'array' THEN [[demo4.self_rel_many]]::jsonb ELSE jsonb_build_array([[demo4.self_rel_many]]) END) "demo4_self_rel_many_je" ON 1=1 LEFT JOIN "demo4" "demo4_self_rel_many" ON [[demo4_self_rel_many.id]] = [[demo4_self_rel_many_je.value]] LEFT JOIN "demo4" "demo4_self_rel_one" ON [[demo4_self_rel_one.id]] = [[demo4.self_rel_one]] WHERE ((([[demo4_self_rel_many.title]] = {:TEST}) AND (NOT EXISTS (SELECT 1 FROM (SELECT [[__mm_demo4_self_rel_many.title]] as [[multiMatchValue]] FROM "demo4" "__mm_demo4" LEFT JOIN jsonb_array_elements_text(CASE WHEN ([[__mm_demo4.self_rel_many]] IS JSON OR json_valid([[__mm_demo4.self_rel_many]]::text)) AND jsonb_typeof([[__mm_demo4.self_rel_many]]::jsonb) = 'array' THEN [[__mm_demo4.self_rel_many]]::jsonb ELSE jsonb_build_array([[__mm_demo4.self_rel_many]]) END) "__mm_demo4_self_rel_many_je" ON 1=1 LEFT JOIN "demo4" "__mm_demo4_self_rel_many" ON [[__mm_demo4_self_rel_many.id]] = [[__mm_demo4_self_rel_many_je.value]] WHERE "__mm_demo4"."id" = "demo4"."id") {{__smTEST}} WHERE NOT ([[__smTEST.multiMatchValue]] = {:TEST})))) OR ((CASE WHEN [[demo4_self_rel_one.json_object]] IS JSON OR json_valid([[demo4_self_rel_one.json_object]]::text) THEN JSON_QUERY([[demo4_self_rel_one.json_object]]::jsonb, '$.a') ELSE NULL END) #>> '{}')::text::numeric > TRUE::numeric)`,
		},
		{
			"back relations via single relation field (without unique index)",
			"demo3",
			"demo4_via_rel_one_cascade.id = true",
			false,
			/* SQLite:
			"SELECT DISTINCT `demo3`.* FROM `demo3` LEFT JOIN `demo4` `demo3_demo4_via_rel_one_cascade` ON [[demo3.id]] IN (SELECT [[demo3_demo4_via_rel_one_cascade_je.value]] FROM json_each(CASE WHEN json_valid([[demo3_demo4_via_rel_one_cascade.rel_one_cascade]]) THEN [[demo3_demo4_via_rel_one_cascade.rel_one_cascade]] ELSE json_array([[demo3_demo4_via_rel_one_cascade.rel_one_cascade]]) END) {{demo3_demo4_via_rel_one_cascade_je}}) WHERE ((([[demo3_demo4_via_rel_one_cascade.id]] = 1) AND (NOT EXISTS (SELECT 1 FROM (SELECT [[__mm_demo3_demo4_via_rel_one_cascade.id]] as [[multiMatchValue]] FROM `demo3` `__mm_demo3` LEFT JOIN `demo4` `__mm_demo3_demo4_via_rel_one_cascade` ON [[__mm_demo3.id]] IN (SELECT [[__mm_demo3_demo4_via_rel_one_cascade_je.value]] FROM json_each(CASE WHEN json_valid([[__mm_demo3_demo4_via_rel_one_cascade.rel_one_cascade]]) THEN [[__mm_demo3_demo4_via_rel_one_cascade.rel_one_cascade]] ELSE json_array([[__mm_demo3_demo4_via_rel_one_cascade.rel_one_cascade]]) END) {{__mm_demo3_demo4_via_rel_one_cascade_je}}) WHERE `__mm_demo3`.`id` = `demo3`.`id`) {{__smTEST}} WHERE NOT ([[__smTEST.multiMatchValue]] = 1)))))",
			*/
			// PostgreSQL:
			`SELECT DISTINCT "demo3".* FROM "demo3" LEFT JOIN "demo4" "demo3_demo4_via_rel_one_cascade" ON [[demo3.id]] IN (SELECT [[demo3_demo4_via_rel_one_cascade_je.value]] FROM jsonb_array_elements_text(CASE WHEN ([[demo3_demo4_via_rel_one_cascade.rel_one_cascade]] IS JSON OR json_valid([[demo3_demo4_via_rel_one_cascade.rel_one_cascade]]::text)) AND jsonb_typeof([[demo3_demo4_via_rel_one_cascade.rel_one_cascade]]::jsonb) = 'array' THEN [[demo3_demo4_via_rel_one_cascade.rel_one_cascade]]::jsonb ELSE jsonb_build_array([[demo3_demo4_via_rel_one_cascade.rel_one_cascade]]) END) {{demo3_demo4_via_rel_one_cascade_je}}) WHERE ((([[demo3_demo4_via_rel_one_cascade.id]] = TRUE) AND (NOT EXISTS (SELECT 1 FROM (SELECT [[__mm_demo3_demo4_via_rel_one_cascade.id]] as [[multiMatchValue]] FROM "demo3" "__mm_demo3" LEFT JOIN "demo4" "__mm_demo3_demo4_via_rel_one_cascade" ON [[__mm_demo3.id]] IN (SELECT [[__mm_demo3_demo4_via_rel_one_cascade_je.value]] FROM jsonb_array_elements_text(CASE WHEN ([[__mm_demo3_demo4_via_rel_one_cascade.rel_one_cascade]] IS JSON OR json_valid([[__mm_demo3_demo4_via_rel_one_cascade.rel_one_cascade]]::text)) AND jsonb_typeof([[__mm_demo3_demo4_via_rel_one_cascade.rel_one_cascade]]::jsonb) = 'array' THEN [[__mm_demo3_demo4_via_rel_one_cascade.rel_one_cascade]]::jsonb ELSE jsonb_build_array([[__mm_demo3_demo4_via_rel_one_cascade.rel_one_cascade]]) END) {{__mm_demo3_demo4_via_rel_one_cascade_je}}) WHERE "__mm_demo3"."id" = "demo3"."id") {{__smTEST}} WHERE NOT ([[__smTEST.multiMatchValue]] = TRUE)))))`,
		},
		{
			"back relations via single relation field (with unique index)",
			"demo3",
			"demo4_via_rel_one_unique.id = true",
			false,
			/* SQLite:
			"SELECT DISTINCT `demo3`.* FROM `demo3` LEFT JOIN `demo4` `demo3_demo4_via_rel_one_unique` ON [[demo3_demo4_via_rel_one_unique.rel_one_unique]] = [[demo3.id]] WHERE [[demo3_demo4_via_rel_one_unique.id]] = 1",
			*/
			// PostgreSQL:
			`SELECT DISTINCT "demo3".* FROM "demo3" LEFT JOIN "demo4" "demo3_demo4_via_rel_one_unique" ON [[demo3_demo4_via_rel_one_unique.rel_one_unique]] = [[demo3.id]] WHERE [[demo3_demo4_via_rel_one_unique.id]] = TRUE`,
		},
		{
			"back relations via multiple relation field (opt/any operators)",
			"demo3",
			"demo4_via_rel_many_cascade.id ?= true",
			false,
			/* SQLite:
			"SELECT DISTINCT `demo3`.* FROM `demo3` LEFT JOIN `demo4` `demo3_demo4_via_rel_many_cascade` ON [[demo3.id]] IN (SELECT [[demo3_demo4_via_rel_many_cascade_je.value]] FROM json_each(CASE WHEN json_valid([[demo3_demo4_via_rel_many_cascade.rel_many_cascade]]) THEN [[demo3_demo4_via_rel_many_cascade.rel_many_cascade]] ELSE json_array([[demo3_demo4_via_rel_many_cascade.rel_many_cascade]]) END) {{demo3_demo4_via_rel_many_cascade_je}}) WHERE [[demo3_demo4_via_rel_many_cascade.id]] = 1",
			*/
			// PostgreSQL:
			`SELECT DISTINCT "demo3".* FROM "demo3" LEFT JOIN "demo4" "demo3_demo4_via_rel_many_cascade" ON [[demo3.id]] IN (SELECT [[demo3_demo4_via_rel_many_cascade_je.value]] FROM jsonb_array_elements_text(CASE WHEN ([[demo3_demo4_via_rel_many_cascade.rel_many_cascade]] IS JSON OR json_valid([[demo3_demo4_via_rel_many_cascade.rel_many_cascade]]::text)) AND jsonb_typeof([[demo3_demo4_via_rel_many_cascade.rel_many_cascade]]::jsonb) = 'array' THEN [[demo3_demo4_via_rel_many_cascade.rel_many_cascade]]::jsonb ELSE jsonb_build_array([[demo3_demo4_via_rel_many_cascade.rel_many_cascade]]) END) {{demo3_demo4_via_rel_many_cascade_je}}) WHERE [[demo3_demo4_via_rel_many_cascade.id]] = TRUE`,
		},
		{
			"back relations via multiple relation field (multi-match operators)",
			"demo3",
			"demo4_via_rel_many_cascade.id = true",
			false,
			/* SQLite:
			"SELECT DISTINCT `demo3`.* FROM `demo3` LEFT JOIN `demo4` `demo3_demo4_via_rel_many_cascade` ON [[demo3.id]] IN (SELECT [[demo3_demo4_via_rel_many_cascade_je.value]] FROM json_each(CASE WHEN json_valid([[demo3_demo4_via_rel_many_cascade.rel_many_cascade]]) THEN [[demo3_demo4_via_rel_many_cascade.rel_many_cascade]] ELSE json_array([[demo3_demo4_via_rel_many_cascade.rel_many_cascade]]) END) {{demo3_demo4_via_rel_many_cascade_je}}) WHERE ((([[demo3_demo4_via_rel_many_cascade.id]] = 1) AND (NOT EXISTS (SELECT 1 FROM (SELECT [[__mm_demo3_demo4_via_rel_many_cascade.id]] as [[multiMatchValue]] FROM `demo3` `__mm_demo3` LEFT JOIN `demo4` `__mm_demo3_demo4_via_rel_many_cascade` ON [[__mm_demo3.id]] IN (SELECT [[__mm_demo3_demo4_via_rel_many_cascade_je.value]] FROM json_each(CASE WHEN json_valid([[__mm_demo3_demo4_via_rel_many_cascade.rel_many_cascade]]) THEN [[__mm_demo3_demo4_via_rel_many_cascade.rel_many_cascade]] ELSE json_array([[__mm_demo3_demo4_via_rel_many_cascade.rel_many_cascade]]) END) {{__mm_demo3_demo4_via_rel_many_cascade_je}}) WHERE `__mm_demo3`.`id` = `demo3`.`id`) {{__smTEST}} WHERE NOT ([[__smTEST.multiMatchValue]] = 1)))))",
			*/
			// PostgreSQL:
			`SELECT DISTINCT "demo3".* FROM "demo3" LEFT JOIN "demo4" "demo3_demo4_via_rel_many_cascade" ON [[demo3.id]] IN (SELECT [[demo3_demo4_via_rel_many_cascade_je.value]] FROM jsonb_array_elements_text(CASE WHEN ([[demo3_demo4_via_rel_many_cascade.rel_many_cascade]] IS JSON OR json_valid([[demo3_demo4_via_rel_many_cascade.rel_many_cascade]]::text)) AND jsonb_typeof([[demo3_demo4_via_rel_many_cascade.rel_many_cascade]]::jsonb) = 'array' THEN [[demo3_demo4_via_rel_many_cascade.rel_many_cascade]]::jsonb ELSE jsonb_build_array([[demo3_demo4_via_rel_many_cascade.rel_many_cascade]]) END) {{demo3_demo4_via_rel_many_cascade_je}}) WHERE ((([[demo3_demo4_via_rel_many_cascade.id]] = TRUE) AND (NOT EXISTS (SELECT 1 FROM (SELECT [[__mm_demo3_demo4_via_rel_many_cascade.id]] as [[multiMatchValue]] FROM "demo3" "__mm_demo3" LEFT JOIN "demo4" "__mm_demo3_demo4_via_rel_many_cascade" ON [[__mm_demo3.id]] IN (SELECT [[__mm_demo3_demo4_via_rel_many_cascade_je.value]] FROM jsonb_array_elements_text(CASE WHEN ([[__mm_demo3_demo4_via_rel_many_cascade.rel_many_cascade]] IS JSON OR json_valid([[__mm_demo3_demo4_via_rel_many_cascade.rel_many_cascade]]::text)) AND jsonb_typeof([[__mm_demo3_demo4_via_rel_many_cascade.rel_many_cascade]]::jsonb) = 'array' THEN [[__mm_demo3_demo4_via_rel_many_cascade.rel_many_cascade]]::jsonb ELSE jsonb_build_array([[__mm_demo3_demo4_via_rel_many_cascade.rel_many_cascade]]) END) {{__mm_demo3_demo4_via_rel_many_cascade_je}}) WHERE "__mm_demo3"."id" = "demo3"."id") {{__smTEST}} WHERE NOT ([[__smTEST.multiMatchValue]] = TRUE)))))`,
		},
		{
			"back relations via unique multiple relation field (should be the same as multi-match)",
			"demo3",
			"demo4_via_rel_many_unique.id = true",
			false,
			/* SQLite:
			"SELECT DISTINCT `demo3`.* FROM `demo3` LEFT JOIN `demo4` `demo3_demo4_via_rel_many_unique` ON [[demo3.id]] IN (SELECT [[demo3_demo4_via_rel_many_unique_je.value]] FROM json_each(CASE WHEN json_valid([[demo3_demo4_via_rel_many_unique.rel_many_unique]]) THEN [[demo3_demo4_via_rel_many_unique.rel_many_unique]] ELSE json_array([[demo3_demo4_via_rel_many_unique.rel_many_unique]]) END) {{demo3_demo4_via_rel_many_unique_je}}) WHERE ((([[demo3_demo4_via_rel_many_unique.id]] = 1) AND (NOT EXISTS (SELECT 1 FROM (SELECT [[__mm_demo3_demo4_via_rel_many_unique.id]] as [[multiMatchValue]] FROM `demo3` `__mm_demo3` LEFT JOIN `demo4` `__mm_demo3_demo4_via_rel_many_unique` ON [[__mm_demo3.id]] IN (SELECT [[__mm_demo3_demo4_via_rel_many_unique_je.value]] FROM json_each(CASE WHEN json_valid([[__mm_demo3_demo4_via_rel_many_unique.rel_many_unique]]) THEN [[__mm_demo3_demo4_via_rel_many_unique.rel_many_unique]] ELSE json_array([[__mm_demo3_demo4_via_rel_many_unique.rel_many_unique]]) END) {{__mm_demo3_demo4_via_rel_many_unique_je}}) WHERE `__mm_demo3`.`id` = `demo3`.`id`) {{__smTEST}} WHERE NOT ([[__smTEST.multiMatchValue]] = 1)))))",
			*/
			// PostgreSQL:
			`SELECT DISTINCT "demo3".* FROM "demo3" LEFT JOIN "demo4" "demo3_demo4_via_rel_many_unique" ON [[demo3.id]] IN (SELECT [[demo3_demo4_via_rel_many_unique_je.value]] FROM jsonb_array_elements_text(CASE WHEN ([[demo3_demo4_via_rel_many_unique.rel_many_unique]] IS JSON OR json_valid([[demo3_demo4_via_rel_many_unique.rel_many_unique]]::text)) AND jsonb_typeof([[demo3_demo4_via_rel_many_unique.rel_many_unique]]::jsonb) = 'array' THEN [[demo3_demo4_via_rel_many_unique.rel_many_unique]]::jsonb ELSE jsonb_build_array([[demo3_demo4_via_rel_many_unique.rel_many_unique]]) END) {{demo3_demo4_via_rel_many_unique_je}}) WHERE ((([[demo3_demo4_via_rel_many_unique.id]] = TRUE) AND (NOT EXISTS (SELECT 1 FROM (SELECT [[__mm_demo3_demo4_via_rel_many_unique.id]] as [[multiMatchValue]] FROM "demo3" "__mm_demo3" LEFT JOIN "demo4" "__mm_demo3_demo4_via_rel_many_unique" ON [[__mm_demo3.id]] IN (SELECT [[__mm_demo3_demo4_via_rel_many_unique_je.value]] FROM jsonb_array_elements_text(CASE WHEN ([[__mm_demo3_demo4_via_rel_many_unique.rel_many_unique]] IS JSON OR json_valid([[__mm_demo3_demo4_via_rel_many_unique.rel_many_unique]]::text)) AND jsonb_typeof([[__mm_demo3_demo4_via_rel_many_unique.rel_many_unique]]::jsonb) = 'array' THEN [[__mm_demo3_demo4_via_rel_many_unique.rel_many_unique]]::jsonb ELSE jsonb_build_array([[__mm_demo3_demo4_via_rel_many_unique.rel_many_unique]]) END) {{__mm_demo3_demo4_via_rel_many_unique_je}}) WHERE "__mm_demo3"."id" = "demo3"."id") {{__smTEST}} WHERE NOT ([[__smTEST.multiMatchValue]] = TRUE)))))`,
		},
		{
			"recursive back relations",
			"demo3",
			"demo4_via_rel_many_cascade.rel_one_cascade.demo4_via_rel_many_cascade.id ?= true",
			false,
			/* SQLite:
			"SELECT DISTINCT `demo3`.* FROM `demo3` LEFT JOIN `demo4` `demo3_demo4_via_rel_many_cascade` ON [[demo3.id]] IN (SELECT [[demo3_demo4_via_rel_many_cascade_je.value]] FROM json_each(CASE WHEN json_valid([[demo3_demo4_via_rel_many_cascade.rel_many_cascade]]) THEN [[demo3_demo4_via_rel_many_cascade.rel_many_cascade]] ELSE json_array([[demo3_demo4_via_rel_many_cascade.rel_many_cascade]]) END) {{demo3_demo4_via_rel_many_cascade_je}}) LEFT JOIN `demo3` `demo3_demo4_via_rel_many_cascade_rel_one_cascade` ON [[demo3_demo4_via_rel_many_cascade_rel_one_cascade.id]] = [[demo3_demo4_via_rel_many_cascade.rel_one_cascade]] LEFT JOIN `demo4` `demo3_demo4_via_rel_many_cascade_rel_one_cascade_demo4_via_rel_many_cascade` ON [[demo3_demo4_via_rel_many_cascade_rel_one_cascade.id]] IN (SELECT [[demo3_demo4_via_rel_many_cascade_rel_one_cascade_demo4_via_rel_many_cascade_je.value]] FROM json_each(CASE WHEN json_valid([[demo3_demo4_via_rel_many_cascade_rel_one_cascade_demo4_via_rel_many_cascade.rel_many_cascade]]) THEN [[demo3_demo4_via_rel_many_cascade_rel_one_cascade_demo4_via_rel_many_cascade.rel_many_cascade]] ELSE json_array([[demo3_demo4_via_rel_many_cascade_rel_one_cascade_demo4_via_rel_many_cascade.rel_many_cascade]]) END) {{demo3_demo4_via_rel_many_cascade_rel_one_cascade_demo4_via_rel_many_cascade_je}}) WHERE [[demo3_demo4_via_rel_many_cascade_rel_one_cascade_demo4_via_rel_many_cascade.id]] = 1",
			*/
			// PostgreSQL:
			`SELECT DISTINCT "demo3".* FROM "demo3" LEFT JOIN "demo4" "demo3_demo4_via_rel_many_cascade" ON [[demo3.id]] IN (SELECT [[demo3_demo4_via_rel_many_cascade_je.value]] FROM jsonb_array_elements_text(CASE WHEN ([[demo3_demo4_via_rel_many_cascade.rel_many_cascade]] IS JSON OR json_valid([[demo3_demo4_via_rel_many_cascade.rel_many_cascade]]::text)) AND jsonb_typeof([[demo3_demo4_via_rel_many_cascade.rel_many_cascade]]::jsonb) = 'array' THEN [[demo3_demo4_via_rel_many_cascade.rel_many_cascade]]::jsonb ELSE jsonb_build_array([[demo3_demo4_via_rel_many_cascade.rel_many_cascade]]) END) {{demo3_demo4_via_rel_many_cascade_je}}) LEFT JOIN "demo3" "demo3_demo4_via_rel_many_cascade_rel_one_cascade" ON [[demo3_demo4_via_rel_many_cascade_rel_one_cascade.id]] = [[demo3_demo4_via_rel_many_cascade.rel_one_cascade]] LEFT JOIN "demo4" "demo3_demo4_via_rel_many_cascade_rel_one_cascade_demo4_via_rel_many_cascade" ON [[demo3_demo4_via_rel_many_cascade_rel_one_cascade.id]] IN (SELECT [[demo3_demo4_via_rel_many_cascade_rel_one_cascade_demo4_via_rel_many_cascade_je.value]] FROM jsonb_array_elements_text(CASE WHEN ([[demo3_demo4_via_rel_many_cascade_rel_one_cascade_demo4_via_rel_many_cascade.rel_many_cascade]] IS JSON OR json_valid([[demo3_demo4_via_rel_many_cascade_rel_one_cascade_demo4_via_rel_many_cascade.rel_many_cascade]]::text)) AND jsonb_typeof([[demo3_demo4_via_rel_many_cascade_rel_one_cascade_demo4_via_rel_many_cascade.rel_many_cascade]]::jsonb) = 'array' THEN [[demo3_demo4_via_rel_many_cascade_rel_one_cascade_demo4_via_rel_many_cascade.rel_many_cascade]]::jsonb ELSE jsonb_build_array([[demo3_demo4_via_rel_many_cascade_rel_one_cascade_demo4_via_rel_many_cascade.rel_many_cascade]]) END) {{demo3_demo4_via_rel_many_cascade_rel_one_cascade_demo4_via_rel_many_cascade_je}}) WHERE [[demo3_demo4_via_rel_many_cascade_rel_one_cascade_demo4_via_rel_many_cascade.id]] = TRUE`,
		},
		{
			"@collection join (opt/any operators)",
			"demo4",
			"@collection.demo1.text ?> true || @collection.demo2.active ?> true || @collection.demo1:demo1_alias.file_one ?> true",
			false,
			/* SQLite:
			"SELECT DISTINCT `demo4`.* FROM `demo4` LEFT JOIN `demo1` `__collection_demo1` LEFT JOIN `demo2` `__collection_demo2` LEFT JOIN `demo1` `__collection_alias_demo1_alias` WHERE ([[__collection_demo1.text]] > 1 OR [[__collection_demo2.active]] > 1 OR [[__collection_alias_demo1_alias.file_one]] > 1)",
			*/
			// PostgreSQL:
			`SELECT DISTINCT "demo4".* FROM "demo4" LEFT JOIN "demo1" "__collection_demo1" ON 1=1 LEFT JOIN "demo2" "__collection_demo2" ON 1=1 LEFT JOIN "demo1" "__collection_alias_demo1_alias" ON 1=1 WHERE ([[__collection_demo1.text]]::numeric > TRUE::numeric OR [[__collection_demo2.active]]::numeric > TRUE::numeric OR [[__collection_alias_demo1_alias.file_one]]::numeric > TRUE::numeric)`,
		},
		{
			"@collection join (multi-match operators)",
			"demo4",
			"@collection.demo1.text > true || @collection.demo2.active > true || @collection.demo1.file_one > true",
			false,
			/* SQLite:
			"SELECT DISTINCT `demo4`.* FROM `demo4` LEFT JOIN `demo1` `__collection_demo1` LEFT JOIN `demo2` `__collection_demo2` WHERE ((([[__collection_demo1.text]] > 1) AND (NOT EXISTS (SELECT 1 FROM (SELECT [[__mm__collection_demo1.text]] as [[multiMatchValue]] FROM `demo4` `__mm_demo4` LEFT JOIN `demo1` `__mm__collection_demo1` WHERE `__mm_demo4`.`id` = `demo4`.`id`) {{__smTEST}} WHERE NOT ([[__smTEST.multiMatchValue]] > 1)))) OR (([[__collection_demo2.active]] > 1) AND (NOT EXISTS (SELECT 1 FROM (SELECT [[__mm__collection_demo2.active]] as [[multiMatchValue]] FROM `demo4` `__mm_demo4` LEFT JOIN `demo2` `__mm__collection_demo2` WHERE `__mm_demo4`.`id` = `demo4`.`id`) {{__smTEST}} WHERE NOT ([[__smTEST.multiMatchValue]] > 1)))) OR (([[__collection_demo1.file_one]] > 1) AND (NOT EXISTS (SELECT 1 FROM (SELECT [[__mm__collection_demo1.file_one]] as [[multiMatchValue]] FROM `demo4` `__mm_demo4` LEFT JOIN `demo1` `__mm__collection_demo1` WHERE `__mm_demo4`.`id` = `demo4`.`id`) {{__smTEST}} WHERE NOT ([[__smTEST.multiMatchValue]] > 1)))))",
			*/
			// PostgreSQL:
			`SELECT DISTINCT "demo4".* FROM "demo4" LEFT JOIN "demo1" "__collection_demo1" ON 1=1 LEFT JOIN "demo2" "__collection_demo2" ON 1=1 WHERE ((([[__collection_demo1.text]]::numeric > TRUE::numeric) AND (NOT EXISTS (SELECT 1 FROM (SELECT [[__mm__collection_demo1.text]] as [[multiMatchValue]] FROM "demo4" "__mm_demo4" LEFT JOIN "demo1" "__mm__collection_demo1" ON 1=1 WHERE "__mm_demo4"."id" = "demo4"."id") {{__smTEST}} WHERE NOT ([[__smTEST.multiMatchValue]]::numeric > TRUE::numeric)))) OR (([[__collection_demo2.active]]::numeric > TRUE::numeric) AND (NOT EXISTS (SELECT 1 FROM (SELECT [[__mm__collection_demo2.active]] as [[multiMatchValue]] FROM "demo4" "__mm_demo4" LEFT JOIN "demo2" "__mm__collection_demo2" ON 1=1 WHERE "__mm_demo4"."id" = "demo4"."id") {{__smTEST}} WHERE NOT ([[__smTEST.multiMatchValue]]::numeric > TRUE::numeric)))) OR (([[__collection_demo1.file_one]]::numeric > TRUE::numeric) AND (NOT EXISTS (SELECT 1 FROM (SELECT [[__mm__collection_demo1.file_one]] as [[multiMatchValue]] FROM "demo4" "__mm_demo4" LEFT JOIN "demo1" "__mm__collection_demo1" ON 1=1 WHERE "__mm_demo4"."id" = "demo4"."id") {{__smTEST}} WHERE NOT ([[__smTEST.multiMatchValue]]::numeric > TRUE::numeric)))))`,
		},
		{
			"@request.auth fields",
			"demo4",
			"@request.auth.id > true || @request.auth.username > true || @request.auth.rel.title > true || @request.body.demo < true || @request.auth.missingA.missingB > false",
			false,
			/* SQLite:
			"SELECT DISTINCT `demo4`.* FROM `demo4` LEFT JOIN `users` `__auth_users` ON `__auth_users`.`id`={:p0} LEFT JOIN `demo2` `__auth_users_rel` ON [[__auth_users_rel.id]] = [[__auth_users.rel]] WHERE ({:TEST} > 1 OR [[__auth_users.username]] > 1 OR [[__auth_users_rel.title]] > 1 OR NULL < 1 OR NULL > 0)",
			*/
			// PostgreSQL:
			`SELECT DISTINCT "demo4".* FROM "demo4" LEFT JOIN "users" "__auth_users" ON "__auth_users"."id"={:p0} LEFT JOIN "demo2" "__auth_users_rel" ON [[__auth_users_rel.id]] = [[__auth_users.rel]] WHERE ({:TEST}::numeric > TRUE::numeric OR [[__auth_users.username]]::numeric > TRUE::numeric OR [[__auth_users_rel.title]]::numeric > TRUE::numeric OR NULL::numeric < TRUE::numeric OR NULL::numeric > FALSE::numeric)`,
		},
		{
			"@request.* static fields",
			"demo4",
			"@request.context = true || @request.query.a = true || @request.query.b = true || @request.query.missing = true || @request.headers.a = true || @request.headers.missing = true",
			false,
			/* SQLite:
			"SELECT `demo4`.* FROM `demo4` WHERE ({:TEST} = 1 OR '' = 1 OR {:TEST} = 1 OR '' = 1 OR {:TEST} = 1 OR '' = 1)",
			*/
			// PostgreSQL:
			`SELECT "demo4".* FROM "demo4" WHERE ({:TEST} = TRUE OR '' = TRUE OR {:TEST} = TRUE OR '' = TRUE OR {:TEST} = TRUE OR '' = TRUE)`,
		},
		{
			"hidden field with system filters (multi-match and ignore emailVisibility)",
			"demo4",
			"@collection.users.email > true || @request.auth.email > true",
			false,
			/* SQLite:
			"SELECT DISTINCT `demo4`.* FROM `demo4` LEFT JOIN `users` `__collection_users` WHERE ((([[__collection_users.email]] > 1) AND (NOT EXISTS (SELECT 1 FROM (SELECT [[__mm__collection_users.email]] as [[multiMatchValue]] FROM `demo4` `__mm_demo4` LEFT JOIN `users` `__mm__collection_users` WHERE `__mm_demo4`.`id` = `demo4`.`id`) {{__smTEST}} WHERE NOT ([[__smTEST.multiMatchValue]] > 1)))) OR {:TEST} > 1)",
			*/
			// PostgreSQL:
			`SELECT DISTINCT "demo4".* FROM "demo4" LEFT JOIN "users" "__collection_users" ON 1=1 WHERE ((([[__collection_users.email]]::numeric > TRUE::numeric) AND (NOT EXISTS (SELECT 1 FROM (SELECT [[__mm__collection_users.email]] as [[multiMatchValue]] FROM "demo4" "__mm_demo4" LEFT JOIN "users" "__mm__collection_users" ON 1=1 WHERE "__mm_demo4"."id" = "demo4"."id") {{__smTEST}} WHERE NOT ([[__smTEST.multiMatchValue]]::numeric > TRUE::numeric)))) OR {:TEST}::numeric > TRUE::numeric)`,
		},
		{
			"hidden field (add emailVisibility)",
			"users",
			"id > true || email > true || email:lower > false",
			false,
			/* SQLite:
			"SELECT `users`.* FROM `users` WHERE ([[users.id]] > 1 OR (([[users.email]] > 1) AND ([[users.emailVisibility]] = TRUE)) OR ((LOWER([[users.email]]) > 0) AND ([[users.emailVisibility]] = TRUE)))",
			*/
			// PostgreSQL:
			`SELECT "users".* FROM "users" WHERE ([[users.id]]::numeric > TRUE::numeric OR (([[users.email]]::numeric > TRUE::numeric) AND ([[users.emailVisibility]] = TRUE)) OR ((LOWER([[users.email]])::numeric > FALSE::numeric) AND ([[users.emailVisibility]] = TRUE)))`,
		},
		{
			"hidden field (force ignore emailVisibility)",
			"users",
			"email > true",
			true,
			/* SQLite:
			"SELECT `users`.* FROM `users` WHERE [[users.email]] > 1",
			*/
			// PostgreSQL:
			`SELECT "users".* FROM "users" WHERE [[users.email]]::numeric > TRUE::numeric`,
		},
		{
			"static @request fields with :lower modifier",
			"demo1",
			"@request.body.a:lower > true ||" +
				"@request.body.b:lower > true ||" +
				"@request.body.c:lower > true ||" +
				"@request.query.a:lower > true ||" +
				"@request.query.b:lower > true ||" +
				"@request.query.c:lower > true ||" +
				"@request.headers.a:lower > true ||" +
				"@request.headers.c:lower > true",
			false,
			/* SQLite:
			"SELECT `demo1`.* FROM `demo1` WHERE (NULL > 1 OR LOWER({:TEST}) > 1 OR NULL > 1 OR LOWER({:TEST}) > 1 OR LOWER({:TEST}) > 1 OR NULL > 1 OR LOWER({:TEST}) > 1 OR NULL > 1)",
			*/
			// PostgreSQL:
			`SELECT "demo1".* FROM "demo1" WHERE (NULL::numeric > TRUE::numeric OR LOWER({:TEST})::numeric > TRUE::numeric OR NULL::numeric > TRUE::numeric OR LOWER({:TEST})::numeric > TRUE::numeric OR LOWER({:TEST})::numeric > TRUE::numeric OR NULL::numeric > TRUE::numeric OR LOWER({:TEST})::numeric > TRUE::numeric OR NULL::numeric > TRUE::numeric)`,
		},
		{
			"collection fields with :lower modifier",
			"demo1",
			"@request.body.rel_one:lower > true ||" +
				"@request.body.rel_many:lower > true ||" +
				"@request.body.rel_many.email:lower > true ||" +
				"text:lower > true ||" +
				"bool:lower > true ||" +
				"url:lower > true ||" +
				"select_one:lower > true ||" +
				"select_many:lower > true ||" +
				"file_one:lower > true ||" +
				"file_many:lower > true ||" +
				"number:lower > true ||" +
				"email:lower > true ||" +
				"datetime:lower > true ||" +
				"json:lower > true ||" +
				"rel_one:lower > true ||" +
				"rel_many:lower > true ||" +
				"rel_many.name:lower > true ||" +
				"created:lower > true",
			false,
			/* SQLite:
			"SELECT DISTINCT `demo1`.* FROM `demo1` LEFT JOIN `users` `__data_users_rel_many` ON [[__data_users_rel_many.id]] IN ({:p0}, {:p1}) LEFT JOIN json_each(CASE WHEN json_valid([[demo1.rel_many]]) THEN [[demo1.rel_many]] ELSE json_array([[demo1.rel_many]]) END) `demo1_rel_many_je` LEFT JOIN `users` `demo1_rel_many` ON [[demo1_rel_many.id]] = [[demo1_rel_many_je.value]] WHERE (LOWER({:infoLowerrel_oneTEST}) > 1 OR LOWER({:infoLowerrel_manyTEST}) > 1 OR ((LOWER([[__data_users_rel_many.email]]) > 1) AND (NOT EXISTS (SELECT 1 FROM (SELECT LOWER([[__data_mm_users_rel_many.email]]) as [[multiMatchValue]] FROM `demo1` `__mm_demo1` LEFT JOIN `users` `__data_mm_users_rel_many` ON [[__data_mm_users_rel_many.id]] IN ({:p4}, {:p5}) WHERE `__mm_demo1`.`id` = `demo1`.`id`) {{__smTEST}} WHERE NOT ([[__smTEST.multiMatchValue]] > 1)))) OR LOWER([[demo1.text]]) > 1 OR LOWER([[demo1.bool]]) > 1 OR LOWER([[demo1.url]]) > 1 OR LOWER([[demo1.select_one]]) > 1 OR LOWER([[demo1.select_many]]) > 1 OR LOWER([[demo1.file_one]]) > 1 OR LOWER([[demo1.file_many]]) > 1 OR LOWER([[demo1.number]]) > 1 OR LOWER([[demo1.email]]) > 1 OR LOWER([[demo1.datetime]]) > 1 OR LOWER((CASE WHEN json_valid([[demo1.json]]) THEN JSON_EXTRACT([[demo1.json]], '$') ELSE JSON_EXTRACT(json_object('pb', [[demo1.json]]), '$.pb') END)) > 1 OR LOWER([[demo1.rel_one]]) > 1 OR LOWER([[demo1.rel_many]]) > 1 OR ((LOWER([[demo1_rel_many.name]]) > 1) AND (NOT EXISTS (SELECT 1 FROM (SELECT LOWER([[__mm_demo1_rel_many.name]]) as [[multiMatchValue]] FROM `demo1` `__mm_demo1` LEFT JOIN json_each(CASE WHEN json_valid([[__mm_demo1.rel_many]]) THEN [[__mm_demo1.rel_many]] ELSE json_array([[__mm_demo1.rel_many]]) END) `__mm_demo1_rel_many_je` LEFT JOIN `users` `__mm_demo1_rel_many` ON [[__mm_demo1_rel_many.id]] = [[__mm_demo1_rel_many_je.value]] WHERE `__mm_demo1`.`id` = `demo1`.`id`) {{__smTEST}} WHERE NOT ([[__smTEST.multiMatchValue]] > 1)))) OR LOWER([[demo1.created]]) > 1)",
			*/
			// PostgreSQL:
			`SELECT DISTINCT "demo1".* FROM "demo1" LEFT JOIN "users" "__data_users_rel_many" ON [[__data_users_rel_many.id]] IN ({:p0}, {:p1}) LEFT JOIN jsonb_array_elements_text(CASE WHEN ([[demo1.rel_many]] IS JSON OR json_valid([[demo1.rel_many]]::text)) AND jsonb_typeof([[demo1.rel_many]]::jsonb) = 'array' THEN [[demo1.rel_many]]::jsonb ELSE jsonb_build_array([[demo1.rel_many]]) END) "demo1_rel_many_je" ON 1=1 LEFT JOIN "users" "demo1_rel_many" ON [[demo1_rel_many.id]] = [[demo1_rel_many_je.value]] WHERE (LOWER({:infoLowerrel_oneTEST})::numeric > TRUE::numeric OR LOWER({:infoLowerrel_manyTEST})::numeric > TRUE::numeric OR ((LOWER([[__data_users_rel_many.email]])::numeric > TRUE::numeric) AND (NOT EXISTS (SELECT 1 FROM (SELECT LOWER([[__data_mm_users_rel_many.email]]) as [[multiMatchValue]] FROM "demo1" "__mm_demo1" LEFT JOIN "users" "__data_mm_users_rel_many" ON [[__data_mm_users_rel_many.id]] IN ({:p4}, {:p5}) WHERE "__mm_demo1"."id" = "demo1"."id") {{__smTEST}} WHERE NOT ([[__smTEST.multiMatchValue]]::numeric > TRUE::numeric)))) OR LOWER([[demo1.text]])::numeric > TRUE::numeric OR LOWER([[demo1.bool]])::numeric > TRUE::numeric OR LOWER([[demo1.url]])::numeric > TRUE::numeric OR LOWER([[demo1.select_one]])::numeric > TRUE::numeric OR LOWER([[demo1.select_many]])::numeric > TRUE::numeric OR LOWER([[demo1.file_one]])::numeric > TRUE::numeric OR LOWER([[demo1.file_many]])::numeric > TRUE::numeric OR LOWER([[demo1.number]])::numeric > TRUE::numeric OR LOWER([[demo1.email]])::numeric > TRUE::numeric OR LOWER([[demo1.datetime]])::numeric > TRUE::numeric OR LOWER(((CASE WHEN [[demo1.json]] IS JSON OR json_valid([[demo1.json]]::text) THEN JSON_QUERY([[demo1.json]]::jsonb, '$') ELSE NULL END) #>> '{}')::text)::numeric > TRUE::numeric OR LOWER([[demo1.rel_one]])::numeric > TRUE::numeric OR LOWER([[demo1.rel_many]])::numeric > TRUE::numeric OR ((LOWER([[demo1_rel_many.name]])::numeric > TRUE::numeric) AND (NOT EXISTS (SELECT 1 FROM (SELECT LOWER([[__mm_demo1_rel_many.name]]) as [[multiMatchValue]] FROM "demo1" "__mm_demo1" LEFT JOIN jsonb_array_elements_text(CASE WHEN ([[__mm_demo1.rel_many]] IS JSON OR json_valid([[__mm_demo1.rel_many]]::text)) AND jsonb_typeof([[__mm_demo1.rel_many]]::jsonb) = 'array' THEN [[__mm_demo1.rel_many]]::jsonb ELSE jsonb_build_array([[__mm_demo1.rel_many]]) END) "__mm_demo1_rel_many_je" ON 1=1 LEFT JOIN "users" "__mm_demo1_rel_many" ON [[__mm_demo1_rel_many.id]] = [[__mm_demo1_rel_many_je.value]] WHERE "__mm_demo1"."id" = "demo1"."id") {{__smTEST}} WHERE NOT ([[__smTEST.multiMatchValue]]::numeric > TRUE::numeric)))) OR LOWER([[demo1.created]])::numeric > TRUE::numeric)`,
		},
		{
			"isset modifier",
			"demo1",
			"@request.body.a:isset > true ||" +
				"@request.body.b:isset > true ||" +
				"@request.body.c:isset > true ||" +
				"@request.query.a:isset > true ||" +
				"@request.query.b:isset > true ||" +
				"@request.query.c:isset > true ||" +
				"@request.headers.a:isset > true ||" +
				"@request.headers.c:isset > true",
			false,
			/* SQLite:
			"SELECT `demo1`.* FROM `demo1` WHERE (TRUE > 1 OR TRUE > 1 OR FALSE > 1 OR TRUE > 1 OR TRUE > 1 OR FALSE > 1 OR TRUE > 1 OR FALSE > 1)",
			*/
			// PostgreSQL:
			`SELECT "demo1".* FROM "demo1" WHERE (TRUE::numeric > TRUE::numeric OR TRUE::numeric > TRUE::numeric OR FALSE::numeric > TRUE::numeric OR TRUE::numeric > TRUE::numeric OR TRUE::numeric > TRUE::numeric OR FALSE::numeric > TRUE::numeric OR TRUE::numeric > TRUE::numeric OR FALSE::numeric > TRUE::numeric)`,
		},
		{
			"@request.body.rel.* fields",
			"demo4",
			"@request.body.rel_one_cascade.title > true &&" +
				// reference the same as rel_one_cascade collection but should use a different join alias
				"@request.body.rel_one_no_cascade.title < true &&" +
				// different collection
				"@request.body.self_rel_many.title = true",
			false,
			/* SQLite:
			"SELECT DISTINCT `demo4`.* FROM `demo4` LEFT JOIN `demo3` `__data_demo3_rel_one_cascade` ON [[__data_demo3_rel_one_cascade.id]]={:p0} LEFT JOIN `demo3` `__data_demo3_rel_one_no_cascade` ON [[__data_demo3_rel_one_no_cascade.id]]={:p1} LEFT JOIN `demo4` `__data_demo4_self_rel_many` ON [[__data_demo4_self_rel_many.id]]={:p2} WHERE ([[__data_demo3_rel_one_cascade.title]] > 1 AND [[__data_demo3_rel_one_no_cascade.title]] < 1 AND (([[__data_demo4_self_rel_many.title]] = 1) AND (NOT EXISTS (SELECT 1 FROM (SELECT [[__data_mm_demo4_self_rel_many.title]] as [[multiMatchValue]] FROM `demo4` `__mm_demo4` LEFT JOIN `demo4` `__data_mm_demo4_self_rel_many` ON [[__data_mm_demo4_self_rel_many.id]]={:p3} WHERE `__mm_demo4`.`id` = `demo4`.`id`) {{__smTEST}} WHERE NOT ([[__smTEST.multiMatchValue]] = 1)))))",
			*/
			// PostgreSQL:
			`SELECT DISTINCT "demo4".* FROM "demo4" LEFT JOIN "demo3" "__data_demo3_rel_one_cascade" ON [[__data_demo3_rel_one_cascade.id]]={:p0} LEFT JOIN "demo3" "__data_demo3_rel_one_no_cascade" ON [[__data_demo3_rel_one_no_cascade.id]]={:p1} LEFT JOIN "demo4" "__data_demo4_self_rel_many" ON [[__data_demo4_self_rel_many.id]]={:p2} WHERE ([[__data_demo3_rel_one_cascade.title]]::numeric > TRUE::numeric AND [[__data_demo3_rel_one_no_cascade.title]]::numeric < TRUE::numeric AND (([[__data_demo4_self_rel_many.title]] = TRUE) AND (NOT EXISTS (SELECT 1 FROM (SELECT [[__data_mm_demo4_self_rel_many.title]] as [[multiMatchValue]] FROM "demo4" "__mm_demo4" LEFT JOIN "demo4" "__data_mm_demo4_self_rel_many" ON [[__data_mm_demo4_self_rel_many.id]]={:p3} WHERE "__mm_demo4"."id" = "demo4"."id") {{__smTEST}} WHERE NOT ([[__smTEST.multiMatchValue]] = TRUE)))))`,
		},
		{
			"@request.body.arrayble:each fields",
			"demo1",
			"@request.body.select_one:each > true &&" +
				"@request.body.select_one:each ?< true &&" +
				"@request.body.select_many:each > true &&" +
				"@request.body.select_many:each ?< true &&" +
				"@request.body.file_one:each > true &&" +
				"@request.body.file_one:each ?< true &&" +
				"@request.body.file_many:each > true &&" +
				"@request.body.file_many:each ?< true &&" +
				"@request.body.rel_one:each > true &&" +
				"@request.body.rel_one:each ?< true &&" +
				"@request.body.rel_many:each > true &&" +
				"@request.body.rel_many:each ?< true",
			false,
			/* SQLite
			"SELECT DISTINCT `demo1`.* FROM `demo1` LEFT JOIN json_each({:dataEachTEST}) `__dataEach_select_one_je` LEFT JOIN json_each({:dataEachTEST}) `__dataEach_select_many_je` LEFT JOIN json_each({:dataEachTEST}) `__dataEach_file_one_je` LEFT JOIN json_each({:dataEachTEST}) `__dataEach_file_many_je` LEFT JOIN json_each({:dataEachTEST}) `__dataEach_rel_one_je` LEFT JOIN json_each({:dataEachTEST}) `__dataEach_rel_many_je` WHERE ([[__dataEach_select_one_je.value]] > 1 AND [[__dataEach_select_one_je.value]] < 1 AND (([[__dataEach_select_many_je.value]] > 1) AND (NOT EXISTS (SELECT 1 FROM (SELECT [[__mm__dataEach_select_many_je.value]] as [[multiMatchValue]] FROM `demo1` `__mm_demo1` LEFT JOIN json_each({:mmdataEachTEST}) `__mm__dataEach_select_many_je` WHERE `__mm_demo1`.`id` = `demo1`.`id`) {{__smTEST}} WHERE NOT ([[__smTEST.multiMatchValue]] > 1)))) AND [[__dataEach_select_many_je.value]] < 1 AND [[__dataEach_file_one_je.value]] > 1 AND [[__dataEach_file_one_je.value]] < 1 AND (([[__dataEach_file_many_je.value]] > 1) AND (NOT EXISTS (SELECT 1 FROM (SELECT [[__mm__dataEach_file_many_je.value]] as [[multiMatchValue]] FROM `demo1` `__mm_demo1` LEFT JOIN json_each({:mmdataEachTEST}) `__mm__dataEach_file_many_je` WHERE `__mm_demo1`.`id` = `demo1`.`id`) {{__smTEST}} WHERE NOT ([[__smTEST.multiMatchValue]] > 1)))) AND [[__dataEach_file_many_je.value]] < 1 AND [[__dataEach_rel_one_je.value]] > 1 AND [[__dataEach_rel_one_je.value]] < 1 AND (([[__dataEach_rel_many_je.value]] > 1) AND (NOT EXISTS (SELECT 1 FROM (SELECT [[__mm__dataEach_rel_many_je.value]] as [[multiMatchValue]] FROM `demo1` `__mm_demo1` LEFT JOIN json_each({:mmdataEachTEST}) `__mm__dataEach_rel_many_je` WHERE `__mm_demo1`.`id` = `demo1`.`id`) {{__smTEST}} WHERE NOT ([[__smTEST.multiMatchValue]] > 1)))) AND [[__dataEach_rel_many_je.value]] < 1)",
			*/
			// PostgreSQL:
			`SELECT DISTINCT "demo1".* FROM "demo1" LEFT JOIN jsonb_array_elements_text({:dataEachTEST}::jsonb) "__dataEach_select_one_je" ON 1=1 LEFT JOIN jsonb_array_elements_text({:dataEachTEST}::jsonb) "__dataEach_select_many_je" ON 1=1 LEFT JOIN jsonb_array_elements_text({:dataEachTEST}::jsonb) "__dataEach_file_one_je" ON 1=1 LEFT JOIN jsonb_array_elements_text({:dataEachTEST}::jsonb) "__dataEach_file_many_je" ON 1=1 LEFT JOIN jsonb_array_elements_text({:dataEachTEST}::jsonb) "__dataEach_rel_one_je" ON 1=1 LEFT JOIN jsonb_array_elements_text({:dataEachTEST}::jsonb) "__dataEach_rel_many_je" ON 1=1 WHERE ([[__dataEach_select_one_je.value]]::numeric > TRUE::numeric AND [[__dataEach_select_one_je.value]]::numeric < TRUE::numeric AND (([[__dataEach_select_many_je.value]]::numeric > TRUE::numeric) AND (NOT EXISTS (SELECT 1 FROM (SELECT [[__mm__dataEach_select_many_je.value]] as [[multiMatchValue]] FROM "demo1" "__mm_demo1" LEFT JOIN jsonb_array_elements_text({:mmdataEachTEST}::jsonb) "__mm__dataEach_select_many_je" ON 1=1 WHERE "__mm_demo1"."id" = "demo1"."id") {{__smTEST}} WHERE NOT ([[__smTEST.multiMatchValue]]::numeric > TRUE::numeric)))) AND [[__dataEach_select_many_je.value]]::numeric < TRUE::numeric AND [[__dataEach_file_one_je.value]]::numeric > TRUE::numeric AND [[__dataEach_file_one_je.value]]::numeric < TRUE::numeric AND (([[__dataEach_file_many_je.value]]::numeric > TRUE::numeric) AND (NOT EXISTS (SELECT 1 FROM (SELECT [[__mm__dataEach_file_many_je.value]] as [[multiMatchValue]] FROM "demo1" "__mm_demo1" LEFT JOIN jsonb_array_elements_text({:mmdataEachTEST}::jsonb) "__mm__dataEach_file_many_je" ON 1=1 WHERE "__mm_demo1"."id" = "demo1"."id") {{__smTEST}} WHERE NOT ([[__smTEST.multiMatchValue]]::numeric > TRUE::numeric)))) AND [[__dataEach_file_many_je.value]]::numeric < TRUE::numeric AND [[__dataEach_rel_one_je.value]]::numeric > TRUE::numeric AND [[__dataEach_rel_one_je.value]]::numeric < TRUE::numeric AND (([[__dataEach_rel_many_je.value]]::numeric > TRUE::numeric) AND (NOT EXISTS (SELECT 1 FROM (SELECT [[__mm__dataEach_rel_many_je.value]] as [[multiMatchValue]] FROM "demo1" "__mm_demo1" LEFT JOIN jsonb_array_elements_text({:mmdataEachTEST}::jsonb) "__mm__dataEach_rel_many_je" ON 1=1 WHERE "__mm_demo1"."id" = "demo1"."id") {{__smTEST}} WHERE NOT ([[__smTEST.multiMatchValue]]::numeric > TRUE::numeric)))) AND [[__dataEach_rel_many_je.value]]::numeric < TRUE::numeric)`,
		},
		{
			"regular arrayble:each fields",
			"demo1",
			"select_one:each > true &&" +
				"select_one:each ?< true &&" +
				"select_many:each > true &&" +
				"select_many:each ?< true &&" +
				"file_one:each > true &&" +
				"file_one:each ?< true &&" +
				"file_many:each > true &&" +
				"file_many:each ?< true &&" +
				"rel_one:each > true &&" +
				"rel_one:each ?< true &&" +
				"rel_many:each > true &&" +
				"rel_many:each ?< true",
			false,
			/* SQLite:
			"SELECT DISTINCT `demo1`.* FROM `demo1` LEFT JOIN json_each(CASE WHEN json_valid([[demo1.select_one]]) THEN [[demo1.select_one]] ELSE json_array([[demo1.select_one]]) END) `demo1_select_one_je` LEFT JOIN json_each(CASE WHEN json_valid([[demo1.select_many]]) THEN [[demo1.select_many]] ELSE json_array([[demo1.select_many]]) END) `demo1_select_many_je` LEFT JOIN json_each(CASE WHEN json_valid([[demo1.file_one]]) THEN [[demo1.file_one]] ELSE json_array([[demo1.file_one]]) END) `demo1_file_one_je` LEFT JOIN json_each(CASE WHEN json_valid([[demo1.file_many]]) THEN [[demo1.file_many]] ELSE json_array([[demo1.file_many]]) END) `demo1_file_many_je` LEFT JOIN json_each(CASE WHEN json_valid([[demo1.rel_one]]) THEN [[demo1.rel_one]] ELSE json_array([[demo1.rel_one]]) END) `demo1_rel_one_je` LEFT JOIN json_each(CASE WHEN json_valid([[demo1.rel_many]]) THEN [[demo1.rel_many]] ELSE json_array([[demo1.rel_many]]) END) `demo1_rel_many_je` WHERE ([[demo1_select_one_je.value]] > 1 AND [[demo1_select_one_je.value]] < 1 AND (([[demo1_select_many_je.value]] > 1) AND (NOT EXISTS (SELECT 1 FROM (SELECT [[__mm_demo1_select_many_je.value]] as [[multiMatchValue]] FROM `demo1` `__mm_demo1` LEFT JOIN json_each(CASE WHEN json_valid([[__mm_demo1.select_many]]) THEN [[__mm_demo1.select_many]] ELSE json_array([[__mm_demo1.select_many]]) END) `__mm_demo1_select_many_je` WHERE `__mm_demo1`.`id` = `demo1`.`id`) {{__smTEST}} WHERE NOT ([[__smTEST.multiMatchValue]] > 1)))) AND [[demo1_select_many_je.value]] < 1 AND [[demo1_file_one_je.value]] > 1 AND [[demo1_file_one_je.value]] < 1 AND (([[demo1_file_many_je.value]] > 1) AND (NOT EXISTS (SELECT 1 FROM (SELECT [[__mm_demo1_file_many_je.value]] as [[multiMatchValue]] FROM `demo1` `__mm_demo1` LEFT JOIN json_each(CASE WHEN json_valid([[__mm_demo1.file_many]]) THEN [[__mm_demo1.file_many]] ELSE json_array([[__mm_demo1.file_many]]) END) `__mm_demo1_file_many_je` WHERE `__mm_demo1`.`id` = `demo1`.`id`) {{__smTEST}} WHERE NOT ([[__smTEST.multiMatchValue]] > 1)))) AND [[demo1_file_many_je.value]] < 1 AND [[demo1_rel_one_je.value]] > 1 AND [[demo1_rel_one_je.value]] < 1 AND (([[demo1_rel_many_je.value]] > 1) AND (NOT EXISTS (SELECT 1 FROM (SELECT [[__mm_demo1_rel_many_je.value]] as [[multiMatchValue]] FROM `demo1` `__mm_demo1` LEFT JOIN json_each(CASE WHEN json_valid([[__mm_demo1.rel_many]]) THEN [[__mm_demo1.rel_many]] ELSE json_array([[__mm_demo1.rel_many]]) END) `__mm_demo1_rel_many_je` WHERE `__mm_demo1`.`id` = `demo1`.`id`) {{__smTEST}} WHERE NOT ([[__smTEST.multiMatchValue]] > 1)))) AND [[demo1_rel_many_je.value]] < 1)",
			*/
			// PostgreSQL:
			`SELECT DISTINCT "demo1".* FROM "demo1" LEFT JOIN jsonb_array_elements_text(CASE WHEN ([[demo1.select_one]] IS JSON OR json_valid([[demo1.select_one]]::text)) AND jsonb_typeof([[demo1.select_one]]::jsonb) = 'array' THEN [[demo1.select_one]]::jsonb ELSE jsonb_build_array([[demo1.select_one]]) END) "demo1_select_one_je" ON 1=1 LEFT JOIN jsonb_array_elements_text(CASE WHEN ([[demo1.select_many]] IS JSON OR json_valid([[demo1.select_many]]::text)) AND jsonb_typeof([[demo1.select_many]]::jsonb) = 'array' THEN [[demo1.select_many]]::jsonb ELSE jsonb_build_array([[demo1.select_many]]) END) "demo1_select_many_je" ON 1=1 LEFT JOIN jsonb_array_elements_text(CASE WHEN ([[demo1.file_one]] IS JSON OR json_valid([[demo1.file_one]]::text)) AND jsonb_typeof([[demo1.file_one]]::jsonb) = 'array' THEN [[demo1.file_one]]::jsonb ELSE jsonb_build_array([[demo1.file_one]]) END) "demo1_file_one_je" ON 1=1 LEFT JOIN jsonb_array_elements_text(CASE WHEN ([[demo1.file_many]] IS JSON OR json_valid([[demo1.file_many]]::text)) AND jsonb_typeof([[demo1.file_many]]::jsonb) = 'array' THEN [[demo1.file_many]]::jsonb ELSE jsonb_build_array([[demo1.file_many]]) END) "demo1_file_many_je" ON 1=1 LEFT JOIN jsonb_array_elements_text(CASE WHEN ([[demo1.rel_one]] IS JSON OR json_valid([[demo1.rel_one]]::text)) AND jsonb_typeof([[demo1.rel_one]]::jsonb) = 'array' THEN [[demo1.rel_one]]::jsonb ELSE jsonb_build_array([[demo1.rel_one]]) END) "demo1_rel_one_je" ON 1=1 LEFT JOIN jsonb_array_elements_text(CASE WHEN ([[demo1.rel_many]] IS JSON OR json_valid([[demo1.rel_many]]::text)) AND jsonb_typeof([[demo1.rel_many]]::jsonb) = 'array' THEN [[demo1.rel_many]]::jsonb ELSE jsonb_build_array([[demo1.rel_many]]) END) "demo1_rel_many_je" ON 1=1 WHERE ([[demo1_select_one_je.value]]::numeric > TRUE::numeric AND [[demo1_select_one_je.value]]::numeric < TRUE::numeric AND (([[demo1_select_many_je.value]]::numeric > TRUE::numeric) AND (NOT EXISTS (SELECT 1 FROM (SELECT [[__mm_demo1_select_many_je.value]] as [[multiMatchValue]] FROM "demo1" "__mm_demo1" LEFT JOIN jsonb_array_elements_text(CASE WHEN ([[__mm_demo1.select_many]] IS JSON OR json_valid([[__mm_demo1.select_many]]::text)) AND jsonb_typeof([[__mm_demo1.select_many]]::jsonb) = 'array' THEN [[__mm_demo1.select_many]]::jsonb ELSE jsonb_build_array([[__mm_demo1.select_many]]) END) "__mm_demo1_select_many_je" ON 1=1 WHERE "__mm_demo1"."id" = "demo1"."id") {{__smTEST}} WHERE NOT ([[__smTEST.multiMatchValue]]::numeric > TRUE::numeric)))) AND [[demo1_select_many_je.value]]::numeric < TRUE::numeric AND [[demo1_file_one_je.value]]::numeric > TRUE::numeric AND [[demo1_file_one_je.value]]::numeric < TRUE::numeric AND (([[demo1_file_many_je.value]]::numeric > TRUE::numeric) AND (NOT EXISTS (SELECT 1 FROM (SELECT [[__mm_demo1_file_many_je.value]] as [[multiMatchValue]] FROM "demo1" "__mm_demo1" LEFT JOIN jsonb_array_elements_text(CASE WHEN ([[__mm_demo1.file_many]] IS JSON OR json_valid([[__mm_demo1.file_many]]::text)) AND jsonb_typeof([[__mm_demo1.file_many]]::jsonb) = 'array' THEN [[__mm_demo1.file_many]]::jsonb ELSE jsonb_build_array([[__mm_demo1.file_many]]) END) "__mm_demo1_file_many_je" ON 1=1 WHERE "__mm_demo1"."id" = "demo1"."id") {{__smTEST}} WHERE NOT ([[__smTEST.multiMatchValue]]::numeric > TRUE::numeric)))) AND [[demo1_file_many_je.value]]::numeric < TRUE::numeric AND [[demo1_rel_one_je.value]]::numeric > TRUE::numeric AND [[demo1_rel_one_je.value]]::numeric < TRUE::numeric AND (([[demo1_rel_many_je.value]]::numeric > TRUE::numeric) AND (NOT EXISTS (SELECT 1 FROM (SELECT [[__mm_demo1_rel_many_je.value]] as [[multiMatchValue]] FROM "demo1" "__mm_demo1" LEFT JOIN jsonb_array_elements_text(CASE WHEN ([[__mm_demo1.rel_many]] IS JSON OR json_valid([[__mm_demo1.rel_many]]::text)) AND jsonb_typeof([[__mm_demo1.rel_many]]::jsonb) = 'array' THEN [[__mm_demo1.rel_many]]::jsonb ELSE jsonb_build_array([[__mm_demo1.rel_many]]) END) "__mm_demo1_rel_many_je" ON 1=1 WHERE "__mm_demo1"."id" = "demo1"."id") {{__smTEST}} WHERE NOT ([[__smTEST.multiMatchValue]]::numeric > TRUE::numeric)))) AND [[demo1_rel_many_je.value]]::numeric < TRUE::numeric)`,
		},
		{
			"arrayble:each vs arrayble:each",
			"demo1",
			"select_one:each != select_many:each &&" +
				"select_many:each > select_one:each &&" +
				"select_many:each ?< select_one:each &&" +
				"select_many:each = @request.body.select_many:each",
			false,
			/* SQLite:
			"SELECT DISTINCT `demo1`.* FROM `demo1` LEFT JOIN json_each(CASE WHEN json_valid([[demo1.select_one]]) THEN [[demo1.select_one]] ELSE json_array([[demo1.select_one]]) END) `demo1_select_one_je` LEFT JOIN json_each(CASE WHEN json_valid([[demo1.select_many]]) THEN [[demo1.select_many]] ELSE json_array([[demo1.select_many]]) END) `demo1_select_many_je` LEFT JOIN json_each({:dataEachTEST}) `__dataEach_select_many_je` WHERE (((COALESCE([[demo1_select_one_je.value]], '') IS NOT COALESCE([[demo1_select_many_je.value]], '')) AND (NOT EXISTS (SELECT 1 FROM (SELECT [[__mm_demo1_select_many_je.value]] as [[multiMatchValue]] FROM `demo1` `__mm_demo1` LEFT JOIN json_each(CASE WHEN json_valid([[__mm_demo1.select_many]]) THEN [[__mm_demo1.select_many]] ELSE json_array([[__mm_demo1.select_many]]) END) `__mm_demo1_select_many_je` WHERE `__mm_demo1`.`id` = `demo1`.`id`) {{__smTEST}} WHERE NOT (COALESCE([[demo1_select_one_je.value]], '') IS NOT COALESCE([[__smTEST.multiMatchValue]], ''))))) AND (([[demo1_select_many_je.value]] > [[demo1_select_one_je.value]]) AND (NOT EXISTS (SELECT 1 FROM (SELECT [[__mm_demo1_select_many_je.value]] as [[multiMatchValue]] FROM `demo1` `__mm_demo1` LEFT JOIN json_each(CASE WHEN json_valid([[__mm_demo1.select_many]]) THEN [[__mm_demo1.select_many]] ELSE json_array([[__mm_demo1.select_many]]) END) `__mm_demo1_select_many_je` WHERE `__mm_demo1`.`id` = `demo1`.`id`) {{__smTEST}} WHERE NOT ([[__smTEST.multiMatchValue]] > [[demo1_select_one_je.value]])))) AND [[demo1_select_many_je.value]] < [[demo1_select_one_je.value]] AND (([[demo1_select_many_je.value]] = [[__dataEach_select_many_je.value]]) AND (NOT EXISTS (SELECT 1 FROM (SELECT [[__mm_demo1_select_many_je.value]] as [[multiMatchValue]] FROM `demo1` `__mm_demo1` LEFT JOIN json_each(CASE WHEN json_valid([[__mm_demo1.select_many]]) THEN [[__mm_demo1.select_many]] ELSE json_array([[__mm_demo1.select_many]]) END) `__mm_demo1_select_many_je` WHERE `__mm_demo1`.`id` = `demo1`.`id`) {{__mlTEST}} LEFT JOIN (SELECT [[__mm__dataEach_select_many_je.value]] as [[multiMatchValue]] FROM `demo1` `__mm_demo1` LEFT JOIN json_each({:mmdataEachTEST}) `__mm__dataEach_select_many_je` WHERE `__mm_demo1`.`id` = `demo1`.`id`) {{__mrTEST}} WHERE NOT (COALESCE([[__mlTEST.multiMatchValue]], '') = COALESCE([[__mrTEST.multiMatchValue]], ''))))))",
			*/
			// PostgreSQL:
			`SELECT DISTINCT "demo1".* FROM "demo1" LEFT JOIN jsonb_array_elements_text(CASE WHEN ([[demo1.select_one]] IS JSON OR json_valid([[demo1.select_one]]::text)) AND jsonb_typeof([[demo1.select_one]]::jsonb) = 'array' THEN [[demo1.select_one]]::jsonb ELSE jsonb_build_array([[demo1.select_one]]) END) "demo1_select_one_je" ON 1=1 LEFT JOIN jsonb_array_elements_text(CASE WHEN ([[demo1.select_many]] IS JSON OR json_valid([[demo1.select_many]]::text)) AND jsonb_typeof([[demo1.select_many]]::jsonb) = 'array' THEN [[demo1.select_many]]::jsonb ELSE jsonb_build_array([[demo1.select_many]]) END) "demo1_select_many_je" ON 1=1 LEFT JOIN jsonb_array_elements_text({:dataEachTEST}::jsonb) "__dataEach_select_many_je" ON 1=1 WHERE ((([[demo1_select_one_je.value]] IS DISTINCT FROM [[demo1_select_many_je.value]]) AND (NOT EXISTS (SELECT 1 FROM (SELECT [[__mm_demo1_select_many_je.value]] as [[multiMatchValue]] FROM "demo1" "__mm_demo1" LEFT JOIN jsonb_array_elements_text(CASE WHEN ([[__mm_demo1.select_many]] IS JSON OR json_valid([[__mm_demo1.select_many]]::text)) AND jsonb_typeof([[__mm_demo1.select_many]]::jsonb) = 'array' THEN [[__mm_demo1.select_many]]::jsonb ELSE jsonb_build_array([[__mm_demo1.select_many]]) END) "__mm_demo1_select_many_je" ON 1=1 WHERE "__mm_demo1"."id" = "demo1"."id") {{__smTEST}} WHERE NOT ([[demo1_select_one_je.value]] IS DISTINCT FROM [[__smTEST.multiMatchValue]])))) AND (([[demo1_select_many_je.value]]::numeric > [[demo1_select_one_je.value]]::numeric) AND (NOT EXISTS (SELECT 1 FROM (SELECT [[__mm_demo1_select_many_je.value]] as [[multiMatchValue]] FROM "demo1" "__mm_demo1" LEFT JOIN jsonb_array_elements_text(CASE WHEN ([[__mm_demo1.select_many]] IS JSON OR json_valid([[__mm_demo1.select_many]]::text)) AND jsonb_typeof([[__mm_demo1.select_many]]::jsonb) = 'array' THEN [[__mm_demo1.select_many]]::jsonb ELSE jsonb_build_array([[__mm_demo1.select_many]]) END) "__mm_demo1_select_many_je" ON 1=1 WHERE "__mm_demo1"."id" = "demo1"."id") {{__smTEST}} WHERE NOT ([[__smTEST.multiMatchValue]]::numeric > [[demo1_select_one_je.value]]::numeric)))) AND [[demo1_select_many_je.value]]::numeric < [[demo1_select_one_je.value]]::numeric AND (([[demo1_select_many_je.value]] = [[__dataEach_select_many_je.value]]) AND (NOT EXISTS (SELECT 1 FROM (SELECT [[__mm_demo1_select_many_je.value]] as [[multiMatchValue]] FROM "demo1" "__mm_demo1" LEFT JOIN jsonb_array_elements_text(CASE WHEN ([[__mm_demo1.select_many]] IS JSON OR json_valid([[__mm_demo1.select_many]]::text)) AND jsonb_typeof([[__mm_demo1.select_many]]::jsonb) = 'array' THEN [[__mm_demo1.select_many]]::jsonb ELSE jsonb_build_array([[__mm_demo1.select_many]]) END) "__mm_demo1_select_many_je" ON 1=1 WHERE "__mm_demo1"."id" = "demo1"."id") {{__mlTEST}} LEFT JOIN (SELECT [[__mm__dataEach_select_many_je.value]] as [[multiMatchValue]] FROM "demo1" "__mm_demo1" LEFT JOIN jsonb_array_elements_text({:mmdataEachTEST}::jsonb) "__mm__dataEach_select_many_je" ON 1=1 WHERE "__mm_demo1"."id" = "demo1"."id") {{__mrTEST}} ON 1 = 1 WHERE NOT ([[__mlTEST.multiMatchValue]] IS NOT DISTINCT FROM [[__mrTEST.multiMatchValue]])))))`,
		},
		{
			"mixed multi-match vs multi-match",
			"demo1",
			"rel_many.rel.active != rel_many.name &&" +
				"rel_many.rel.active ?= rel_many.name &&" +
				"rel_many.rel.title ~ rel_one.email &&" +
				"@collection.demo2.active = rel_many.rel.active &&" +
				"@collection.demo2.active ?= rel_many.rel.active &&" +
				"rel_many.email > @request.body.rel_many.email",
			false,
			/* SQLite:
			"SELECT DISTINCT `demo1`.* FROM `demo1` LEFT JOIN json_each(CASE WHEN json_valid([[demo1.rel_many]]) THEN [[demo1.rel_many]] ELSE json_array([[demo1.rel_many]]) END) `demo1_rel_many_je` LEFT JOIN `users` `demo1_rel_many` ON [[demo1_rel_many.id]] = [[demo1_rel_many_je.value]] LEFT JOIN `demo2` `demo1_rel_many_rel` ON [[demo1_rel_many_rel.id]] = [[demo1_rel_many.rel]] LEFT JOIN `demo1` `demo1_rel_one` ON [[demo1_rel_one.id]] = [[demo1.rel_one]] LEFT JOIN `demo2` `__collection_demo2` LEFT JOIN `users` `__data_users_rel_many` ON [[__data_users_rel_many.id]] IN ({:p0}, {:p1}) WHERE (((COALESCE([[demo1_rel_many_rel.active]], '') IS NOT COALESCE([[demo1_rel_many.name]], '')) AND (NOT EXISTS (SELECT 1 FROM (SELECT [[__mm_demo1_rel_many_rel.active]] as [[multiMatchValue]] FROM `demo1` `__mm_demo1` LEFT JOIN json_each(CASE WHEN json_valid([[__mm_demo1.rel_many]]) THEN [[__mm_demo1.rel_many]] ELSE json_array([[__mm_demo1.rel_many]]) END) `__mm_demo1_rel_many_je` LEFT JOIN `users` `__mm_demo1_rel_many` ON [[__mm_demo1_rel_many.id]] = [[__mm_demo1_rel_many_je.value]] LEFT JOIN `demo2` `__mm_demo1_rel_many_rel` ON [[__mm_demo1_rel_many_rel.id]] = [[__mm_demo1_rel_many.rel]] WHERE `__mm_demo1`.`id` = `demo1`.`id`) {{__mlTEST}} LEFT JOIN (SELECT [[__mm_demo1_rel_many.name]] as [[multiMatchValue]] FROM `demo1` `__mm_demo1` LEFT JOIN json_each(CASE WHEN json_valid([[__mm_demo1.rel_many]]) THEN [[__mm_demo1.rel_many]] ELSE json_array([[__mm_demo1.rel_many]]) END) `__mm_demo1_rel_many_je` LEFT JOIN `users` `__mm_demo1_rel_many` ON [[__mm_demo1_rel_many.id]] = [[__mm_demo1_rel_many_je.value]] WHERE `__mm_demo1`.`id` = `demo1`.`id`) {{__mrTEST}} WHERE NOT (COALESCE([[__mlTEST.multiMatchValue]], '') IS NOT COALESCE([[__mrTEST.multiMatchValue]], ''))))) AND COALESCE([[demo1_rel_many_rel.active]], '') = COALESCE([[demo1_rel_many.name]], '') AND (([[demo1_rel_many_rel.title]] LIKE ('%' || [[demo1_rel_one.email]] || '%') ESCAPE '\\') AND (NOT EXISTS (SELECT 1 FROM (SELECT [[__mm_demo1_rel_many_rel.title]] as [[multiMatchValue]] FROM `demo1` `__mm_demo1` LEFT JOIN json_each(CASE WHEN json_valid([[__mm_demo1.rel_many]]) THEN [[__mm_demo1.rel_many]] ELSE json_array([[__mm_demo1.rel_many]]) END) `__mm_demo1_rel_many_je` LEFT JOIN `users` `__mm_demo1_rel_many` ON [[__mm_demo1_rel_many.id]] = [[__mm_demo1_rel_many_je.value]] LEFT JOIN `demo2` `__mm_demo1_rel_many_rel` ON [[__mm_demo1_rel_many_rel.id]] = [[__mm_demo1_rel_many.rel]] WHERE `__mm_demo1`.`id` = `demo1`.`id`) {{__smTEST}} WHERE NOT ([[__smTEST.multiMatchValue]] LIKE ('%' || [[demo1_rel_one.email]] || '%') ESCAPE '\\')))) AND ((COALESCE([[__collection_demo2.active]], '') = COALESCE([[demo1_rel_many_rel.active]], '')) AND (NOT EXISTS (SELECT 1 FROM (SELECT [[__mm__collection_demo2.active]] as [[multiMatchValue]] FROM `demo1` `__mm_demo1` LEFT JOIN `demo2` `__mm__collection_demo2` WHERE `__mm_demo1`.`id` = `demo1`.`id`) {{__mlTEST}} LEFT JOIN (SELECT [[__mm_demo1_rel_many_rel.active]] as [[multiMatchValue]] FROM `demo1` `__mm_demo1` LEFT JOIN json_each(CASE WHEN json_valid([[__mm_demo1.rel_many]]) THEN [[__mm_demo1.rel_many]] ELSE json_array([[__mm_demo1.rel_many]]) END) `__mm_demo1_rel_many_je` LEFT JOIN `users` `__mm_demo1_rel_many` ON [[__mm_demo1_rel_many.id]] = [[__mm_demo1_rel_many_je.value]] LEFT JOIN `demo2` `__mm_demo1_rel_many_rel` ON [[__mm_demo1_rel_many_rel.id]] = [[__mm_demo1_rel_many.rel]] WHERE `__mm_demo1`.`id` = `demo1`.`id`) {{__mrTEST}} WHERE NOT (COALESCE([[__mlTEST.multiMatchValue]], '') = COALESCE([[__mrTEST.multiMatchValue]], ''))))) AND COALESCE([[__collection_demo2.active]], '') = COALESCE([[demo1_rel_many_rel.active]], '') AND (((([[demo1_rel_many.email]] > [[__data_users_rel_many.email]]) AND (NOT EXISTS (SELECT 1 FROM (SELECT [[__mm_demo1_rel_many.email]] as [[multiMatchValue]] FROM `demo1` `__mm_demo1` LEFT JOIN json_each(CASE WHEN json_valid([[__mm_demo1.rel_many]]) THEN [[__mm_demo1.rel_many]] ELSE json_array([[__mm_demo1.rel_many]]) END) `__mm_demo1_rel_many_je` LEFT JOIN `users` `__mm_demo1_rel_many` ON [[__mm_demo1_rel_many.id]] = [[__mm_demo1_rel_many_je.value]] WHERE `__mm_demo1`.`id` = `demo1`.`id`) {{__mlTEST}} LEFT JOIN (SELECT [[__data_mm_users_rel_many.email]] as [[multiMatchValue]] FROM `demo1` `__mm_demo1` LEFT JOIN `users` `__data_mm_users_rel_many` ON [[__data_mm_users_rel_many.id]] IN ({:p2}, {:p3}) WHERE `__mm_demo1`.`id` = `demo1`.`id`) {{__mrTEST}} WHERE NOT ([[__mlTEST.multiMatchValue]] > [[__mrTEST.multiMatchValue]]))))) AND ([[demo1_rel_many.emailVisibility]] = TRUE)))",
			*/
			// PostgreSQL:
			`SELECT DISTINCT "demo1".* FROM "demo1" LEFT JOIN jsonb_array_elements_text(CASE WHEN ([[demo1.rel_many]] IS JSON OR json_valid([[demo1.rel_many]]::text)) AND jsonb_typeof([[demo1.rel_many]]::jsonb) = 'array' THEN [[demo1.rel_many]]::jsonb ELSE jsonb_build_array([[demo1.rel_many]]) END) "demo1_rel_many_je" ON 1=1 LEFT JOIN "users" "demo1_rel_many" ON [[demo1_rel_many.id]] = [[demo1_rel_many_je.value]] LEFT JOIN "demo2" "demo1_rel_many_rel" ON [[demo1_rel_many_rel.id]] = [[demo1_rel_many.rel]] LEFT JOIN "demo1" "demo1_rel_one" ON [[demo1_rel_one.id]] = [[demo1.rel_one]] LEFT JOIN "demo2" "__collection_demo2" ON 1=1 LEFT JOIN "users" "__data_users_rel_many" ON [[__data_users_rel_many.id]] IN ({:p0}, {:p1}) WHERE ((([[demo1_rel_many_rel.active]] IS DISTINCT FROM [[demo1_rel_many.name]]) AND (NOT EXISTS (SELECT 1 FROM (SELECT [[__mm_demo1_rel_many_rel.active]] as [[multiMatchValue]] FROM "demo1" "__mm_demo1" LEFT JOIN jsonb_array_elements_text(CASE WHEN ([[__mm_demo1.rel_many]] IS JSON OR json_valid([[__mm_demo1.rel_many]]::text)) AND jsonb_typeof([[__mm_demo1.rel_many]]::jsonb) = 'array' THEN [[__mm_demo1.rel_many]]::jsonb ELSE jsonb_build_array([[__mm_demo1.rel_many]]) END) "__mm_demo1_rel_many_je" ON 1=1 LEFT JOIN "users" "__mm_demo1_rel_many" ON [[__mm_demo1_rel_many.id]] = [[__mm_demo1_rel_many_je.value]] LEFT JOIN "demo2" "__mm_demo1_rel_many_rel" ON [[__mm_demo1_rel_many_rel.id]] = [[__mm_demo1_rel_many.rel]] WHERE "__mm_demo1"."id" = "demo1"."id") {{__mlTEST}} LEFT JOIN (SELECT [[__mm_demo1_rel_many.name]] as [[multiMatchValue]] FROM "demo1" "__mm_demo1" LEFT JOIN jsonb_array_elements_text(CASE WHEN ([[__mm_demo1.rel_many]] IS JSON OR json_valid([[__mm_demo1.rel_many]]::text)) AND jsonb_typeof([[__mm_demo1.rel_many]]::jsonb) = 'array' THEN [[__mm_demo1.rel_many]]::jsonb ELSE jsonb_build_array([[__mm_demo1.rel_many]]) END) "__mm_demo1_rel_many_je" ON 1=1 LEFT JOIN "users" "__mm_demo1_rel_many" ON [[__mm_demo1_rel_many.id]] = [[__mm_demo1_rel_many_je.value]] WHERE "__mm_demo1"."id" = "demo1"."id") {{__mrTEST}} ON 1 = 1 WHERE NOT ([[__mlTEST.multiMatchValue]] IS DISTINCT FROM [[__mrTEST.multiMatchValue]])))) AND [[demo1_rel_many_rel.active]] IS NOT DISTINCT FROM [[demo1_rel_many.name]] AND (([[demo1_rel_many_rel.title]] LIKE ('%' || [[demo1_rel_one.email]] || '%') ESCAPE '\') AND (NOT EXISTS (SELECT 1 FROM (SELECT [[__mm_demo1_rel_many_rel.title]] as [[multiMatchValue]] FROM "demo1" "__mm_demo1" LEFT JOIN jsonb_array_elements_text(CASE WHEN ([[__mm_demo1.rel_many]] IS JSON OR json_valid([[__mm_demo1.rel_many]]::text)) AND jsonb_typeof([[__mm_demo1.rel_many]]::jsonb) = 'array' THEN [[__mm_demo1.rel_many]]::jsonb ELSE jsonb_build_array([[__mm_demo1.rel_many]]) END) "__mm_demo1_rel_many_je" ON 1=1 LEFT JOIN "users" "__mm_demo1_rel_many" ON [[__mm_demo1_rel_many.id]] = [[__mm_demo1_rel_many_je.value]] LEFT JOIN "demo2" "__mm_demo1_rel_many_rel" ON [[__mm_demo1_rel_many_rel.id]] = [[__mm_demo1_rel_many.rel]] WHERE "__mm_demo1"."id" = "demo1"."id") {{__smTEST}} WHERE NOT ([[__smTEST.multiMatchValue]] LIKE ('%' || [[demo1_rel_one.email]] || '%') ESCAPE '\')))) AND (([[__collection_demo2.active]] IS NOT DISTINCT FROM [[demo1_rel_many_rel.active]]) AND (NOT EXISTS (SELECT 1 FROM (SELECT [[__mm__collection_demo2.active]] as [[multiMatchValue]] FROM "demo1" "__mm_demo1" LEFT JOIN "demo2" "__mm__collection_demo2" ON 1=1 WHERE "__mm_demo1"."id" = "demo1"."id") {{__mlTEST}} LEFT JOIN (SELECT [[__mm_demo1_rel_many_rel.active]] as [[multiMatchValue]] FROM "demo1" "__mm_demo1" LEFT JOIN jsonb_array_elements_text(CASE WHEN ([[__mm_demo1.rel_many]] IS JSON OR json_valid([[__mm_demo1.rel_many]]::text)) AND jsonb_typeof([[__mm_demo1.rel_many]]::jsonb) = 'array' THEN [[__mm_demo1.rel_many]]::jsonb ELSE jsonb_build_array([[__mm_demo1.rel_many]]) END) "__mm_demo1_rel_many_je" ON 1=1 LEFT JOIN "users" "__mm_demo1_rel_many" ON [[__mm_demo1_rel_many.id]] = [[__mm_demo1_rel_many_je.value]] LEFT JOIN "demo2" "__mm_demo1_rel_many_rel" ON [[__mm_demo1_rel_many_rel.id]] = [[__mm_demo1_rel_many.rel]] WHERE "__mm_demo1"."id" = "demo1"."id") {{__mrTEST}} ON 1 = 1 WHERE NOT ([[__mlTEST.multiMatchValue]] IS NOT DISTINCT FROM [[__mrTEST.multiMatchValue]])))) AND [[__collection_demo2.active]] IS NOT DISTINCT FROM [[demo1_rel_many_rel.active]] AND (((([[demo1_rel_many.email]]::numeric > [[__data_users_rel_many.email]]::numeric) AND (NOT EXISTS (SELECT 1 FROM (SELECT [[__mm_demo1_rel_many.email]] as [[multiMatchValue]] FROM "demo1" "__mm_demo1" LEFT JOIN jsonb_array_elements_text(CASE WHEN ([[__mm_demo1.rel_many]] IS JSON OR json_valid([[__mm_demo1.rel_many]]::text)) AND jsonb_typeof([[__mm_demo1.rel_many]]::jsonb) = 'array' THEN [[__mm_demo1.rel_many]]::jsonb ELSE jsonb_build_array([[__mm_demo1.rel_many]]) END) "__mm_demo1_rel_many_je" ON 1=1 LEFT JOIN "users" "__mm_demo1_rel_many" ON [[__mm_demo1_rel_many.id]] = [[__mm_demo1_rel_many_je.value]] WHERE "__mm_demo1"."id" = "demo1"."id") {{__mlTEST}} LEFT JOIN (SELECT [[__data_mm_users_rel_many.email]] as [[multiMatchValue]] FROM "demo1" "__mm_demo1" LEFT JOIN "users" "__data_mm_users_rel_many" ON [[__data_mm_users_rel_many.id]] IN ({:p2}, {:p3}) WHERE "__mm_demo1"."id" = "demo1"."id") {{__mrTEST}} ON 1 = 1 WHERE NOT ([[__mlTEST.multiMatchValue]]::numeric > [[__mrTEST.multiMatchValue]]::numeric))))) AND ([[demo1_rel_many.emailVisibility]] = TRUE)))`,
		},
		{
			"@request.body.arrayable:length fields",
			"demo1",
			"@request.body.select_one:length > 1 &&" +
				"@request.body.select_one:length ?> 2 &&" +
				"@request.body.select_many:length < 3 &&" +
				"@request.body.select_many:length ?> 4 &&" +
				"@request.body.rel_one:length = 5 &&" +
				"@request.body.rel_one:length ?= 6 &&" +
				"@request.body.rel_many:length != 7 &&" +
				"@request.body.rel_many:length ?!= 8 &&" +
				"@request.body.file_one:length = 9 &&" +
				"@request.body.file_one:length ?= 0 &&" +
				"@request.body.file_many:length != 1 &&" +
				"@request.body.file_many:length ?!= 2",
			false,
			/* SQLite:
			"SELECT `demo1`.* FROM `demo1` WHERE (0 > {:TEST} AND 0 > {:TEST} AND 2 < {:TEST} AND 2 > {:TEST} AND 1 = {:TEST} AND 1 = {:TEST} AND 2 IS NOT {:TEST} AND 2 IS NOT {:TEST} AND 1 = {:TEST} AND 1 = {:TEST} AND 3 IS NOT {:TEST} AND 3 IS NOT {:TEST})",
			*/
			// PostgreSQL:
			`SELECT "demo1".* FROM "demo1" WHERE (0::numeric > {:TEST}::numeric AND 0::numeric > {:TEST}::numeric AND 2::numeric < {:TEST}::numeric AND 2::numeric > {:TEST}::numeric AND 1::numeric = {:TEST}::numeric AND 1::numeric = {:TEST}::numeric AND 2::numeric != {:TEST}::numeric AND 2::numeric != {:TEST}::numeric AND 1::numeric = {:TEST}::numeric AND 1::numeric = {:TEST}::numeric AND 3::numeric != {:TEST}::numeric AND 3::numeric != {:TEST}::numeric)`,
		},
		{
			"regular arrayable:length fields",
			"demo4",
			"@request.body.self_rel_one.self_rel_many:length > 1 &&" +
				"@request.body.self_rel_one.self_rel_many:length ?> 2 &&" +
				"@request.body.rel_many_cascade.files:length ?< 3 &&" +
				"@request.body.rel_many_cascade.files:length < 4 &&" +
				"@request.body.rel_one_cascade.files:length < 4.1 &&" + // to ensure that the join to the same as above table will be aliased
				"self_rel_one.self_rel_many:length = 5 &&" +
				"self_rel_one.self_rel_many:length ?= 6 &&" +
				"self_rel_one.rel_many_cascade.files:length != 7 &&" +
				"self_rel_one.rel_many_cascade.files:length ?!= 8",
			false,
			/* SQLite:
			"SELECT DISTINCT `demo4`.* FROM `demo4` LEFT JOIN `demo4` `__data_demo4_self_rel_one` ON [[__data_demo4_self_rel_one.id]]={:p0} LEFT JOIN `demo3` `__data_demo3_rel_many_cascade` ON [[__data_demo3_rel_many_cascade.id]] IN ({:p1}, {:p2}) LEFT JOIN `demo3` `__data_demo3_rel_one_cascade` ON [[__data_demo3_rel_one_cascade.id]]={:p3} LEFT JOIN `demo4` `demo4_self_rel_one` ON [[demo4_self_rel_one.id]] = [[demo4.self_rel_one]] LEFT JOIN json_each(CASE WHEN json_valid([[demo4_self_rel_one.rel_many_cascade]]) THEN [[demo4_self_rel_one.rel_many_cascade]] ELSE json_array([[demo4_self_rel_one.rel_many_cascade]]) END) `demo4_self_rel_one_rel_many_cascade_je` LEFT JOIN `demo3` `demo4_self_rel_one_rel_many_cascade` ON [[demo4_self_rel_one_rel_many_cascade.id]] = [[demo4_self_rel_one_rel_many_cascade_je.value]] WHERE (json_array_length(CASE WHEN json_valid([[__data_demo4_self_rel_one.self_rel_many]]) THEN [[__data_demo4_self_rel_one.self_rel_many]] ELSE (CASE WHEN [[__data_demo4_self_rel_one.self_rel_many]] = '' OR [[__data_demo4_self_rel_one.self_rel_many]] IS NULL THEN json_array() ELSE json_array([[__data_demo4_self_rel_one.self_rel_many]]) END) END) > {:TEST} AND json_array_length(CASE WHEN json_valid([[__data_demo4_self_rel_one.self_rel_many]]) THEN [[__data_demo4_self_rel_one.self_rel_many]] ELSE (CASE WHEN [[__data_demo4_self_rel_one.self_rel_many]] = '' OR [[__data_demo4_self_rel_one.self_rel_many]] IS NULL THEN json_array() ELSE json_array([[__data_demo4_self_rel_one.self_rel_many]]) END) END) > {:TEST} AND json_array_length(CASE WHEN json_valid([[__data_demo3_rel_many_cascade.files]]) THEN [[__data_demo3_rel_many_cascade.files]] ELSE (CASE WHEN [[__data_demo3_rel_many_cascade.files]] = '' OR [[__data_demo3_rel_many_cascade.files]] IS NULL THEN json_array() ELSE json_array([[__data_demo3_rel_many_cascade.files]]) END) END) < {:TEST} AND ((json_array_length(CASE WHEN json_valid([[__data_demo3_rel_many_cascade.files]]) THEN [[__data_demo3_rel_many_cascade.files]] ELSE (CASE WHEN [[__data_demo3_rel_many_cascade.files]] = '' OR [[__data_demo3_rel_many_cascade.files]] IS NULL THEN json_array() ELSE json_array([[__data_demo3_rel_many_cascade.files]]) END) END) < {:TEST}) AND (NOT EXISTS (SELECT 1 FROM (SELECT json_array_length(CASE WHEN json_valid([[__data_mm_demo3_rel_many_cascade.files]]) THEN [[__data_mm_demo3_rel_many_cascade.files]] ELSE (CASE WHEN [[__data_mm_demo3_rel_many_cascade.files]] = '' OR [[__data_mm_demo3_rel_many_cascade.files]] IS NULL THEN json_array() ELSE json_array([[__data_mm_demo3_rel_many_cascade.files]]) END) END) as [[multiMatchValue]] FROM `demo4` `__mm_demo4` LEFT JOIN `demo3` `__data_mm_demo3_rel_many_cascade` ON [[__data_mm_demo3_rel_many_cascade.id]] IN ({:p8}, {:p9}) WHERE `__mm_demo4`.`id` = `demo4`.`id`) {{__smTEST}} WHERE NOT ([[__smTEST.multiMatchValue]] < {:TEST})))) AND json_array_length(CASE WHEN json_valid([[__data_demo3_rel_one_cascade.files]]) THEN [[__data_demo3_rel_one_cascade.files]] ELSE (CASE WHEN [[__data_demo3_rel_one_cascade.files]] = '' OR [[__data_demo3_rel_one_cascade.files]] IS NULL THEN json_array() ELSE json_array([[__data_demo3_rel_one_cascade.files]]) END) END) < {:TEST} AND json_array_length(CASE WHEN json_valid([[demo4_self_rel_one.self_rel_many]]) THEN [[demo4_self_rel_one.self_rel_many]] ELSE (CASE WHEN [[demo4_self_rel_one.self_rel_many]] = '' OR [[demo4_self_rel_one.self_rel_many]] IS NULL THEN json_array() ELSE json_array([[demo4_self_rel_one.self_rel_many]]) END) END) = {:TEST} AND json_array_length(CASE WHEN json_valid([[demo4_self_rel_one.self_rel_many]]) THEN [[demo4_self_rel_one.self_rel_many]] ELSE (CASE WHEN [[demo4_self_rel_one.self_rel_many]] = '' OR [[demo4_self_rel_one.self_rel_many]] IS NULL THEN json_array() ELSE json_array([[demo4_self_rel_one.self_rel_many]]) END) END) = {:TEST} AND ((json_array_length(CASE WHEN json_valid([[demo4_self_rel_one_rel_many_cascade.files]]) THEN [[demo4_self_rel_one_rel_many_cascade.files]] ELSE (CASE WHEN [[demo4_self_rel_one_rel_many_cascade.files]] = '' OR [[demo4_self_rel_one_rel_many_cascade.files]] IS NULL THEN json_array() ELSE json_array([[demo4_self_rel_one_rel_many_cascade.files]]) END) END) IS NOT {:TEST}) AND (NOT EXISTS (SELECT 1 FROM (SELECT json_array_length(CASE WHEN json_valid([[__mm_demo4_self_rel_one_rel_many_cascade.files]]) THEN [[__mm_demo4_self_rel_one_rel_many_cascade.files]] ELSE (CASE WHEN [[__mm_demo4_self_rel_one_rel_many_cascade.files]] = '' OR [[__mm_demo4_self_rel_one_rel_many_cascade.files]] IS NULL THEN json_array() ELSE json_array([[__mm_demo4_self_rel_one_rel_many_cascade.files]]) END) END) as [[multiMatchValue]] FROM `demo4` `__mm_demo4` LEFT JOIN `demo4` `__mm_demo4_self_rel_one` ON [[__mm_demo4_self_rel_one.id]] = [[__mm_demo4.self_rel_one]] LEFT JOIN json_each(CASE WHEN json_valid([[__mm_demo4_self_rel_one.rel_many_cascade]]) THEN [[__mm_demo4_self_rel_one.rel_many_cascade]] ELSE json_array([[__mm_demo4_self_rel_one.rel_many_cascade]]) END) `__mm_demo4_self_rel_one_rel_many_cascade_je` LEFT JOIN `demo3` `__mm_demo4_self_rel_one_rel_many_cascade` ON [[__mm_demo4_self_rel_one_rel_many_cascade.id]] = [[__mm_demo4_self_rel_one_rel_many_cascade_je.value]] WHERE `__mm_demo4`.`id` = `demo4`.`id`) {{__smTEST}} WHERE NOT ([[__smTEST.multiMatchValue]] IS NOT {:TEST})))) AND json_array_length(CASE WHEN json_valid([[demo4_self_rel_one_rel_many_cascade.files]]) THEN [[demo4_self_rel_one_rel_many_cascade.files]] ELSE (CASE WHEN [[demo4_self_rel_one_rel_many_cascade.files]] = '' OR [[demo4_self_rel_one_rel_many_cascade.files]] IS NULL THEN json_array() ELSE json_array([[demo4_self_rel_one_rel_many_cascade.files]]) END) END) IS NOT {:TEST})",
			*/
			// PostgreSQL:
			`SELECT DISTINCT "demo4".* FROM "demo4" LEFT JOIN "demo4" "__data_demo4_self_rel_one" ON [[__data_demo4_self_rel_one.id]]={:p0} LEFT JOIN "demo3" "__data_demo3_rel_many_cascade" ON [[__data_demo3_rel_many_cascade.id]] IN ({:p1}, {:p2}) LEFT JOIN "demo3" "__data_demo3_rel_one_cascade" ON [[__data_demo3_rel_one_cascade.id]]={:p3} LEFT JOIN "demo4" "demo4_self_rel_one" ON [[demo4_self_rel_one.id]] = [[demo4.self_rel_one]] LEFT JOIN jsonb_array_elements_text(CASE WHEN ([[demo4_self_rel_one.rel_many_cascade]] IS JSON OR json_valid([[demo4_self_rel_one.rel_many_cascade]]::text)) AND jsonb_typeof([[demo4_self_rel_one.rel_many_cascade]]::jsonb) = 'array' THEN [[demo4_self_rel_one.rel_many_cascade]]::jsonb ELSE jsonb_build_array([[demo4_self_rel_one.rel_many_cascade]]) END) "demo4_self_rel_one_rel_many_cascade_je" ON 1=1 LEFT JOIN "demo3" "demo4_self_rel_one_rel_many_cascade" ON [[demo4_self_rel_one_rel_many_cascade.id]] = [[demo4_self_rel_one_rel_many_cascade_je.value]] WHERE ((CASE WHEN ([[__data_demo4_self_rel_one.self_rel_many]] IS JSON OR JSON_VALID([[__data_demo4_self_rel_one.self_rel_many]]::text)) AND jsonb_typeof([[__data_demo4_self_rel_one.self_rel_many]]::jsonb) = 'array' THEN jsonb_array_length([[__data_demo4_self_rel_one.self_rel_many]]::jsonb) ELSE 0 END)::numeric > {:TEST}::numeric AND (CASE WHEN ([[__data_demo4_self_rel_one.self_rel_many]] IS JSON OR JSON_VALID([[__data_demo4_self_rel_one.self_rel_many]]::text)) AND jsonb_typeof([[__data_demo4_self_rel_one.self_rel_many]]::jsonb) = 'array' THEN jsonb_array_length([[__data_demo4_self_rel_one.self_rel_many]]::jsonb) ELSE 0 END)::numeric > {:TEST}::numeric AND (CASE WHEN ([[__data_demo3_rel_many_cascade.files]] IS JSON OR JSON_VALID([[__data_demo3_rel_many_cascade.files]]::text)) AND jsonb_typeof([[__data_demo3_rel_many_cascade.files]]::jsonb) = 'array' THEN jsonb_array_length([[__data_demo3_rel_many_cascade.files]]::jsonb) ELSE 0 END)::numeric < {:TEST}::numeric AND (((CASE WHEN ([[__data_demo3_rel_many_cascade.files]] IS JSON OR JSON_VALID([[__data_demo3_rel_many_cascade.files]]::text)) AND jsonb_typeof([[__data_demo3_rel_many_cascade.files]]::jsonb) = 'array' THEN jsonb_array_length([[__data_demo3_rel_many_cascade.files]]::jsonb) ELSE 0 END)::numeric < {:TEST}::numeric) AND (NOT EXISTS (SELECT 1 FROM (SELECT (CASE WHEN ([[__data_mm_demo3_rel_many_cascade.files]] IS JSON OR JSON_VALID([[__data_mm_demo3_rel_many_cascade.files]]::text)) AND jsonb_typeof([[__data_mm_demo3_rel_many_cascade.files]]::jsonb) = 'array' THEN jsonb_array_length([[__data_mm_demo3_rel_many_cascade.files]]::jsonb) ELSE 0 END) as [[multiMatchValue]] FROM "demo4" "__mm_demo4" LEFT JOIN "demo3" "__data_mm_demo3_rel_many_cascade" ON [[__data_mm_demo3_rel_many_cascade.id]] IN ({:p8}, {:p9}) WHERE "__mm_demo4"."id" = "demo4"."id") {{__smTEST}} WHERE NOT ([[__smTEST.multiMatchValue]]::numeric < {:TEST}::numeric)))) AND (CASE WHEN ([[__data_demo3_rel_one_cascade.files]] IS JSON OR JSON_VALID([[__data_demo3_rel_one_cascade.files]]::text)) AND jsonb_typeof([[__data_demo3_rel_one_cascade.files]]::jsonb) = 'array' THEN jsonb_array_length([[__data_demo3_rel_one_cascade.files]]::jsonb) ELSE 0 END)::numeric < {:TEST}::numeric AND (CASE WHEN ([[demo4_self_rel_one.self_rel_many]] IS JSON OR JSON_VALID([[demo4_self_rel_one.self_rel_many]]::text)) AND jsonb_typeof([[demo4_self_rel_one.self_rel_many]]::jsonb) = 'array' THEN jsonb_array_length([[demo4_self_rel_one.self_rel_many]]::jsonb) ELSE 0 END)::numeric = {:TEST}::numeric AND (CASE WHEN ([[demo4_self_rel_one.self_rel_many]] IS JSON OR JSON_VALID([[demo4_self_rel_one.self_rel_many]]::text)) AND jsonb_typeof([[demo4_self_rel_one.self_rel_many]]::jsonb) = 'array' THEN jsonb_array_length([[demo4_self_rel_one.self_rel_many]]::jsonb) ELSE 0 END)::numeric = {:TEST}::numeric AND (((CASE WHEN ([[demo4_self_rel_one_rel_many_cascade.files]] IS JSON OR JSON_VALID([[demo4_self_rel_one_rel_many_cascade.files]]::text)) AND jsonb_typeof([[demo4_self_rel_one_rel_many_cascade.files]]::jsonb) = 'array' THEN jsonb_array_length([[demo4_self_rel_one_rel_many_cascade.files]]::jsonb) ELSE 0 END)::numeric != {:TEST}::numeric) AND (NOT EXISTS (SELECT 1 FROM (SELECT (CASE WHEN ([[__mm_demo4_self_rel_one_rel_many_cascade.files]] IS JSON OR JSON_VALID([[__mm_demo4_self_rel_one_rel_many_cascade.files]]::text)) AND jsonb_typeof([[__mm_demo4_self_rel_one_rel_many_cascade.files]]::jsonb) = 'array' THEN jsonb_array_length([[__mm_demo4_self_rel_one_rel_many_cascade.files]]::jsonb) ELSE 0 END) as [[multiMatchValue]] FROM "demo4" "__mm_demo4" LEFT JOIN "demo4" "__mm_demo4_self_rel_one" ON [[__mm_demo4_self_rel_one.id]] = [[__mm_demo4.self_rel_one]] LEFT JOIN jsonb_array_elements_text(CASE WHEN ([[__mm_demo4_self_rel_one.rel_many_cascade]] IS JSON OR json_valid([[__mm_demo4_self_rel_one.rel_many_cascade]]::text)) AND jsonb_typeof([[__mm_demo4_self_rel_one.rel_many_cascade]]::jsonb) = 'array' THEN [[__mm_demo4_self_rel_one.rel_many_cascade]]::jsonb ELSE jsonb_build_array([[__mm_demo4_self_rel_one.rel_many_cascade]]) END) "__mm_demo4_self_rel_one_rel_many_cascade_je" ON 1=1 LEFT JOIN "demo3" "__mm_demo4_self_rel_one_rel_many_cascade" ON [[__mm_demo4_self_rel_one_rel_many_cascade.id]] = [[__mm_demo4_self_rel_one_rel_many_cascade_je.value]] WHERE "__mm_demo4"."id" = "demo4"."id") {{__smTEST}} WHERE NOT ([[__smTEST.multiMatchValue]]::numeric != {:TEST}::numeric)))) AND (CASE WHEN ([[demo4_self_rel_one_rel_many_cascade.files]] IS JSON OR JSON_VALID([[demo4_self_rel_one_rel_many_cascade.files]]::text)) AND jsonb_typeof([[demo4_self_rel_one_rel_many_cascade.files]]::jsonb) = 'array' THEN jsonb_array_length([[demo4_self_rel_one_rel_many_cascade.files]]::jsonb) ELSE 0 END)::numeric != {:TEST}::numeric)`,
		},
		{
			"json_extract and json_array_length COALESCE equal normalizations",
			"demo4",
			"json_object.a.b = '' && self_rel_many:length != 2 && json_object.a.b > 3 && self_rel_many:length <= 4",
			false,
			/* SQLite:
			"SELECT `demo4`.* FROM `demo4` WHERE ((CASE WHEN json_valid([[demo4.json_object]]) THEN JSON_EXTRACT([[demo4.json_object]], '$.a.b') ELSE JSON_EXTRACT(json_object('pb', [[demo4.json_object]]), '$.pb.a.b') END) IS {:TEST} AND json_array_length(CASE WHEN json_valid([[demo4.self_rel_many]]) THEN [[demo4.self_rel_many]] ELSE (CASE WHEN [[demo4.self_rel_many]] = '' OR [[demo4.self_rel_many]] IS NULL THEN json_array() ELSE json_array([[demo4.self_rel_many]]) END) END) IS NOT {:TEST} AND (CASE WHEN json_valid([[demo4.json_object]]) THEN JSON_EXTRACT([[demo4.json_object]], '$.a.b') ELSE JSON_EXTRACT(json_object('pb', [[demo4.json_object]]), '$.pb.a.b') END) > {:TEST} AND json_array_length(CASE WHEN json_valid([[demo4.self_rel_many]]) THEN [[demo4.self_rel_many]] ELSE (CASE WHEN [[demo4.self_rel_many]] = '' OR [[demo4.self_rel_many]] IS NULL THEN json_array() ELSE json_array([[demo4.self_rel_many]]) END) END) <= {:TEST})",
			*/
			// PostgreSQL:
			`SELECT "demo4".* FROM "demo4" WHERE (((CASE WHEN [[demo4.json_object]] IS JSON OR json_valid([[demo4.json_object]]::text) THEN JSON_QUERY([[demo4.json_object]]::jsonb, '$.a.b') ELSE NULL END) #>> '{}')::text = ''::text AND (CASE WHEN ([[demo4.self_rel_many]] IS JSON OR JSON_VALID([[demo4.self_rel_many]]::text)) AND jsonb_typeof([[demo4.self_rel_many]]::jsonb) = 'array' THEN jsonb_array_length([[demo4.self_rel_many]]::jsonb) ELSE 0 END)::numeric != {:TEST}::numeric AND ((CASE WHEN [[demo4.json_object]] IS JSON OR json_valid([[demo4.json_object]]::text) THEN JSON_QUERY([[demo4.json_object]]::jsonb, '$.a.b') ELSE NULL END) #>> '{}')::text::numeric > {:TEST}::numeric AND (CASE WHEN ([[demo4.self_rel_many]] IS JSON OR JSON_VALID([[demo4.self_rel_many]]::text)) AND jsonb_typeof([[demo4.self_rel_many]]::jsonb) = 'array' THEN jsonb_array_length([[demo4.self_rel_many]]::jsonb) ELSE 0 END)::numeric <= {:TEST}::numeric)`,
		},
		{
			"json field equal normalization checks",
			"demo4",
			"json_object = '' || json_object != '' || '' = json_object || '' != json_object ||" +
				"json_object = null || json_object != null || null = json_object || null != json_object ||" +
				"json_object = true || json_object != true || true = json_object || true != json_object ||" +
				"json_object = json_object || json_object != json_object ||" +
				"json_object = title || title != json_object ||" +
				// multimatch expressions
				"self_rel_many.json_object = '' || null = self_rel_many.json_object ||" +
				"self_rel_many.json_object = self_rel_many.json_object",
			false,
			/* SQLite:
			"SELECT DISTINCT `demo4`.* FROM `demo4` LEFT JOIN json_each(CASE WHEN json_valid([[demo4.self_rel_many]]) THEN [[demo4.self_rel_many]] ELSE json_array([[demo4.self_rel_many]]) END) `demo4_self_rel_many_je` LEFT JOIN `demo4` `demo4_self_rel_many` ON [[demo4_self_rel_many.id]] = [[demo4_self_rel_many_je.value]] WHERE ((CASE WHEN json_valid([[demo4.json_object]]) THEN JSON_EXTRACT([[demo4.json_object]], '$') ELSE JSON_EXTRACT(json_object('pb', [[demo4.json_object]]), '$.pb') END) IS {:TEST} OR (CASE WHEN json_valid([[demo4.json_object]]) THEN JSON_EXTRACT([[demo4.json_object]], '$') ELSE JSON_EXTRACT(json_object('pb', [[demo4.json_object]]), '$.pb') END) IS NOT {:TEST} OR {:TEST} IS (CASE WHEN json_valid([[demo4.json_object]]) THEN JSON_EXTRACT([[demo4.json_object]], '$') ELSE JSON_EXTRACT(json_object('pb', [[demo4.json_object]]), '$.pb') END) OR {:TEST} IS NOT (CASE WHEN json_valid([[demo4.json_object]]) THEN JSON_EXTRACT([[demo4.json_object]], '$') ELSE JSON_EXTRACT(json_object('pb', [[demo4.json_object]]), '$.pb') END) OR (CASE WHEN json_valid([[demo4.json_object]]) THEN JSON_EXTRACT([[demo4.json_object]], '$') ELSE JSON_EXTRACT(json_object('pb', [[demo4.json_object]]), '$.pb') END) IS NULL OR (CASE WHEN json_valid([[demo4.json_object]]) THEN JSON_EXTRACT([[demo4.json_object]], '$') ELSE JSON_EXTRACT(json_object('pb', [[demo4.json_object]]), '$.pb') END) IS NOT NULL OR NULL IS (CASE WHEN json_valid([[demo4.json_object]]) THEN JSON_EXTRACT([[demo4.json_object]], '$') ELSE JSON_EXTRACT(json_object('pb', [[demo4.json_object]]), '$.pb') END) OR NULL IS NOT (CASE WHEN json_valid([[demo4.json_object]]) THEN JSON_EXTRACT([[demo4.json_object]], '$') ELSE JSON_EXTRACT(json_object('pb', [[demo4.json_object]]), '$.pb') END) OR (CASE WHEN json_valid([[demo4.json_object]]) THEN JSON_EXTRACT([[demo4.json_object]], '$') ELSE JSON_EXTRACT(json_object('pb', [[demo4.json_object]]), '$.pb') END) IS 1 OR (CASE WHEN json_valid([[demo4.json_object]]) THEN JSON_EXTRACT([[demo4.json_object]], '$') ELSE JSON_EXTRACT(json_object('pb', [[demo4.json_object]]), '$.pb') END) IS NOT 1 OR 1 IS (CASE WHEN json_valid([[demo4.json_object]]) THEN JSON_EXTRACT([[demo4.json_object]], '$') ELSE JSON_EXTRACT(json_object('pb', [[demo4.json_object]]), '$.pb') END) OR 1 IS NOT (CASE WHEN json_valid([[demo4.json_object]]) THEN JSON_EXTRACT([[demo4.json_object]], '$') ELSE JSON_EXTRACT(json_object('pb', [[demo4.json_object]]), '$.pb') END) OR (CASE WHEN json_valid([[demo4.json_object]]) THEN JSON_EXTRACT([[demo4.json_object]], '$') ELSE JSON_EXTRACT(json_object('pb', [[demo4.json_object]]), '$.pb') END) IS (CASE WHEN json_valid([[demo4.json_object]]) THEN JSON_EXTRACT([[demo4.json_object]], '$') ELSE JSON_EXTRACT(json_object('pb', [[demo4.json_object]]), '$.pb') END) OR (CASE WHEN json_valid([[demo4.json_object]]) THEN JSON_EXTRACT([[demo4.json_object]], '$') ELSE JSON_EXTRACT(json_object('pb', [[demo4.json_object]]), '$.pb') END) IS NOT (CASE WHEN json_valid([[demo4.json_object]]) THEN JSON_EXTRACT([[demo4.json_object]], '$') ELSE JSON_EXTRACT(json_object('pb', [[demo4.json_object]]), '$.pb') END) OR (CASE WHEN json_valid([[demo4.json_object]]) THEN JSON_EXTRACT([[demo4.json_object]], '$') ELSE JSON_EXTRACT(json_object('pb', [[demo4.json_object]]), '$.pb') END) IS [[demo4.title]] OR [[demo4.title]] IS NOT (CASE WHEN json_valid([[demo4.json_object]]) THEN JSON_EXTRACT([[demo4.json_object]], '$') ELSE JSON_EXTRACT(json_object('pb', [[demo4.json_object]]), '$.pb') END) OR (((CASE WHEN json_valid([[demo4_self_rel_many.json_object]]) THEN JSON_EXTRACT([[demo4_self_rel_many.json_object]], '$') ELSE JSON_EXTRACT(json_object('pb', [[demo4_self_rel_many.json_object]]), '$.pb') END) IS {:TEST}) AND (NOT EXISTS (SELECT 1 FROM (SELECT (CASE WHEN json_valid([[__mm_demo4_self_rel_many.json_object]]) THEN JSON_EXTRACT([[__mm_demo4_self_rel_many.json_object]], '$') ELSE JSON_EXTRACT(json_object('pb', [[__mm_demo4_self_rel_many.json_object]]), '$.pb') END) as [[multiMatchValue]] FROM `demo4` `__mm_demo4` LEFT JOIN json_each(CASE WHEN json_valid([[__mm_demo4.self_rel_many]]) THEN [[__mm_demo4.self_rel_many]] ELSE json_array([[__mm_demo4.self_rel_many]]) END) `__mm_demo4_self_rel_many_je` LEFT JOIN `demo4` `__mm_demo4_self_rel_many` ON [[__mm_demo4_self_rel_many.id]] = [[__mm_demo4_self_rel_many_je.value]] WHERE `__mm_demo4`.`id` = `demo4`.`id`) {{__smTEST}} WHERE NOT ([[__smTEST.multiMatchValue]] IS {:TEST})))) OR ((NULL IS (CASE WHEN json_valid([[demo4_self_rel_many.json_object]]) THEN JSON_EXTRACT([[demo4_self_rel_many.json_object]], '$') ELSE JSON_EXTRACT(json_object('pb', [[demo4_self_rel_many.json_object]]), '$.pb') END)) AND (NOT EXISTS (SELECT 1 FROM (SELECT (CASE WHEN json_valid([[__mm_demo4_self_rel_many.json_object]]) THEN JSON_EXTRACT([[__mm_demo4_self_rel_many.json_object]], '$') ELSE JSON_EXTRACT(json_object('pb', [[__mm_demo4_self_rel_many.json_object]]), '$.pb') END) as [[multiMatchValue]] FROM `demo4` `__mm_demo4` LEFT JOIN json_each(CASE WHEN json_valid([[__mm_demo4.self_rel_many]]) THEN [[__mm_demo4.self_rel_many]] ELSE json_array([[__mm_demo4.self_rel_many]]) END) `__mm_demo4_self_rel_many_je` LEFT JOIN `demo4` `__mm_demo4_self_rel_many` ON [[__mm_demo4_self_rel_many.id]] = [[__mm_demo4_self_rel_many_je.value]] WHERE `__mm_demo4`.`id` = `demo4`.`id`) {{__smTEST}} WHERE NOT (NULL IS [[__smTEST.multiMatchValue]])))) OR (((CASE WHEN json_valid([[demo4_self_rel_many.json_object]]) THEN JSON_EXTRACT([[demo4_self_rel_many.json_object]], '$') ELSE JSON_EXTRACT(json_object('pb', [[demo4_self_rel_many.json_object]]), '$.pb') END) IS (CASE WHEN json_valid([[demo4_self_rel_many.json_object]]) THEN JSON_EXTRACT([[demo4_self_rel_many.json_object]], '$') ELSE JSON_EXTRACT(json_object('pb', [[demo4_self_rel_many.json_object]]), '$.pb') END)) AND (NOT EXISTS (SELECT 1 FROM (SELECT (CASE WHEN json_valid([[__mm_demo4_self_rel_many.json_object]]) THEN JSON_EXTRACT([[__mm_demo4_self_rel_many.json_object]], '$') ELSE JSON_EXTRACT(json_object('pb', [[__mm_demo4_self_rel_many.json_object]]), '$.pb') END) as [[multiMatchValue]] FROM `demo4` `__mm_demo4` LEFT JOIN json_each(CASE WHEN json_valid([[__mm_demo4.self_rel_many]]) THEN [[__mm_demo4.self_rel_many]] ELSE json_array([[__mm_demo4.self_rel_many]]) END) `__mm_demo4_self_rel_many_je` LEFT JOIN `demo4` `__mm_demo4_self_rel_many` ON [[__mm_demo4_self_rel_many.id]] = [[__mm_demo4_self_rel_many_je.value]] WHERE `__mm_demo4`.`id` = `demo4`.`id`) {{__mlTEST}} LEFT JOIN (SELECT (CASE WHEN json_valid([[__mm_demo4_self_rel_many.json_object]]) THEN JSON_EXTRACT([[__mm_demo4_self_rel_many.json_object]], '$') ELSE JSON_EXTRACT(json_object('pb', [[__mm_demo4_self_rel_many.json_object]]), '$.pb') END) as [[multiMatchValue]] FROM `demo4` `__mm_demo4` LEFT JOIN json_each(CASE WHEN json_valid([[__mm_demo4.self_rel_many]]) THEN [[__mm_demo4.self_rel_many]] ELSE json_array([[__mm_demo4.self_rel_many]]) END) `__mm_demo4_self_rel_many_je` LEFT JOIN `demo4` `__mm_demo4_self_rel_many` ON [[__mm_demo4_self_rel_many.id]] = [[__mm_demo4_self_rel_many_je.value]] WHERE `__mm_demo4`.`id` = `demo4`.`id`) {{__mrTEST}} WHERE NOT ([[__mlTEST.multiMatchValue]] IS [[__mrTEST.multiMatchValue]])))))",
			*/
			// PostgreSQL:
			`SELECT DISTINCT "demo4".* FROM "demo4" LEFT JOIN jsonb_array_elements_text(CASE WHEN ([[demo4.self_rel_many]] IS JSON OR json_valid([[demo4.self_rel_many]]::text)) AND jsonb_typeof([[demo4.self_rel_many]]::jsonb) = 'array' THEN [[demo4.self_rel_many]]::jsonb ELSE jsonb_build_array([[demo4.self_rel_many]]) END) "demo4_self_rel_many_je" ON 1=1 LEFT JOIN "demo4" "demo4_self_rel_many" ON [[demo4_self_rel_many.id]] = [[demo4_self_rel_many_je.value]] WHERE (((CASE WHEN [[demo4.json_object]] IS JSON OR json_valid([[demo4.json_object]]::text) THEN JSON_QUERY([[demo4.json_object]]::jsonb, '$') ELSE NULL END) #>> '{}')::text = ''::text OR ((CASE WHEN [[demo4.json_object]] IS JSON OR json_valid([[demo4.json_object]]::text) THEN JSON_QUERY([[demo4.json_object]]::jsonb, '$') ELSE NULL END) #>> '{}')::text != ''::text OR ''::text = ((CASE WHEN [[demo4.json_object]] IS JSON OR json_valid([[demo4.json_object]]::text) THEN JSON_QUERY([[demo4.json_object]]::jsonb, '$') ELSE NULL END) #>> '{}')::text OR ''::text != ((CASE WHEN [[demo4.json_object]] IS JSON OR json_valid([[demo4.json_object]]::text) THEN JSON_QUERY([[demo4.json_object]]::jsonb, '$') ELSE NULL END) #>> '{}')::text OR ((CASE WHEN [[demo4.json_object]] IS JSON OR json_valid([[demo4.json_object]]::text) THEN JSON_QUERY([[demo4.json_object]]::jsonb, '$') ELSE NULL END) #>> '{}')::text = ''::text OR ((CASE WHEN [[demo4.json_object]] IS JSON OR json_valid([[demo4.json_object]]::text) THEN JSON_QUERY([[demo4.json_object]]::jsonb, '$') ELSE NULL END) #>> '{}')::text != ''::text OR ''::text = ((CASE WHEN [[demo4.json_object]] IS JSON OR json_valid([[demo4.json_object]]::text) THEN JSON_QUERY([[demo4.json_object]]::jsonb, '$') ELSE NULL END) #>> '{}')::text OR ''::text != ((CASE WHEN [[demo4.json_object]] IS JSON OR json_valid([[demo4.json_object]]::text) THEN JSON_QUERY([[demo4.json_object]]::jsonb, '$') ELSE NULL END) #>> '{}')::text OR ((CASE WHEN [[demo4.json_object]] IS JSON OR json_valid([[demo4.json_object]]::text) THEN JSON_QUERY([[demo4.json_object]]::jsonb, '$') ELSE NULL END) #>> '{}')::text = TRUE::text OR ((CASE WHEN [[demo4.json_object]] IS JSON OR json_valid([[demo4.json_object]]::text) THEN JSON_QUERY([[demo4.json_object]]::jsonb, '$') ELSE NULL END) #>> '{}')::text != TRUE::text OR TRUE::text = ((CASE WHEN [[demo4.json_object]] IS JSON OR json_valid([[demo4.json_object]]::text) THEN JSON_QUERY([[demo4.json_object]]::jsonb, '$') ELSE NULL END) #>> '{}')::text OR TRUE::text != ((CASE WHEN [[demo4.json_object]] IS JSON OR json_valid([[demo4.json_object]]::text) THEN JSON_QUERY([[demo4.json_object]]::jsonb, '$') ELSE NULL END) #>> '{}')::text OR ((CASE WHEN [[demo4.json_object]] IS JSON OR json_valid([[demo4.json_object]]::text) THEN JSON_QUERY([[demo4.json_object]]::jsonb, '$') ELSE NULL END) #>> '{}')::text = ((CASE WHEN [[demo4.json_object]] IS JSON OR json_valid([[demo4.json_object]]::text) THEN JSON_QUERY([[demo4.json_object]]::jsonb, '$') ELSE NULL END) #>> '{}')::text OR ((CASE WHEN [[demo4.json_object]] IS JSON OR json_valid([[demo4.json_object]]::text) THEN JSON_QUERY([[demo4.json_object]]::jsonb, '$') ELSE NULL END) #>> '{}')::text != ((CASE WHEN [[demo4.json_object]] IS JSON OR json_valid([[demo4.json_object]]::text) THEN JSON_QUERY([[demo4.json_object]]::jsonb, '$') ELSE NULL END) #>> '{}')::text OR ((CASE WHEN [[demo4.json_object]] IS JSON OR json_valid([[demo4.json_object]]::text) THEN JSON_QUERY([[demo4.json_object]]::jsonb, '$') ELSE NULL END) #>> '{}')::text = [[demo4.title]]::text OR [[demo4.title]]::text != ((CASE WHEN [[demo4.json_object]] IS JSON OR json_valid([[demo4.json_object]]::text) THEN JSON_QUERY([[demo4.json_object]]::jsonb, '$') ELSE NULL END) #>> '{}')::text OR ((((CASE WHEN [[demo4_self_rel_many.json_object]] IS JSON OR json_valid([[demo4_self_rel_many.json_object]]::text) THEN JSON_QUERY([[demo4_self_rel_many.json_object]]::jsonb, '$') ELSE NULL END) #>> '{}')::text = ''::text) AND (NOT EXISTS (SELECT 1 FROM (SELECT ((CASE WHEN [[__mm_demo4_self_rel_many.json_object]] IS JSON OR json_valid([[__mm_demo4_self_rel_many.json_object]]::text) THEN JSON_QUERY([[__mm_demo4_self_rel_many.json_object]]::jsonb, '$') ELSE NULL END) #>> '{}')::text as [[multiMatchValue]] FROM "demo4" "__mm_demo4" LEFT JOIN jsonb_array_elements_text(CASE WHEN ([[__mm_demo4.self_rel_many]] IS JSON OR json_valid([[__mm_demo4.self_rel_many]]::text)) AND jsonb_typeof([[__mm_demo4.self_rel_many]]::jsonb) = 'array' THEN [[__mm_demo4.self_rel_many]]::jsonb ELSE jsonb_build_array([[__mm_demo4.self_rel_many]]) END) "__mm_demo4_self_rel_many_je" ON 1=1 LEFT JOIN "demo4" "__mm_demo4_self_rel_many" ON [[__mm_demo4_self_rel_many.id]] = [[__mm_demo4_self_rel_many_je.value]] WHERE "__mm_demo4"."id" = "demo4"."id") {{__smTEST}} WHERE NOT ([[__smTEST.multiMatchValue]] = '')))) OR ((''::text = ((CASE WHEN [[demo4_self_rel_many.json_object]] IS JSON OR json_valid([[demo4_self_rel_many.json_object]]::text) THEN JSON_QUERY([[demo4_self_rel_many.json_object]]::jsonb, '$') ELSE NULL END) #>> '{}')::text) AND (NOT EXISTS (SELECT 1 FROM (SELECT ((CASE WHEN [[__mm_demo4_self_rel_many.json_object]] IS JSON OR json_valid([[__mm_demo4_self_rel_many.json_object]]::text) THEN JSON_QUERY([[__mm_demo4_self_rel_many.json_object]]::jsonb, '$') ELSE NULL END) #>> '{}')::text as [[multiMatchValue]] FROM "demo4" "__mm_demo4" LEFT JOIN jsonb_array_elements_text(CASE WHEN ([[__mm_demo4.self_rel_many]] IS JSON OR json_valid([[__mm_demo4.self_rel_many]]::text)) AND jsonb_typeof([[__mm_demo4.self_rel_many]]::jsonb) = 'array' THEN [[__mm_demo4.self_rel_many]]::jsonb ELSE jsonb_build_array([[__mm_demo4.self_rel_many]]) END) "__mm_demo4_self_rel_many_je" ON 1=1 LEFT JOIN "demo4" "__mm_demo4_self_rel_many" ON [[__mm_demo4_self_rel_many.id]] = [[__mm_demo4_self_rel_many_je.value]] WHERE "__mm_demo4"."id" = "demo4"."id") {{__smTEST}} WHERE NOT ('' = [[__smTEST.multiMatchValue]])))) OR ((((CASE WHEN [[demo4_self_rel_many.json_object]] IS JSON OR json_valid([[demo4_self_rel_many.json_object]]::text) THEN JSON_QUERY([[demo4_self_rel_many.json_object]]::jsonb, '$') ELSE NULL END) #>> '{}')::text = ((CASE WHEN [[demo4_self_rel_many.json_object]] IS JSON OR json_valid([[demo4_self_rel_many.json_object]]::text) THEN JSON_QUERY([[demo4_self_rel_many.json_object]]::jsonb, '$') ELSE NULL END) #>> '{}')::text) AND (NOT EXISTS (SELECT 1 FROM (SELECT ((CASE WHEN [[__mm_demo4_self_rel_many.json_object]] IS JSON OR json_valid([[__mm_demo4_self_rel_many.json_object]]::text) THEN JSON_QUERY([[__mm_demo4_self_rel_many.json_object]]::jsonb, '$') ELSE NULL END) #>> '{}')::text as [[multiMatchValue]] FROM "demo4" "__mm_demo4" LEFT JOIN jsonb_array_elements_text(CASE WHEN ([[__mm_demo4.self_rel_many]] IS JSON OR json_valid([[__mm_demo4.self_rel_many]]::text)) AND jsonb_typeof([[__mm_demo4.self_rel_many]]::jsonb) = 'array' THEN [[__mm_demo4.self_rel_many]]::jsonb ELSE jsonb_build_array([[__mm_demo4.self_rel_many]]) END) "__mm_demo4_self_rel_many_je" ON 1=1 LEFT JOIN "demo4" "__mm_demo4_self_rel_many" ON [[__mm_demo4_self_rel_many.id]] = [[__mm_demo4_self_rel_many_je.value]] WHERE "__mm_demo4"."id" = "demo4"."id") {{__mlTEST}} LEFT JOIN (SELECT ((CASE WHEN [[__mm_demo4_self_rel_many.json_object]] IS JSON OR json_valid([[__mm_demo4_self_rel_many.json_object]]::text) THEN JSON_QUERY([[__mm_demo4_self_rel_many.json_object]]::jsonb, '$') ELSE NULL END) #>> '{}')::text as [[multiMatchValue]] FROM "demo4" "__mm_demo4" LEFT JOIN jsonb_array_elements_text(CASE WHEN ([[__mm_demo4.self_rel_many]] IS JSON OR json_valid([[__mm_demo4.self_rel_many]]::text)) AND jsonb_typeof([[__mm_demo4.self_rel_many]]::jsonb) = 'array' THEN [[__mm_demo4.self_rel_many]]::jsonb ELSE jsonb_build_array([[__mm_demo4.self_rel_many]]) END) "__mm_demo4_self_rel_many_je" ON 1=1 LEFT JOIN "demo4" "__mm_demo4_self_rel_many" ON [[__mm_demo4_self_rel_many.id]] = [[__mm_demo4_self_rel_many_je.value]] WHERE "__mm_demo4"."id" = "demo4"."id") {{__mrTEST}} ON 1 = 1 WHERE NOT ([[__mlTEST.multiMatchValue]] = [[__mrTEST.multiMatchValue]])))))`,
		},
		{
			"geoPoint props access",
			"demo1",
			"point = '' || point.lat > 1 || point.lon < 2 || point.something > 3",
			false,
			/* SQLite:
			"SELECT `demo1`.* FROM `demo1` WHERE (([[demo1.point]] = '' OR [[demo1.point]] IS NULL) OR (CASE WHEN json_valid([[demo1.point]]) THEN JSON_EXTRACT([[demo1.point]], '$.lat') ELSE JSON_EXTRACT(json_object('pb', [[demo1.point]]), '$.pb.lat') END) > {:TEST} OR (CASE WHEN json_valid([[demo1.point]]) THEN JSON_EXTRACT([[demo1.point]], '$.lon') ELSE JSON_EXTRACT(json_object('pb', [[demo1.point]]), '$.pb.lon') END) < {:TEST} OR (CASE WHEN json_valid([[demo1.point]]) THEN JSON_EXTRACT([[demo1.point]], '$.something') ELSE JSON_EXTRACT(json_object('pb', [[demo1.point]]), '$.pb.something') END) > {:TEST})",
			*/
			// PostgreSQL:
			`SELECT "demo1".* FROM "demo1" WHERE (([[demo1.point]] IS NOT DISTINCT FROM '' OR [[demo1.point]] IS NULL) OR ((CASE WHEN [[demo1.point]] IS JSON OR json_valid([[demo1.point]]::text) THEN JSON_QUERY([[demo1.point]]::jsonb, '$.lat') ELSE NULL END) #>> '{}')::text::numeric > {:TEST}::numeric OR ((CASE WHEN [[demo1.point]] IS JSON OR json_valid([[demo1.point]]::text) THEN JSON_QUERY([[demo1.point]]::jsonb, '$.lon') ELSE NULL END) #>> '{}')::text::numeric < {:TEST}::numeric OR ((CASE WHEN [[demo1.point]] IS JSON OR json_valid([[demo1.point]]::text) THEN JSON_QUERY([[demo1.point]]::jsonb, '$.something') ELSE NULL END) #>> '{}')::text::numeric > {:TEST}::numeric)`,
		},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			collection, err := app.FindCollectionByNameOrId(s.collectionIdOrName)
			if err != nil {
				t.Fatalf("[%s] Failed to load collection %s: %v", s.name, s.collectionIdOrName, err)
			}

			query := app.RecordQuery(collection)

			r := core.NewRecordFieldResolver(app, collection, requestInfo, s.allowHiddenFields)

			expr, err := search.FilterData(s.rule).BuildExpr(r)
			if err != nil {
				t.Fatalf("[%s] BuildExpr failed with error %v", s.name, err)
			}

			if err := r.UpdateQuery(query); err != nil {
				t.Fatalf("[%s] UpdateQuery failed with error %v", s.name, err)
			}

			rawQuery := query.AndWhere(expr).Build().SQL()

			// replace TEST placeholder with .+ regex pattern
			expectQuery := strings.ReplaceAll(
				"^"+regexp.QuoteMeta(s.expectQuery)+"$",
				"TEST",
				`\w+`,
			)

			if !list.ExistInSliceWithRegex(rawQuery, []string{expectQuery}) {
				t.Fatalf("[%s] Expected query\n %v \ngot:\n %v", s.name, s.expectQuery, rawQuery)
			}
		})
	}
}

func TestRecordFieldResolverResolveCollectionFields(t *testing.T) {
	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	collection, err := app.FindCollectionByNameOrId("demo4")
	if err != nil {
		t.Fatal(err)
	}

	authRecord, err := app.FindRecordById("users", "4q1xlclmfloku33")
	if err != nil {
		t.Fatal(err)
	}

	requestInfo := &core.RequestInfo{
		Auth: authRecord,
	}

	r := core.NewRecordFieldResolver(app, collection, requestInfo, true)

	scenarios := []struct {
		fieldName   string
		expectError bool
		expectName  string
	}{
		{"", true, ""},
		{" ", true, ""},
		{"unknown", true, ""},
		{"invalid format", true, ""},
		{"id", false, "[[demo4.id]]"},
		{"created", false, "[[demo4.created]]"},
		{"updated", false, "[[demo4.updated]]"},
		{"title", false, "[[demo4.title]]"},
		{"title.test", true, ""},
		{"self_rel_many", false, "[[demo4.self_rel_many]]"},
		{"self_rel_many.", true, ""},
		{"self_rel_many.unknown", true, ""},
		{"self_rel_many.title", false, "[[demo4_self_rel_many.title]]"},
		{"self_rel_many.self_rel_one.self_rel_many.title", false, "[[demo4_self_rel_many_self_rel_one_self_rel_many.title]]"},

		// max relations limit
		{"self_rel_many.self_rel_many.self_rel_many.self_rel_many.self_rel_many.self_rel_many.id", false, "[[demo4_self_rel_many_self_rel_many_self_rel_many_self_rel_many_self_rel_many_self_rel_many.id]]"},
		{"self_rel_many.self_rel_many.self_rel_many.self_rel_many.self_rel_many.self_rel_many.self_rel_many.id", true, ""},

		// back relations
		{"rel_one_cascade.demo4_via_title.id", true, ""},        // not a relation field
		{"rel_one_cascade.demo4_via_self_rel_one.id", true, ""}, // relation field but to a different collection
		{"rel_one_cascade.demo4_via_rel_one_cascade.id", false, "[[demo4_rel_one_cascade_demo4_via_rel_one_cascade.id]]"},
		{"rel_one_cascade.demo4_via_rel_one_cascade.rel_one_cascade.demo4_via_rel_one_cascade.id", false, "[[demo4_rel_one_cascade_demo4_via_rel_one_cascade_rel_one_cascade_demo4_via_rel_one_cascade.id]]"},

		// json_extract
		/* SQLite:
		{"json_array.0", false, "(CASE WHEN json_valid([[demo4.json_array]]) THEN JSON_EXTRACT([[demo4.json_array]], '$[0]') ELSE JSON_EXTRACT(json_object('pb', [[demo4.json_array]]), '$.pb[0]') END)"},
		{"json_object.a.b.c", false, "(CASE WHEN json_valid([[demo4.json_object]]) THEN JSON_EXTRACT([[demo4.json_object]], '$.a.b.c') ELSE JSON_EXTRACT(json_object('pb', [[demo4.json_object]]), '$.pb.a.b.c') END)"},

		// max relations limit shouldn't apply for json paths
		{"json_object.a.b.c.e.f.g.h.i.j.k.l.m.n.o.p", false, "(CASE WHEN json_valid([[demo4.json_object]]) THEN JSON_EXTRACT([[demo4.json_object]], '$.a.b.c.e.f.g.h.i.j.k.l.m.n.o.p') ELSE JSON_EXTRACT(json_object('pb', [[demo4.json_object]]), '$.pb.a.b.c.e.f.g.h.i.j.k.l.m.n.o.p') END)"},
		*/
		// PostgreSQL:
		{"json_array.0", false, "((CASE WHEN [[demo4.json_array]] IS JSON OR json_valid([[demo4.json_array]]::text) THEN JSON_QUERY([[demo4.json_array]]::jsonb, '$[0]') ELSE NULL END) #>> '{}')::text"},
		{"json_object.a.b.c", false, "((CASE WHEN [[demo4.json_object]] IS JSON OR json_valid([[demo4.json_object]]::text) THEN JSON_QUERY([[demo4.json_object]]::jsonb, '$.a.b.c') ELSE NULL END) #>> '{}')::text"},
		// max relations limit shouldn't apply for json paths
		{"json_object.a.b.c.e.f.g.h.i.j.k.l.m.n.o.p", false, "((CASE WHEN [[demo4.json_object]] IS JSON OR json_valid([[demo4.json_object]]::text) THEN JSON_QUERY([[demo4.json_object]]::jsonb, '$.a.b.c.e.f.g.h.i.j.k.l.m.n.o.p') ELSE NULL END) #>> '{}')::text"},

		// @request.auth relation join
		{"@request.auth.rel", false, "[[__auth_users.rel]]"},
		{"@request.auth.rel.title", false, "[[__auth_users_rel.title]]"},
		{"@request.auth.demo1_via_rel_many.id", false, "[[__auth_users_demo1_via_rel_many.id]]"},
		{"@request.auth.rel.missing", false, "NULL"},
		{"@request.auth.missing_via_rel", false, "NULL"},
		{"@request.auth.demo1_via_file_one.id", false, "NULL"}, // not a relation field
		{"@request.auth.demo1_via_rel_one.id", false, "NULL"},  // relation field but to a different collection

		// @collection fieds
		{"@collect", true, ""},
		{"collection.demo4.title", true, ""},
		{"@collection", true, ""},
		{"@collection.unknown", true, ""},
		{"@collection.demo2", true, ""},
		{"@collection.demo2.", true, ""},
		{"@collection.demo2:someAlias", true, ""},
		{"@collection.demo2:someAlias.", true, ""},
		{"@collection.demo2.title", false, "[[__collection_demo2.title]]"},
		{"@collection.demo2:someAlias.title", false, "[[__collection_alias_someAlias.title]]"},
		{"@collection.demo4.id", false, "[[__collection_demo4.id]]"},
		{"@collection.demo4.created", false, "[[__collection_demo4.created]]"},
		{"@collection.demo4.updated", false, "[[__collection_demo4.updated]]"},
		{"@collection.demo4.self_rel_many.missing", true, ""},
		{"@collection.demo4.self_rel_many.self_rel_one.self_rel_many.self_rel_one.title", false, "[[__collection_demo4_self_rel_many_self_rel_one_self_rel_many_self_rel_one.title]]"},
	}

	for _, s := range scenarios {
		t.Run(s.fieldName, func(t *testing.T) {
			r, err := r.Resolve(s.fieldName)

			hasErr := err != nil
			if hasErr != s.expectError {
				t.Fatalf("Expected hasErr %v, got %v (%v)", s.expectError, hasErr, err)
			}

			if hasErr {
				return
			}

			if r.Identifier != s.expectName {
				t.Fatalf("Expected r.Identifier\n%q\ngot\n%q", s.expectName, r.Identifier)
			}

			// params should be empty for non @request fields
			if len(r.Params) != 0 {
				t.Fatalf("Expected 0 r.Params, got\n%v", r.Params)
			}
		})
	}
}

func TestRecordFieldResolverResolveStaticRequestInfoFields(t *testing.T) {
	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	collection, err := app.FindCollectionByNameOrId("demo1")
	if err != nil {
		t.Fatal(err)
	}

	authRecord, err := app.FindRecordById("users", "4q1xlclmfloku33")
	if err != nil {
		t.Fatal(err)
	}

	requestInfo := &core.RequestInfo{
		Context: "ctx",
		Method:  "get",
		Query: map[string]string{
			"a": "123",
		},
		Body: map[string]any{
			"number":          "10",
			"number_unknown":  "20",
			"raw_json_obj":    types.JSONRaw(`{"a":123}`),
			"raw_json_arr1":   types.JSONRaw(`[123, 456]`),
			"raw_json_arr2":   types.JSONRaw(`[{"a":123},{"b":456}]`),
			"raw_json_simple": types.JSONRaw(`123`),
			"b":               456,
			"c":               map[string]int{"sub": 1},
		},
		Headers: map[string]string{
			"d": "789",
		},
		Auth: authRecord,
	}

	r := core.NewRecordFieldResolver(app, collection, requestInfo, true)

	scenarios := []struct {
		fieldName        string
		expectError      bool
		expectParamValue string // encoded json
	}{
		{"@request", true, ""},
		{"@request.invalid format", true, ""},
		{"@request.invalid_format2!", true, ""},
		{"@request.missing", true, ""},
		{"@request.context", false, `"ctx"`},
		{"@request.method", false, `"get"`},
		{"@request.query", true, ``},
		{"@request.query.a", false, `"123"`},
		{"@request.query.a.missing", false, ``},
		{"@request.headers", true, ``},
		{"@request.headers.missing", false, ``},
		{"@request.headers.d", false, `"789"`},
		{"@request.headers.d.sub", false, ``},
		{"@request.body", true, ``},
		{"@request.body.b", false, `456`},
		{"@request.body.number", false, `10`},           // number field normalization
		{"@request.body.number_unknown", false, `"20"`}, // no numeric normalizations for unknown fields
		{"@request.body.b.missing", false, ``},
		{"@request.body.c", false, `"{\"sub\":1}"`},
		{"@request.auth", true, ""},
		{"@request.auth.id", false, `"4q1xlclmfloku33"`},
		{"@request.auth.collectionId", false, `"` + authRecord.Collection().Id + `"`},
		{"@request.auth.collectionName", false, `"` + authRecord.Collection().Name + `"`},
		{"@request.auth.verified", false, `false`},
		{"@request.auth.emailVisibility", false, `false`},
		{"@request.auth.email", false, `"test@example.com"`}, // should always be returned no matter of the emailVisibility state
		{"@request.auth.missing", false, `NULL`},
		{"@request.body.raw_json_simple", false, `"123"`},
		{"@request.body.raw_json_simple.a", false, `NULL`},
		{"@request.body.raw_json_obj.a", false, `123`},
		{"@request.body.raw_json_obj.b", false, `NULL`},
		{"@request.body.raw_json_arr1.1", false, `456`},
		{"@request.body.raw_json_arr1.3", false, `NULL`},
		{"@request.body.raw_json_arr2.0.a", false, `123`},
		{"@request.body.raw_json_arr2.0.b", false, `NULL`},
	}

	for _, s := range scenarios {
		t.Run(s.fieldName, func(t *testing.T) {
			r, err := r.Resolve(s.fieldName)

			hasErr := err != nil
			if hasErr != s.expectError {
				t.Fatalf("Expected hasErr %v, got %v (%v)", s.expectError, hasErr, err)
			}

			if hasErr {
				return
			}

			// missing key
			// ---
			if len(r.Params) == 0 {
				if r.Identifier != "NULL" {
					t.Fatalf("Expected 0 placeholder parameters for %v, got %v", r.Identifier, r.Params)
				}
				return
			}

			// existing key
			// ---
			if len(r.Params) != 1 {
				t.Fatalf("Expected 1 placeholder parameter for %v, got %v", r.Identifier, r.Params)
			}

			var paramName string
			var paramValue any
			for k, v := range r.Params {
				paramName = k
				paramValue = v
			}

			if r.Identifier != ("{:" + paramName + "}") {
				t.Fatalf("Expected parameter r.Identifier %q, got %q", paramName, r.Identifier)
			}

			encodedParamValue, _ := json.Marshal(paramValue)
			if string(encodedParamValue) != s.expectParamValue {
				t.Fatalf("Expected r.Params %#v for %s, got %#v", s.expectParamValue, r.Identifier, string(encodedParamValue))
			}
		})
	}

	// ensure that the original email visibility was restored
	if authRecord.EmailVisibility() {
		t.Fatal("Expected the original authRecord emailVisibility to remain unchanged")
	}
	if v, ok := authRecord.PublicExport()[core.FieldNameEmail]; ok {
		t.Fatalf("Expected the original authRecord email to not be exported, got %q", v)
	}
}
