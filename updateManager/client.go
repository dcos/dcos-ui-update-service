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

var (
	// ErrUIDistSymlinkNotFound occurs if the Configured UIDistSymlink doesn't exists or can't be accessed
	ErrUIDistSymlinkNotFound = errors.New("Cannot read UI-dist symlink")
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
	// ErrRemovingVersion occurs if removing the version fails
	ErrRemovingVersion = errors.New("Failed to remove the version")
)

// Client handles access to common setup question
type Client struct {
	Cosmos      *cosmos.Client
	Loader      *downloader.Client
	UniverseURL *url.URL
	Config      *config.Config
	Fs          afero.Fs
	sync.Mutex
}

type UpdateManager interface {
	UpdateToVersion(string, func(string) error) error
	RemoveVersion(string) error
	CurrentVersion() (string, error)
	PathToCurrentVersion() (string, error)
}

// NewClient creates a new instance of Client
func NewClient(cfg *config.Config) (*Client, error) {
	universeURL, err := url.Parse(cfg.UniverseURL())
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse configured Universe URL")
	}
	fs := afero.NewOsFs()

	return &Client{
		Cosmos:      cosmos.NewClient(universeURL),
		Loader:      downloader.New(fs),
		UniverseURL: universeURL,
		Config:      cfg,
		Fs:          fs,
	}, nil
}

// LoadVersion downloads the given DC/OS UI version to the target directory.
func (um *Client) loadVersion(version string, targetDirectory string) error {
	pkgName := um.Config.PackageName()
	listVersionResp, listErr := um.Cosmos.ListPackageVersions(pkgName)
	if listErr != nil {
		logrus.WithError(listErr).Error("Cosmos ListPackageVersions request failed")
		return ErrCosmosRequestFailure
	}
	logrus.WithFields(logrus.Fields{"versions": listVersionResp}).Info("Loading Version: Retrieved package versions from cosmos")

	if !listVersionResp.IncludesTargetVersion(version) {
		return ErrRequestedVersionNotFound
	}

	assets, getAssetsErr := um.Cosmos.GetPackageAssets(pkgName, version)
	if getAssetsErr != nil {
		logrus.WithError(listErr).Error("Cosmos GetPackageAssets request failed")
		return ErrCosmosRequestFailure
	}
	logrus.Info("Loading Version: Retrieved package assets from cosmos")

	uiBundleName := cosmos.PackageAssetNameString(pkgName + "-bundle")
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

// CurrentVersion retrieves the current version being served
func (um *Client) CurrentVersion() (string, error) {
	// Locking here so we don't try to read the version while updating
	um.Lock()
	defer um.Unlock()

	servedVersionPath, err := os.Readlink(um.Config.UIDistSymlink())
	if err != nil {
		return "", ErrUIDistSymlinkNotFound
	}

	if servedVersionPath == um.Config.DefaultDocRoot() {
		return "", nil
	}

	versionPath, distDir := path.Split(servedVersionPath)
	if distDir != "dist" {
		return "", fmt.Errorf("Expected served version directory to be `dist` but got %s", distDir)
	}

	currentVersion := path.Base(versionPath)

	logrus.WithFields(logrus.Fields{"currentVersion": currentVersion}).Info("Found current version")
	// by looking at the dirs for now
	return currentVersion, nil
}

// PathToCurrentVersion return the filesystem path to the current UI version
// or returns an error is the current version cannot be determined
func (um *Client) PathToCurrentVersion() (string, error) {
	servedVersionPath, err := os.Readlink(um.Config.UIDistSymlink())
	if err != nil {
		return "", ErrUIDistSymlinkNotFound
	}
	return servedVersionPath, nil
}

// UpdateToVersion updates the ui to the given version
func (um *Client) UpdateToVersion(version string, updateCompleteCallback func(string) error) error {
	// Find out which version we currently have
	currentVersion, cvErr := um.CurrentVersion()

	if cvErr != nil {
		logrus.WithError(cvErr).Error("Could not get current version for update")
		return ErrCouldNotGetCurrentVersion
	}

	if len(currentVersion) > 0 && currentVersion == version {
		// noop if we are currently on the requested version
		logrus.Info("Currently on requested version")
		return nil
	}
	um.Lock()
	defer um.Unlock()

	if exists, err := afero.DirExists(um.Fs, um.Config.VersionsRoot()); err != nil || !exists {
		if err != nil {
			logrus.WithError(err).Error("DirExists check for VersionsRoot failed")
		} else {
			logrus.Error("DirExists check for VersionsRoot failed")
		}

		return ErrVersionsPathDoesNotExist
	}

	targetDir := path.Join(um.Config.VersionsRoot(), version)
	// Create directory for next version
	err := um.Fs.MkdirAll(targetDir, 0755)
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
		// Remove the old version
		return um.RemoveVersion(currentVersion)
	}

	return nil
}

func (um *Client) RemoveVersion(version string) error {
	versionPath := path.Join(um.Config.VersionsRoot(), version)
	if exists, err := afero.DirExists(um.Fs, versionPath); err != nil || !exists {
		if err != nil {
			logrus.WithError(err).Error("RemoveVersion failed, version path check failed.")
		} else {
			logrus.Error("RemoveVersion failed, version path check failed.")
		}
		return ErrRequestedVersionNotFound
	}

	err := um.Fs.RemoveAll(versionPath)
	if err != nil {
		logrus.WithError(err).Error("Could not remove version.")
		return ErrRemovingVersion
	}
	logrus.Infof("Removed version v%s", version)
	return nil
}
