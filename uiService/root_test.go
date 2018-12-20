package uiService

import (
	"testing"

	"github.com/dcos/dcos-ui-update-service/config"
	"github.com/dcos/dcos-ui-update-service/dcos"
	"github.com/dcos/dcos-ui-update-service/fileHandler"
	"github.com/dcos/dcos-ui-update-service/tests"
	"github.com/dcos/dcos-ui-update-service/updateManager"
	"github.com/spf13/afero"
)

func setupTestUIService() *UIService {
	cfg := config.NewDefaultConfig()
	cfg.DefaultDocRoot = "../public"
	cfg.VersionsRoot = "/ui-versions"
	cfg.MasterCountFile = "../fixtures/single-master"

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
		VersionStore: VersionStoreDouble(),
	}
}

func setupUIServiceWithVersion() *UIService {
	cfg := config.NewDefaultConfig()
	cfg.DefaultDocRoot = "../public"
	cfg.VersionsRoot = "/ui-versions"
	cfg.MasterCountFile = "../fixtures/single-master"

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
		VersionStore: VersionStoreDouble(),
	}
}

func setupUIServiceWithMemoryFs() (*UIService, afero.Fs) {
	cfg := config.NewDefaultConfig()
	cfg.DefaultDocRoot = "/usr/public"
	cfg.VersionsRoot = "/ui-versions"
	cfg.MasterCountFile = "../fixtures/single-master"

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
		VersionStore: VersionStoreDouble(),
	}, fs
}

func TestSetupUIHandler(t *testing.T) {
	t.Run("sets DefaultDocRoot as document root if no current version", func(t *testing.T) {
		cfg := config.NewDefaultConfig()
		cfg.DefaultDocRoot = "../public"
		cfg.VersionsRoot = "/ui-versions"
		cfg.MasterCountFile = "../fixtures/single-master"

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
		cfg.DefaultDocRoot = "../public"
		cfg.VersionsRoot = "/ui-versions"
		cfg.MasterCountFile = "../fixtures/single-master"

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
