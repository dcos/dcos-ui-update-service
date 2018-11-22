package main

import (
	"errors"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/dcos/dcos-ui-update-service/config"
	"github.com/dcos/dcos-ui-update-service/dcos"
	"github.com/dcos/dcos-ui-update-service/fileHandler"
	"github.com/dcos/dcos-ui-update-service/tests"
	"github.com/dcos/dcos-ui-update-service/uiService"
	"github.com/dcos/dcos-ui-update-service/updateManager"
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

	um, _ := updateManager.NewClient(cfg)
	um.Fs = afero.NewMemMapFs()
	um.Fs.MkdirAll("/ui-versions", 0755)

	uiHandler := SetupUIHandler(cfg, um)

	return &UIService{
		Config:        cfg,
		UpdateManager: um,
		UIHandler:     uiHandler,
		MasterCounter: dcos.DCOS{
			MasterCountLocation: cfg.MasterCountFile,
		},
		versionStore: VersionStoreDouble(),
	}
}

func setupUIServiceWithVersion() *UIService {
	cfg := config.NewDefaultConfig()
	cfg.DefaultDocRoot = "./public"
	cfg.VersionsRoot = "/ui-versions"
	cfg.MasterCountFile = "./fixtures/single-master"

	um, _ := updateManager.NewClient(cfg)
	um.Fs = afero.NewMemMapFs()
	um.Fs.MkdirAll("/ui-versions/2.24.4/dist", 0755)

	uiHandler := SetupUIHandler(cfg, um)

	return &UIService{
		Config:        cfg,
		UpdateManager: um,
		UIHandler:     uiHandler,
		MasterCounter: dcos.DCOS{
			MasterCountLocation: cfg.MasterCountFile,
		},
		versionStore: VersionStoreDouble(),
	}
}

func setupUIServiceWithMemoryFs() (*UIService, afero.Fs) {
	cfg := config.NewDefaultConfig()
	cfg.DefaultDocRoot = "/usr/public"
	cfg.VersionsRoot = "/ui-versions"
	cfg.MasterCountFile = "./fixtures/single-master"

	fs := afero.NewMemMapFs()
	fs.MkdirAll(cfg.DefaultDocRoot, 0755)

	um, _ := updateManager.NewClient(cfg)
	um.Fs = fs

	uiHandler := fileHandler.NewUIFileHandler("/static/", cfg.DefaultDocRoot, fs)

	return &UIService{
		Config:        cfg,
		UpdateManager: um,
		UIHandler:     uiHandler,
		MasterCounter: dcos.DCOS{
			MasterCountLocation: cfg.MasterCountFile,
		},
		versionStore: VersionStoreDouble(),
	}, fs
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
	t.Parallel()

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

		if status := rr.Code; status != http.StatusOK {
			t.Errorf("handler returned wrong status code: got %v want %v",
				status, http.StatusOK)
		}
	})

	t.Run("Version Update - update manager error", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/api/v1/update/2.24.4/", nil)
		if err != nil {
			t.Fatal(err)
		}
		service, _ := setupUIServiceWithMemoryFs()

		um := UpdateManagerDouble()
		um.UpdateError = errors.New("Things went boom!")
		service.UpdateManager = um

		rr := httptest.NewRecorder()
		newRouter(service).ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusLocked {
			t.Errorf("handler returned wrong status code: got %v want %v",
				status, http.StatusLocked)
		}
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

		service.versionStore = vsd

		rr := httptest.NewRecorder()
		newRouter(service).ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusLocked {
			t.Errorf("handler returned wrong status code: got %v want %v",
				status, http.StatusLocked)
		}
	})
}

func TestSetupUIHandler(t *testing.T) {
	t.Run("sets DefaultDocRoot as document root if no current version", func(t *testing.T) {
		cfg := config.NewDefaultConfig()
		cfg.DefaultDocRoot = "./public"
		cfg.VersionsRoot = "/ui-versions"
		cfg.MasterCountFile = "./fixtures/single-master"

		um, _ := updateManager.NewClient(cfg)
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

		um, _ := updateManager.NewClient(cfg)
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

func TestVersionChange(t *testing.T) {
	t.Parallel()

	t.Run("Reset if new version is empty", func(t *testing.T) {
		var resetCalled, updateCalled bool
		service, _ := setupUIServiceWithMemoryFs()

		um := UpdateManagerDouble()
		um.VersionResult = "2.24.4"
		um.ResetCall = func() error {
			resetCalled = true
			return nil
		}
		um.UpdateCall = func(newVer string) {
			updateCalled = true
		}
		service.UpdateManager = um

		handleVersionChange(service, "")

		tests.H(t).BoolEql(resetCalled, true)
		tests.H(t).BoolEql(updateCalled, false)
	})

	t.Run("Upgrade if new version out of sync", func(t *testing.T) {
		var resetCalled, updateCalled bool
		service, fs := setupUIServiceWithMemoryFs()

		um := UpdateManagerDouble()
		um.VersionResult = "2.24.4"
		um.UpdateNewVersionPath = "/ui-versions/2.24.5/dist"
		um.ResetCall = func() error {
			resetCalled = true
			return nil
		}
		um.UpdateCall = func(newVer string) {
			fs.MkdirAll("/ui-versions/2.24.5/dist", 0755)
			updateCalled = true
		}
		service.UpdateManager = um

		handleVersionChange(service, "2.24.5")

		tests.H(t).BoolEql(resetCalled, false)
		tests.H(t).BoolEql(updateCalled, true)
		tests.H(t).StringEql(service.UIHandler.DocumentRoot(), "/ui-versions/2.24.5/dist")
	})

	t.Run("do nothing if version matches current", func(t *testing.T) {
		var resetCalled, updateCalled bool
		service, _ := setupUIServiceWithMemoryFs()

		um := UpdateManagerDouble()
		um.VersionResult = "2.24.4"
		um.ResetCall = func() error {
			resetCalled = true
			return nil
		}
		um.UpdateCall = func(newVer string) {
			updateCalled = true
		}
		service.UpdateManager = um

		handleVersionChange(service, "2.24.4")

		tests.H(t).BoolEql(resetCalled, false)
		tests.H(t).BoolEql(updateCalled, false)
	})
}

type fakeUpdateManager struct {
	VersionResult        string
	VersionError         error
	VersionPathResult    string
	VersionPathError     error
	ResetError           error
	ResetCall            func() error
	UpdateError          error
	UpdateCall           func(string)
	UpdateNewVersionPath string
}

func UpdateManagerDouble() *fakeUpdateManager {
	return &fakeUpdateManager{
		VersionResult: "2.24.4",
	}
}

func (um *fakeUpdateManager) UpdateToVersion(newVer string, cb func(string) error) error {
	if um.UpdateError != nil {
		return um.UpdateError
	}
	if um.UpdateCall != nil {
		um.UpdateCall(newVer)
	}
	if cberr := cb(um.UpdateNewVersionPath); cberr != nil {
		return cberr
	}
	return nil
}

func (um *fakeUpdateManager) ResetVersion() error {
	if um.ResetError != nil {
		return um.ResetError
	}
	if um.ResetCall != nil {
		tErr := um.ResetCall()
		return tErr
	}
	return nil
}

func (um *fakeUpdateManager) CurrentVersion() (string, error) {
	if um.VersionError != nil {
		return "", um.VersionError
	}
	return um.VersionResult, nil
}

func (um *fakeUpdateManager) PathToCurrentVersion() (string, error) {
	if um.VersionPathError != nil {
		return "", um.VersionPathError
	}
	return um.VersionPathResult, nil
}

type fakeVersionStore struct {
	VersionResult uiService.UIVersion
	UpdateError   error
}

func VersionStoreDouble() *fakeVersionStore {
	return &fakeVersionStore{
		VersionResult: uiService.UIVersion("2.24.4"),
	}
}

func (vs *fakeVersionStore) CurrentVersion() (uiService.UIVersion, error) {
	return vs.VersionResult, nil
}

func (vs *fakeVersionStore) UpdateCurrentVersion(newVersion uiService.UIVersion) error {
	if vs.UpdateError != nil {
		return vs.UpdateError
	}
	return nil
}

func (vs *fakeVersionStore) WatchForVersionChange(listener uiService.VersionChangeListener) error {
	return nil
}
