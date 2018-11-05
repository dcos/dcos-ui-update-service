package main

import (
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/dcos/dcos-ui-update-service/config"
	our_http "github.com/dcos/dcos-ui-update-service/http"
	"github.com/spf13/afero"
)

func listen() (net.Listener, error) {
	// Allocate a new port in the ephemeral range and listen on it.
	return net.Listen("tcp", "127.0.0.1:0")
}

func setupUIService() *UIService {
	cfg := config.NewDefaultConfig()
	cfg.DefaultDocRoot = "./public"
	cfg.VersionsRoot = "/ui-versions"
	cfg.MasterCountFile = "./fixtures/single-master"

	um, _ := NewUpdateManager(cfg, &our_http.Client{})
	um.Fs = afero.NewMemMapFs()
	um.Fs.MkdirAll("/ui-versions", 0755)

	uiHandler := SetupUIHandler(cfg, um)

	return &UIService{
		Config:        cfg,
		UpdateManager: um,
		UIHandler:     uiHandler,
	}
}

func TestApplication(t *testing.T) {
	// Get a socket to listen on.
	l, err := listen()
	if err != nil {
		t.Fatal(err)
	}
	// appDoneCh waits for Run() to exit and receives the error.
	appDoneCh := make(chan error)
	// Wait for the server to exit and return an error before returning
	// from the test. This is important as having zombie goroutines in your
	// tests can cause some very hard to debug issues.
	defer func() { t.Logf("Server stopped: %v", <-appDoneCh) }()
	// Close the listener after this test exits. This triggers the server
	// to stop running and return an error from Run().
	defer l.Close()
	// Start a test server.
	service := setupUIService()
	service.UIHandler.UpdateDocumentRoot("./testdata/docroot/public")
	go func() {
		appDoneCh <- Run(service, l)
	}()
	// Yay! we're finally ready to perform requests against our server.
	addr := "http://" + l.Addr().String()
	resp, err := http.Get(addr + "/static/test.html")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if status := resp.StatusCode; status != http.StatusOK {
		t.Fatalf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}
	got, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	documentRoot := service.UIHandler.DocumentRoot()
	exp, err := ioutil.ReadFile(filepath.Join(documentRoot, "test.html"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(exp) {
		t.Fatalf("Expected %q but got %q", string(exp), string(got))
	}
}

func TestRouter(t *testing.T) {

	t.Run("serves static files", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/static/", nil)
		if err != nil {
			t.Fatal(err)
		}
		service := setupUIService()

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
				service := setupUIService()

				rr := httptest.NewRecorder()
				newRouter(service).ServeHTTP(rr, req)

				if rr.Code != tt.statusCode {
					t.Errorf("handler for %v returned unexpected statuscode: got %v want %v",
						tt.uri, rr.Code, tt.statusCode)
				}

			})
		}
	})
}

func TestSetupUIHandler(t *testing.T) {
	t.Run("sets DefaultDocRoot as document root if no current version", func(t *testing.T) {
		cfg := config.NewDefaultConfig()
		cfg.DefaultDocRoot = "./public"
		cfg.VersionsRoot = "/ui-versions"
		cfg.MasterCountFile = "./fixtures/single-master"

		um, _ := NewUpdateManager(cfg, &our_http.Client{})
		um.Fs = afero.NewMemMapFs()
		um.Fs.MkdirAll("/ui-versions", 0755)

		uiHandler := SetupUIHandler(cfg, um)

		docRoot := uiHandler.DocumentRoot()
		expected := cfg.DefaultDocRoot
		if docRoot != expected {
			t.Errorf("ui handler documentroot set to %v, expected %v", docRoot, expected)
		}
	})

	t.Run("sets version as document root if there is a current version", func(t *testing.T) {
		cfg := config.NewDefaultConfig()
		cfg.DefaultDocRoot = "./public"
		cfg.VersionsRoot = "/ui-versions"
		cfg.MasterCountFile = "./fixtures/single-master"

		um, _ := NewUpdateManager(cfg, &our_http.Client{})
		um.Fs = afero.NewMemMapFs()
		um.Fs.MkdirAll("/ui-versions/2.25.3", 0755)

		uiHandler := SetupUIHandler(cfg, um)

		docRoot := uiHandler.DocumentRoot()
		expected, err := um.PathToCurrentVersion()
		if err != nil {
			t.Fatal(err)
		}
		if docRoot != expected {
			t.Errorf("ui handler documentroot set to %v, expected %v", docRoot, expected)
		}
	})
}
