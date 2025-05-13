package tests

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/tools/security"
)

var db *dbx.DB
var once sync.Once

/*
cd tests/data
PGPASSWORD=pass createdb -h 127.0.0.1 -U user pb-test-tokens
PGPASSWORD=pass psql -h 127.0.0.1 -U user -d pb-test-tokens < data.pg-dump.sql
*/

func initOnce() {
	once.Do(func() {
		var err error
		db, err = dbx.MustOpen("pgx", "postgres://user:pass@127.0.0.1:5432/pb-test-tokens?sslmode=disable")
		if err != nil {
			panic(err)
		}
	})
}

func NewAuthTokenForTest(collectionName string, userEmail string, opts ...*options) string {
	initOnce()

	tokenType := "auth"
	for _, opt := range opts {
		if opt.TokenType != nil {
			tokenType = *opt.TokenType
		}
	}

	var row struct {
		CollectionId    string `db:"collection_id"`
		CollectionToken string `db:"collection_token"`
		UserId          string `db:"user_id"`
		UserToken       string `db:"user_token"`
	}
	sql := `
		select "_collections".id as collection_id, options#>>'{authToken,secret}' as collection_token, "users".id as user_id, "users"."tokenKey" as user_token
		from _collections
		cross join "users"
		where _collections.name = 'users'
			and "users".email = 'test@example.com';
	`
	sql = strings.ReplaceAll(sql, "authToken", tokenType+"Token")
	sql = strings.ReplaceAll(sql, "users", collectionName)
	sql = strings.ReplaceAll(sql, "test@example.com", userEmail)
	fmt.Println(sql)
	err := db.NewQuery(sql).One(&row)
	if err != nil || row.CollectionToken == "" || row.UserToken == "" {
		panic(fmt.Sprintf("Failed to get auth token: %v", err))
	}

	userClaims := jwt.MapClaims{
		"id":           row.UserId,
		"type":         tokenType,
		"collectionId": row.CollectionId,
		"refreshable":  true,
	}
	duration := time.Hour * 10000 * 10
	invalidSuffix := ""
	for _, opt := range opts {
		if opt.Refreshable != nil {
			userClaims["refreshable"] = *opt.Refreshable
		}
		if opt.Expired != nil {
			if *opt.Expired {
				duration = -1 * time.Second
			} else {
				duration = time.Hour * 10000 * 10
			}
		}
		if opt.Invalid != nil {
			if *opt.Invalid {
				invalidSuffix = ":invalid"
			} else {
				invalidSuffix = ""
			}
		}
		if opt.AdditionalClaims != nil {
			delete(userClaims, "refreshable") // refreshable is for auth token only. Remove it if we use different types.
			for k, v := range *opt.AdditionalClaims {
				userClaims[k] = v
			}
		}
	}
	token, err := security.NewJWT(userClaims, row.UserToken+row.CollectionToken+invalidSuffix, duration)
	if err != nil {
		panic(err)
	}
	return token
}

type options struct {
	Refreshable      *bool
	Expired          *bool
	Invalid          *bool
	TokenType        *string
	AdditionalClaims *map[string]any
}

func TokenRefreshable(refreshable bool) *options {
	return &options{
		Refreshable: &refreshable,
	}
}

func TokenExpired(expired bool) *options {
	return &options{
		Expired: &expired,
	}
}

func TokenInvalid(invalid bool) *options {
	return &options{
		Invalid: &invalid,
	}
}

func CustomToken(tokenType string, claims map[string]any) *options {
	return &options{
		TokenType:        &tokenType,
		AdditionalClaims: &claims,
	}
}
