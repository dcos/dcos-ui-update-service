package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/pkg/errors"
	"github.com/spf13/afero"
)

// UpdateManager handles access to common setup question
type UpdateManager struct {
	Cosmos      CosmosClient
	Loader      Downloader
	UniverseURL string // maybe use url instead of string
	VersionPath string
	Fs          afero.Fs
}

func (l *ListVersionResponse) includesTargetVersion(version string) bool {
	return len(l.Results[version]) > 0
}

// NewUpdateManager creates a new instance of UpdateManager
func NewUpdateManager(universeURL, versionPath string) UpdateManager {
	// TODO: write better clients
	fs := afero.NewOsFs()

	return UpdateManager{
		Cosmos: CosmosClient{
			Client:      &http.Client{},
			UniverseURL: universeURL,
		},
		Loader: Downloader{
			Client: &http.Client{},
			Fs:     fs,
		},
		UniverseURL: universeURL,
		VersionPath: versionPath,
		Fs:          fs,
	}
}

// LoadVersion downloads the given DC/OS UI version to the target directory.
func (um *UpdateManager) LoadVersion(version string, targetDirectory string) error {
	listVersionResp, listErr := um.Cosmos.listPackageVersions("dcos-ui")
	if listErr != nil {
		return fmt.Errorf("Could not reach the server: %#v", listErr)
	}

	if !listVersionResp.includesTargetVersion(version) {
		return fmt.Errorf("The requested version is not available")
	}

	if _, err := um.Fs.Stat(targetDirectory); os.IsNotExist(err) {
		return fmt.Errorf("%q is no directory", targetDirectory)
	}

	assets, getAssetsErr := um.Cosmos.getPackageAssets("dcos-ui", version)
	if getAssetsErr != nil {
		return errors.Wrap(getAssetsErr, "Could not reach the server")
	}

	uiBundleName := "dcos-ui-bundle"
	uiBundleURI := assets[uiBundleName]

	if len(uiBundleURI) == 0 {
		return fmt.Errorf("Could not find asset with the name %q in %#v", uiBundleName, assets)
	}

	if umErr := um.Loader.downloadAndUnpack(uiBundleURI, targetDirectory); umErr != nil {
		return errors.Wrap(umErr, fmt.Sprintf("Could not load %q", uiBundleURI))
	}

	return nil
}

// GetCurrentVersion retrieves the current version of the package
func (um *UpdateManager) GetCurrentVersion() (string, error) {
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

	if len(dirs) != 1 {
		return "", fmt.Errorf("Detected more than one directory: %#v", dirs)
	}

	// by looking at the dirs for now
	return dirs[0], nil
}

// UpdateToVersion updates the ui to the given version
func (um *UpdateManager) UpdateToVersion(version string) error {
	// Find out which version we currently have
	currentVersion, err := um.GetCurrentVersion()

	if err != nil {
		return errors.Wrap(err, "Could not get current version")
	}

	if currentVersion == version {
		return fmt.Errorf("Trying to update to the same version")
	}

	targetDir := um.VersionPath + "/" + version
	// Create directory for next version
	err = um.Fs.MkdirAll(targetDir, 0755)
	if err != nil {
		return errors.Wrap(err, "Could not create directory")
	}

	// Update to next version
	err = um.LoadVersion(version, targetDir)
	if err != nil {
		return errors.Wrap(err, "Could not load new version")
	}

	// Removes old version directory
	err = um.Fs.RemoveAll(um.VersionPath + "/" + currentVersion)
	if err != nil {
		return errors.Wrap(err, "Could not remove old version")
	}

	return nil
}
