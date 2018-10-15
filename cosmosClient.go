package main

import (
	"bytes"
	"encoding/json"
	"net/http"
)

// CosmosClient abstracts common API calls against Cosmos
type CosmosClient struct {
	Client      *http.Client
	UniverseURL string // maybe use url instead of string
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

// TODO: think about if we can use the roundtripper api to set the headers in an easier way
// TODO: think about credentials and how we use them (forward maybe?)
func (dl *CosmosClient) listPackageVersions(packageName string) (*ListVersionResponse, error) {
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
