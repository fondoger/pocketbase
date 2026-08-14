package jsvm

import (
	"net/http/httptest"
	"testing"

	"github.com/dop251/goja"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

func TestHooksAppReset(t *testing.T) {
	t.Parallel()

	testApp, _ := tests.NewTestApp()
	defer testApp.Cleanup()

	createVM := func() *goja.Runtime {
		vm := goja.New()
		vm.SetFieldNameMapper(FieldMapper{})
		vm.Set("$app", testApp)
		return vm
	}

	loader := createVM()

	pool := newPool(1, createVM)

	hooksBinds(testApp, loader, pool)

	// register any hook
	_, err := loader.RunScript("stub", `
		onRecordCreate((e) => {
			e.next()

			$app = 123; // overwrite
		})
	`)
	if err != nil {
		t.Fatal(err)
	}

	// insert a dummy record to trigger the JS hook
	{
		collection, err := testApp.FindCollectionByNameOrId("demo2")
		if err != nil {
			t.Fatal(err)
		}

		record := core.NewRecord(collection)
		record.Set("title", "test")
		if err := testApp.Save(record); err != nil {
			t.Fatal(err)
		}
	}

	// check the executor state
	pool.run(func(vm *goja.Runtime) error {
		val, err := vm.RunScript("verify", `$app`)
		if err != nil {
			t.Fatal(err)
		}

		if valApp := val.Export(); valApp != testApp {
			t.Fatalf("Expected $app to reset to its original value, got %v", valApp)
		}

		return nil
	})
}

func TestRouterHandlerAppReset(t *testing.T) {
	t.Parallel()

	testApp, _ := tests.NewTestApp()
	defer testApp.Cleanup()

	createVM := func() *goja.Runtime {
		vm := goja.New()
		vm.SetFieldNameMapper(FieldMapper{})
		vm.Set("$app", testApp)
		return vm
	}

	loader := createVM()

	pool := newPool(1, createVM)

	routerBinds(testApp, loader, pool)

	// register route handler hook
	_, err := loader.RunScript("stub", `
		routerAdd("GET", "/test", (e) => {
			$app = 123; // overwrite

			return e.string(200, "test")
		})
	`)
	if err != nil {
		t.Fatal(err)
	}

	// create mock web server
	{
		baseRouter, err := apis.NewRouter(testApp)
		if err != nil {
			t.Fatal(err)
		}

		// manually trigger the serve event to ensure that custom app routes and middlewares are registered
		serveEvent := new(core.ServeEvent)
		serveEvent.App = testApp
		serveEvent.Router = baseRouter
		_ = testApp.OnServe().Trigger(serveEvent, func(e *core.ServeEvent) error {
			req := httptest.NewRequest("GET", "/test", nil)

			recorder := httptest.NewRecorder()

			mux, err := e.Router.BuildMux()
			if err != nil {
				t.Fatalf("Failed to build router mux: %v", err)
			}
			mux.ServeHTTP(recorder, req)

			if recorder.Code != 200 {
				t.Fatalf("Expected status code %d, got %d", 200, recorder.Code)
			}

			body := recorder.Body.String()
			if body != "test" {
				t.Fatalf("Expected body %q, got %q", "test", body)
			}

			return nil
		})
	}

	pool.run(func(vm *goja.Runtime) error {
		val, err := vm.RunScript("verify", `$app`)
		if err != nil {
			t.Fatal(err)
		}

		if valApp := val.Export(); valApp != testApp {
			t.Fatalf("Expected $app to reset to its original value, got %v", valApp)
		}

		return nil
	})
}

func TestRouterMiddlewareFuncAppReset(t *testing.T) {
	t.Parallel()

	testApp, _ := tests.NewTestApp()
	defer testApp.Cleanup()

	createVM := func() *goja.Runtime {
		vm := goja.New()
		vm.SetFieldNameMapper(FieldMapper{})
		vm.Set("$app", testApp)
		return vm
	}

	loader := createVM()

	pool := newPool(1, createVM)

	routerBinds(testApp, loader, pool)

	// register route middleware func
	_, err := loader.RunScript("stub", `
		routerUse((e) => {
			e.string(200, "test")

			$app = 123; // overwrite
		})
	`)
	if err != nil {
		t.Fatal(err)
	}

	// create mock web server
	{
		baseRouter, err := apis.NewRouter(testApp)
		if err != nil {
			t.Fatal(err)
		}

		// manually trigger the serve event to ensure that custom app routes and middlewares are registered
		serveEvent := new(core.ServeEvent)
		serveEvent.App = testApp
		serveEvent.Router = baseRouter
		_ = testApp.OnServe().Trigger(serveEvent, func(e *core.ServeEvent) error {
			// it doesn't matter as long as the middleware is called
			req := httptest.NewRequest("GET", "/anything", nil)

			recorder := httptest.NewRecorder()

			mux, err := e.Router.BuildMux()
			if err != nil {
				t.Fatalf("Failed to build router mux: %v", err)
			}
			mux.ServeHTTP(recorder, req)

			if recorder.Code != 200 {
				t.Fatalf("Expected status code %d, got %d", 200, recorder.Code)
			}

			body := recorder.Body.String()
			if body != "test" {
				t.Fatalf("Expected body %q, got %q", "test", body)
			}

			return nil
		})
	}

	pool.run(func(vm *goja.Runtime) error {
		val, err := vm.RunScript("verify", `$app`)
		if err != nil {
			t.Fatal(err)
		}

		if valApp := val.Export(); valApp != testApp {
			t.Fatalf("Expected $app to reset to its original value, got %v", valApp)
		}

		return nil
	})
}

func TestRouterMiddlewareClassAppReset(t *testing.T) {
	t.Parallel()

	testApp, _ := tests.NewTestApp()
	defer testApp.Cleanup()

	createVM := func() *goja.Runtime {
		vm := goja.New()
		vm.SetFieldNameMapper(FieldMapper{})
		vm.Set("$app", testApp)
		BindCore(vm)
		return vm
	}

	loader := createVM()

	pool := newPool(1, createVM)

	routerBinds(testApp, loader, pool)

	// register route middleware class
	_, err := loader.RunScript("stub", `
		routerUse(new Middleware((e) => {
			e.string(200, "test")

			$app = 123; // overwrite
		}))
	`)
	if err != nil {
		t.Fatal(err)
	}

	// create mock web server
	{
		baseRouter, err := apis.NewRouter(testApp)
		if err != nil {
			t.Fatal(err)
		}

		// manually trigger the serve event to ensure that custom app routes and middlewares are registered
		serveEvent := new(core.ServeEvent)
		serveEvent.App = testApp
		serveEvent.Router = baseRouter
		_ = testApp.OnServe().Trigger(serveEvent, func(e *core.ServeEvent) error {
			// it doesn't matter as long as the middleware is called
			req := httptest.NewRequest("GET", "/anything", nil)

			recorder := httptest.NewRecorder()

			mux, err := e.Router.BuildMux()
			if err != nil {
				t.Fatalf("Failed to build router mux: %v", err)
			}
			mux.ServeHTTP(recorder, req)

			if recorder.Code != 200 {
				t.Fatalf("Expected status code %d, got %d", 200, recorder.Code)
			}

			body := recorder.Body.String()
			if body != "test" {
				t.Fatalf("Expected body %q, got %q", "test", body)
			}

			return nil
		})
	}

	pool.run(func(vm *goja.Runtime) error {
		val, err := vm.RunScript("verify", `$app`)
		if err != nil {
			t.Fatal(err)
		}

		if valApp := val.Export(); valApp != testApp {
			t.Fatalf("Expected $app to reset to its original value, got %v", valApp)
		}

		return nil
	})
}
