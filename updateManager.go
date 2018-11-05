package main

import (
	"fmt"
	"net/url"
	"os"
	"path"

	"github.com/dcos/dcos-ui-update-service/config"
	"github.com/dcos/dcos-ui-update-service/http"
	"github.com/pkg/errors"
	"github.com/spf13/afero"
)

// UpdateManager handles access to common setup question
type UpdateManager struct {
	Cosmos      *CosmosClient
	Loader      Downloader
	UniverseURL *url.URL
	VersionPath string
	Fs          afero.Fs
	client      *http.Client
}

func (l *ListVersionResponse) includesTargetVersion(version string) bool {
	resultVersion := VersionNumberString(version)
	return len(l.Results[resultVersion]) > 0
}

// NewUpdateManager creates a new instance of UpdateManager
func NewUpdateManager(cfg *config.Config, httpClient *http.Client) (*UpdateManager, error) {
	universeURL, err := url.Parse(cfg.UniverseURL)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse configured Universe URL")
	}
	fs := afero.NewOsFs()
	cosmos, err := NewCosmosClient(httpClient, cfg.UniverseURL)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create cosmos client with universe url provided")
	}

	return &UpdateManager{
		Cosmos: cosmos,
		Loader: Downloader{
			client: httpClient,
			Fs:     fs,
		},
		UniverseURL: universeURL,
		VersionPath: cfg.VersionsRoot,
		Fs:          fs,
		client:      httpClient,
	}, nil
}

// LoadVersion downloads the given DC/OS UI version to the target directory.
func (um *UpdateManager) LoadVersion(version string, targetDirectory string) error {
	listVersionResp, listErr := um.Cosmos.ListPackageVersions("dcos-ui")
	if listErr != nil {
		return fmt.Errorf("Could not reach the server: %#v", listErr)
	}

	if !listVersionResp.includesTargetVersion(version) {
		return fmt.Errorf("The requested version is not available")
	}

	if _, err := um.Fs.Stat(targetDirectory); os.IsNotExist(err) {
		return fmt.Errorf("%q is no directory", targetDirectory)
	}

	assets, getAssetsErr := um.Cosmos.GetPackageAssets("dcos-ui", version)
	if getAssetsErr != nil {
		return errors.Wrap(getAssetsErr, "Could not reach the server")
	}

	uiBundleName := PackageAssetNameString("dcos-ui-bundle")
	uiBundleURI, found := assets[uiBundleName]
	if !found {
		return fmt.Errorf("Could not find asset with the name %s", uiBundleName)
	}
	uiBundleURL, err := url.Parse(string(uiBundleURI))
	if err != nil {
		return errors.Wrap(err, "ui bundle URI could not be parsed to a URL")
	}

	if umErr := um.Loader.downloadAndUnpack(uiBundleURL, targetDirectory); umErr != nil {
		return errors.Wrap(umErr, fmt.Sprintf("Could not load %q", uiBundleURI))
	}

	return nil
}

// CurrentVersion retrieves the current version of the package
func (um *UpdateManager) CurrentVersion() (string, error) {
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
		return "", nil
	}

	if len(dirs) != 1 {
		return "", fmt.Errorf("Detected more than one directory: %#v", dirs)
	}

	// by looking at the dirs for now
	return dirs[0], nil
}

// PathToCurrentVersion return the filesystem path to the current UI version
// or returns an error is the current version cannot be determined
func (um *UpdateManager) PathToCurrentVersion() (string, error) {
	currentVersion, err := um.CurrentVersion()
	if err != nil {
		return "", err
	}
	if len(currentVersion) == 0 {
		return "", fmt.Errorf("there is no current version available")
	}

	versionPath := path.Join(um.VersionPath, currentVersion)
	return versionPath, nil
}

// UpdateToVersion updates the ui to the given version
func (um *UpdateManager) UpdateToVersion(version string, fileServer UIFileServer) error {
	// Find out which version we currently have
	currentVersion, err := um.CurrentVersion()

	if err != nil {
		return errors.Wrap(err, "Could not get current version")
	}

	if len(currentVersion) > 0 && currentVersion == version {
		// noop if we are currently on the requested version
		return nil
	}

	targetDir := path.Join(um.VersionPath, version)
	// Create directory for next version
	err = um.Fs.MkdirAll(targetDir, 0755)
	if err != nil {
		return errors.Wrap(err, "Could not create directory")
	}

	// Update to next version
	err = um.LoadVersion(version, targetDir)
	if err != nil {
		// Install failed delete the targetDir
		um.Fs.RemoveAll(targetDir)
		return errors.Wrap(err, "Could not load new version")
	}
	err = fileServer.UpdateDocumentRoot(targetDir)
	if err != nil {
		// Swap to new version failed, abort update
		um.Fs.RemoveAll(targetDir)
		return errors.Wrap(err, "Could not load new version")
	}

	if len(currentVersion) > 0 {
		// Removes old version directory
		err = um.Fs.RemoveAll(path.Join(um.VersionPath, currentVersion))
		if err != nil {
			return errors.Wrap(err, "Could not remove old version")
		}
	}

	return nil
}

func (um *UpdateManager) ResetVersion() error {
	currentVersion, err := um.CurrentVersion()

	if err != nil {
		return errors.Wrap(err, "Could not get current version")
	}

	if len(currentVersion) == 0 {
		return nil
	}

	err = um.Fs.RemoveAll(path.Join(um.VersionPath, currentVersion))
	if err != nil {
		return errors.Wrap(err, "Could not remove current version")
	}
	return nil
}
