package uiService

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dcos/dcos-ui-update-service/tests"
	"github.com/dcos/dcos-ui-update-service/updateManager"
)

func TestRouter(t *testing.T) {
	t.Parallel()

	t.Run("serves static files", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/static/", nil)
		if err != nil {
			t.Fatal(err)
		}
		service := setupTestUIService()

		rr := httptest.NewRecorder()
		newRouter(service).ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusOK {
			t.Errorf("handler returned wrong status code: got %v want %v",
				status, http.StatusOK)
		}

		expected := `<h1>Test</h1>`
		if rr.Body.String() != expected {
			t.Errorf("handler returned unexpected body: got %v want %v",
				rr.Body.String(), expected)
		}
	})

	t.Run("Status codes", func(t *testing.T) {
		var testCases = []struct {
			name       string
			uri        string
			statusCode int
		}{
			{"returns with 404 on root", "/", http.StatusNotFound},
			{"returns with 200 on static", "/static/", http.StatusOK},
			{"returns with 501 on api/v1", "/api/v1/", http.StatusNotImplemented},
			{"returns with 404 on api", "/api", http.StatusNotFound},
			{"returns with 405 on GET api/v1/reset", "/api/v1/reset/", http.StatusMethodNotAllowed},
		}

		for _, tt := range testCases {
			t.Run(tt.name, func(t *testing.T) {
				req, err := http.NewRequest("GET", tt.uri, nil)
				if err != nil {
					t.Fatal(err)
				}
				service := setupTestUIService()

				rr := httptest.NewRecorder()
				newRouter(service).ServeHTTP(rr, req)

				if rr.Code != tt.statusCode {
					t.Errorf("handler for %v returned unexpected statuscode: got %v want %v",
						tt.uri, rr.Code, tt.statusCode)
				}

			})
		}
	})

	t.Run("Reset to prebundled UI", func(t *testing.T) {
		req, err := http.NewRequest("DELETE", "/api/v1/reset/", nil)
		if err != nil {
			t.Fatal(err)
		}
		service := setupUIServiceWithVersion()
		umDouble := UpdateManagerDouble()
		service.UpdateManager = umDouble

		rr := httptest.NewRecorder()
		newRouter(service).ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusOK {
			t.Errorf("handler returned wrong status code: got %v want %v",
				status, http.StatusOK)
		}
	})

	t.Run("Version Update", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/api/v1/update/2.24.4/", nil)
		if err != nil {
			t.Fatal(err)
		}
		service, fs := setupUIServiceWithMemoryFs()
		fs.MkdirAll("/ui-versions/2.24.4/dist", 0755)

		um := UpdateManagerDouble()
		um.UpdateNewVersionPath = "/ui-versions/2.24.4/dist"
		service.UpdateManager = um

		rr := httptest.NewRecorder()
		newRouter(service).ServeHTTP(rr, req)

		tests.H(t).IntEql(rr.Code, http.StatusOK)
		tests.H(t).StringContains(rr.Body.String(), "Update to 2.24.4 completed")
	})

	t.Run("Version Update - unlocks after update", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/api/v1/update/2.24.4/", nil)
		if err != nil {
			t.Fatal(err)
		}
		service, fs := setupUIServiceWithMemoryFs()
		fs.MkdirAll("/ui-versions/2.24.4/dist", 0755)

		um := UpdateManagerDouble()
		um.UpdateNewVersionPath = "/ui-versions/2.24.4/dist"
		service.UpdateManager = um

		rr := httptest.NewRecorder()
		newRouter(service).ServeHTTP(rr, req)

		tests.H(t).BoolEql(service.updating, false)
	})

	t.Run("Version Update - locked during update", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/api/v1/update/2.24.4/", nil)
		if err != nil {
			t.Fatal(err)
		}
		service, _ := setupUIServiceWithMemoryFs()

		um := UpdateManagerDouble()
		service.UpdateManager = um
		service.updating = true
		service.updatingVersion = "2.24.4"

		rr := httptest.NewRecorder()
		newRouter(service).ServeHTTP(rr, req)

		tests.H(t).IntEql(rr.Code, http.StatusAccepted)
		tests.H(t).StringContains(rr.Body.String(), "Service is currently processing an update request")
	})

	t.Run("Version Update - locked during update to different version", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/api/v1/update/2.24.4/", nil)
		if err != nil {
			t.Fatal(err)
		}
		service, _ := setupUIServiceWithMemoryFs()

		um := UpdateManagerDouble()
		service.UpdateManager = um
		service.updating = true
		service.updatingVersion = "2.24.3"

		rr := httptest.NewRecorder()
		newRouter(service).ServeHTTP(rr, req)

		tests.H(t).IntEql(rr.Code, http.StatusConflict)
		tests.H(t).StringContains(rr.Body.String(), "Service is currently processing an update request to 2.24.3")
	})

	t.Run("Version Update - store error", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/api/v1/update/2.24.4/", nil)
		if err != nil {
			t.Fatal(err)
		}
		service, fs := setupUIServiceWithMemoryFs()
		fs.MkdirAll("/ui-versions/2.24.4/dist", 0755)

		um := UpdateManagerDouble()
		um.UpdateNewVersionPath = "/ui-versions/2.24.4/dist"
		service.UpdateManager = um

		vsd := VersionStoreDouble()
		vsd.UpdateError = errors.New("Failed to update version in store")

		service.VersionStore = vsd

		rr := httptest.NewRecorder()
		newRouter(service).ServeHTTP(rr, req)

		tests.H(t).IntEql(rr.Code, http.StatusInternalServerError)
		tests.H(t).StringContains(rr.Body.String(), "Failed to update version in store")
	})

	t.Run("Version Update - version not available", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/api/v1/update/2.24.4/", nil)
		if err != nil {
			t.Fatal(err)
		}
		service, _ := setupUIServiceWithMemoryFs()

		um := UpdateManagerDouble()
		um.UpdateError = updateManager.ErrRequestedVersionNotFound
		service.UpdateManager = um

		rr := httptest.NewRecorder()
		newRouter(service).ServeHTTP(rr, req)

		tests.H(t).IntEql(rr.Code, http.StatusBadRequest)
		tests.H(t).StringContains(rr.Body.String(), updateManager.ErrRequestedVersionNotFound.Error())
	})
}
