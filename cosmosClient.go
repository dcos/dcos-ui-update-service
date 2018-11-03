package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/dcos/dcos-ui-update-service/client"
	"github.com/pkg/errors"
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
				Uris map[string]string `json:"uris"`
			} `json:"assets"`
		} `json:"resource"`
		Config struct {
			Type       string            `json:"type"`
			Properties map[string]string `json:"properties"`
		} `json:"config"`
	} `json:"package"`
}

// TODO: think about if we can use the roundtripper api to set the headers in an easier way
// TODO: think about credentials and how we use them (forward maybe?)
func (c *CosmosClient) ListPackageVersions(packageName string) (*ListVersionResponse, error) {
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

func (c *CosmosClient) GetPackageAssets(packageName string, packageVersion string) (map[string]string, error) {
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
	var response PackageDetailResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		return nil, errors.Wrap(err, "could not decode package detail response")
	}
	assets := response.Package.Resource.Assets.Uris

	if len(assets) == 0 {
		return nil, fmt.Errorf("Could not get asset uris from JSON: %#v", assets)
	}

	return assets, nil
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
