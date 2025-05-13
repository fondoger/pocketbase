package core_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
	"github.com/pocketbase/pocketbase/tools/dbutils"
	"github.com/pocketbase/pocketbase/tools/types"
)

func TestRecordQueryWithDifferentCollectionValues(t *testing.T) {
	t.Parallel()

	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	collection, err := app.FindCollectionByNameOrId("demo1")
	if err != nil {
		t.Fatal(err)
	}

	scenarios := []struct {
		name          string
		collection    any
		expectedTotal int
		expectError   bool
	}{
		{"with nil value", nil, 0, true},
		{"with invalid or missing collection id/name", "missing", 0, true},
		{"with pointer model", collection, 3, false},
		{"with value model", *collection, 3, false},
		{"with name", "demo1", 3, false},
		{"with id", "0196afca-7951-7653-beca-d69f40c17bcd", 3, false},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			var records []*core.Record
			err := app.RecordQuery(s.collection).All(&records)

			hasErr := err != nil
			if hasErr != s.expectError {
				t.Fatalf("Expected hasError %v, got %v", s.expectError, hasErr)
			}

			if total := len(records); total != s.expectedTotal {
				t.Fatalf("Expected %d records, got %d", s.expectedTotal, total)
			}
		})
	}
}

func TestRecordQueryOne(t *testing.T) {
	t.Parallel()

	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	scenarios := []struct {
		name       string
		collection string
		recordId   string
		model      any
	}{
		{
			"record model",
			"demo1",
			"0196afca-7951-7ba1-8cef-b59777e4d838",
			&core.Record{},
		},
		{
			"record proxy",
			"demo1",
			"0196afca-7951-7ba1-8cef-b59777e4d838",
			&struct {
				core.BaseRecordProxy
			}{},
		},
		{
			"custom struct",
			"demo1",
			"0196afca-7951-7ba1-8cef-b59777e4d838",
			&struct {
				Id string `db:"id" json:"id"`
			}{},
		},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			collection, err := app.FindCollectionByNameOrId(s.collection)
			if err != nil {
				t.Fatal(err)
			}

			q := app.RecordQuery(collection).
				Where(dbx.HashExp{"id": s.recordId})

			if err := q.One(s.model); err != nil {
				t.Fatal(err)
			}

			raw, err := json.Marshal(s.model)
			if err != nil {
				t.Fatal(err)
			}
			rawStr := string(raw)

			if !strings.Contains(rawStr, fmt.Sprintf(`"id":%q`, s.recordId)) {
				t.Fatalf("Missing id %q in\n%s", s.recordId, rawStr)
			}
		})
	}
}

func TestRecordQueryAll(t *testing.T) {
	t.Parallel()

	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	type customStructs struct {
		Id string `db:"id" json:"id"`
	}

	type mockRecordProxy struct {
		core.BaseRecordProxy
	}

	scenarios := []struct {
		name       string
		collection string
		recordIds  []any
		result     any
	}{
		{
			"slice of Record models",
			"demo1",
			[]any{"0196afca-7951-7ba1-8cef-b59777e4d838", "0196afca-7951-752e-972d-502c0843467d"},
			&[]core.Record{},
		},
		{
			"slice of pointer Record models",
			"demo1",
			[]any{"0196afca-7951-7ba1-8cef-b59777e4d838", "0196afca-7951-752e-972d-502c0843467d"},
			&[]*core.Record{},
		},
		{
			"slice of Record proxies",
			"demo1",
			[]any{"0196afca-7951-7ba1-8cef-b59777e4d838", "0196afca-7951-752e-972d-502c0843467d"},
			&[]mockRecordProxy{},
		},
		{
			"slice of pointer Record proxies",
			"demo1",
			[]any{"0196afca-7951-7ba1-8cef-b59777e4d838", "0196afca-7951-752e-972d-502c0843467d"},
			&[]mockRecordProxy{},
		},
		{
			"slice of custom structs",
			"demo1",
			[]any{"0196afca-7951-7ba1-8cef-b59777e4d838", "0196afca-7951-752e-972d-502c0843467d"},
			&[]customStructs{},
		},
		{
			"slice of pointer custom structs",
			"demo1",
			[]any{"0196afca-7951-7ba1-8cef-b59777e4d838", "0196afca-7951-752e-972d-502c0843467d"},
			&[]customStructs{},
		},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			collection, err := app.FindCollectionByNameOrId(s.collection)
			if err != nil {
				t.Fatal(err)
			}

			q := app.RecordQuery(collection).
				Where(dbx.HashExp{"id": s.recordIds})

			if err := q.All(s.result); err != nil {
				t.Fatal(err)
			}

			raw, err := json.Marshal(s.result)
			if err != nil {
				t.Fatal(err)
			}
			rawStr := string(raw)

			sliceOfMaps := []any{}
			if err := json.Unmarshal(raw, &sliceOfMaps); err != nil {
				t.Fatal(err)
			}

			if len(sliceOfMaps) != len(s.recordIds) {
				t.Fatalf("Expected %d items, got %d", len(s.recordIds), len(sliceOfMaps))
			}

			for _, id := range s.recordIds {
				if !strings.Contains(rawStr, fmt.Sprintf(`"id":%q`, id)) {
					t.Fatalf("Missing id %q in\n%s", id, rawStr)
				}
			}
		})
	}
}

func TestFindRecordById(t *testing.T) {
	t.Parallel()

	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	scenarios := []struct {
		collectionIdOrName string
		id                 string
		filters            []func(q *dbx.SelectQuery) error
		expectError        bool
	}{
		{"demo2", "missing", nil, true},
		{"missing", "0196afca-7951-753b-abd9-264df800cf28", nil, true},
		{"demo2", "0196afca-7951-753b-abd9-264df800cf28", nil, false},
		{"demo2", "0196afca-7951-753b-abd9-264df800cf28", []func(q *dbx.SelectQuery) error{}, false},
		{"demo2", "0196afca-7951-753b-abd9-264df800cf28", []func(q *dbx.SelectQuery) error{nil, nil}, false},
		{"demo2", "0196afca-7951-753b-abd9-264df800cf28", []func(q *dbx.SelectQuery) error{
			nil,
			func(q *dbx.SelectQuery) error { return nil },
		}, false},
		{"demo2", "0196afca-7951-753b-abd9-264df800cf28", []func(q *dbx.SelectQuery) error{
			func(q *dbx.SelectQuery) error {
				q.AndWhere(dbx.HashExp{"title": "missing"})
				return nil
			},
		}, true},
		{"demo2", "0196afca-7951-753b-abd9-264df800cf28", []func(q *dbx.SelectQuery) error{
			func(q *dbx.SelectQuery) error {
				return errors.New("test error")
			},
		}, true},
		{"demo2", "0196afca-7951-753b-abd9-264df800cf28", []func(q *dbx.SelectQuery) error{
			func(q *dbx.SelectQuery) error {
				q.AndWhere(dbx.HashExp{"title": "test3"})
				return nil
			},
		}, false},
		{"demo2", "0196afca-7951-753b-abd9-264df800cf28", []func(q *dbx.SelectQuery) error{
			func(q *dbx.SelectQuery) error {
				q.AndWhere(dbx.HashExp{"title": "test3"})
				return nil
			},
			nil,
		}, false},
		{"demo2", "0196afca-7951-753b-abd9-264df800cf28", []func(q *dbx.SelectQuery) error{
			func(q *dbx.SelectQuery) error {
				q.AndWhere(dbx.HashExp{"title": "test3"})
				return nil
			},
			func(q *dbx.SelectQuery) error {
				q.AndWhere(dbx.HashExp{"active": false})
				return nil
			},
		}, true},
		{"0196afca-7951-7db1-a330-feb700e70dfc", "0196afca-7951-753b-abd9-264df800cf28", []func(q *dbx.SelectQuery) error{
			func(q *dbx.SelectQuery) error {
				q.AndWhere(dbx.HashExp{"title": "test3"})
				return nil
			},
			func(q *dbx.SelectQuery) error {
				q.AndWhere(dbx.HashExp{"active": true})
				return nil
			},
		}, false},
	}

	for i, s := range scenarios {
		t.Run(fmt.Sprintf("%d_%s_%s_%d", i, s.collectionIdOrName, s.id, len(s.filters)), func(t *testing.T) {
			record, err := app.FindRecordById(
				s.collectionIdOrName,
				s.id,
				s.filters...,
			)

			hasErr := err != nil
			if hasErr != s.expectError {
				t.Fatalf("Expected hasErr to be %v, got %v (%v)", s.expectError, hasErr, err)
			}

			if record != nil && record.Id != s.id {
				t.Fatalf("Expected record with id %s, got %s", s.id, record.Id)
			}
		})
	}
}

func TestFindRecordsByIds(t *testing.T) {
	t.Parallel()

	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	scenarios := []struct {
		collectionIdOrName string
		ids                []string
		filters            []func(q *dbx.SelectQuery) error
		expectTotal        int
		expectError        bool
	}{
		{"demo2", []string{}, nil, 0, false},
		{"demo2", []string{""}, nil, 0, false},
		{"demo2", []string{"missing"}, nil, 0, false},
		{"missing", []string{"0196afca-7951-753b-abd9-264df800cf28"}, nil, 0, true},
		{"demo2", []string{"0196afca-7951-753b-abd9-264df800cf28"}, nil, 1, false},
		{"0196afca-7951-7db1-a330-feb700e70dfc", []string{"0196afca-7951-753b-abd9-264df800cf28"}, nil, 1, false},
		{
			"demo2",
			[]string{"0196afca-7951-753b-abd9-264df800cf28", "0196afca-7951-70d0-bcc5-206ed6a14bea"},
			nil,
			2,
			false,
		},
		{
			"demo2",
			[]string{"0196afca-7951-753b-abd9-264df800cf28", "0196afca-7951-70d0-bcc5-206ed6a14bea"},
			[]func(q *dbx.SelectQuery) error{},
			2,
			false,
		},
		{
			"demo2",
			[]string{"0196afca-7951-753b-abd9-264df800cf28", "0196afca-7951-70d0-bcc5-206ed6a14bea"},
			[]func(q *dbx.SelectQuery) error{nil, nil},
			2,
			false,
		},
		{
			"demo2",
			[]string{"0196afca-7951-753b-abd9-264df800cf28", "0196afca-7951-70d0-bcc5-206ed6a14bea"},
			[]func(q *dbx.SelectQuery) error{
				func(q *dbx.SelectQuery) error {
					return nil // empty filter
				},
			},
			2,
			false,
		},
		{
			"demo2",
			[]string{"0196afca-7951-753b-abd9-264df800cf28", "0196afca-7951-70d0-bcc5-206ed6a14bea"},
			[]func(q *dbx.SelectQuery) error{
				func(q *dbx.SelectQuery) error {
					return nil // empty filter
				},
				func(q *dbx.SelectQuery) error {
					return errors.New("test error")
				},
			},
			0,
			true,
		},
		{
			"demo2",
			[]string{"0196afca-7951-753b-abd9-264df800cf28", "0196afca-7951-70d0-bcc5-206ed6a14bea"},
			[]func(q *dbx.SelectQuery) error{
				func(q *dbx.SelectQuery) error {
					q.AndWhere(dbx.HashExp{"active": true})
					return nil
				},
				nil,
			},
			1,
			false,
		},
		{
			"0196afca-7951-7db1-a330-feb700e70dfc",
			[]string{"0196afca-7951-753b-abd9-264df800cf28", "0196afca-7951-70d0-bcc5-206ed6a14bea"},
			[]func(q *dbx.SelectQuery) error{
				func(q *dbx.SelectQuery) error {
					q.AndWhere(dbx.HashExp{"active": true})
					return nil
				},
				func(q *dbx.SelectQuery) error {
					q.AndWhere(dbx.Not(dbx.HashExp{"title": ""}))
					return nil
				},
			},
			1,
			false,
		},
	}

	for i, s := range scenarios {
		t.Run(fmt.Sprintf("%d_%s_%v_%d", i, s.collectionIdOrName, s.ids, len(s.filters)), func(t *testing.T) {
			records, err := app.FindRecordsByIds(
				s.collectionIdOrName,
				s.ids,
				s.filters...,
			)

			hasErr := err != nil
			if hasErr != s.expectError {
				t.Fatalf("Expected hasErr to be %v, got %v (%v)", s.expectError, hasErr, err)
			}

			if len(records) != s.expectTotal {
				t.Fatalf("Expected %d records, got %d", s.expectTotal, len(records))
			}

			for _, r := range records {
				if !slices.Contains(s.ids, r.Id) {
					t.Fatalf("Couldn't find id %s in %v", r.Id, s.ids)
				}
			}
		})
	}
}

func TestFindAllRecords(t *testing.T) {
	t.Parallel()

	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	scenarios := []struct {
		collectionIdOrName string
		expressions        []dbx.Expression
		expectIds          []string
		expectError        bool
	}{
		{
			"missing",
			nil,
			[]string{},
			true,
		},
		{
			"demo2",
			nil,
			[]string{
				"0196afca-7951-78f8-bbc8-59d5d917adff",
				"0196afca-7951-70d0-bcc5-206ed6a14bea",
				"0196afca-7951-753b-abd9-264df800cf28",
			},
			false,
		},
		{
			"demo2",
			[]dbx.Expression{
				nil,
				dbx.HashExp{"id": "123"},
			},
			[]string{},
			false,
		},
		{
			"0196afca-7951-7db1-a330-feb700e70dfc",
			[]dbx.Expression{
				dbx.Like("title", "test").Match(true, true),
				dbx.HashExp{"active": true},
			},
			[]string{
				"0196afca-7951-78f8-bbc8-59d5d917adff",
				"0196afca-7951-753b-abd9-264df800cf28",
			},
			false,
		},
	}

	for i, s := range scenarios {
		t.Run(fmt.Sprintf("%d_%s", i, s.collectionIdOrName), func(t *testing.T) {
			records, err := app.FindAllRecords(s.collectionIdOrName, s.expressions...)

			hasErr := err != nil
			if hasErr != s.expectError {
				t.Fatalf("Expected hasErr to be %v, got %v (%v)", s.expectError, hasErr, err)
			}

			if len(records) != len(s.expectIds) {
				t.Fatalf("Expected %d records, got %d", len(s.expectIds), len(records))
			}

			for _, r := range records {
				if !slices.Contains(s.expectIds, r.Id) {
					t.Fatalf("Couldn't find id %s in %v", r.Id, s.expectIds)
				}
			}
		})
	}
}

func TestFindFirstRecordByData(t *testing.T) {
	t.Parallel()

	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	scenarios := []struct {
		collectionIdOrName string
		key                string
		value              any
		expectId           string
		expectError        bool
	}{
		{
			"missing",
			"id",
			"0196afca-7951-70d0-bcc5-206ed6a14bea",
			"0196afca-7951-70d0-bcc5-206ed6a14bea",
			true,
		},
		{
			"demo2",
			"",
			"0196afca-7951-70d0-bcc5-206ed6a14bea",
			"",
			true,
		},
		{
			"demo2",
			"id",
			"invalid",
			"",
			true,
		},
		{
			"demo2",
			"id",
			"0196afca-7951-70d0-bcc5-206ed6a14bea",
			"0196afca-7951-70d0-bcc5-206ed6a14bea",
			false,
		},
		{
			"0196afca-7951-7db1-a330-feb700e70dfc",
			"title",
			"test3",
			"0196afca-7951-753b-abd9-264df800cf28",
			false,
		},
	}

	for i, s := range scenarios {
		t.Run(fmt.Sprintf("%d_%s_%s_%v", i, s.collectionIdOrName, s.key, s.value), func(t *testing.T) {
			record, err := app.FindFirstRecordByData(s.collectionIdOrName, s.key, s.value)

			hasErr := err != nil
			if hasErr != s.expectError {
				t.Fatalf("Expected hasErr to be %v, got %v (%v)", s.expectError, hasErr, err)
			}

			if !s.expectError && record.Id != s.expectId {
				t.Fatalf("Expected record with id %s, got %v", s.expectId, record.Id)
			}
		})
	}
}

func TestFindRecordsByFilter(t *testing.T) {
	t.Parallel()

	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	scenarios := []struct {
		name               string
		collectionIdOrName string
		filter             string
		sort               string
		limit              int
		offset             int
		params             []dbx.Params
		expectError        bool
		expectRecordIds    []string
	}{
		{
			"missing collection",
			"missing",
			"id != ''",
			"",
			0,
			0,
			nil,
			true,
			nil,
		},
		{
			"invalid filter",
			"demo2",
			"someMissingField > 1",
			"",
			0,
			0,
			nil,
			true,
			nil,
		},
		{
			"empty filter",
			"demo2",
			"",
			"",
			0,
			0,
			nil,
			false,
			[]string{
				"0196afca-7951-70d0-bcc5-206ed6a14bea",
				"0196afca-7951-78f8-bbc8-59d5d917adff",
				"0196afca-7951-753b-abd9-264df800cf28",
			},
		},
		{
			"simple filter",
			"demo2",
			"id != ''",
			"",
			0,
			0,
			nil,
			false,
			[]string{
				"0196afca-7951-70d0-bcc5-206ed6a14bea",
				"0196afca-7951-78f8-bbc8-59d5d917adff",
				"0196afca-7951-753b-abd9-264df800cf28",
			},
		},
		{
			"multi-condition filter with sort",
			"demo2",
			"id != '' && active=true",
			"-created,title",
			-1, // should behave the same as 0
			0,
			nil,
			false,
			[]string{
				"0196afca-7951-753b-abd9-264df800cf28",
				"0196afca-7951-78f8-bbc8-59d5d917adff",
			},
		},
		{
			"with limit and offset",
			"0196afca-7951-7db1-a330-feb700e70dfc",
			"id != ''",
			"title",
			2,
			1,
			nil,
			false,
			[]string{
				"0196afca-7951-78f8-bbc8-59d5d917adff",
				"0196afca-7951-753b-abd9-264df800cf28",
			},
		},
		{
			"with placeholder params",
			"demo2",
			"active = {:active}",
			"",
			10,
			0,
			[]dbx.Params{{"active": false}},
			false,
			[]string{
				"0196afca-7951-70d0-bcc5-206ed6a14bea",
			},
		},
		{
			"with json filter and sort",
			"demo4",
			"json_object != null && json_object.a.b = 'test'",
			"-json_object.a",
			10,
			0,
			[]dbx.Params{{"active": false}},
			false,
			[]string{
				"0196afca-7951-7bca-95b3-3b8b92760ec5",
			},
		},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			records, err := app.FindRecordsByFilter(
				s.collectionIdOrName,
				s.filter,
				s.sort,
				s.limit,
				s.offset,
				s.params...,
			)

			hasErr := err != nil
			if hasErr != s.expectError {
				t.Fatalf("Expected hasErr to be %v, got %v (%v)", s.expectError, hasErr, err)
			}

			if hasErr {
				return
			}

			if len(records) != len(s.expectRecordIds) {
				t.Fatalf("Expected %d records, got %d", len(s.expectRecordIds), len(records))
			}

			for i, id := range s.expectRecordIds {
				if id != records[i].Id {
					t.Fatalf("Expected record with id %q, got %q at index %d", id, records[i].Id, i)
				}
			}
		})
	}
}

func TestFindFirstRecordByFilter(t *testing.T) {
	t.Parallel()

	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	scenarios := []struct {
		name               string
		collectionIdOrName string
		filter             string
		params             []dbx.Params
		expectError        bool
		expectRecordId     string
	}{
		{
			"missing collection",
			"missing",
			"id != ''",
			nil,
			true,
			"",
		},
		{
			"invalid filter",
			"demo2",
			"someMissingField > 1",
			nil,
			true,
			"",
		},
		{
			"empty filter",
			"demo2",
			"",
			nil,
			false,
			"0196afca-7951-70d0-bcc5-206ed6a14bea",
		},
		{
			"valid filter but no matches",
			"demo2",
			"id = 'test'",
			nil,
			true,
			"",
		},
		{
			"valid filter and multiple matches",
			"0196afca-7951-7db1-a330-feb700e70dfc",
			"id != ''",
			nil,
			false,
			"0196afca-7951-70d0-bcc5-206ed6a14bea",
		},
		{
			"with placeholder params",
			"demo2",
			"active = {:active}",
			[]dbx.Params{{"active": false}},
			false,
			"0196afca-7951-70d0-bcc5-206ed6a14bea",
		},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			record, err := app.FindFirstRecordByFilter(s.collectionIdOrName, s.filter, s.params...)

			hasErr := err != nil
			if hasErr != s.expectError {
				t.Fatalf("Expected hasErr to be %v, got %v (%v)", s.expectError, hasErr, err)
			}

			if hasErr {
				return
			}

			if record.Id != s.expectRecordId {
				t.Fatalf("Expected record with id %q, got %q", s.expectRecordId, record.Id)
			}
		})
	}
}

func TestCountRecords(t *testing.T) {
	t.Parallel()

	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	scenarios := []struct {
		name               string
		collectionIdOrName string
		expressions        []dbx.Expression
		expectTotal        int64
		expectError        bool
	}{
		{
			"missing collection",
			"missing",
			nil,
			0,
			true,
		},
		{
			"valid collection name",
			"demo2",
			nil,
			3,
			false,
		},
		{
			"valid collection id",
			"0196afca-7951-7db1-a330-feb700e70dfc",
			nil,
			3,
			false,
		},
		{
			"nil expression",
			"demo2",
			[]dbx.Expression{nil},
			3,
			false,
		},
		{
			"no matches",
			"demo2",
			[]dbx.Expression{
				nil,
				dbx.Like("title", "missing"),
				dbx.HashExp{"active": true},
			},
			0,
			false,
		},
		{
			"with matches",
			"demo2",
			[]dbx.Expression{
				nil,
				dbx.Like("title", "test"),
				dbx.HashExp{"active": true},
			},
			2,
			false,
		},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			total, err := app.CountRecords(s.collectionIdOrName, s.expressions...)

			hasErr := err != nil
			if hasErr != s.expectError {
				t.Fatalf("Expected hasErr to be %v, got %v (%v)", s.expectError, hasErr, err)
			}

			if total != s.expectTotal {
				t.Fatalf("Expected total %d, got %d", s.expectTotal, total)
			}
		})
	}
}

func TestFindAuthRecordByToken(t *testing.T) {
	t.Parallel()

	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	scenarios := []struct {
		name       string
		token      string
		types      []string
		expectedId string
	}{
		{
			"empty token",
			"",
			nil,
			"",
		},
		{
			"invalid token",
			"invalid",
			nil,
			"",
		},
		{
			"expired token",
			tests.NewAuthTokenForTest("users", "test@example.com", tests.TokenExpired(true)),
			nil,
			"",
		},
		{
			"valid auth token",
			tests.NewAuthTokenForTest("users", "test@example.com"),
			nil,
			"0196afca-7951-76f3-b344-ae38a366ade2",
		},
		{
			"valid verification token",
			tests.NewAuthTokenForTest("nologin", "test@example.com", tests.CustomToken("verification", map[string]any { "email" : "test@example.com"})),
			nil,
			"0196afca-7951-7d0f-a64c-cd080e9956d5",
		},
		{
			"auth token with file type only check",
			tests.NewAuthTokenForTest("users", "test@example.com"),
			[]string{core.TokenTypeFile},
			"",
		},
		{
			"auth token with file and auth type check",
			tests.NewAuthTokenForTest("users", "test@example.com"),
			[]string{core.TokenTypeFile, core.TokenTypeAuth},
			"0196afca-7951-76f3-b344-ae38a366ade2",
		},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			record, err := app.FindAuthRecordByToken(s.token, s.types...)

			hasErr := err != nil
			expectErr := s.expectedId == ""
			if hasErr != expectErr {
				t.Fatalf("Expected hasErr to be %v, got %v (%v)", expectErr, hasErr, err)
			}

			if hasErr {
				return
			}

			if record.Id != s.expectedId {
				t.Fatalf("Expected record with id %q, got %q", s.expectedId, record.Id)
			}
		})
	}
}

func TestFindAuthRecordByEmail(t *testing.T) {
	t.Parallel()

	scenarios := []struct {
		collectionIdOrName string
		email              string
		nocaseIndex        bool
		expectError        bool
	}{
		{"missing", "test@example.com", false, true},
		{"demo2", "test@example.com", false, true},
		{"users", "missing@example.com", false, true},
		{"users", "test@example.com", false, false},
		{"clients", "test2@example.com", false, false},
		// case-insensitive tests
		{"clients", "TeSt2@example.com", false, true},
		{"clients", "TeSt2@example.com", true, false},
	}

	for _, s := range scenarios {
		t.Run(fmt.Sprintf("%s_%s", s.collectionIdOrName, s.email), func(t *testing.T) {
			app, _ := tests.NewTestApp()
			defer app.Cleanup()

			collection, _ := app.FindCollectionByNameOrId(s.collectionIdOrName)
			if collection != nil {
				emailIndex, ok := dbutils.FindSingleColumnUniqueIndex(collection.Indexes, core.FieldNameEmail)
				if ok {
					if s.nocaseIndex {
						emailIndex.Columns[0].Collate = "nocase"
					} else {
						emailIndex.Columns[0].Collate = ""
					}

					collection.RemoveIndex(emailIndex.IndexName)
					collection.Indexes = append(collection.Indexes, emailIndex.Build())
					err := app.Save(collection)
					if err != nil {
						t.Fatalf("Failed to update email index: %v", err)
					}
				}
			}

			record, err := app.FindAuthRecordByEmail(s.collectionIdOrName, s.email)

			hasErr := err != nil
			if hasErr != s.expectError {
				t.Fatalf("Expected hasErr to be %v, got %v (%v)", s.expectError, hasErr, err)
			}

			if hasErr {
				return
			}

			if !strings.EqualFold(record.Email(), s.email) {
				t.Fatalf("Expected record with email %s, got %s", s.email, record.Email())
			}
		})
	}
}

func TestCanAccessRecord(t *testing.T) {
	t.Parallel()

	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	superuser, err := app.FindAuthRecordByEmail(core.CollectionNameSuperusers, "test@example.com")
	if err != nil {
		t.Fatal(err)
	}

	user, err := app.FindAuthRecordByEmail("users", "test@example.com")
	if err != nil {
		t.Fatal(err)
	}

	record, err := app.FindRecordById("demo1", "0196afca-7951-7a62-9100-f77edbf6f060")
	if err != nil {
		t.Fatal(err)
	}

	scenarios := []struct {
		name        string
		record      *core.Record
		requestInfo *core.RequestInfo
		rule        *string
		expected    bool
		expectError bool
	}{
		{
			"as superuser with nil rule",
			record,
			&core.RequestInfo{
				Auth: superuser,
			},
			nil,
			true,
			false,
		},
		{
			"as superuser with non-empty rule",
			record,
			&core.RequestInfo{
				Auth: superuser,
			},
			types.Pointer("id = ''"), // the filter rule should be ignored
			true,
			false,
		},
		{
			"as superuser with invalid rule",
			record,
			&core.RequestInfo{
				Auth: superuser,
			},
			types.Pointer("id ?!@ 1"), // the filter rule should be ignored
			true,
			false,
		},
		{
			"as guest with nil rule",
			record,
			&core.RequestInfo{},
			nil,
			false,
			false,
		},
		{
			"as guest with empty rule",
			record,
			&core.RequestInfo{},
			types.Pointer(""),
			true,
			false,
		},
		{
			"as guest with invalid rule",
			record,
			&core.RequestInfo{},
			types.Pointer("id ?!@ 1"),
			false,
			true,
		},
		{
			"as guest with mismatched rule",
			record,
			&core.RequestInfo{},
			types.Pointer("@request.auth.id != ''"),
			false,
			false,
		},
		{
			"as guest with matched rule",
			record,
			&core.RequestInfo{
				Body: map[string]any{"test": 1},
			},
			types.Pointer("@request.auth.id != '' || @request.body.test = 1"),
			true,
			false,
		},
		{
			"as auth record with nil rule",
			record,
			&core.RequestInfo{
				Auth: user,
			},
			nil,
			false,
			false,
		},
		{
			"as auth record with empty rule",
			record,
			&core.RequestInfo{
				Auth: user,
			},
			types.Pointer(""),
			true,
			false,
		},
		{
			"as auth record with invalid rule",
			record,
			&core.RequestInfo{
				Auth: user,
			},
			types.Pointer("id ?!@ 1"),
			false,
			true,
		},
		{
			"as auth record with mismatched rule",
			record,
			&core.RequestInfo{
				Auth: user,
				Body: map[string]any{"test": 1},
			},
			types.Pointer("@request.auth.id != '' && @request.body.test > 1"),
			false,
			false,
		},
		{
			"as auth record with matched rule",
			record,
			&core.RequestInfo{
				Auth: user,
				Body: map[string]any{"test": 2},
			},
			types.Pointer("@request.auth.id != '' && @request.body.test > 1"),
			true,
			false,
		},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			result, err := app.CanAccessRecord(s.record, s.requestInfo, s.rule)

			if result != s.expected {
				t.Fatalf("Expected %v, got %v", s.expected, result)
			}

			hasErr := err != nil
			if hasErr != s.expectError {
				t.Fatalf("Expected hasErr %v, got %v (%v)", s.expectError, hasErr, err)
			}
		})
	}
}
