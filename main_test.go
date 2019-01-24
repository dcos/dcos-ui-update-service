package main

import (
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"testing"

	"github.com/dcos/dcos-ui-update-service/config"
	"github.com/dcos/dcos-ui-update-service/dcos"
	"github.com/dcos/dcos-ui-update-service/tests"
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
	defer tearDown(t)
	// Start a test server.
	service := setupTestUIService()

	go func() {
		appDoneCh <- service.Run(l)
	}()
	// Yay! we're finally ready to perform requests against our server.
	addr := "http://" + l.Addr().String()
	resp, err := http.Get(addr + "/api/v1/version/")
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
	tests.H(t).StringEql(string(got), "Default")
}

func listen() (net.Listener, error) {
	// Allocate a new port in the ephemeral range and listen on it.
	return net.Listen("tcp", "127.0.0.1:0")
}

func setupTestUIService() *uiService.UIService {
	cfg := config.NewDefaultConfig()
	cfg.DefaultDocRoot = "./testdata/main-sandbox/dcos-ui"
	cfg.VersionsRoot = "./testdata/main-sandbox/ui-versions"
	cfg.UIDistSymlink = "./testdata/main-sandbox/dcos-ui-dist"
	cfg.MasterCountFile = "./fixtures/single-master"

	um, _ := updateManager.NewClient(cfg)
	um.Fs = afero.NewOsFs()

	os.MkdirAll(cfg.VersionsRoot, 0755)
	os.MkdirAll(cfg.DefaultDocRoot, 0755)
	os.Symlink(cfg.DefaultDocRoot, cfg.UIDistSymlink)

	return &uiService.UIService{
		Config:        cfg,
		UpdateManager: um,
		MasterCounter: dcos.DCOS{
			MasterCountLocation: cfg.MasterCountFile,
		},
		VersionStore: VersionStoreDouble(),
	}
}

func tearDown(t *testing.T) {
	t.Log("Teardown testdata sandbox")
	os.RemoveAll("./testdata/main-sandbox")
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
