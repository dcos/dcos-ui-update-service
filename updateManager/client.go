package updateManager

import (
	"fmt"
	"net/url"
	"path"
	"sync"

	"github.com/dcos/dcos-ui-update-service/config"
	"github.com/dcos/dcos-ui-update-service/cosmos"
	"github.com/dcos/dcos-ui-update-service/downloader"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"
)

var (
	// ErrVersionsPathDoesNotExist occurs if the configured VersionPath doesn't exist
	ErrVersionsPathDoesNotExist = errors.New("Versions path does not exist")
	// ErrMultipleVersionFound occurs if there is more than one version found when getting current version
	ErrMultipleVersionFound = errors.New("Multiple versions found on disk, cannot determine a single current version")
	// ErrCouldNotGetCurrentVersion occurs if we encounter an error determining the current ui version
	ErrCouldNotGetCurrentVersion = errors.New("Could not get current version")
	// ErrCouldNotCreateNewVersionDirectory occurs if we fail to create the directory to hold the new version
	ErrCouldNotCreateNewVersionDirectory = errors.New("Could not create new version directory")
	// ErrCosmosRequestFailure occurs if our API requires to Cosmos fail for any reason
	ErrCosmosRequestFailure = errors.New("Retrieving data from Cosmos failed")
	// ErrRequestedVersionNotFound occurs if the version requests is not available in Cosmos
	ErrRequestedVersionNotFound = errors.New("The requested version is not available")
	// ErrUIPackageAssetNotFound occurs if we cannot find the dcos-ui-bundle asset URI in the version package details
	ErrUIPackageAssetNotFound = errors.New("Could not find dcos-ui-bundle asset in package details")
	// ErrUIPackageAssetBadURI occurs if the dcos-ui-bundle asset URI cannot be parsed
	ErrUIPackageAssetBadURI = errors.New("Failed to parse dcos-ui-bundle asset URI")
	// ErrRemovingOldVersion occurs if removing the old version fails while updating
	ErrRemovingOldVersion = errors.New("Failed to remove the old version after update")
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

type UpdateManager interface {
	UpdateToVersion(string, func(string) error) error
	ResetVersion() error
	CurrentVersion() (string, error)
	PathToCurrentVersion() (string, error)
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
func (um *Client) loadVersion(version string, targetDirectory string) error {
	listVersionResp, listErr := um.Cosmos.ListPackageVersions("dcos-ui")
	if listErr != nil {
		logrus.WithError(listErr).Error("Cosmos ListPackageVersions request failed")
		return ErrCosmosRequestFailure
	}
	logrus.WithFields(logrus.Fields{"versions": listVersionResp}).Info("Loading Version: Retrieved package versions from cosmos")

	if !listVersionResp.IncludesTargetVersion(version) {
		return ErrRequestedVersionNotFound
	}

	assets, getAssetsErr := um.Cosmos.GetPackageAssets("dcos-ui", version)
	if getAssetsErr != nil {
		logrus.WithError(listErr).Error("Cosmos GetPackageAssets request failed")
		return ErrCosmosRequestFailure
	}
	logrus.Info("Loading Version: Retrieved package assets from cosmos")

	uiBundleName := cosmos.PackageAssetNameString("dcos-ui-bundle")
	uiBundleURI, found := assets[uiBundleName]
	if !found {
		return ErrUIPackageAssetNotFound
	}
	logrus.WithFields(logrus.Fields{
		"asset": uiBundleURI,
		"name":  uiBundleName,
	}).Info("Loading Version: Found asset by name")

	uiBundleURL, err := url.Parse(string(uiBundleURI))
	if err != nil {
		logrus.WithError(err).Error("Failed to parse dcos-ui-bundle asset URI")
		return ErrUIPackageAssetBadURI
	}
	logrus.WithFields(logrus.Fields{"url": uiBundleURL}).Info("Loading Version: Bundle URI parsed to a URL")

	if umErr := um.Loader.DownloadAndUnpack(uiBundleURL, targetDirectory); umErr != nil {
		logrus.WithError(umErr).Errorf("Download and unpack failed for %s", uiBundleURI)
		return umErr
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
		return "", ErrVersionsPathDoesNotExist
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
		logrus.Errorf("Detected more than one directory: %#v", dirs)
		return "", ErrMultipleVersionFound
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
		logrus.WithError(err).Error("Could not get current version for update")
		return ErrCouldNotGetCurrentVersion
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
		logrus.WithError(err).Error("Failed to create new version directory for update")
		return ErrCouldNotCreateNewVersionDirectory
	}
	logrus.WithFields(logrus.Fields{"directory": targetDir}).Info("Created directory for next version")

	// Update to next version
	err = um.loadVersion(version, targetDir)
	if err != nil {
		// Install failed delete the targetDir
		um.Fs.RemoveAll(targetDir)
		logrus.Error("Update to new version failed, deleted target directory")
		return err
	}
	err = updateCompleteCallback(path.Join(targetDir, "dist"))
	if err != nil {
		// Swap to new version failed, abort update
		um.Fs.RemoveAll(targetDir)
		logrus.WithError(err).Error("Update complete callback failed. Update aborted")
		return err
	}

	if len(currentVersion) > 0 {
		// Removes old version directory
		err = um.Fs.RemoveAll(path.Join(um.VersionPath, currentVersion))
		if err != nil {
			logrus.WithError(err).Error("Could not remove old version after update")
			return ErrRemovingOldVersion
		}
		logrus.Info("Removed old version directory")
	}

	return nil
}

func (um *Client) ResetVersion() error {
	currentVersion, err := um.CurrentVersion()

	if err != nil {
		logrus.WithError(err).Error("Could not get current version for reset")
		return ErrCouldNotGetCurrentVersion
	}

	if len(currentVersion) == 0 {
		logrus.Info("No current version to reset")
		return nil
	}

	err = um.Fs.RemoveAll(path.Join(um.VersionPath, currentVersion))
	if err != nil {
		logrus.WithError(err).Error("Could not remove old version after reset")
		return ErrRemovingOldVersion
	}
	logrus.Info("Removed current version")
	return nil
}
