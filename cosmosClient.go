package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/tidwall/gjson"
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

// PackageDetailRequest is the request body sent to /package/describe
type PackageDetailRequest struct {
	PackageName    string
	PackageVersion string
}

// TODO: think about if we can use the roundtripper api to set the headers in an easier way
// TODO: think about credentials and how we use them (forward maybe?)
func (c *CosmosClient) listPackageVersions(packageName string) (*ListVersionResponse, error) {
	listVersionReq := ListVersionRequest{IncludePackageVersions: true, PackageName: "dcos-ui"}
	body := new(bytes.Buffer)
	err := json.NewEncoder(body).Encode(listVersionReq)

	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", c.UniverseURL+"/package/list-versions", body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("accept", "application/vnd.dcos.package.list-versions-response+json;charset=utf-8;version=v1")
	req.Header.Set("content-type", "application/vnd.dcos.package.list-versions-request+json;charset=utf-8;version=v1")

	resp, err := c.Client.Do(req)
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

func (c *CosmosClient) getPackageAssets(packageName string, packageVersion string) (map[string]string, error) {
	packageDetailReq := PackageDetailRequest{PackageName: packageName, PackageVersion: packageVersion}
	body := new(bytes.Buffer)
	err := json.NewEncoder(body).Encode(packageDetailReq)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", c.UniverseURL+"/package/describe", body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("accept", "application/vnd.dcos.package.describe-response+json;charset=utf-8;version=v3")
	req.Header.Set("content-type", "application/vnd.dcos.package.describe-request+json;charset=UTF-8;version=v1")
	resp, err := c.Client.Do(req)
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
	assets := gjson.Get(json, "package.resource.assets.uris").Value()
	if assets == nil {
		return nil, fmt.Errorf("Could not get asset uris from JSON: %#v", assets)
	}
	castedAssets := assets.(map[string]interface{})

	stringifyResult := make(map[string]string)

	for key, value := range castedAssets {
		stringifyResult[key] = fmt.Sprintf("%v", value)
	}

	return stringifyResult, nil
}