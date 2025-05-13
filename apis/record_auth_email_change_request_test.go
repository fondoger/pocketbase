package apis_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

func TestRecordRequestEmailChange(t *testing.T) {
	t.Parallel()

	scenarios := []tests.ApiScenario{
		{
			Name:            "unauthorized",
			Method:          http.MethodPost,
			URL:             "/api/collections/users/request-email-change",
			Body:            strings.NewReader(`{"newEmail":"change@example.com"}`),
			ExpectedStatus:  401,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:            "not an auth collection",
			Method:          http.MethodPost,
			URL:             "/api/collections/demo1/request-email-change",
			Body:            strings.NewReader(`{"newEmail":"change@example.com"}`),
			ExpectedStatus:  401,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "record authentication but from different auth collection",
			Method: http.MethodPost,
			URL:    "/api/collections/clients/request-email-change",
			Body:   strings.NewReader(`{"newEmail":"change@example.com"}`),
			Headers: map[string]string{
				"Authorization": tests.NewAuthTokenForTest("users", "test@example.com"),
			},
			ExpectedStatus:  403,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "superuser authentication",
			Method: http.MethodPost,
			URL:    "/api/collections/users/request-email-change",
			Body:   strings.NewReader(`{"newEmail":"change@example.com"}`),
			Headers: map[string]string{
				"Authorization": tests.NewAuthTokenForTest("_superusers", "test@example.com"),
			},
			ExpectedStatus:  403,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "invalid data",
			Method: http.MethodPost,
			URL:    "/api/collections/users/request-email-change",
			Body:   strings.NewReader(`{"newEmail`),
			Headers: map[string]string{
				"Authorization": tests.NewAuthTokenForTest("users", "test@example.com"),
			},
			ExpectedStatus:  400,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "empty data",
			Method: http.MethodPost,
			URL:    "/api/collections/users/request-email-change",
			Body:   strings.NewReader(`{}`),
			Headers: map[string]string{
				"Authorization": tests.NewAuthTokenForTest("users", "test@example.com"),
			},
			ExpectedStatus: 400,
			ExpectedContent: []string{
				`"data":`,
				`"newEmail":{"code":"validation_required"`,
			},
			ExpectedEvents: map[string]int{"*": 0},
		},
		{
			Name:   "valid data (existing email)",
			Method: http.MethodPost,
			URL:    "/api/collections/users/request-email-change",
			Body:   strings.NewReader(`{"newEmail":"test2@example.com"}`),
			Headers: map[string]string{
				"Authorization": tests.NewAuthTokenForTest("users", "test@example.com"),
			},
			ExpectedStatus: 400,
			ExpectedContent: []string{
				`"data":`,
				`"newEmail":{"code":"validation_invalid_new_email"`,
			},
			ExpectedEvents: map[string]int{"*": 0},
		},
		{
			Name:   "valid data (new email)",
			Method: http.MethodPost,
			URL:    "/api/collections/users/request-email-change",
			Body:   strings.NewReader(`{"newEmail":"change@example.com"}`),
			Headers: map[string]string{
				"Authorization": tests.NewAuthTokenForTest("users", "test@example.com"),
			},
			ExpectedStatus: 204,
			ExpectedEvents: map[string]int{
				"*":                                 0,
				"OnRecordRequestEmailChangeRequest": 1,
				"OnMailerSend":                      1,
				"OnMailerRecordEmailChangeSend":     1,
			},
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				if !strings.Contains(app.TestMailer.LastMessage().HTML, "/auth/confirm-email-change") {
					t.Fatalf("Expected email change email, got\n%v", app.TestMailer.LastMessage().HTML)
				}
			},
		},

		// rate limit checks
		// -----------------------------------------------------------
		{
			Name:   "RateLimit rule - users:requestEmailChange",
			Method: http.MethodPost,
			URL:    "/api/collections/users/request-email-change",
			Body:   strings.NewReader(`{"newEmail":"change@example.com"}`),
			Headers: map[string]string{
				"Authorization": tests.NewAuthTokenForTest("users", "test@example.com"),
			},
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				app.Settings().RateLimits.Enabled = true
				app.Settings().RateLimits.Rules = []core.RateLimitRule{
					{MaxRequests: 100, Label: "abc"},
					{MaxRequests: 100, Label: "*:requestEmailChange"},
					{MaxRequests: 0, Label: "users:requestEmailChange"},
				}
			},
			ExpectedStatus:  429,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "RateLimit rule - *:requestEmailChange",
			Method: http.MethodPost,
			URL:    "/api/collections/users/request-email-change",
			Body:   strings.NewReader(`{"newEmail":"change@example.com"}`),
			Headers: map[string]string{
				"Authorization": tests.NewAuthTokenForTest("users", "test@example.com"),
			},
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				app.Settings().RateLimits.Enabled = true
				app.Settings().RateLimits.Rules = []core.RateLimitRule{
					{MaxRequests: 100, Label: "abc"},
					{MaxRequests: 0, Label: "*:requestEmailChange"},
				}
			},
			ExpectedStatus:  429,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
	}

	for _, scenario := range scenarios {
		scenario.Test(t)
	}
}
