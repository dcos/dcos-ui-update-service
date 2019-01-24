package uiService

import (
	"os"
	"path"
	"testing"

	"github.com/dcos/dcos-ui-update-service/config"
	"github.com/dcos/dcos-ui-update-service/dcos"
	"github.com/dcos/dcos-ui-update-service/tests"
	"github.com/dcos/dcos-ui-update-service/updateManager"
	"github.com/spf13/afero"
)

func tearDown(t *testing.T) {
	t.Log("Teardown testdata sandbox")
	os.RemoveAll("../testdata/uiserv-sandbox")
}

func setupTestUIService() *UIService {
	cfg := config.NewDefaultConfig()
	cfg.DefaultDocRoot = "../testdata/uiserv-sandbox/dcos-ui"
	cfg.VersionsRoot = "../testdata/uiserv-sandbox/ui-versions"
	cfg.UIDistSymlink = "../testdata/uiserv-sandbox/dcos-ui-dist"
	cfg.UIDistStageSymlink = "../testdata/uiserv-sandbox/new-dcos-ui-dist"
	cfg.MasterCountFile = "../fixtures/single-master"

	um, _ := updateManager.NewClient(cfg)
	um.Fs = afero.NewOsFs()
	os.MkdirAll(cfg.VersionsRoot, 0755)
	os.MkdirAll(cfg.DefaultDocRoot, 0755)
	os.Symlink(cfg.DefaultDocRoot, cfg.UIDistSymlink)

	return &UIService{
		Config:        cfg,
		UpdateManager: um,
		MasterCounter: dcos.DCOS{
			MasterCountLocation: cfg.MasterCountFile,
		},
		VersionStore: VersionStoreDouble(),
	}
}

func setupUIServiceWithVersion() *UIService {
	cfg := config.NewDefaultConfig()
	cfg.DefaultDocRoot = "../testdata/uiserv-sandbox/dcos-ui"
	cfg.VersionsRoot = "../testdata/uiserv-sandbox/ui-versions"
	cfg.UIDistSymlink = "../testdata/uiserv-sandbox/dcos-ui-dist"
	cfg.UIDistStageSymlink = "../testdata/uiserv-sandbox/new-dcos-ui-dist"
	cfg.MasterCountFile = "../fixtures/single-master"

	um, _ := updateManager.NewClient(cfg)
	um.Fs = afero.NewOsFs()
	versionPath := path.Join(path.Join(cfg.VersionsRoot, "2.24.4"), "dist")
	os.MkdirAll(cfg.VersionsRoot, 0755)
	os.MkdirAll(cfg.DefaultDocRoot, 0755)
	os.MkdirAll(versionPath, 0755)
	os.Symlink(versionPath, cfg.UIDistSymlink)

	return &UIService{
		Config:        cfg,
		UpdateManager: um,
		MasterCounter: dcos.DCOS{
			MasterCountLocation: cfg.MasterCountFile,
		},
		VersionStore: VersionStoreDouble(),
	}
}

func TestVersionChange(t *testing.T) {
	t.Run("Reset if new version is empty", func(t *testing.T) {
		var resetCalled, updateCalled bool
		defer tearDown(t)
		service := setupTestUIService()

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
		defer tearDown(t)
		service := setupUIServiceWithVersion()
		newVersionPath := path.Join(path.Join(service.Config.VersionsRoot, "2.24.5"), "dist")

		um := UpdateManagerDouble()
		um.VersionResult = "2.24.4"
		um.UpdateNewVersionPath = newVersionPath
		um.ResetCall = func() error {
			resetCalled = true
			return nil
		}
		um.UpdateCall = func(newVer string) {
			os.MkdirAll(um.UpdateNewVersionPath, 0755)
			updateCalled = true
		}
		service.UpdateManager = um

		handleVersionChange(service, "2.24.5")

		tests.H(t).BoolEql(resetCalled, false)
		tests.H(t).BoolEql(updateCalled, true)
	})

	t.Run("do nothing if version matches current", func(t *testing.T) {
		var resetCalled, updateCalled bool
		defer tearDown(t)
		service := setupTestUIService()

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

func (um *fakeUpdateManager) RemoveVersion(version string) error {
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
	VersionResult UIVersion
	UpdateError   error
}

func VersionStoreDouble() *fakeVersionStore {
	return &fakeVersionStore{
		VersionResult: UIVersion("2.24.4"),
	}
}

func (vs *fakeVersionStore) CurrentVersion() (UIVersion, error) {
	return vs.VersionResult, nil
}

func (vs *fakeVersionStore) UpdateCurrentVersion(newVersion UIVersion) error {
	if vs.UpdateError != nil {
		return vs.UpdateError
	}
	return nil
}

func (vs *fakeVersionStore) WatchForVersionChange(listener VersionChangeListener) error {
	return nil
}
