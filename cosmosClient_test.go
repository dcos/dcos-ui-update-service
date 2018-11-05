package main

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	our_http "github.com/dcos/dcos-ui-update-service/http"
)

var (
	sucessListResponse      = `{"results":{"2.25.0":"11","1.0.5-2.2.5":"7","1.0.15-3.0.7":"15","1.0.20-3.0.10":"20","1.0.17-3.0.8":"17","1.0.21-3.0.10":"21","2.0.1-3.0.14":"27","1.0.22-3.0.10":"22","1.0.23-3.0.10":"23","1.0.24-3.0.10":"24","1.0.13-2.2.5":"13","2.1.0-3.0.16":"100","1.0.12-2.2.5":"12","1.0.2-2.2.5":"4","1.0.18-3.0.9":"18","0.2.0-2":"1","1.0.16-3.0.8":"16","1.0.25-3.0.10":"25","2.0.2-3.0.14":"28","2.2.5-0.2.0":"3","1.0.8-2.2.5":"10","2.0.0-3.0.14":"26","2.2.0-3.0.16":"200","1.0.14-3.0.7":"14","1.0.6-2.2.5":"8","2.0.3-3.0.14":"29","2.3.0-3.0.16":"300","1.0.7-2.2.5":"9","0.2.0-1":"0","1.0.4-2.2.5":"5"}}`
	successDescribeResponse = `{
	"package": {
		"resource": {
			"assets": {
				"uris": {
					"dcos-ui-bundle": "https://frontend-elasticl-11uu7xp48vh9c-805473783.eu-central-1.elb.amazonaws.com/package/resource?url=https://downloads.mesosphere.io/dcos-ui/master%2Bdcos-ui-v2.24.4.tar.gz"
				}
			}
		}
	}}`

	emptyDescribeResponse = `{
	"package": {
		"resource": {
			"assets": {
			}
		}
	}}`
)

func makeTestClient(server *httptest.Server) (*CosmosClient, error) {
	return NewCosmosClient(our_http.NewClient(server.Client()), server.URL)
}

func serveSuccessfulListVersionResponseTestServer(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		accept := req.Header.Get("accept")
		if accept != "application/vnd.dcos.package.list-versions-response+json;charset=utf-8;version=v1" {
			t.Fatalf("Accept header is set incorrectly")
		}

		contentType := req.Header.Get("content-type")
		if contentType != "application/vnd.dcos.package.list-versions-request+json;charset=utf-8;version=v1" {
			t.Fatalf("content-type header is set incorrectly")
		}

		path := req.URL.Path
		if path != "/package/list-versions" {
			t.Fatalf("Expected path %q, got %q", "/package/list-versions", path)
		}

		body, err := ioutil.ReadAll(req.Body)
		defer req.Body.Close()
		if err != nil {
			t.Fatalf("Could not read the body")
			return
		}

		var request ListVersionRequest

		err = json.Unmarshal(body, &request)
		if err != nil {
			t.Fatalf("Could not parse the body to JSON")
			return
		}

		if !request.IncludePackageVersions {
			t.Fatalf("Expect to include request for package versions")
		}

		if request.PackageName != "dcos-ui" {
			t.Fatalf("Expect to request info for dcos-ui, instead got %q", request.PackageName)
		}

		io.WriteString(rw, sucessListResponse)
	}))
}

func TestCosmosListVersions(t *testing.T) {

	t.Run("sends a request to /package/list-versions", func(t *testing.T) {
		server := serveSuccessfulListVersionResponseTestServer(t)
		// Close the server when test finishes
		defer server.Close()

		cosmos, err := makeTestClient(server)
		if err != nil {
			t.Fatalf("Expected no error, got %q", err.Error())
		}

		resp, err := cosmos.ListPackageVersions("dcos-ui")

		if err != nil {
			t.Fatalf("Expected no error, got %q", err.Error())
		}

		res := resp.Results["2.25.0"]

		if res != "11" {
			t.Fatalf("Expected 11 as a result, got %q from %#v", res, resp.Results)
		}
	})

	t.Run("returns error if API call does not return OK", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			rw.WriteHeader(http.StatusNotFound)
		}))
		// Close the server when test finishes
		defer server.Close()

		cosmos, err := makeTestClient(server)
		if err != nil {
			t.Fatalf("Expected no error, got %q", err.Error())
		}

		_, err = cosmos.ListPackageVersions("dcos-ui")

		if err == nil {
			t.Fatalf("Expected error, got nil")
		}
	})

	t.Run("returns error if no JSON is returned", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			io.WriteString(rw, "Not found")
		}))
		// Close the server when test finishes
		defer server.Close()

		cosmos, err := makeTestClient(server)
		if err != nil {
			t.Fatalf("Expected no error, got %q", err.Error())
		}

		_, err = cosmos.ListPackageVersions("dcos-ui")

		if err == nil {
			t.Fatalf("Expected error, got nil")
		}
	})
}

func serveSuccessfulDescribeResponseServer(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		accept := req.Header.Get("accept")
		if accept != "application/vnd.dcos.package.describe-response+json;charset=utf-8;version=v3" {
			t.Fatalf("Accept header is set incorrectly")
		}

		contentType := req.Header.Get("content-type")
		if contentType != "application/vnd.dcos.package.describe-request+json;charset=UTF-8;version=v1" {
			t.Fatalf("content-type header is set incorrectly")
		}

		path := req.URL.Path
		if path != "/package/describe" {
			t.Fatalf("Expected path %q, got %q", "/package/describe", path)
		}

		body, err := ioutil.ReadAll(req.Body)
		defer req.Body.Close()
		if err != nil {
			t.Fatalf("Could not read the body")
			return
		}

		var request PackageDetailRequest

		err = json.Unmarshal(body, &request)
		if err != nil {
			t.Fatalf("Could not parse the body to JSON")
			return
		}

		if request.PackageVersion != "2.25.0" {
			t.Fatalf("Expect to request details for version 2.25.0, instead got %q", request.PackageVersion)
		}

		if request.PackageName != "dcos-ui" {
			t.Fatalf("Expect to request details for dcos-ui, instead got %q", request.PackageName)
		}

		io.WriteString(rw, successDescribeResponse)
	}))
}

func TestCosmosDetail(t *testing.T) {
	t.Run("sends a request to /package/describe", func(t *testing.T) {
		server := serveSuccessfulDescribeResponseServer(t)
		// Close the server when test finishes
		defer server.Close()

		cosmos, err := makeTestClient(server)
		if err != nil {
			t.Fatalf("Expected no error, got %q", err.Error())
		}

		resp, err := cosmos.GetPackageAssets("dcos-ui", "2.25.0")

		if err != nil {
			t.Fatalf("Expected no error, got %q", err.Error())
		}

		res := resp[PackageAssetNameString("dcos-ui-bundle")]
		expected := PackageAssetURIString("https://frontend-elasticl-11uu7xp48vh9c-805473783.eu-central-1.elb.amazonaws.com/package/resource?url=https://downloads.mesosphere.io/dcos-ui/master%2Bdcos-ui-v2.24.4.tar.gz")

		if res != expected {
			t.Fatalf("Expected %q as a result, got %q from %#v", expected, res, resp)
		}
	})

	t.Run("returns error if API call does not return OK", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			rw.WriteHeader(http.StatusUnauthorized)
		}))
		// Close the server when test finishes
		defer server.Close()

		cosmos, err := makeTestClient(server)
		if err != nil {
			t.Fatalf("Expected no error, got %q", err.Error())
		}

		_, err = cosmos.GetPackageAssets("dcos-ui", "2.25.0")

		if err == nil {
			t.Fatalf("Expected error, got nil")
		}
	})

	t.Run("returns error if no JSON is returned", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			io.WriteString(rw, "Not found")
		}))
		// Close the server when test finishes
		defer server.Close()

		cosmos, err := makeTestClient(server)
		if err != nil {
			t.Fatalf("Expected no error, got %q", err.Error())
		}

		_, err = cosmos.GetPackageAssets("dcos-ui", "2.25.0")

		if err == nil {
			t.Fatalf("Expected error, got nil")
		}
	})

	t.Run("returns error if incomplete JSON is returned", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			io.WriteString(rw, emptyDescribeResponse)
		}))
		// Close the server when test finishes
		defer server.Close()

		cosmos, err := makeTestClient(server)
		if err != nil {
			t.Fatalf("Expected no error, got %q", err.Error())
		}

		_, err = cosmos.GetPackageAssets("dcos-ui", "2.25.0")

		if err == nil {
			t.Fatalf("Expected error, got nil")
		}
	})
}
