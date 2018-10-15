package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/tidwall/gjson"
)

//go:generate mockgen -destination=mocks/mock_cosmos.go -package=mocks -source ./cosmosClient.go CosmosAPI

// CosmosAPI exposes the interface to interact with cosmos
type CosmosAPI interface {
	listPackageVersions(string) (*ListVersionResponse, error)
	getPackageAssets(string, string) (map[string]string, error)
}

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

func (dl *CosmosClient) getPackageAssets(packageName string, packageVersion string) (map[string]string, error) {
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

	defer resp.Body.Close()
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Could not load response body")
	}

	json := string(bodyBytes)

	if !gjson.Valid(json) {
		return nil, fmt.Errorf("Could not parse JSON")
	}

	assets := gjson.Get(json, "package.resource.assets.uris")

	if assets.Exists() {
		return nil, fmt.Errorf("Could not get asset uris from JSON")
	}

	return assets.Value().(map[string]string), nil
}
