package issues

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
	"github.com/pocketbase/pocketbase/tools/search"
)

// https://github.com/fondoger/pocketbase/issues/56
func TestIssue56_FilterByJsonField(t *testing.T) {
	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	var CalledQueries []string
	db := app.DB().(*dbx.DB)
	db.QueryLogFunc = func(ctx context.Context, t time.Duration, sql string, rows *sql.Rows, err error) {
		CalledQueries = append(CalledQueries, sql)
	}

	// Create a collection with a DateTime field `updated`
	collection := core.NewBaseCollection("test_issue_56")
	collection.Fields.Add(&core.JSONField{
		Name: "json_column",
	})
	err := app.Save(collection)
	if err != nil {
		t.Fatal(err)
	}
	resolver := core.NewRecordFieldResolver(app, collection, nil, false)

	// Insert a record with a specific date
	record := core.NewRecord(collection)
	record.Set("json_column", map[string]interface{}{"strValue": "Hello, World!", "numValue": 42})
	err = app.Save(record)
	if err != nil {
		t.Fatal(err)
	}
	record2 := core.NewRecord(collection)
	record2.Set("json_column", map[string]interface{}{"strValue": "Another Record", "numValue": 100})
	err = app.Save(record2)
	if err != nil {
		t.Fatal(err)
	}

	scenarios := []struct {
		filter      string
		expectCount int
		expectSQL   string
	}{
		{
			`json_column.numValue = 42`, 1,
			`SELECT COUNT(*) AS "count" FROM "test_issue_56" WHERE JSON_QUERY_OR_NULL([[test_issue_56.json_column]], '$.numValue')::jsonb IS NOT DISTINCT FROM to_jsonb(42)`,
		},
		{
			`json_column.numValue > 50`, 1,
			`SELECT COUNT(*) AS "count" FROM "test_issue_56" WHERE JSON_QUERY_OR_NULL([[test_issue_56.json_column]], '$.numValue')::jsonb::numeric > 50`,
		},
		{
			`json_column.strValue = "Hello, World!"`, 1,
			`SELECT COUNT(*) AS "count" FROM "test_issue_56" WHERE JSON_QUERY_OR_NULL([[test_issue_56.json_column]], '$.strValue')::jsonb IS NOT DISTINCT FROM to_jsonb('Hello, World!'::text)`,
		},
		{
			`json_column.strValue != "Hello, World!"`, 1,
			`SELECT COUNT(*) AS "count" FROM "test_issue_56" WHERE JSON_QUERY_OR_NULL([[test_issue_56.json_column]], '$.strValue')::jsonb IS DISTINCT FROM to_jsonb('Hello, World!'::text)`,
		},
		// like operator is the only one that throws errors at this moment.
		{
			`json_column.strValue ~ 'ello, Worl'`, 1,
			`SELECT COUNT(*) AS "count" FROM "test_issue_56" WHERE JSON_QUERY_OR_NULL([[test_issue_56.json_column]], '$.strValue')::jsonb::text LIKE '%ello, Worl%' ESCAPE '\'`,
		},
	}

	for _, scenario := range scenarios {
		whereExpr, _ := search.FilterData(scenario.filter).BuildExpr(resolver)
		query := app.RecordQuery(collection).Select("COUNT(*) as count")
		_ = resolver.UpdateQuery(query)
		var count int
		err = query.AndWhere(whereExpr).Row(&count)
		if err != nil {
			t.Fatal(err)
		}

		if count != scenario.expectCount {
			t.Fatalf("Expected %d record, got %d", scenario.expectCount, count)
		}

		lastQuery := CalledQueries[len(CalledQueries)-1]
		if lastQuery != scenario.expectSQL {
			t.Fatalf("Expected executed SQL:\n%s\nGot:\n%s", scenario.expectSQL, lastQuery)
		}
	}
}
