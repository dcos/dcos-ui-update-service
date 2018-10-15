package main

import (
	"fmt"
	"os"
)

// UniverseDownloader handles access to common setup question
type UniverseDownloader struct {
	Cosmos      CosmosClient
	UniverseURL string // maybe use url instead of string
}

func (l *ListVersionResponse) includesTargetVersion(version string) bool {
	return len(l.Results[version]) > 0
}

func (dl *UniverseDownloader) getAssetsForPackage(packageName string, version string) ([]string, error) {
	return []string{}, nil
}

// LoadVersion downloads the given DC/OS UI version to the target directory.
func (dl *UniverseDownloader) LoadVersion(version string, targetDirectory string) error {
	listVersionResp, err := dl.Cosmos.listPackageVersions("dcos-ui")
	if err != nil {
		return fmt.Errorf("Could not reach the server: %#v", err)
	}

	if !listVersionResp.includesTargetVersion(version) {
		return fmt.Errorf("The requested version is not available")
	}

	if _, err := os.Stat(targetDirectory); os.IsNotExist(err) {
		return fmt.Errorf("%q is no directory", targetDirectory)
	}

	return nil
}
