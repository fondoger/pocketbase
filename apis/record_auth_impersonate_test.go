package apis_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/tests"
)

func TestRecordAuthImpersonate(t *testing.T) {
	t.Parallel()

	scenarios := []tests.ApiScenario{
		{
			Name:            "unauthorized",
			Method:          http.MethodPost,
			URL:             "/api/collections/users/impersonate/0196afca-7951-76f3-b344-ae38a366ade2",
			ExpectedStatus:  401,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "authorized as different user",
			Method: http.MethodPost,
			URL:    "/api/collections/users/impersonate/0196afca-7951-76f3-b344-ae38a366ade2",
			Headers: map[string]string{
				"Authorization": tests.NewAuthTokenForTest("users", "test2@example.com"),
			},
			ExpectedStatus:  403,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "authorized as the same user",
			Method: http.MethodPost,
			URL:    "/api/collections/users/impersonate/0196afca-7951-76f3-b344-ae38a366ade2",
			Headers: map[string]string{
				"Authorization": tests.NewAuthTokenForTest("users", "test@example.com"),
			},
			ExpectedStatus:  403,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "authorized as superuser",
			Method: http.MethodPost,
			URL:    "/api/collections/users/impersonate/0196afca-7951-76f3-b344-ae38a366ade2",
			Headers: map[string]string{
				"Authorization": tests.NewAuthTokenForTest("_superusers", "test@example.com"),
			},
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"token":"`,
				`"id":"0196afca-7951-76f3-b344-ae38a366ade2"`,
				`"record":{`,
			},
			NotExpectedContent: []string{
				// hidden fields should remain hidden even though we are authenticated as superuser
				`"tokenKey"`,
				`"password"`,
			},
			ExpectedEvents: map[string]int{
				"*":                   0,
				"OnRecordAuthRequest": 1,
				"OnRecordEnrich":      1,
			},
		},
		{
			Name:   "authorized as superuser with custom invalid duration",
			Method: http.MethodPost,
			URL:    "/api/collections/users/impersonate/0196afca-7951-76f3-b344-ae38a366ade2",
			Headers: map[string]string{
				"Authorization": tests.NewAuthTokenForTest("_superusers", "test@example.com"),
			},
			Body:           strings.NewReader(`{"duration":-1}`),
			ExpectedStatus: 400,
			ExpectedContent: []string{
				`"data":{`,
				`"duration":{`,
			},
			ExpectedEvents: map[string]int{"*": 0},
		},
		{
			Name:   "authorized as superuser with custom valid duration",
			Method: http.MethodPost,
			URL:    "/api/collections/users/impersonate/0196afca-7951-76f3-b344-ae38a366ade2",
			Headers: map[string]string{
				"Authorization": tests.NewAuthTokenForTest("_superusers", "test@example.com"),
			},
			Body:           strings.NewReader(`{"duration":100}`),
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"token":"`,
				`"id":"0196afca-7951-76f3-b344-ae38a366ade2"`,
				`"record":{`,
			},
			ExpectedEvents: map[string]int{
				"*":                   0,
				"OnRecordAuthRequest": 1,
				"OnRecordEnrich":      1,
			},
		},
	}

	for _, scenario := range scenarios {
		scenario.Test(t)
	}
}
