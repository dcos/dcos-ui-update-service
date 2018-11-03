package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/dcos/dcos-ui-update-service/client"
	"github.com/pkg/errors"
	"github.com/tidwall/gjson"
)

// CosmosClient abstracts common API calls against Cosmos
type CosmosClient struct {
	client      *client.HTTP
	UniverseURL *url.URL
}

// ListVersionResponse is the parsed result of /package/list-versions requests
type ListVersionResponse struct {
	Results map[string]string `json:"results"`
}

// ListVersionRequest is the request body send to /package/list-versions
type ListVersionRequest struct {
	IncludePackageVersions bool   `json:"includePackageVersions"`
	PackageName            string `json:"packageName"`
}

// PackageDetailRequest is the request body sent to /package/describe
type PackageDetailRequest struct {
	PackageName    string `json:"packageName"`
	PackageVersion string `json:"packageVersion"`
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

	req, err := http.NewRequest("POST", c.UniverseURL.String()+"/package/list-versions", body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("accept", "application/vnd.dcos.package.list-versions-response+json;charset=utf-8;version=v1")
	req.Header.Set("content-type", "application/vnd.dcos.package.list-versions-request+json;charset=utf-8;version=v1")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to query cosmos")
	}
	var response ListVersionResponse
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

	req, err := http.NewRequest("POST", c.UniverseURL.String()+"/package/describe", body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("accept", "application/vnd.dcos.package.describe-response+json;charset=utf-8;version=v3")
	req.Header.Set("content-type", "application/vnd.dcos.package.describe-request+json;charset=UTF-8;version=v1")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to query cosmos")
	}
	defer resp.Body.Close()
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("could not read response body: %s", err)
	}
	json := string(respBody)
	if !gjson.Valid(json) {
		return nil, fmt.Errorf("could not parse JSON")
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

func NewCosmosClient(client *client.HTTP, universeURL string) (*CosmosClient, error) {
	parsedURL, err := url.Parse(universeURL)
	if err != nil {
		return nil, errors.Wrap(err, "error parsing universe URL for cosmos client")
	}

	return &CosmosClient{
		client:      client,
		UniverseURL: parsedURL,
	}, nil
}
