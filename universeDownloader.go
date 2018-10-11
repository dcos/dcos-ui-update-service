package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

// UniverseDownloader handles access to common setup question
type UniverseDownloader struct {
	Client      *http.Client
	UniverseURL string // maybe use url instead of strnig
}

// ListVersionResponse is the parsed result of /package/list-versions requests
type ListVersionResponse struct {
	Results map[string]string
}

// ListVersionRequest is the request body send to /package/list-versions
type ListVersionRequest struct {
	IncludePackageVersions bool
	PackageName            string
}

// TODO: think about credentials and how we use them (forward maybe?)
func (dl *UniverseDownloader) getPackageVersions(packageName string) (*ListVersionResponse, error) {
	listVersionReq := ListVersionRequest{IncludePackageVersions: true, PackageName: "dcos-ui"}
	body := new(bytes.Buffer)
	err := json.NewEncoder(body).Encode(listVersionReq)

	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", dl.UniverseURL+"/package/list-versions", body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("accept", "application/vnd.dcos.package.list-versions-response+json;charset=utf-8;version=v1")
	req.Header.Set("content-type", "application/vnd.dcos.package.list-versions-request+json;charset=utf-8;version=v1")

	resp, err := dl.Client.Do(req)
	if err != nil {
		return nil, err
	}

	var response ListVersionResponse
	defer resp.Body.Close()
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		return nil, err
	}

	return &response, nil
}

func (l *ListVersionResponse) includesTargetVersion(version string) bool {
	return len(l.Results[version]) > 0
}

// LoadVersion downloads the given DC/OS UI version to the target directory.
func (dl *UniverseDownloader) LoadVersion(version string, targetDirectory string) error {
	listVersionResp, err := dl.getPackageVersions("dcos-ui")
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
