package apis_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

func TestRecordCrudAuthOriginList(t *testing.T) {
	t.Parallel()

	scenarios := []tests.ApiScenario{
		{
			Name:           "guest",
			Method:         http.MethodGet,
			URL:            "/api/collections/" + core.CollectionNameAuthOrigins + "/records",
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
			Name:   "regular auth with authOrigins",
			Method: http.MethodGet,
			URL:    "/api/collections/" + core.CollectionNameAuthOrigins + "/records",
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
				`"id":"0196afca-7950-7e99-906f-93f836ec07bf"`,
			},
			ExpectedEvents: map[string]int{
				"*":                    0,
				"OnRecordsListRequest": 1,
				"OnRecordEnrich":       1,
			},
		},
		{
			Name:   "regular auth without authOrigins",
			Method: http.MethodGet,
			URL:    "/api/collections/" + core.CollectionNameAuthOrigins + "/records",
			Headers: map[string]string{
				// users/test@example.com
				"Authorization": tests.NewAuthTokenForTest("users", "test@example.com"),
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

func TestRecordCrudAuthOriginView(t *testing.T) {
	t.Parallel()

	scenarios := []tests.ApiScenario{
		{
			Name:            "guest",
			Method:          http.MethodGet,
			URL:             "/api/collections/" + core.CollectionNameAuthOrigins + "/records/0196afca-7950-7e99-906f-93f836ec07bf",
			ExpectedStatus:  404,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "non-owner",
			Method: http.MethodGet,
			URL:    "/api/collections/" + core.CollectionNameAuthOrigins + "/records/0196afca-7950-7e99-906f-93f836ec07bf",
			Headers: map[string]string{
				// users, test@example.com
				"Authorization": tests.NewAuthTokenForTest("users", "test@example.com"),
			},
			ExpectedStatus:  404,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "owner",
			Method: http.MethodGet,
			URL:    "/api/collections/" + core.CollectionNameAuthOrigins + "/records/0196afca-7950-7e99-906f-93f836ec07bf",
			Headers: map[string]string{
				// clients, test@example.com
				"Authorization": tests.NewAuthTokenForTest("clients", "test@example.com"),
			},
			ExpectedStatus:  200,
			ExpectedContent: []string{`"id":"0196afca-7950-7e99-906f-93f836ec07bf"`},
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

func TestRecordCrudAuthOriginDelete(t *testing.T) {
	t.Parallel()

	scenarios := []tests.ApiScenario{
		{
			Name:            "guest",
			Method:          http.MethodDelete,
			URL:             "/api/collections/" + core.CollectionNameAuthOrigins + "/records/0196afca-7950-7e99-906f-93f836ec07bf",
			ExpectedStatus:  404,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "non-owner",
			Method: http.MethodDelete,
			URL:    "/api/collections/" + core.CollectionNameAuthOrigins + "/records/0196afca-7950-7e99-906f-93f836ec07bf",
			Headers: map[string]string{
				// users, test@example.com
				"Authorization": tests.NewAuthTokenForTest("users", "test@example.com"),
			},
			ExpectedStatus:  404,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "owner",
			Method: http.MethodDelete,
			URL:    "/api/collections/" + core.CollectionNameAuthOrigins + "/records/0196afca-7950-7e99-906f-93f836ec07bf",
			Headers: map[string]string{
				// clients, test@example.com
				"Authorization": tests.NewAuthTokenForTest("clients", "test@example.com"),
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

func TestRecordCrudAuthOriginCreate(t *testing.T) {
	t.Parallel()

	body := func() *strings.Reader {
		return strings.NewReader(`{
			"recordRef":     "0196afca-7951-76f3-b344-ae38a366ade2",
			"collectionRef": "11111111-1111-1111-1111-111111111111",
			"fingerprint":   "abc"
		}`)
	}

	scenarios := []tests.ApiScenario{
		{
			Name:            "guest",
			Method:          http.MethodPost,
			URL:             "/api/collections/" + core.CollectionNameAuthOrigins + "/records",
			Body:            body(),
			ExpectedStatus:  403,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "owner regular auth",
			Method: http.MethodPost,
			URL:    "/api/collections/" + core.CollectionNameAuthOrigins + "/records",
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
			URL:    "/api/collections/" + core.CollectionNameAuthOrigins + "/records",
			Headers: map[string]string{
				// superusers, test@example.com
				"Authorization": tests.NewAuthTokenForTest("_superusers", "test@example.com"),
			},
			Body: body(),
			ExpectedContent: []string{
				`"fingerprint":"abc"`,
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

func TestRecordCrudAuthOriginUpdate(t *testing.T) {
	t.Parallel()

	body := func() *strings.Reader {
		return strings.NewReader(`{
			"fingerprint":"abc"
		}`)
	}

	scenarios := []tests.ApiScenario{
		{
			Name:            "guest",
			Method:          http.MethodPatch,
			URL:             "/api/collections/" + core.CollectionNameAuthOrigins + "/records/0196afca-7950-7e99-906f-93f836ec07bf",
			Body:            body(),
			ExpectedStatus:  403,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "owner regular auth",
			Method: http.MethodPatch,
			URL:    "/api/collections/" + core.CollectionNameAuthOrigins + "/records/0196afca-7950-7e99-906f-93f836ec07bf",
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
			URL:    "/api/collections/" + core.CollectionNameAuthOrigins + "/records/0196afca-7950-7e99-906f-93f836ec07bf",
			Headers: map[string]string{
				// superusers, test@example.com
				"Authorization": tests.NewAuthTokenForTest("_superusers", "test@example.com"),
			},
			Body: body(),
			ExpectedContent: []string{
				`"id":"0196afca-7950-7e99-906f-93f836ec07bf"`,
				`"fingerprint":"abc"`,
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
