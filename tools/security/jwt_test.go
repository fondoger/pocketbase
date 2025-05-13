package security_test

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/pocketbase/pocketbase/tools/security"
)

func TestParseUnverifiedJWT(t *testing.T) {
	// invalid formatted JWT
	result1, err1 := security.ParseUnverifiedJWT("eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJuYW1lIjoidGVzdCJ9")
	if err1 == nil {
		t.Error("Expected error got nil")
	}
	if len(result1) > 0 {
		t.Error("Expected no parsed claims, got", result1)
	}

	// properly formatted JWT with INVALID claims
	// {"name": "test", "exp":1516239022}
	result2, err2 := security.ParseUnverifiedJWT("eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJuYW1lIjoidGVzdCIsImV4cCI6MTUxNjIzOTAyMn0.xYHirwESfSEW3Cq2BL47CEASvD_p_ps3QCA54XtNktU")
	if err2 == nil {
		t.Error("Expected error got nil")
	}
	if len(result2) != 2 || result2["name"] != "test" {
		t.Errorf("Expected to have 2 claims, got %v", result2)
	}

	// properly formatted JWT with VALID claims (missing exp)
	// {"name": "test"}
	result3, err3 := security.ParseUnverifiedJWT("eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJuYW1lIjoidGVzdCJ9.ml0QsTms3K9wMygTu41ZhKlTyjmW9zHQtoS8FUsCCjU")
	if err3 != nil {
		t.Error("Expected nil, got", err3)
	}
	if len(result3) != 1 || result3["name"] != "test" {
		t.Errorf("Expected to have 1 claim, got %v", result3)
	}

	// properly formatted JWT with VALID claims (valid exp)
	// {"name": "test", "exp": 2208985261}
	result4, err4 := security.ParseUnverifiedJWT("eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJuYW1lIjoidGVzdCIsImV4cCI6MjIwODk4NTI2MX0._0KQu60hYNx5wkBIpEaoX35shXRicb0X_0VdWKWb-3k")
	if err4 != nil {
		t.Error("Expected nil, got", err4)
	}
	if len(result4) != 2 || result4["name"] != "test" {
		t.Errorf("Expected to have 2 claims, got %v", result4)
	}
}

func TestParseJWT(t *testing.T) {
	scenarios := []struct {
		token        string
		secret       string
		expectError  bool
		expectClaims jwt.MapClaims
	}{
		// invalid formatted JWT
		{
			"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJuYW1lIjoidGVzdCJ9",
			"test",
			true,
			nil,
		},
		// properly formatted JWT with INVALID claims and INVALID secret
		// {"name": "test", "exp": 1516239022}
		{
			"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJuYW1lIjoidGVzdCIsImV4cCI6MTUxNjIzOTAyMn0.xYHirwESfSEW3Cq2BL47CEASvD_p_ps3QCA54XtNktU",
			"invalid",
			true,
			nil,
		},
		// properly formatted JWT with INVALID claims and VALID secret
		// {"name": "test", "exp": 1516239022}
		{
			"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJuYW1lIjoidGVzdCIsImV4cCI6MTUxNjIzOTAyMn0.xYHirwESfSEW3Cq2BL47CEASvD_p_ps3QCA54XtNktU",
			"test",
			true,
			nil,
		},
		// properly formatted JWT with VALID claims and INVALID secret
		// {"name": "test", "exp": 1898636137}
		{
			"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJuYW1lIjoidGVzdCIsImV4cCI6MTg5ODYzNjEzN30.gqRkHjpK5s1PxxBn9qPaWEWxTbpc1PPSD-an83TsXRY",
			"invalid",
			true,
			nil,
		},
		// properly formatted EXPIRED JWT with VALID secret
		// {"name": "test", "exp": 1652097610}
		{
			"eyJhbGciOiJIUzI1NiJ9.eyJuYW1lIjoidGVzdCIsImV4cCI6OTU3ODczMzc0fQ.0oUUKUnsQHs4nZO1pnxQHahKtcHspHu4_AplN2sGC4A",
			"test",
			true,
			nil,
		},
		// properly formatted JWT with VALID claims and VALID secret
		// {"name": "test", "exp": 1898636137}
		{
			"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJuYW1lIjoidGVzdCIsImV4cCI6MTg5ODYzNjEzN30.gqRkHjpK5s1PxxBn9qPaWEWxTbpc1PPSD-an83TsXRY",
			"test",
			false,
			jwt.MapClaims{"name": "test", "exp": 1898636137.0},
		},
		// properly formatted JWT with VALID claims (without exp) and VALID secret
		// {"name": "test"}
		{
			"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJuYW1lIjoidGVzdCJ9.ml0QsTms3K9wMygTu41ZhKlTyjmW9zHQtoS8FUsCCjU",
			"test",
			false,
			jwt.MapClaims{"name": "test"},
		},
	}

	for i, s := range scenarios {
		t.Run(fmt.Sprintf("%d_%s", i, s.token), func(t *testing.T) {
			result, err := security.ParseJWT(s.token, s.secret)

			hasErr := err != nil

			if hasErr != s.expectError {
				t.Fatalf("Expected hasErr %v, got %v (%v)", s.expectError, hasErr, err)
			}

			if len(result) != len(s.expectClaims) {
				t.Fatalf("Expected %v claims got %v", s.expectClaims, result)
			}

			for k, v := range s.expectClaims {
				v2, ok := result[k]
				if !ok {
					t.Fatalf("Missing expected claim %q", k)
				}
				if v != v2 {
					t.Fatalf("Expected %v for %q claim, got %v", v, k, v2)
				}
			}
		})
	}
}

func TestNewJWT(t *testing.T) {
	scenarios := []struct {
		claims      jwt.MapClaims
		key         string
		duration    time.Duration
		expectError bool
	}{
		// empty, zero duration
		{jwt.MapClaims{}, "", 0, true},
		// empty, 10 seconds duration
		{jwt.MapClaims{}, "", 10 * time.Second, false},
		// non-empty, 10 seconds duration
		{jwt.MapClaims{"name": "test"}, "test", 10 * time.Second, false},
	}

	for i, scenario := range scenarios {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			token, tokenErr := security.NewJWT(scenario.claims, scenario.key, scenario.duration)
			if tokenErr != nil {
				t.Fatalf("Expected NewJWT to succeed, got error %v", tokenErr)
			}

			claims, parseErr := security.ParseJWT(token, scenario.key)

			hasParseErr := parseErr != nil
			if hasParseErr != scenario.expectError {
				t.Fatalf("Expected hasParseErr to be %v, got %v (%v)", scenario.expectError, hasParseErr, parseErr)
			}

			if scenario.expectError {
				return
			}

			if _, ok := claims["exp"]; !ok {
				t.Fatalf("Missing required claim exp, got %v", claims)
			}

			// clear exp claim to match with the scenario ones
			delete(claims, "exp")

			if len(claims) != len(scenario.claims) {
				t.Fatalf("Expected %v claims, got %v", scenario.claims, claims)
			}

			for j, k := range claims {
				if claims[j] != scenario.claims[j] {
					t.Fatalf("Expected %v for %q claim, got %v", claims[j], k, scenario.claims[j])
				}
			}
		})
	}
}

func TestCreateJWTForUnitTest(t *testing.T) {
	// users/test@example.com
	userClaims := jwt.MapClaims{
		"id":           "0196afca-7951-76f3-b344-ae38a366ade2",
		"type":         "auth",
		"collectionId": "11111111-1111-1111-1111-111111111111",
		"refreshable":  true,
	}
	usersRootToken := "PjVU4hAV7CZIWbCByJHkDcMUlSEWCLI6M5aWSZOpEq0a3rYxKT"
	usersUserToken := "tfYe7rCTX4D2KuWQY3pJjBifgsrMbecyXBatEPjrSfGEGS2jh6"
	token, _ := security.NewJWT(userClaims, usersUserToken+usersRootToken, time.Hour*10000)
	fmt.Println("Normal user JWT:", token)
	// eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJjb2xsZWN0aW9uSWQiOiIxMTExMTExMS0xMTExLTExMTEtMTExMS0xMTExMTExMTExMTEiLCJleHAiOjE3ODI4Njc4NTYsImlkIjoiMDE5NmFmY2EtNzk1MS03NmYzLWIzNDQtYWUzOGEzNjZhZGUyIiwicmVmcmVzaGFibGUiOnRydWUsInR5cGUiOiJhdXRoIn0.y8yOhOhB68CdO5oo54qL3sIWaJh-5-elr0QnSfbuim8

	// users/test2@example.com
	userClaims["id"] = "0196afca-7951-77d1-ba15-923db9b774b2"
	token, _ = security.NewJWT(userClaims, usersUserToken+usersRootToken, time.Hour*10000)
	fmt.Println("Normal user JWT:", token)
	// eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJjb2xsZWN0aW9uSWQiOiIxMTExMTExMS0xMTExLTExMTEtMTExMS0xMTExMTExMTExMTEiLCJleHAiOjE3ODMwNTcwNjMsImlkIjoiMDE5NmFmY2EtNzk1MS03N2QxLWJhMTUtOTIzZGI5Yjc3NGIyIiwicmVmcmVzaGFibGUiOnRydWUsInR5cGUiOiJhdXRoIn0.z-qWhoRfps_3JlXYvLeij9G_EFRWGEEnons3cF-2h48

	// _superusers/test@example.com
	superUserClaims := jwt.MapClaims{
		"id":           "0196afca-7951-7dc4-a3a4-35b24b1bdccd",
		"type":         "auth",
		"collectionId": "0196afca-09e0-7d9a-82c8-1e040135f09f",
		"refreshable":  true,
	}
	superUserRootToken := "MyN3nDlzmHnuCjd35vb6cyIdqNr7Os0PmgiPVDMxmbFToSpBvS"
	superUserUserToken := "O4rvW9FSUyTA3xUuQmXR3wHF2db9bHs19nBHeSgVTxerOsTAl4"
	superUserToken, _ := security.NewJWT(superUserClaims, superUserUserToken+superUserRootToken, time.Hour*10000)
	fmt.Println("Generated super user JWT:", superUserToken)
	// eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJjb2xsZWN0aW9uSWQiOiIwMTk2YWZjYS0wOWUwLTdkOWEtODJjOC0xZTA0MDEzNWYwOWYiLCJleHAiOjE3ODI4Njc4NTYsImlkIjoiMDE5NmFmY2EtNzk1MS03ZGM0LWEzYTQtMzViMjRiMWJkY2NkIiwicmVmcmVzaGFibGUiOnRydWUsInR5cGUiOiJhdXRoIn0.AS_Er29Xmyo1CtCf8W40b3zwSLsL5icCxdEZjcLLgFk

	// clients/test@example.com
	clientsClaims := jwt.MapClaims{
		"id":           "0196afca-7951-7ab7-afc2-cd8438fef6fa",
		"type":         "auth",
		"collectionId": "0196afca-09e0-717d-9a85-9c276d28c33c",
		"refreshable":  true,
	}
	clientsRootToken := "PjVU4hAV7CZIWbCByJHkDcMUlSEWCLI6M5aWSZOpEq0a3rYxKT"
	clientsUserToken := "rMb1gUpn27s53t66gOGscSHfYsa272cgOgn4nhTZIl4fIC8XP8"
	clientsToken, _ := security.NewJWT(clientsClaims, clientsUserToken+clientsRootToken, time.Hour*10000)
	fmt.Println("Generated clients JWT:", clientsToken)
	// eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJjb2xsZWN0aW9uSWQiOiIwMTk2YWZjYS0wOWUwLTcxN2QtOWE4NS05YzI3NmQyOGMzM2MiLCJleHAiOjE3ODI4NjgzNjIsImlkIjoiMDE5NmFmY2EtNzk1MS03YWI3LWFmYzItY2Q4NDM4ZmVmNmZhIiwicmVmcmVzaGFibGUiOnRydWUsInR5cGUiOiJhdXRoIn0.Fd8qpj1yIZN3pQhQFWm5tqc5abCXDnyhkgxC-ifg1qk
}
