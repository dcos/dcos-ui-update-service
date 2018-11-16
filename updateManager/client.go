package updateManager

import (
	"fmt"
	"net/url"
	"os"
	"path"
	"sync"

	"github.com/dcos/dcos-ui-update-service/config"
	"github.com/dcos/dcos-ui-update-service/cosmos"
	"github.com/dcos/dcos-ui-update-service/downloader"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"
)

// Client handles access to common setup question
type Client struct {
	Cosmos      *cosmos.Client
	Loader      *downloader.Client
	UniverseURL *url.URL
	VersionPath string
	Fs          afero.Fs
	sync.Mutex
}

// NewClient creates a new instance of Client
func NewClient(cfg *config.Config) (*Client, error) {
	universeURL, err := url.Parse(cfg.UniverseURL)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse configured Universe URL")
	}
	fs := afero.NewOsFs()

	return &Client{
		Cosmos:      cosmos.NewClient(universeURL),
		Loader:      downloader.New(fs),
		UniverseURL: universeURL,
		VersionPath: cfg.VersionsRoot,
		Fs:          fs,
	}, nil
}

// LoadVersion downloads the given DC/OS UI version to the target directory.
func (um *Client) LoadVersion(version string, targetDirectory string) error {
	listVersionResp, listErr := um.Cosmos.ListPackageVersions("dcos-ui")
	if listErr != nil {
		return fmt.Errorf("Could not reach the server: %#v", listErr)
	}
	logrus.WithFields(logrus.Fields{"versions": listVersionResp}).Info("Loading Version: Retrieved package versions from cosmos")

	if !listVersionResp.IncludesTargetVersion(version) {
		return fmt.Errorf("The requested version is not available")
	}

	if _, err := um.Fs.Stat(targetDirectory); os.IsNotExist(err) {
		return fmt.Errorf("%q is no directory", targetDirectory)
	}

	assets, getAssetsErr := um.Cosmos.GetPackageAssets("dcos-ui", version)
	if getAssetsErr != nil {
		return errors.Wrap(getAssetsErr, "Could not reach the server")
	}
	logrus.Info("Loading Version: Retrieved package assets from cosmos")

	uiBundleName := cosmos.PackageAssetNameString("dcos-ui-bundle")
	uiBundleURI, found := assets[uiBundleName]
	if !found {
		return fmt.Errorf("Could not find asset with the name %s", uiBundleName)
	}
	logrus.WithFields(logrus.Fields{
		"asset": uiBundleURI,
		"name":  uiBundleName,
	}).Info("Loading Version: Found asset by name")

	uiBundleURL, err := url.Parse(string(uiBundleURI))
	if err != nil {
		return errors.Wrap(err, "ui bundle URI could not be parsed to a URL")
	}
	logrus.WithFields(logrus.Fields{"url": uiBundleURL}).Info("Loading Version: Bundle URI parsed to a URL")

	if umErr := um.Loader.DownloadAndUnpack(uiBundleURL, targetDirectory); umErr != nil {
		return errors.Wrap(umErr, fmt.Sprintf("Could not load %q", uiBundleURI))
	}
	logrus.Info("Loading Version: Completed download and unpack")

	return nil
}

// CurrentVersion retrieves the current version of the package
func (um *Client) CurrentVersion() (string, error) {
	um.Lock()
	defer um.Unlock()

	exists, err := afero.DirExists(um.Fs, um.VersionPath)

	if !exists || err != nil {
		return "", fmt.Errorf("%q does not exist on the fs", um.VersionPath)
	}

	files, err := afero.ReadDir(um.Fs, um.VersionPath)

	if err != nil {
		return "", fmt.Errorf("could not read files from verion path")
	}

	var dirs []string

	for _, file := range files {
		if file.IsDir() {
			dirs = append(dirs, file.Name())
		}
	}

	if len(dirs) == 0 {
		logrus.Info("Retrieving current version: No version directory found")
		return "", nil
	}

	if len(dirs) != 1 {
		return "", fmt.Errorf("Detected more than one directory: %#v", dirs)
	}

	logrus.WithFields(logrus.Fields{"currentVersion": dirs[0]}).Info("Found current version")
	// by looking at the dirs for now
	return dirs[0], nil
}

// PathToCurrentVersion return the filesystem path to the current UI version
// or returns an error is the current version cannot be determined
func (um *Client) PathToCurrentVersion() (string, error) {
	currentVersion, err := um.CurrentVersion()
	logrus.WithFields(logrus.Fields{"currentVersion": currentVersion}).Info("Retrieving path to current version: Found current version")
	if err != nil {
		return "", err
	}
	if len(currentVersion) == 0 {
		return "", fmt.Errorf("there is no current version available")
	}

	versionPath := path.Join(um.VersionPath, currentVersion, "dist")
	logrus.WithFields(logrus.Fields{"versionPath": currentVersion}).Info("Found path to current version")
	return versionPath, nil
}

// UpdateToVersion updates the ui to the given version
func (um *Client) UpdateToVersion(version string, updateCompleteCallback func(string) error) error {
	// Find out which version we currently have
	currentVersion, err := um.CurrentVersion()

	if err != nil {
		return errors.Wrap(err, "Could not get current version")
	}

	if len(currentVersion) > 0 && currentVersion == version {
		// noop if we are currently on the requested version
		logrus.Info("Currently on requested version")
		return nil
	}
	um.Lock()
	defer um.Unlock()

	targetDir := path.Join(um.VersionPath, version)
	// Create directory for next version
	err = um.Fs.MkdirAll(targetDir, 0755)
	if err != nil {
		return errors.Wrap(err, "Could not create directory")
	}
	logrus.WithFields(logrus.Fields{"directory": targetDir}).Info("Created directory for next version")

	// Update to next version
	err = um.LoadVersion(version, targetDir)
	if err != nil {
		// Install failed delete the targetDir
		um.Fs.RemoveAll(targetDir)
		logrus.Error("Update to new version failed, deleted target directory")
		return errors.Wrap(err, "Could not load new version")
	}
	err = updateCompleteCallback(targetDir)
	if err != nil {
		// Swap to new version failed, abort update
		um.Fs.RemoveAll(targetDir)
		logrus.Error("Switch to new version failed, abort update")
		return errors.Wrap(err, "Could not load new version")
	}

	if len(currentVersion) > 0 {
		// Removes old version directory
		err = um.Fs.RemoveAll(path.Join(um.VersionPath, currentVersion))
		if err != nil {
			return errors.Wrap(err, "Could not remove old version")
		}
		logrus.Error("Removed old version directory")
	}

	return nil
}

func (um *Client) ResetVersion() error {
	currentVersion, err := um.CurrentVersion()

	if err != nil {
		return errors.Wrap(err, "Could not get current version")
	}

	if len(currentVersion) == 0 {
		logrus.Info("No current version to reset")
		return nil
	}

	err = um.Fs.RemoveAll(path.Join(um.VersionPath, currentVersion))
	if err != nil {
		return errors.Wrap(err, "Could not remove current version")
	}
	logrus.Info("Removed current version")
	return nil
}
