package apis_test

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

func TestRecordConfirmPasswordReset(t *testing.T) {
	t.Parallel()

	validPasswordResetToken := tests.NewAuthTokenForTest("users", "test@example.com", tests.CustomToken("passwordReset", map[string]any{
		"email": "test@example.com",
	}))
	validPasswordResetBody := strings.NewReader(strings.ReplaceAll(`{
		"token":"[token-placeholder]",
		"password":"1234567!",
		"passwordConfirm":"1234567!"
	}`, "[token-placeholder]", validPasswordResetToken))

	scenarios := []tests.ApiScenario{
		{
			Name:           "empty data",
			Method:         http.MethodPost,
			URL:            "/api/collections/users/confirm-password-reset",
			Body:           strings.NewReader(``),
			ExpectedStatus: 400,
			ExpectedContent: []string{
				`"data":{`,
				`"password":{"code":"validation_required"`,
				`"passwordConfirm":{"code":"validation_required"`,
				`"token":{"code":"validation_required"`,
			},
			ExpectedEvents: map[string]int{"*": 0},
		},
		{
			Name:            "invalid data format",
			Method:          http.MethodPost,
			URL:             "/api/collections/users/confirm-password-reset",
			Body:            strings.NewReader(`{"password`),
			ExpectedStatus:  400,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "expired token and invalid password",
			Method: http.MethodPost,
			URL:    "/api/collections/users/confirm-password-reset",
			Body: strings.NewReader(strings.ReplaceAll(`{
				"token":"[token-placeholder]",
				"password":"1234567",
				"passwordConfirm":"7654321"
			}`, "[token-placeholder]", tests.NewAuthTokenForTest("users", "test@example.com", tests.CustomToken("passwordReset", map[string]any{
				"email": "test@example.com",
			}), tests.TokenExpired(true)))),
			ExpectedStatus: 400,
			ExpectedContent: []string{
				`"data":{`,
				`"token":{"code":"validation_invalid_token"`,
				`"password":{"code":"validation_length_out_of_range"`,
				`"passwordConfirm":{"code":"validation_values_mismatch"`,
			},
			ExpectedEvents: map[string]int{"*": 0},
		},
		{
			Name:   "non-password reset token",
			Method: http.MethodPost,
			URL:    "/api/collections/users/confirm-password-reset",
			Body: strings.NewReader(strings.ReplaceAll(`{
				"token":"[token-placeholder]",
				"password":"1234567!",
				"passwordConfirm":"1234567!"
			}`, "[token-placeholder]", tests.NewAuthTokenForTest("users", "test@example.com", tests.CustomToken("verification", map[string]any{
				"email": "test@example.com",
			})))),
			ExpectedStatus: 400,
			ExpectedContent: []string{
				`"data":{`,
				`"token":{"code":"validation_invalid_token"`,
			},
			ExpectedEvents: map[string]int{"*": 0},
		},
		{
			Name:            "non auth collection",
			Method:          http.MethodPost,
			URL:             "/api/collections/demo1/confirm-password-reset?expand=rel,missing",
			Body:            validPasswordResetBody,
			ExpectedStatus:  404,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:           "different auth collection",
			Method:         http.MethodPost,
			URL:            "/api/collections/clients/confirm-password-reset?expand=rel,missing",
			Body:           validPasswordResetBody,
			ExpectedStatus: 400,
			ExpectedContent: []string{
				`"data":{"token":{"code":"validation_token_collection_mismatch"`,
			},
			ExpectedEvents: map[string]int{"*": 0},
		},
		{
			Name:           "valid token and data (unverified user)",
			Method:         http.MethodPost,
			URL:            "/api/collections/users/confirm-password-reset",
			Body:           validPasswordResetBody,
			ExpectedStatus: 204,
			ExpectedEvents: map[string]int{
				"*":                                   0,
				"OnRecordConfirmPasswordResetRequest": 1,
				"OnModelUpdate":                       1,
				"OnModelUpdateExecute":                1,
				"OnModelAfterUpdateSuccess":           1,
				"OnModelValidate":                     1,
				"OnRecordUpdate":                      1,
				"OnRecordUpdateExecute":               1,
				"OnRecordAfterUpdateSuccess":          1,
				"OnRecordValidate":                    1,
			},
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				user, err := app.FindAuthRecordByEmail("users", "test@example.com")
				if err != nil {
					t.Fatalf("Failed to fetch confirm password user: %v", err)
				}

				if user.Verified() {
					t.Fatal("Expected the user to be unverified")
				}
			},
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				_, err := app.FindAuthRecordByToken(
					tests.NewAuthTokenForTest("users", "test@example.com", tests.CustomToken("passwordReset", map[string]any{
						"email": "test@example.com",
					})),
					core.TokenTypePasswordReset,
				)
				if err == nil {
					t.Fatal("Expected the password reset token to be invalidated")
				}

				user, err := app.FindAuthRecordByEmail("users", "test@example.com")
				if err != nil {
					t.Fatalf("Failed to fetch confirm password user: %v", err)
				}

				if !user.Verified() {
					t.Fatal("Expected the user to be marked as verified")
				}

				if !user.ValidatePassword("1234567!") {
					t.Fatal("Password wasn't changed")
				}
			},
		},
		{
			Name:           "valid token and data (unverified user with different email from the one in the token)",
			Method:         http.MethodPost,
			URL:            "/api/collections/users/confirm-password-reset",
			Body:           validPasswordResetBody,
			ExpectedStatus: 204,
			ExpectedEvents: map[string]int{
				"*":                                   0,
				"OnRecordConfirmPasswordResetRequest": 1,
				"OnModelUpdate":                       1,
				"OnModelUpdateExecute":                1,
				"OnModelAfterUpdateSuccess":           1,
				"OnModelValidate":                     1,
				"OnRecordUpdate":                      1,
				"OnRecordUpdateExecute":               1,
				"OnRecordAfterUpdateSuccess":          1,
				"OnRecordValidate":                    1,
			},
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				user, err := app.FindAuthRecordByEmail("users", "test@example.com")
				if err != nil {
					t.Fatalf("Failed to fetch confirm password user: %v", err)
				}

				if user.Verified() {
					t.Fatal("Expected the user to be unverified")
				}

				oldTokenKey := user.TokenKey()

				// manually change the email to check whether the verified state will be updated
				user.SetEmail("test_update@example.com")
				if err = app.Save(user); err != nil {
					t.Fatalf("Failed to update user test email: %v", err)
				}

				// resave with the old token key since the email change above
				// would change it and will make the password token invalid
				user.SetTokenKey(oldTokenKey)
				if err = app.Save(user); err != nil {
					t.Fatalf("Failed to restore original user tokenKey: %v", err)
				}
			},
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				_, err := app.FindAuthRecordByToken(
					validPasswordResetToken,
					core.TokenTypePasswordReset,
				)
				if err == nil {
					t.Fatalf("Expected the password reset token to be invalidated")
				}

				user, err := app.FindAuthRecordByEmail("users", "test_update@example.com")
				if err != nil {
					t.Fatalf("Failed to fetch confirm password user: %v", err)
				}

				if user.Verified() {
					t.Fatal("Expected the user to remain unverified")
				}

				if !user.ValidatePassword("1234567!") {
					t.Fatal("Password wasn't changed")
				}
			},
		},
		{
			Name:           "valid token and data (verified user)",
			Method:         http.MethodPost,
			URL:            "/api/collections/users/confirm-password-reset",
			Body:           validPasswordResetBody,
			ExpectedStatus: 204,
			ExpectedEvents: map[string]int{
				"*":                                   0,
				"OnRecordConfirmPasswordResetRequest": 1,
				"OnModelUpdate":                       1,
				"OnModelUpdateExecute":                1,
				"OnModelAfterUpdateSuccess":           1,
				"OnModelValidate":                     1,
				"OnRecordUpdate":                      1,
				"OnRecordUpdateExecute":               1,
				"OnRecordAfterUpdateSuccess":          1,
				"OnRecordValidate":                    1,
			},
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				user, err := app.FindAuthRecordByEmail("users", "test@example.com")
				if err != nil {
					t.Fatalf("Failed to fetch confirm password user: %v", err)
				}

				// ensure that the user is already verified
				user.SetVerified(true)
				if err := app.Save(user); err != nil {
					t.Fatalf("Failed to update user verified state")
				}
			},
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				_, err := app.FindAuthRecordByToken(
					validPasswordResetToken,
					core.TokenTypePasswordReset,
				)
				if err == nil {
					t.Fatal("Expected the password reset token to be invalidated")
				}

				user, err := app.FindAuthRecordByEmail("users", "test@example.com")
				if err != nil {
					t.Fatalf("Failed to fetch confirm password user: %v", err)
				}

				if !user.Verified() {
					t.Fatal("Expected the user to remain verified")
				}

				if !user.ValidatePassword("1234567!") {
					t.Fatal("Password wasn't changed")
				}
			},
		},
		{
			Name:   "OnRecordAfterConfirmPasswordResetRequest error response",
			Method: http.MethodPost,
			URL:    "/api/collections/users/confirm-password-reset",
			Body:   validPasswordResetBody,
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				app.OnRecordConfirmPasswordResetRequest().BindFunc(func(e *core.RecordConfirmPasswordResetRequestEvent) error {
					return errors.New("error")
				})
			},
			ExpectedStatus:  400,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents: map[string]int{
				"*":                                   0,
				"OnRecordConfirmPasswordResetRequest": 1,
			},
		},

		// rate limit checks
		// -----------------------------------------------------------
		{
			Name:   "RateLimit rule - users:confirmPasswordReset",
			Method: http.MethodPost,
			URL:    "/api/collections/users/confirm-password-reset",
			Body:   validPasswordResetBody,
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				app.Settings().RateLimits.Enabled = true
				app.Settings().RateLimits.Rules = []core.RateLimitRule{
					{MaxRequests: 100, Label: "abc"},
					{MaxRequests: 100, Label: "*:confirmPasswordReset"},
					{MaxRequests: 0, Label: "users:confirmPasswordReset"},
				}
			},
			ExpectedStatus:  429,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "RateLimit rule - *:confirmPasswordReset",
			Method: http.MethodPost,
			URL:    "/api/collections/users/confirm-password-reset",
			Body:   validPasswordResetBody,
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				app.Settings().RateLimits.Enabled = true
				app.Settings().RateLimits.Rules = []core.RateLimitRule{
					{MaxRequests: 100, Label: "abc"},
					{MaxRequests: 0, Label: "*:confirmPasswordReset"},
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
