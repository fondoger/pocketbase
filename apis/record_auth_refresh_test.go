package apis_test

import (
	"errors"
	"net/http"
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

func TestRecordAuthRefresh(t *testing.T) {
	t.Parallel()

	scenarios := []tests.ApiScenario{
		{
			Name:            "unauthorized",
			Method:          http.MethodPost,
			URL:             "/api/collections/users/auth-refresh",
			ExpectedStatus:  401,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "superuser trying to refresh the auth of another auth collection",
			Method: http.MethodPost,
			URL:    "/api/collections/users/auth-refresh",
			Headers: map[string]string{
				"Authorization": tests.NewAuthTokenForTest("_superusers", "test@example.com"),
			},
			ExpectedStatus:  403,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "auth record + not an auth collection",
			Method: http.MethodPost,
			URL:    "/api/collections/demo1/auth-refresh",
			Headers: map[string]string{
				"Authorization": tests.NewAuthTokenForTest("users", "test@example.com"),
			},
			ExpectedStatus:  403,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "auth record + different auth collection",
			Method: http.MethodPost,
			URL:    "/api/collections/clients/auth-refresh?expand=rel,missing",
			Headers: map[string]string{
				"Authorization": tests.NewAuthTokenForTest("users", "test@example.com"),
			},
			ExpectedStatus:  403,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "auth record + same auth collection as the token",
			Method: http.MethodPost,
			URL:    "/api/collections/users/auth-refresh?expand=rel,missing",
			Headers: map[string]string{
				"Authorization": tests.NewAuthTokenForTest("users", "test@example.com"),
			},
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"token":`,
				`"record":`,
				`"id":"0196afca-7951-76f3-b344-ae38a366ade2"`,
				`"emailVisibility":false`,
				`"email":"test@example.com"`, // the owner can always view their email address
				`"expand":`,
				`"rel":`,
				`"id":"0196afca-7951-70d0-bcc5-206ed6a14bea"`,
			},
			NotExpectedContent: []string{
				`"missing":`,
			},
			ExpectedEvents: map[string]int{
				"*":                          0,
				"OnRecordAuthRefreshRequest": 1,
				"OnRecordAuthRequest":        1,
				"OnRecordEnrich":             2,
			},
		},
		{
			Name:   "auth record + same auth collection as the token but static/unrefreshable",
			Method: http.MethodPost,
			URL:    "/api/collections/users/auth-refresh",
			Headers: map[string]string{
				"Authorization": tests.NewAuthTokenForTest("users", "test@example.com", tests.TokenRefreshable(false)),
			},
			ExpectedStatus:  403,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "unverified auth record in onlyVerified collection",
			Method: http.MethodPost,
			URL:    "/api/collections/clients/auth-refresh",
			Headers: map[string]string{
				"Authorization": tests.NewAuthTokenForTest("clients", "test2@example.com"),
			},
			ExpectedStatus:  403,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents: map[string]int{
				"*":                          0,
				"OnRecordAuthRefreshRequest": 1,
			},
		},
		{
			Name:   "verified auth record in onlyVerified collection",
			Method: http.MethodPost,
			URL:    "/api/collections/clients/auth-refresh",
			Headers: map[string]string{
				"Authorization": tests.NewAuthTokenForTest("clients", "test@example.com"),
			},
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"token":`,
				`"record":`,
				`"id":"0196afca-7951-7ab7-afc2-cd8438fef6fa"`,
				`"verified":true`,
				`"email":"test@example.com"`,
			},
			ExpectedEvents: map[string]int{
				"*":                          0,
				"OnRecordAuthRefreshRequest": 1,
				"OnRecordAuthRequest":        1,
				"OnRecordEnrich":             1,
			},
		},
		{
			Name:   "OnRecordAfterAuthRefreshRequest error response",
			Method: http.MethodPost,
			URL:    "/api/collections/users/auth-refresh?expand=rel,missing",
			Headers: map[string]string{
				"Authorization": tests.NewAuthTokenForTest("users", "test@example.com"),
			},
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				app.OnRecordAuthRefreshRequest().BindFunc(func(e *core.RecordAuthRefreshRequestEvent) error {
					return errors.New("error")
				})
			},
			ExpectedStatus:  400,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents: map[string]int{
				"*":                          0,
				"OnRecordAuthRefreshRequest": 1,
			},
		},

		// rate limit checks
		// -----------------------------------------------------------
		{
			Name:   "RateLimit rule - users:authRefresh",
			Method: http.MethodPost,
			URL:    "/api/collections/users/auth-refresh",
			Headers: map[string]string{
				"Authorization": tests.NewAuthTokenForTest("users", "test@example.com"),
			},
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				app.Settings().RateLimits.Enabled = true
				app.Settings().RateLimits.Rules = []core.RateLimitRule{
					{MaxRequests: 100, Label: "abc"},
					{MaxRequests: 100, Label: "*:authRefresh"},
					{MaxRequests: 0, Label: "users:authRefresh"},
				}
			},
			ExpectedStatus:  429,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "RateLimit rule - *:authRefresh",
			Method: http.MethodPost,
			URL:    "/api/collections/users/auth-refresh",
			Headers: map[string]string{
				"Authorization": tests.NewAuthTokenForTest("users", "test@example.com"),
			},
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				app.Settings().RateLimits.Enabled = true
				app.Settings().RateLimits.Rules = []core.RateLimitRule{
					{MaxRequests: 100, Label: "abc"},
					{MaxRequests: 0, Label: "*:authRefresh"},
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
