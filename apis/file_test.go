package apis_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
	"github.com/pocketbase/pocketbase/tools/types"
)

func TestFileToken(t *testing.T) {
	t.Parallel()

	scenarios := []tests.ApiScenario{
		{
			Name:            "unauthorized",
			Method:          http.MethodPost,
			URL:             "/api/files/token",
			ExpectedStatus:  401,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "regular user",
			Method: http.MethodPost,
			URL:    "/api/files/token",
			Headers: map[string]string{
				"Authorization": tests.NewAuthTokenForTest("users", "test@example.com"),
			},
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"token":"`,
			},
			ExpectedEvents: map[string]int{
				"*":                  0,
				"OnFileTokenRequest": 1,
			},
		},
		{
			Name:   "superuser",
			Method: http.MethodPost,
			URL:    "/api/files/token",
			Headers: map[string]string{
				"Authorization": tests.NewAuthTokenForTest("_superusers", "test@example.com"),
			},
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"token":"`,
			},
			ExpectedEvents: map[string]int{
				"*":                  0,
				"OnFileTokenRequest": 1,
			},
		},
		{
			Name:   "hook token overwrite",
			Method: http.MethodPost,
			URL:    "/api/files/token",
			Headers: map[string]string{
				"Authorization": tests.NewAuthTokenForTest("_superusers", "test@example.com"),
			},
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				app.OnFileTokenRequest().BindFunc(func(e *core.FileTokenRequestEvent) error {
					e.Token = "test"
					return e.Next()
				})
			},
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"token":"test"`,
			},
			ExpectedEvents: map[string]int{
				"*":                  0,
				"OnFileTokenRequest": 1,
			},
		},
	}

	for _, scenario := range scenarios {
		scenario.Test(t)
	}
}

func TestFileDownload(t *testing.T) {
	t.Parallel()

	_, currentFile, _, _ := runtime.Caller(0)
	dataDirRelPath := "../tests/data/"

	testFilePath := filepath.Join(path.Dir(currentFile), dataDirRelPath, "storage/11111111-1111-1111-1111-111111111111/0196afca-7951-77d1-ba15-923db9b774b2/test_kfd2wYLxkz.txt")
	testImgPath := filepath.Join(path.Dir(currentFile), dataDirRelPath, "storage/11111111-1111-1111-1111-111111111111/0196afca-7951-76f3-b344-ae38a366ade2/300_1SEi6Q6U72.png")
	testThumbCropCenterPath := filepath.Join(path.Dir(currentFile), dataDirRelPath, "storage/11111111-1111-1111-1111-111111111111/0196afca-7951-76f3-b344-ae38a366ade2/thumbs_300_1SEi6Q6U72.png/70x50_300_1SEi6Q6U72.png")
	testThumbCropTopPath := filepath.Join(path.Dir(currentFile), dataDirRelPath, "storage/11111111-1111-1111-1111-111111111111/0196afca-7951-76f3-b344-ae38a366ade2/thumbs_300_1SEi6Q6U72.png/70x50t_300_1SEi6Q6U72.png")
	testThumbCropBottomPath := filepath.Join(path.Dir(currentFile), dataDirRelPath, "storage/11111111-1111-1111-1111-111111111111/0196afca-7951-76f3-b344-ae38a366ade2/thumbs_300_1SEi6Q6U72.png/70x50b_300_1SEi6Q6U72.png")
	testThumbFitPath := filepath.Join(path.Dir(currentFile), dataDirRelPath, "storage/11111111-1111-1111-1111-111111111111/0196afca-7951-76f3-b344-ae38a366ade2/thumbs_300_1SEi6Q6U72.png/70x50f_300_1SEi6Q6U72.png")
	testThumbZeroWidthPath := filepath.Join(path.Dir(currentFile), dataDirRelPath, "storage/11111111-1111-1111-1111-111111111111/0196afca-7951-76f3-b344-ae38a366ade2/thumbs_300_1SEi6Q6U72.png/0x50_300_1SEi6Q6U72.png")
	testThumbZeroHeightPath := filepath.Join(path.Dir(currentFile), dataDirRelPath, "storage/11111111-1111-1111-1111-111111111111/0196afca-7951-76f3-b344-ae38a366ade2/thumbs_300_1SEi6Q6U72.png/70x0_300_1SEi6Q6U72.png")

	testFile, fileErr := os.ReadFile(testFilePath)
	if fileErr != nil {
		t.Fatal(fileErr)
	}

	testImg, imgErr := os.ReadFile(testImgPath)
	if imgErr != nil {
		t.Fatal(imgErr)
	}

	testThumbCropCenter, thumbErr := os.ReadFile(testThumbCropCenterPath)
	if thumbErr != nil {
		t.Fatal(thumbErr)
	}

	testThumbCropTop, thumbErr := os.ReadFile(testThumbCropTopPath)
	if thumbErr != nil {
		t.Fatal(thumbErr)
	}

	testThumbCropBottom, thumbErr := os.ReadFile(testThumbCropBottomPath)
	if thumbErr != nil {
		t.Fatal(thumbErr)
	}

	testThumbFit, thumbErr := os.ReadFile(testThumbFitPath)
	if thumbErr != nil {
		t.Fatal(thumbErr)
	}

	testThumbZeroWidth, thumbErr := os.ReadFile(testThumbZeroWidthPath)
	if thumbErr != nil {
		t.Fatal(thumbErr)
	}

	testThumbZeroHeight, thumbErr := os.ReadFile(testThumbZeroHeightPath)
	if thumbErr != nil {
		t.Fatal(thumbErr)
	}

	scenarios := []tests.ApiScenario{
		{
			Name:            "missing collection",
			Method:          http.MethodGet,
			URL:             "/api/files/missing/0196afca-7951-76f3-b344-ae38a366ade2/300_1SEi6Q6U72.png",
			ExpectedStatus:  404,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:            "missing record",
			Method:          http.MethodGet,
			URL:             "/api/files/11111111-1111-1111-1111-111111111111/missing/300_1SEi6Q6U72.png",
			ExpectedStatus:  404,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:            "missing file",
			Method:          http.MethodGet,
			URL:             "/api/files/11111111-1111-1111-1111-111111111111/0196afca-7951-76f3-b344-ae38a366ade2/missing.png",
			ExpectedStatus:  404,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:            "existing image",
			Method:          http.MethodGet,
			URL:             "/api/files/11111111-1111-1111-1111-111111111111/0196afca-7951-76f3-b344-ae38a366ade2/300_1SEi6Q6U72.png",
			ExpectedStatus:  200,
			ExpectedContent: []string{string(testImg)},
			ExpectedEvents: map[string]int{
				"*":                     0,
				"OnFileDownloadRequest": 1,
			},
		},
		{
			Name:            "existing image - missing thumb (should fallback to the original)",
			Method:          http.MethodGet,
			URL:             "/api/files/11111111-1111-1111-1111-111111111111/0196afca-7951-76f3-b344-ae38a366ade2/300_1SEi6Q6U72.png?thumb=999x999",
			ExpectedStatus:  200,
			ExpectedContent: []string{string(testImg)},
			ExpectedEvents: map[string]int{
				"*":                     0,
				"OnFileDownloadRequest": 1,
			},
		},
		{
			Name:            "existing image - existing thumb (crop center)",
			Method:          http.MethodGet,
			URL:             "/api/files/11111111-1111-1111-1111-111111111111/0196afca-7951-76f3-b344-ae38a366ade2/300_1SEi6Q6U72.png?thumb=70x50",
			ExpectedStatus:  200,
			ExpectedContent: []string{string(testThumbCropCenter)},
			ExpectedEvents: map[string]int{
				"*":                     0,
				"OnFileDownloadRequest": 1,
			},
		},
		{
			Name:            "existing image - existing thumb (crop top)",
			Method:          http.MethodGet,
			URL:             "/api/files/11111111-1111-1111-1111-111111111111/0196afca-7951-76f3-b344-ae38a366ade2/300_1SEi6Q6U72.png?thumb=70x50t",
			ExpectedStatus:  200,
			ExpectedContent: []string{string(testThumbCropTop)},
			ExpectedEvents: map[string]int{
				"*":                     0,
				"OnFileDownloadRequest": 1,
			},
		},
		{
			Name:            "existing image - existing thumb (crop bottom)",
			Method:          http.MethodGet,
			URL:             "/api/files/11111111-1111-1111-1111-111111111111/0196afca-7951-76f3-b344-ae38a366ade2/300_1SEi6Q6U72.png?thumb=70x50b",
			ExpectedStatus:  200,
			ExpectedContent: []string{string(testThumbCropBottom)},
			ExpectedEvents: map[string]int{
				"*":                     0,
				"OnFileDownloadRequest": 1,
			},
		},
		{
			Name:            "existing image - existing thumb (fit)",
			Method:          http.MethodGet,
			URL:             "/api/files/11111111-1111-1111-1111-111111111111/0196afca-7951-76f3-b344-ae38a366ade2/300_1SEi6Q6U72.png?thumb=70x50f",
			ExpectedStatus:  200,
			ExpectedContent: []string{string(testThumbFit)},
			ExpectedEvents: map[string]int{
				"*":                     0,
				"OnFileDownloadRequest": 1,
			},
		},
		{
			Name:            "existing image - existing thumb (zero width)",
			Method:          http.MethodGet,
			URL:             "/api/files/11111111-1111-1111-1111-111111111111/0196afca-7951-76f3-b344-ae38a366ade2/300_1SEi6Q6U72.png?thumb=0x50",
			ExpectedStatus:  200,
			ExpectedContent: []string{string(testThumbZeroWidth)},
			ExpectedEvents: map[string]int{
				"*":                     0,
				"OnFileDownloadRequest": 1,
			},
		},
		{
			Name:            "existing image - existing thumb (zero height)",
			Method:          http.MethodGet,
			URL:             "/api/files/11111111-1111-1111-1111-111111111111/0196afca-7951-76f3-b344-ae38a366ade2/300_1SEi6Q6U72.png?thumb=70x0",
			ExpectedStatus:  200,
			ExpectedContent: []string{string(testThumbZeroHeight)},
			ExpectedEvents: map[string]int{
				"*":                     0,
				"OnFileDownloadRequest": 1,
			},
		},
		{
			Name:            "existing non image file - thumb parameter should be ignored",
			Method:          http.MethodGet,
			URL:             "/api/files/11111111-1111-1111-1111-111111111111/0196afca-7951-77d1-ba15-923db9b774b2/test_kfd2wYLxkz.txt?thumb=100x100",
			ExpectedStatus:  200,
			ExpectedContent: []string{string(testFile)},
			ExpectedEvents: map[string]int{
				"*":                     0,
				"OnFileDownloadRequest": 1,
			},
		},

		// protected file access checks
		{
			Name:            "protected file - superuser with expired file token",
			Method:          http.MethodGet,
			URL:             "/api/files/demo1/0196afca-7951-752e-972d-502c0843467d/300_Jsjq7RdBgA.png?token=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6InN5d2JoZWNuaDQ2cmhtMCIsImV4cCI6MTY0MDk5MTY2MSwidHlwZSI6ImZpbGUiLCJjb2xsZWN0aW9uSWQiOiJwYmNfMzE0MjYzNTgyMyJ9.nqqtqpPhxU0045F4XP_ruAkzAidYBc5oPy9ErN3XBq0",
			ExpectedStatus:  404,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:            "protected file - superuser with valid file token",
			Method:          http.MethodGet,
			URL:             "/api/files/demo1/0196afca-7951-752e-972d-502c0843467d/300_Jsjq7RdBgA.png?token=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6InN5d2JoZWNuaDQ2cmhtMCIsImV4cCI6MjUyNDYwNDQ2MSwidHlwZSI6ImZpbGUiLCJjb2xsZWN0aW9uSWQiOiJwYmNfMzE0MjYzNTgyMyJ9.Lupz541xRvrktwkrl55p5pPCF77T69ZRsohsIcb2dxc",
			ExpectedStatus:  200,
			ExpectedContent: []string{"PNG"},
			ExpectedEvents: map[string]int{
				"*":                     0,
				"OnFileDownloadRequest": 1,
			},
		},
		{
			Name:            "protected file - guest without view access",
			Method:          http.MethodGet,
			URL:             "/api/files/demo1/0196afca-7951-752e-972d-502c0843467d/300_Jsjq7RdBgA.png",
			ExpectedStatus:  404,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "protected file - guest with view access",
			Method: http.MethodGet,
			URL:    "/api/files/demo1/0196afca-7951-752e-972d-502c0843467d/300_Jsjq7RdBgA.png",
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				// mock public view access
				c, err := app.FindCachedCollectionByNameOrId("demo1")
				if err != nil {
					t.Fatalf("Failed to fetch mock collection: %v", err)
				}
				c.ViewRule = types.Pointer("")
				if err := app.UnsafeWithoutHooks().Save(c); err != nil {
					t.Fatalf("Failed to update mock collection: %v", err)
				}
			},
			ExpectedStatus:  200,
			ExpectedContent: []string{"PNG"},
			ExpectedEvents: map[string]int{
				"*":                     0,
				"OnFileDownloadRequest": 1,
			},
		},
		{
			Name:   "protected file - auth record without view access",
			Method: http.MethodGet,
			URL:    "/api/files/demo1/0196afca-7951-752e-972d-502c0843467d/300_Jsjq7RdBgA.png?token=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6IjRxMXhsY2xtZmxva3UzMyIsImV4cCI6MjUyNDYwNDQ2MSwidHlwZSI6ImZpbGUiLCJjb2xsZWN0aW9uSWQiOiJfcGJfdXNlcnNfYXV0aF8ifQ.nSTLuCPcGpWn2K2l-BFkC3Vlzc-ZTDPByYq8dN1oPSo",
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				// mock restricted user view access
				c, err := app.FindCachedCollectionByNameOrId("demo1")
				if err != nil {
					t.Fatalf("Failed to fetch mock collection: %v", err)
				}
				c.ViewRule = types.Pointer("@request.auth.verified = true")
				if err := app.UnsafeWithoutHooks().Save(c); err != nil {
					t.Fatalf("Failed to update mock collection: %v", err)
				}
			},
			ExpectedStatus:  404,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "protected file - auth record with view access",
			Method: http.MethodGet,
			URL:    "/api/files/demo1/0196afca-7951-752e-972d-502c0843467d/300_Jsjq7RdBgA.png?token=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6IjRxMXhsY2xtZmxva3UzMyIsImV4cCI6MjUyNDYwNDQ2MSwidHlwZSI6ImZpbGUiLCJjb2xsZWN0aW9uSWQiOiJfcGJfdXNlcnNfYXV0aF8ifQ.nSTLuCPcGpWn2K2l-BFkC3Vlzc-ZTDPByYq8dN1oPSo",
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				// mock user view access
				c, err := app.FindCachedCollectionByNameOrId("demo1")
				if err != nil {
					t.Fatalf("Failed to fetch mock collection: %v", err)
				}
				c.ViewRule = types.Pointer("@request.auth.verified = false")
				if err := app.UnsafeWithoutHooks().Save(c); err != nil {
					t.Fatalf("Failed to update mock collection: %v", err)
				}
			},
			ExpectedStatus:  200,
			ExpectedContent: []string{"PNG"},
			ExpectedEvents: map[string]int{
				"*":                     0,
				"OnFileDownloadRequest": 1,
			},
		},
		{
			Name:            "protected file in view (view's View API rule failure)",
			Method:          http.MethodGet,
			URL:             "/api/files/view1/0196afca-7951-752e-972d-502c0843467d/300_Jsjq7RdBgA.png?token=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6IjRxMXhsY2xtZmxva3UzMyIsImV4cCI6MjUyNDYwNDQ2MSwidHlwZSI6ImZpbGUiLCJjb2xsZWN0aW9uSWQiOiJfcGJfdXNlcnNfYXV0aF8ifQ.nSTLuCPcGpWn2K2l-BFkC3Vlzc-ZTDPByYq8dN1oPSo",
			ExpectedStatus:  404,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:            "protected file in view (view's View API rule success)",
			Method:          http.MethodGet,
			URL:             "/api/files/view1/0196afca-7951-7ba1-8cef-b59777e4d838/test_d61b33QdDU.txt?token=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6IjRxMXhsY2xtZmxva3UzMyIsImV4cCI6MjUyNDYwNDQ2MSwidHlwZSI6ImZpbGUiLCJjb2xsZWN0aW9uSWQiOiJfcGJfdXNlcnNfYXV0aF8ifQ.nSTLuCPcGpWn2K2l-BFkC3Vlzc-ZTDPByYq8dN1oPSo",
			ExpectedStatus:  200,
			ExpectedContent: []string{"test"},
			ExpectedEvents: map[string]int{
				"*":                     0,
				"OnFileDownloadRequest": 1,
			},
		},

		// rate limit checks
		// -----------------------------------------------------------
		{
			Name:   "RateLimit rule - users:file",
			Method: http.MethodGet,
			URL:    "/api/files/11111111-1111-1111-1111-111111111111/0196afca-7951-76f3-b344-ae38a366ade2/300_1SEi6Q6U72.png",
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				app.Settings().RateLimits.Enabled = true
				app.Settings().RateLimits.Rules = []core.RateLimitRule{
					{MaxRequests: 100, Label: "abc"},
					{MaxRequests: 100, Label: "*:file"},
					{MaxRequests: 0, Label: "users:file"},
				}
			},
			ExpectedStatus:  429,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "RateLimit rule - *:file",
			Method: http.MethodGet,
			URL:    "/api/files/11111111-1111-1111-1111-111111111111/0196afca-7951-76f3-b344-ae38a366ade2/300_1SEi6Q6U72.png",
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				app.Settings().RateLimits.Enabled = true
				app.Settings().RateLimits.Rules = []core.RateLimitRule{
					{MaxRequests: 100, Label: "abc"},
					{MaxRequests: 0, Label: "*:file"},
				}
			},
			ExpectedStatus:  429,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
	}

	for _, scenario := range scenarios {
		// clone for the HEAD test (the same as the original scenario but without body)
		head := scenario
		head.Method = http.MethodHead
		head.Name = ("(HEAD) " + scenario.Name)
		head.ExpectedContent = nil
		head.Test(t)

		// regular request test
		scenario.Test(t)
	}
}

func TestConcurrentThumbsGeneration(t *testing.T) {
	t.Parallel()

	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	fsys, err := app.NewFilesystem()
	if err != nil {
		t.Fatal(err)
	}
	defer fsys.Close()

	// create a dummy file field collection
	demo1, err := app.FindCollectionByNameOrId("demo1")
	if err != nil {
		t.Fatal(err)
	}
	fileField := demo1.Fields.GetByName("file_one").(*core.FileField)
	fileField.Protected = false
	fileField.MaxSelect = 1
	fileField.MaxSize = 999999
	// new thumbs
	fileField.Thumbs = []string{"111x111", "111x222", "111x333"}
	demo1.Fields.Add(fileField)
	if err = app.Save(demo1); err != nil {
		t.Fatal(err)
	}

	fileKey := "0196afca-7951-7653-beca-d69f40c17bcd/0196afca-7951-752e-972d-502c0843467d/300_Jsjq7RdBgA.png"

	urls := []string{
		"/api/files/" + fileKey + "?thumb=111x111",
		"/api/files/" + fileKey + "?thumb=111x111", // should still result in single thumb
		"/api/files/" + fileKey + "?thumb=111x222",
		"/api/files/" + fileKey + "?thumb=111x333",
	}

	var wg sync.WaitGroup

	wg.Add(len(urls))

	for _, url := range urls {
		go func() {
			defer wg.Done()

			recorder := httptest.NewRecorder()

			req := httptest.NewRequest("GET", url, nil)

			pbRouter, _ := apis.NewRouter(app)
			mux, _ := pbRouter.BuildMux()
			if mux != nil {
				mux.ServeHTTP(recorder, req)
			}
		}()
	}

	wg.Wait()

	// ensure that all new requested thumbs were created
	thumbKeys := []string{
		"0196afca-7951-7653-beca-d69f40c17bcd/0196afca-7951-752e-972d-502c0843467d/thumbs_300_Jsjq7RdBgA.png/111x111_" + filepath.Base(fileKey),
		"0196afca-7951-7653-beca-d69f40c17bcd/0196afca-7951-752e-972d-502c0843467d/thumbs_300_Jsjq7RdBgA.png/111x222_" + filepath.Base(fileKey),
		"0196afca-7951-7653-beca-d69f40c17bcd/0196afca-7951-752e-972d-502c0843467d/thumbs_300_Jsjq7RdBgA.png/111x333_" + filepath.Base(fileKey),
	}
	for _, k := range thumbKeys {
		if exists, _ := fsys.Exists(k); !exists {
			t.Fatalf("Missing thumb %q: %v", k, err)
		}
	}
}
