package core_test

import (
	"fmt"
	"slices"
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

func TestFindAllAuthOriginsByRecord(t *testing.T) {
	t.Parallel()

	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	demo1, err := app.FindRecordById("demo1", "0196afca-7951-7ba1-8cef-b59777e4d838")
	if err != nil {
		t.Fatal(err)
	}

	superuser2, err := app.FindAuthRecordByEmail(core.CollectionNameSuperusers, "test2@example.com")
	if err != nil {
		t.Fatal(err)
	}

	superuser4, err := app.FindAuthRecordByEmail(core.CollectionNameSuperusers, "test4@example.com")
	if err != nil {
		t.Fatal(err)
	}

	client1, err := app.FindAuthRecordByEmail("clients", "test@example.com")
	if err != nil {
		t.Fatal(err)
	}

	scenarios := []struct {
		record   *core.Record
		expected []string
	}{
		{demo1, nil},
		{superuser2, []string{"0196afca-7950-7596-b740-a6167a4ab938", "0196afca-7950-737c-9b22-ef2eb4690b37", "0196afca-7950-70c4-b130-6660e4c8d20d"}},
		{superuser4, nil},
		{client1, []string{"0196afca-7950-7e99-906f-93f836ec07bf"}},
	}

	for _, s := range scenarios {
		t.Run(s.record.Collection().Name+"_"+s.record.Id, func(t *testing.T) {
			result, err := app.FindAllAuthOriginsByRecord(s.record)
			if err != nil {
				t.Fatal(err)
			}

			if len(result) != len(s.expected) {
				t.Fatalf("Expected total origins %d, got %d", len(s.expected), len(result))
			}

			for i, id := range s.expected {
				if result[i].Id != id {
					t.Errorf("[%d] Expected id %q, got %q", i, id, result[i].Id)
				}
			}
		})
	}
}

func TestFindAllAuthOriginsByCollection(t *testing.T) {
	t.Parallel()

	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	demo1, err := app.FindCollectionByNameOrId("demo1")
	if err != nil {
		t.Fatal(err)
	}

	superusers, err := app.FindCollectionByNameOrId(core.CollectionNameSuperusers)
	if err != nil {
		t.Fatal(err)
	}

	clients, err := app.FindCollectionByNameOrId("clients")
	if err != nil {
		t.Fatal(err)
	}

	scenarios := []struct {
		collection *core.Collection
		expected   []string
	}{
		{demo1, nil},
		{superusers, []string{"0196afca-7950-7596-b740-a6167a4ab938", "0196afca-7950-737c-9b22-ef2eb4690b37", "0196afca-7950-70c4-b130-6660e4c8d20d", "0196afca-7950-7437-9124-6025212f636e"}},
		{clients, []string{"0196afca-7950-7e99-906f-93f836ec07bf"}},
	}

	for _, s := range scenarios {
		t.Run(s.collection.Name, func(t *testing.T) {
			result, err := app.FindAllAuthOriginsByCollection(s.collection)
			if err != nil {
				t.Fatal(err)
			}

			if len(result) != len(s.expected) {
				t.Fatalf("Expected total origins %d, got %d", len(s.expected), len(result))
			}

			for i, id := range s.expected {
				if result[i].Id != id {
					t.Errorf("[%d] Expected id %q, got %q", i, id, result[i].Id)
				}
			}
		})
	}
}

func TestFindAuthOriginById(t *testing.T) {
	t.Parallel()

	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	scenarios := []struct {
		id          string
		expectError bool
	}{
		{"", true},
		{"0196afca-7951-7ba1-8cef-b59777e4d838", true}, // non-origin id
		{"0196afca-7950-7e99-906f-93f836ec07bf", false},
	}

	for _, s := range scenarios {
		t.Run(s.id, func(t *testing.T) {
			result, err := app.FindAuthOriginById(s.id)

			hasErr := err != nil
			if hasErr != s.expectError {
				t.Fatalf("Expected hasErr %v, got %v (%v)", s.expectError, hasErr, err)
			}

			if hasErr {
				return
			}

			if result.Id != s.id {
				t.Fatalf("Expected record with id %q, got %q", s.id, result.Id)
			}
		})
	}
}

func TestFindAuthOriginByRecordAndFingerprint(t *testing.T) {
	t.Parallel()

	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	demo1, err := app.FindRecordById("demo1", "0196afca-7951-7ba1-8cef-b59777e4d838")
	if err != nil {
		t.Fatal(err)
	}

	superuser2, err := app.FindAuthRecordByEmail(core.CollectionNameSuperusers, "test2@example.com")
	if err != nil {
		t.Fatal(err)
	}

	scenarios := []struct {
		record      *core.Record
		fingerprint string
		expectError bool
	}{
		{demo1, "6afbfe481c31c08c55a746cccb88ece0", true},
		{superuser2, "", true},
		{superuser2, "abc", true},
		{superuser2, "22bbbcbed36e25321f384ccf99f60057", false}, // fingerprint from different origin
		{superuser2, "6afbfe481c31c08c55a746cccb88ece0", false},
	}

	for i, s := range scenarios {
		t.Run(fmt.Sprintf("%d_%s_%s", i, s.record.Id, s.fingerprint), func(t *testing.T) {
			result, err := app.FindAuthOriginByRecordAndFingerprint(s.record, s.fingerprint)

			hasErr := err != nil
			if hasErr != s.expectError {
				t.Fatalf("Expected hasErr %v, got %v (%v)", s.expectError, hasErr, err)
			}

			if hasErr {
				return
			}

			if result.Fingerprint() != s.fingerprint {
				t.Fatalf("Expected origin with fingerprint %q, got %q", s.fingerprint, result.Fingerprint())
			}

			if result.RecordRef() != s.record.Id || result.CollectionRef() != s.record.Collection().Id {
				t.Fatalf("Expected record %q (%q), got %q (%q)", s.record.Id, s.record.Collection().Id, result.RecordRef(), result.CollectionRef())
			}
		})
	}
}

func TestDeleteAllAuthOriginsByRecord(t *testing.T) {
	t.Parallel()

	testApp, _ := tests.NewTestApp()
	defer testApp.Cleanup()

	demo1, err := testApp.FindRecordById("demo1", "0196afca-7951-7ba1-8cef-b59777e4d838")
	if err != nil {
		t.Fatal(err)
	}

	superuser2, err := testApp.FindAuthRecordByEmail(core.CollectionNameSuperusers, "test2@example.com")
	if err != nil {
		t.Fatal(err)
	}

	superuser4, err := testApp.FindAuthRecordByEmail(core.CollectionNameSuperusers, "test4@example.com")
	if err != nil {
		t.Fatal(err)
	}

	client1, err := testApp.FindAuthRecordByEmail("clients", "test@example.com")
	if err != nil {
		t.Fatal(err)
	}

	scenarios := []struct {
		record     *core.Record
		deletedIds []string
	}{
		{demo1, nil}, // non-auth record
		{superuser2, []string{"0196afca-7950-7596-b740-a6167a4ab938", "0196afca-7950-737c-9b22-ef2eb4690b37", "0196afca-7950-70c4-b130-6660e4c8d20d"}},
		{superuser4, nil},
		{client1, []string{"0196afca-7950-7e99-906f-93f836ec07bf"}},
	}

	for i, s := range scenarios {
		t.Run(fmt.Sprintf("%d_%s_%s", i, s.record.Collection().Name, s.record.Id), func(t *testing.T) {
			app, _ := tests.NewTestApp()
			defer app.Cleanup()

			deletedIds := []string{}
			app.OnRecordDelete().BindFunc(func(e *core.RecordEvent) error {
				deletedIds = append(deletedIds, e.Record.Id)
				return e.Next()
			})

			err := app.DeleteAllAuthOriginsByRecord(s.record)
			if err != nil {
				t.Fatal(err)
			}

			if len(deletedIds) != len(s.deletedIds) {
				t.Fatalf("Expected deleted ids\n%v\ngot\n%v", s.deletedIds, deletedIds)
			}

			for _, id := range s.deletedIds {
				if !slices.Contains(deletedIds, id) {
					t.Errorf("Expected to find deleted id %q in %v", id, deletedIds)
				}
			}
		})
	}
}
