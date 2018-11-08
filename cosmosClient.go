package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"

	our_http "github.com/dcos/dcos-ui-update-service/http"
	"github.com/pkg/errors"
)

// CosmosClient abstracts common API calls against Cosmos
type CosmosClient struct {
	httpClient  *our_http.Client
	UniverseURL *url.URL
}

type VersionNumberString string
type CosmosPackageNumberRevision string

// ListVersionResponse is the parsed result of /package/list-versions requests
type ListVersionResponse struct {
	Results map[VersionNumberString]CosmosPackageNumberRevision `json:"results"`
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

type PackageAssetNameString string
type PackageAssetURIString string

type PackageDetailResponse struct {
	Package struct {
		PackagingVersion      string            `json:"packagingVersion"`
		Name                  string            `json:"name"`
		Version               string            `json:"version"`
		ReleaseVersion        int               `json:"releaseVersion"`
		Maintainer            string            `json:"maintainer"`
		Description           string            `json:"description"`
		Tags                  []string          `json:"tags"`
		Scm                   string            `json:"scm"`
		Website               string            `json:"website"`
		Framework             bool              `json:"framework"`
		MinDcosReleaseVersion string            `json:"minDcosReleaseVersion"`
		Marathon              map[string]string `json:"marathon"`
		Resource              struct {
			Assets struct {
				Uris map[PackageAssetNameString]PackageAssetURIString `json:"uris"`
			} `json:"assets"`
		} `json:"resource"`
		Config struct {
			Type       string            `json:"type"`
			Properties map[string]string `json:"properties"`
		} `json:"config"`
	} `json:"package"`
}

// ListPackageVersions retrieves a list of package versions from Cosmos matching the packageName provided
func (c *CosmosClient) ListPackageVersions(packageName string) (*ListVersionResponse, error) {
	listVersionReq := ListVersionRequest{IncludePackageVersions: true, PackageName: "dcos-ui"}
	body, err := json.Marshal(listVersionReq)

	if err != nil {
		return nil, errors.Wrap(err, "could not create json body from ListVersionRequest")
	}

	reqURL := *c.UniverseURL
	reqURL.Path = path.Join(reqURL.Path, "/package/list-versions")
	req, err := http.NewRequest("POST", reqURL.String(), bytes.NewBuffer(body))
	if err != nil {
		return nil, errors.Wrap(err, "request to cosmos /package/list-versions failed")
	}
	req.Header.Set("accept", "application/vnd.dcos.package.list-versions-response+json;charset=utf-8;version=v1")
	req.Header.Set("content-type", "application/vnd.dcos.package.list-versions-request+json;charset=utf-8;version=v1")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request to cosmos /package/list-versions failed with status %v", resp.StatusCode)
	}
	var response ListVersionResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode cosmos /package/list-versions response.")
	}

	return &response, nil
}

// GetPackageAssets retrieves the package assets from Cosmos matching the packageName and packageVersion provided
func (c *CosmosClient) GetPackageAssets(packageName string, packageVersion string) (map[PackageAssetNameString]PackageAssetURIString, error) {
	packageDetailReq := PackageDetailRequest{PackageName: packageName, PackageVersion: packageVersion}
	body, err := json.Marshal(packageDetailReq)
	if err != nil {
		return nil, err
	}

	reqURL := *c.UniverseURL
	reqURL.Path = path.Join(reqURL.Path, "/package/describe")
	req, err := http.NewRequest("POST", reqURL.String(), bytes.NewBuffer(body))
	if err != nil {
		return nil, errors.Wrap(err, "could not create json body from PackageDetailRequest")
	}
	req.Header.Set("accept", "application/vnd.dcos.package.describe-response+json;charset=utf-8;version=v3")
	req.Header.Set("content-type", "application/vnd.dcos.package.describe-request+json;charset=UTF-8;version=v1")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to query cosmos")
	}
	defer resp.Body.Close()
	var response PackageDetailResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		return nil, errors.Wrap(err, "could not decode package detail response")
	}
	assets := response.Package.Resource.Assets.Uris

	if len(assets) == 0 {
		return nil, fmt.Errorf("Could not get asset uris from JSON")
	}

	return assets, nil
}

func NewCosmosClient(httpClient *our_http.Client, universeURL *url.URL) *CosmosClient {
	return &CosmosClient{
		httpClient:  httpClient,
		UniverseURL: universeURL,
	}
}
