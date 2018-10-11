package main

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUniverseDownloader(t *testing.T) {
	// Use single quote backticks instead of escape
	defaultResponse := "{\"results\":{\"2.25.0\":\"11\",\"1.0.5-2.2.5\":\"7\",\"1.0.15-3.0.7\":\"15\",\"1.0.20-3.0.10\":\"20\",\"1.0.17-3.0.8\":\"17\",\"1.0.21-3.0.10\":\"21\",\"2.0.1-3.0.14\":\"27\",\"1.0.22-3.0.10\":\"22\",\"1.0.23-3.0.10\":\"23\",\"1.0.24-3.0.10\":\"24\",\"1.0.13-2.2.5\":\"13\",\"2.1.0-3.0.16\":\"100\",\"1.0.12-2.2.5\":\"12\",\"1.0.2-2.2.5\":\"4\",\"1.0.18-3.0.9\":\"18\",\"0.2.0-2\":\"1\",\"1.0.16-3.0.8\":\"16\",\"1.0.25-3.0.10\":\"25\",\"2.0.2-3.0.14\":\"28\",\"2.2.5-0.2.0\":\"3\",\"1.0.8-2.2.5\":\"10\",\"2.0.0-3.0.14\":\"26\",\"2.2.0-3.0.16\":\"200\",\"1.0.14-3.0.7\":\"14\",\"1.0.6-2.2.5\":\"8\",\"2.0.3-3.0.14\":\"29\",\"2.3.0-3.0.16\":\"300\",\"1.0.7-2.2.5\":\"9\",\"0.2.0-1\":\"0\",\"1.0.4-2.2.5\":\"5\"}}"
	defaultHandler := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		io.WriteString(rw, defaultResponse)
	})

	t.Run("GetPackageVersions", func(t *testing.T) {
		t.Run("sends a request to /package/list-versions", func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
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

				io.WriteString(rw, defaultResponse)
			}))
			// Close the server when test finishes
			defer server.Close()

			loader := UniverseDownloader{
				Client:      server.Client(),
				UniverseURL: server.URL,
			}

			resp, err := loader.getPackageVersions("dcos-ui")

			if err != nil {
				t.Fatalf("Expected no error, got %q", err.Error())
			}

			res := resp.Results["2.25.0"]

			if res != "11" {
				t.Fatalf("Expected 11 as a result, got %q from %#v", res, resp.Results)
			}
		})

		t.Run("returns error if no JSON is returned", func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
				io.WriteString(rw, "Not found")
			}))
			// Close the server when test finishes
			defer server.Close()

			loader := UniverseDownloader{
				Client:      server.Client(),
				UniverseURL: server.URL,
			}

			_, err := loader.getPackageVersions("dcos-ui")

			if err == nil {
				t.Fatalf("Expected error, got nil")
			}
		})
	})

	t.Run("GetAssetsForPackage", func(t *testing.T) {
		t.Run("should make a call to cosmos describe", func(t *testing.T) {
			t.Skip()
		})
		t.Run("should throw if server errors", func(t *testing.T) {
			t.Skip()
		})
		t.Run("should throw if server returns no JSON", func(t *testing.T) {
			t.Skip()
		})
		t.Run("should return list of URLs download", func(t *testing.T) {
			t.Skip()
		})
	})

	t.Run("DownloadAndUnpack", func(t *testing.T) {
		t.Run("should download and unpack", func(t *testing.T) {
			t.Skip()
		})
		t.Run("should throw if server errors", func(t *testing.T) {
			t.Skip()
		})
	})

	t.Run("LoadVersion", func(t *testing.T) {
		t.Parallel()

		t.Run("throws an error if it can't reach the server", func(t *testing.T) {
			server := httptest.NewServer(defaultHandler)
			// Close the server when test finishes
			defer server.Close()

			loader := UniverseDownloader{
				Client:      server.Client(),
				UniverseURL: "http://unkonwn",
			}

			err := loader.LoadVersion("2.25.0", "/")

			if err == nil {
				t.Fatalf("Expected error, got nil")
			}

			if !strings.Contains(err.Error(), "Could not reach") {
				t.Fatalf("Error message should hint that the server is not reachable. Instead got error message %q", err.Error())
			}
		})

		t.Run("throws an error if the requested version is not available", func(t *testing.T) {
			server := httptest.NewServer(defaultHandler)
			// Close the server when test finishes
			defer server.Close()

			loader := UniverseDownloader{
				Client:      server.Client(),
				UniverseURL: server.URL,
			}

			err := loader.LoadVersion("3.25.0", "/")

			if err == nil {
				t.Fatalf("Expected error, got nil")
			}

			if !strings.Contains(err.Error(), "version is not available") {
				t.Fatalf("Error message should hint that the version is absent. Instead got error message %q", err.Error())
			}
		})

		t.Run("throws an error if the directory is not present", func(t *testing.T) {
			server := httptest.NewServer(defaultHandler)
			// Close the server when test finishes
			defer server.Close()

			loader := UniverseDownloader{
				Client:      server.Client(),
				UniverseURL: server.URL,
			}

			err := loader.LoadVersion("2.25.0", "/ponies")

			if err == nil {
				t.Fatalf("Expected error, got nil")
			}

			if !strings.Contains(err.Error(), "is no directory") {
				t.Fatalf("Error message should hint that the directory is absent. Instead got error message %q", err.Error())
			}
		})

		t.Run("throws error if one of the files could not be downloaded", func(t *testing.T) {
			t.Skip()
		})

		// For describe call https://github.com/dcos/dcos-cli/blob/master/pkg/cosmos/client.go
		// For DL of files
		t.Run("downloads files from universe", func(t *testing.T) {
			t.Skip()
		})

		t.Run("extracts files from universe", func(t *testing.T) {
			t.Skip()
		})

		t.Run("returns no error", func(t *testing.T) {
			t.Skip()
		})
	})
}
