package validators_test

import (
	"fmt"
	"testing"

	"github.com/pocketbase/pocketbase/core/validators"
)

func TestIsRegex(t *testing.T) {
	t.Parallel()

	scenarios := []struct {
		val         string
		expectError bool
	}{
		{"", false},
		{`abc`, false},
		{`\w+`, false},
		{`\w*((abc+`, true},
	}

	for i, s := range scenarios {
		t.Run(fmt.Sprintf("%d_%#v", i, s.val), func(t *testing.T) {
			err := validators.IsRegex(s.val)

			hasErr := err != nil
			if hasErr != s.expectError {
				t.Fatalf("Expected hasErr to be %v, got %v (%v)", s.expectError, hasErr, err)
			}
		})
	}
}

func TestIPOrSubnet(t *testing.T) {
	t.Parallel()

	scenarios := []struct {
		val         string
		expectError bool
	}{
		{"", false},
		{`invalid`, true},
		{`127.0`, true}, // incomplete
		{`127.0.0.1`, false},
		{`::1`, false},
		{`0000:0000:0000:0000:0000:0000:0000:0001`, false},
		{`127.0.0.1/24`, false},
		{`::/128`, false},
	}

	for i, s := range scenarios {
		t.Run(fmt.Sprintf("%d_%#v", i, s.val), func(t *testing.T) {
			err := validators.IPOrSubnet(s.val)

			hasErr := err != nil
			if hasErr != s.expectError {
				t.Fatalf("Expected hasErr to be %v, got %v (%v)", s.expectError, hasErr, err)
			}
		})
	}
}
