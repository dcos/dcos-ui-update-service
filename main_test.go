package main

import (
	"io/ioutil"
	"net"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/dcos/dcos-ui-update-service/config"
	"github.com/dcos/dcos-ui-update-service/dcos"
	"github.com/dcos/dcos-ui-update-service/uiService"
	"github.com/dcos/dcos-ui-update-service/updateManager"
	"github.com/spf13/afero"
)

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
	service := setupTestUIService()
	service.UIHandler.UpdateDocumentRoot("./testdata/docroot/public")
	go func() {
		appDoneCh <- service.Run(l)
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

func listen() (net.Listener, error) {
	// Allocate a new port in the ephemeral range and listen on it.
	return net.Listen("tcp", "127.0.0.1:0")
}

func setupTestUIService() *uiService.UIService {
	cfg := config.NewDefaultConfig()
	cfg.DefaultDocRoot = "./public"
	cfg.VersionsRoot = "/ui-versions"
	cfg.MasterCountFile = "./fixtures/single-master"

	um, _ := updateManager.NewClient(cfg)
	um.Fs = afero.NewMemMapFs()
	um.Fs.MkdirAll("/ui-versions", 0755)

	uiHandler := uiService.SetupUIHandler(cfg, um)

	return &uiService.UIService{
		Config:        cfg,
		UpdateManager: um,
		UIHandler:     uiHandler,
		MasterCounter: dcos.DCOS{
			MasterCountLocation: cfg.MasterCountFile,
		},
		VersionStore: VersionStoreDouble(),
	}
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
