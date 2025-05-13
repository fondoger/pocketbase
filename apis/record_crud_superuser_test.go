package apis_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

func TestRecordCrudSuperuserList(t *testing.T) {
	t.Parallel()

	scenarios := []tests.ApiScenario{
		{
			Name:            "guest",
			Method:          http.MethodGet,
			URL:             "/api/collections/" + core.CollectionNameSuperusers + "/records",
			ExpectedStatus:  403,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "non-superusers auth",
			Method: http.MethodGet,
			URL:    "/api/collections/" + core.CollectionNameSuperusers + "/records",
			Headers: map[string]string{
				// users, test@example.com
				"Authorization": tests.NewAuthTokenForTest("users", "test@example.com"),
			},
			ExpectedStatus:  403,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "superusers auth",
			Method: http.MethodGet,
			URL:    "/api/collections/" + core.CollectionNameSuperusers + "/records",
			Headers: map[string]string{
				// _superusers, test@example.com
				"Authorization": tests.NewAuthTokenForTest("_superusers", "test@example.com"),
			},
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"page":1`,
				`"perPage":30`,
				`"totalPages":1`,
				`"totalItems":4`,
				`"items":[{`,
			},
			ExpectedEvents: map[string]int{
				"*":                    0,
				"OnRecordsListRequest": 1,
				"OnRecordEnrich":       4,
			},
		},
	}

	for _, scenario := range scenarios {
		scenario.Test(t)
	}
}

func TestRecordCrudSuperuserView(t *testing.T) {
	t.Parallel()

	scenarios := []tests.ApiScenario{
		{
			Name:            "guest",
			Method:          http.MethodGet,
			URL:             "/api/collections/" + core.CollectionNameSuperusers + "/records/0196afca-7951-7dc4-a3a4-35b24b1bdccd",
			ExpectedStatus:  403,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "non-superusers auth",
			Method: http.MethodGet,
			URL:    "/api/collections/" + core.CollectionNameSuperusers + "/records/0196afca-7951-7dc4-a3a4-35b24b1bdccd",
			Headers: map[string]string{
				// users, test@example.com
				"Authorization": tests.NewAuthTokenForTest("users", "test@example.com"),
			},
			ExpectedStatus:  403,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "superusers auth",
			Method: http.MethodGet,
			URL:    "/api/collections/" + core.CollectionNameSuperusers + "/records/0196afca-7951-7dc4-a3a4-35b24b1bdccd",
			Headers: map[string]string{
				// _superusers, test@example.com
				"Authorization": tests.NewAuthTokenForTest("_superusers", "test@example.com"),
			},
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"id":"0196afca-7951-7dc4-a3a4-35b24b1bdccd"`,
			},
			ExpectedEvents: map[string]int{
				"*":                   0,
				"OnRecordViewRequest": 1,
				"OnRecordEnrich":      1,
			},
		},
	}

	for _, scenario := range scenarios {
		scenario.Test(t)
	}
}

func TestRecordCrudSuperuserDelete(t *testing.T) {
	t.Parallel()

	scenarios := []tests.ApiScenario{
		{
			Name:            "guest",
			Method:          http.MethodDelete,
			URL:             "/api/collections/" + core.CollectionNameSuperusers + "/records/0196afca-7951-76c6-adca-1029b7f143b2",
			ExpectedStatus:  403,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "non-superusers auth",
			Method: http.MethodDelete,
			URL:    "/api/collections/" + core.CollectionNameSuperusers + "/records/0196afca-7951-76c6-adca-1029b7f143b2",
			Headers: map[string]string{
				// users, test@example.com
				"Authorization": tests.NewAuthTokenForTest("users", "test@example.com"),
			},
			ExpectedStatus:  403,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "superusers auth",
			Method: http.MethodDelete,
			URL:    "/api/collections/" + core.CollectionNameSuperusers + "/records/0196afca-7951-76c6-adca-1029b7f143b2",
			Headers: map[string]string{
				// _superusers, test@example.com
				"Authorization": tests.NewAuthTokenForTest("_superusers", "test@example.com"),
			},
			ExpectedStatus: 204,
			ExpectedEvents: map[string]int{
				"*":                          0,
				"OnRecordDeleteRequest":      1,
				"OnModelDelete":              4, // + 3 AuthOrigins
				"OnModelDeleteExecute":       4,
				"OnModelAfterDeleteSuccess":  4,
				"OnRecordDelete":             4,
				"OnRecordDeleteExecute":      4,
				"OnRecordAfterDeleteSuccess": 4,
			},
		},
		{
			Name:   "delete the last superuser",
			Method: http.MethodDelete,
			URL:    "/api/collections/" + core.CollectionNameSuperusers + "/records/0196afca-7951-7dc4-a3a4-35b24b1bdccd",
			Headers: map[string]string{
				// _superusers, test@example.com
				"Authorization": tests.NewAuthTokenForTest("_superusers", "test@example.com"),
			},
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				// delete all other superusers
				superusers, err := app.FindAllRecords(core.CollectionNameSuperusers, dbx.Not(dbx.HashExp{"id": "0196afca-7951-7dc4-a3a4-35b24b1bdccd"}))
				if err != nil {
					t.Fatal(err)
				}
				for _, superuser := range superusers {
					if err = app.Delete(superuser); err != nil {
						t.Fatal(err)
					}
				}
			},
			ExpectedStatus:  400,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents: map[string]int{
				"*":                        0,
				"OnRecordDeleteRequest":    1,
				"OnModelDelete":            1,
				"OnModelAfterDeleteError":  1,
				"OnRecordDelete":           1,
				"OnRecordAfterDeleteError": 1,
			},
		},
	}

	for _, scenario := range scenarios {
		scenario.Test(t)
	}
}

func TestRecordCrudSuperuserCreate(t *testing.T) {
	t.Parallel()

	body := func() *strings.Reader {
		return strings.NewReader(`{
			"email":           "test_new@example.com",
			"password":        "1234567890",
			"passwordConfirm": "1234567890",
			"verified":        false
		}`)
	}

	scenarios := []tests.ApiScenario{
		{
			Name:            "guest",
			Method:          http.MethodPost,
			URL:             "/api/collections/" + core.CollectionNameSuperusers + "/records",
			Body:            body(),
			ExpectedStatus:  403,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "non-superusers auth",
			Method: http.MethodPost,
			URL:    "/api/collections/" + core.CollectionNameSuperusers + "/records",
			Headers: map[string]string{
				// users, test@example.com
				"Authorization": tests.NewAuthTokenForTest("users", "test@example.com"),
			},
			Body:            body(),
			ExpectedStatus:  403,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "superusers auth",
			Method: http.MethodPost,
			URL:    "/api/collections/" + core.CollectionNameSuperusers + "/records",
			Headers: map[string]string{
				// _superusers, test@example.com
				"Authorization": tests.NewAuthTokenForTest("_superusers", "test@example.com"),
			},
			Body: body(),
			ExpectedContent: []string{
				`"collectionName":"_superusers"`,
				`"email":"test_new@example.com"`,
				`"verified":true`,
			},
			ExpectedStatus: 200,
			ExpectedEvents: map[string]int{
				"*":                          0,
				"OnRecordCreateRequest":      1,
				"OnRecordEnrich":             1,
				"OnModelCreate":              1,
				"OnModelCreateExecute":       1,
				"OnModelAfterCreateSuccess":  1,
				"OnModelValidate":            1,
				"OnRecordCreate":             1,
				"OnRecordCreateExecute":      1,
				"OnRecordAfterCreateSuccess": 1,
				"OnRecordValidate":           1,
			},
		},
	}

	for _, scenario := range scenarios {
		scenario.Test(t)
	}
}

func TestRecordCrudSuperuserUpdate(t *testing.T) {
	t.Parallel()

	body := func() *strings.Reader {
		return strings.NewReader(`{
			"email":    "test_new@example.com",
			"verified": true
		}`)
	}

	scenarios := []tests.ApiScenario{
		{
			Name:            "guest",
			Method:          http.MethodPatch,
			URL:             "/api/collections/" + core.CollectionNameSuperusers + "/records/0196afca-7951-7dc4-a3a4-35b24b1bdccd",
			Body:            body(),
			ExpectedStatus:  403,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "non-superusers auth",
			Method: http.MethodPatch,
			URL:    "/api/collections/" + core.CollectionNameSuperusers + "/records/0196afca-7951-7dc4-a3a4-35b24b1bdccd",
			Headers: map[string]string{
				// users, test@example.com
				"Authorization": tests.NewAuthTokenForTest("users", "test@example.com"),
			},
			Body:            body(),
			ExpectedStatus:  403,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "superusers auth",
			Method: http.MethodPatch,
			URL:    "/api/collections/" + core.CollectionNameSuperusers + "/records/0196afca-7951-7dc4-a3a4-35b24b1bdccd",
			Headers: map[string]string{
				// _superusers, test@example.com
				"Authorization": tests.NewAuthTokenForTest("_superusers", "test@example.com"),
			},
			Body: body(),
			ExpectedContent: []string{
				`"collectionName":"_superusers"`,
				`"id":"0196afca-7951-7dc4-a3a4-35b24b1bdccd"`,
				`"email":"test_new@example.com"`,
				`"verified":true`,
			},
			ExpectedStatus: 200,
			ExpectedEvents: map[string]int{
				"*":                          0,
				"OnRecordUpdateRequest":      1,
				"OnRecordEnrich":             1,
				"OnModelUpdate":              1,
				"OnModelUpdateExecute":       1,
				"OnModelAfterUpdateSuccess":  1,
				"OnModelValidate":            1,
				"OnRecordUpdate":             1,
				"OnRecordUpdateExecute":      1,
				"OnRecordAfterUpdateSuccess": 1,
				"OnRecordValidate":           1,
			},
		},
	}

	for _, scenario := range scenarios {
		scenario.Test(t)
	}
}
