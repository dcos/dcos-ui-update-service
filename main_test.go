package main

import (
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"testing"

	"github.com/dcos/dcos-ui-update-service/config"
	"github.com/dcos/dcos-ui-update-service/tests"
	"github.com/dcos/dcos-ui-update-service/uiservice"
	"github.com/dcos/dcos-ui-update-service/updatemanager"
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
	tests.H(t).StringEql(string(got), `{"default":true,"packageVersion":"Default","buildVersion":""}`)
}

func listen() (net.Listener, error) {
	// Allocate a new port in the ephemeral range and listen on it.
	return net.Listen("tcp", "127.0.0.1:0")
}

func setupTestUIService() *uiservice.UIService {
	cfg, _ := config.Parse([]string{
		"--default-ui-path", "../testdata/uiserv-sandbox/dcos-ui",
		"--versions-root", "../testdata/uiserv-sandbox/ui-versions",
		"--ui-dist-symlink", "../testdata/uiserv-sandbox/dcos-ui-dist",
		"--ui-dist-stage-symlink", "../testdata/uiserv-sandbox/new-dcos-ui-dist",
		"--master-count-file", "../fixtures/single-master",
	})

	um, _ := updatemanager.NewClient(cfg)
	um.Fs = afero.NewOsFs()

	os.MkdirAll(cfg.VersionsRoot(), 0755)
	os.MkdirAll(cfg.DefaultDocRoot(), 0755)
	os.Symlink(cfg.DefaultDocRoot(), cfg.UIDistSymlink())

	return &uiservice.UIService{
		Config:        cfg,
		UpdateManager: um,
		VersionStore:  VersionStoreDouble(),
	}
}

func tearDown(t *testing.T) {
	t.Log("Teardown testdata sandbox")
	os.RemoveAll("./testdata/main-sandbox")
}

type fakeVersionStore struct {
	VersionResult uiservice.UIVersion
	UpdateError   error
}

func VersionStoreDouble() *fakeVersionStore {
	return &fakeVersionStore{
		VersionResult: uiservice.UIVersion("2.24.4"),
	}
}

func (vs *fakeVersionStore) CurrentVersion() (uiservice.UIVersion, error) {
	return vs.VersionResult, nil
}

func (vs *fakeVersionStore) UpdateCurrentVersion(newVersion uiservice.UIVersion) error {
	if vs.UpdateError != nil {
		return vs.UpdateError
	}
	return nil
}

func (vs *fakeVersionStore) WatchForVersionChange(listener uiservice.VersionChangeListener) error {
	return nil
}
