package apis_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

func TestRecordCrudExternalAuthList(t *testing.T) {
	t.Parallel()

	scenarios := []tests.ApiScenario{
		{
			Name:           "guest",
			Method:         http.MethodGet,
			URL:            "/api/collections/" + core.CollectionNameExternalAuths + "/records",
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"page":1`,
				`"perPage":30`,
				`"totalItems":0`,
				`"totalPages":0`,
				`"items":[]`,
			},
			ExpectedEvents: map[string]int{
				"*":                    0,
				"OnRecordsListRequest": 1,
			},
		},
		{
			Name:   "regular auth with externalAuths",
			Method: http.MethodGet,
			URL:    "/api/collections/" + core.CollectionNameExternalAuths + "/records",
			Headers: map[string]string{
				// clients, test@example.com
				"Authorization": tests.NewAuthTokenForTest("clients", "test@example.com"),
			},
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"page":1`,
				`"perPage":30`,
				`"totalItems":1`,
				`"totalPages":1`,
				`"id":"0196afca-7951-7d5f-bc27-ab60c8e0aee6"`,
			},
			ExpectedEvents: map[string]int{
				"*":                    0,
				"OnRecordsListRequest": 1,
				"OnRecordEnrich":       1,
			},
		},
		{
			Name:   "regular auth without externalAuths",
			Method: http.MethodGet,
			URL:    "/api/collections/" + core.CollectionNameExternalAuths + "/records",
			Headers: map[string]string{
				// users, test2@example.com
				"Authorization": tests.NewAuthTokenForTest("users", "test2@example.com"),
			},
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"page":1`,
				`"perPage":30`,
				`"totalItems":0`,
				`"totalPages":0`,
				`"items":[]`,
			},
			ExpectedEvents: map[string]int{
				"*":                    0,
				"OnRecordsListRequest": 1,
			},
		},
	}

	for _, scenario := range scenarios {
		scenario.Test(t)
	}
}

func TestRecordCrudExternalAuthView(t *testing.T) {
	t.Parallel()

	scenarios := []tests.ApiScenario{
		{
			Name:            "guest",
			Method:          http.MethodGet,
			URL:             "/api/collections/" + core.CollectionNameExternalAuths + "/records/0196afca-7951-71e7-8791-b7490a47960e",
			ExpectedStatus:  404,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "non-owner",
			Method: http.MethodGet,
			URL:    "/api/collections/" + core.CollectionNameExternalAuths + "/records/0196afca-7951-71e7-8791-b7490a47960e",
			Headers: map[string]string{
				// clients, test@example.com
				"Authorization": tests.NewAuthTokenForTest("clients", "test@example.com"),
			},
			ExpectedStatus:  404,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "owner",
			Method: http.MethodGet,
			URL:    "/api/collections/" + core.CollectionNameExternalAuths + "/records/0196afca-7951-71e7-8791-b7490a47960e",
			Headers: map[string]string{
				// users, test@example.com
				"Authorization": tests.NewAuthTokenForTest("users", "test@example.com"),
			},
			ExpectedStatus:  200,
			ExpectedContent: []string{`"id":"0196afca-7951-71e7-8791-b7490a47960e"`},
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

func TestRecordCrudExternalAuthDelete(t *testing.T) {
	t.Parallel()

	scenarios := []tests.ApiScenario{
		{
			Name:            "guest",
			Method:          http.MethodDelete,
			URL:             "/api/collections/" + core.CollectionNameExternalAuths + "/records/0196afca-7951-71e7-8791-b7490a47960e",
			ExpectedStatus:  404,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "non-owner",
			Method: http.MethodDelete,
			URL:    "/api/collections/" + core.CollectionNameExternalAuths + "/records/0196afca-7951-71e7-8791-b7490a47960e",
			Headers: map[string]string{
				// clients, test@example.com
				"Authorization": tests.NewAuthTokenForTest("clients", "test@example.com"),
			},
			ExpectedStatus:  404,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "owner",
			Method: http.MethodDelete,
			URL:    "/api/collections/" + core.CollectionNameExternalAuths + "/records/0196afca-7951-71e7-8791-b7490a47960e",
			Headers: map[string]string{
				// users, test@example.com
				"Authorization": tests.NewAuthTokenForTest("users", "test@example.com"),
			},
			ExpectedStatus: 204,
			ExpectedEvents: map[string]int{
				"*":                          0,
				"OnRecordDeleteRequest":      1,
				"OnModelDelete":              1,
				"OnModelDeleteExecute":       1,
				"OnModelAfterDeleteSuccess":  1,
				"OnRecordDelete":             1,
				"OnRecordDeleteExecute":      1,
				"OnRecordAfterDeleteSuccess": 1,
			},
		},
	}

	for _, scenario := range scenarios {
		scenario.Test(t)
	}
}

func TestRecordCrudExternalAuthCreate(t *testing.T) {
	t.Parallel()

	body := func() *strings.Reader {
		return strings.NewReader(`{
			"recordRef":     "0196afca-7951-76f3-b344-ae38a366ade2",
			"collectionRef": "11111111-1111-1111-1111-111111111111",
			"provider":      "github",
			"providerId":    "abc"
		}`)
	}

	scenarios := []tests.ApiScenario{
		{
			Name:            "guest",
			Method:          http.MethodPost,
			URL:             "/api/collections/" + core.CollectionNameExternalAuths + "/records",
			Body:            body(),
			ExpectedStatus:  403,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "owner regular auth",
			Method: http.MethodPost,
			URL:    "/api/collections/" + core.CollectionNameExternalAuths + "/records",
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
			URL:    "/api/collections/" + core.CollectionNameExternalAuths + "/records",
			Headers: map[string]string{
				// superusers, test@example.com
				"Authorization": tests.NewAuthTokenForTest("_superusers", "test@example.com"),
			},
			Body: body(),
			ExpectedContent: []string{
				`"recordRef":"0196afca-7951-76f3-b344-ae38a366ade2"`,
				`"providerId":"abc"`,
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

func TestRecordCrudExternalAuthUpdate(t *testing.T) {
	t.Parallel()

	body := func() *strings.Reader {
		return strings.NewReader(`{
			"providerId": "abc"
		}`)
	}

	scenarios := []tests.ApiScenario{
		{
			Name:            "guest",
			Method:          http.MethodPatch,
			URL:             "/api/collections/" + core.CollectionNameExternalAuths + "/records/0196afca-7951-71e7-8791-b7490a47960e",
			Body:            body(),
			ExpectedStatus:  403,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "owner regular auth",
			Method: http.MethodPatch,
			URL:    "/api/collections/" + core.CollectionNameExternalAuths + "/records/0196afca-7951-71e7-8791-b7490a47960e",
			Headers: map[string]string{
				// clients, test@example.com
				"Authorization": tests.NewAuthTokenForTest("clients", "test@example.com"),
			},
			Body:            body(),
			ExpectedStatus:  403,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "superusers auth",
			Method: http.MethodPatch,
			URL:    "/api/collections/" + core.CollectionNameExternalAuths + "/records/0196afca-7951-71e7-8791-b7490a47960e",
			Headers: map[string]string{
				// superusers, test@example.com
				"Authorization": tests.NewAuthTokenForTest("_superusers", "test@example.com"),
			},
			Body: body(),
			ExpectedContent: []string{
				`"id":"0196afca-7951-71e7-8791-b7490a47960e"`,
				`"providerId":"abc"`,
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
