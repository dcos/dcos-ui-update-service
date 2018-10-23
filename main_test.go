package main

import (
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/dcos/dcos-ui-update-service/client"
	"github.com/dcos/dcos-ui-update-service/config"
)

func listen() (net.Listener, error) {
	// Allocate a new port in the ephemeral range and listen on it.
	return net.Listen("tcp", "127.0.0.1:0")
}

func testAppState() *ApplicationState {
	return makeAppState("./testdata/empty-versions")
}

func makeAppState(versionsRoot string) *ApplicationState {
	cfg := config.NewDefaultConfig()
	cfg.ClusterUIPath = "./public"
	cfg.VersionsRoot = versionsRoot
	cfg.MasterCountFile = "./fixtures/single-master"

	um := NewUpdateManager(cfg, &client.HTTP{})
	uiHandler := LoadUIHandler(cfg, um)

	state := &ApplicationState{
		Config:        cfg,
		UpdateManager: um,
		UIHandler:     uiHandler,
	}
	return state
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
	appState := testAppState()
	appState.UIHandler.UpdateDocumentRoot("./testdata/docroot/public")
	go func() {
		appDoneCh <- Run(appState, l)
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
	documentRoot := appState.UIHandler.GetDocumentRoot()
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
		appState := testAppState()

		rr := httptest.NewRecorder()
		newRouter(appState).ServeHTTP(rr, req)

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
				appState := testAppState()

				rr := httptest.NewRecorder()
				newRouter(appState).ServeHTTP(rr, req)

				if rr.Code != tt.statusCode {
					t.Errorf("handler for %v returned unexpected statuscode: got %v want %v",
						tt.uri, rr.Code, tt.statusCode)
				}

			})
		}
	})
}

func TestLoadUIHandler(t *testing.T) {
	t.Run("sets ClusterUIPath as document root if no current version", func(t *testing.T) {
		appState := testAppState()

		docRoot := appState.UIHandler.GetDocumentRoot()
		expected := appState.Config.ClusterUIPath
		if docRoot != expected {
			t.Errorf("ui handler documentroot set to %v, expected %v", docRoot, expected)
		}
	})

	t.Run("sets ClusterUIPath as document root if no current version", func(t *testing.T) {
		appState := makeAppState("./testdata/one-version")

		docRoot := appState.UIHandler.GetDocumentRoot()
		expected, err := appState.UpdateManager.GetPathToCurrentVersion()
		if err != nil {
			t.Fatal(err)
		}
		if docRoot != expected {
			t.Errorf("ui handler documentroot set to %v, expected %v", docRoot, expected)
		}
	})
}
