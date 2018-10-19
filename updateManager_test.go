package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"path"
	"strings"
	"testing"

	"github.com/spf13/afero"
)

const defaultListResponse = "{\"results\":{\"2.25.0\":\"11\",\"2.25.1\":\"7\",\"2.25.2\":\"15\",\"1.0.20-3.0.10\":\"20\",\"1.0.17-3.0.8\":\"17\",\"1.0.21-3.0.10\":\"21\",\"2.0.1-3.0.14\":\"27\",\"1.0.22-3.0.10\":\"22\",\"1.0.23-3.0.10\":\"23\",\"1.0.24-3.0.10\":\"24\",\"1.0.13-2.2.5\":\"13\",\"2.1.0-3.0.16\":\"100\",\"1.0.12-2.2.5\":\"12\",\"1.0.2-2.2.5\":\"4\",\"1.0.18-3.0.9\":\"18\",\"0.2.0-2\":\"1\",\"1.0.16-3.0.8\":\"16\",\"1.0.25-3.0.10\":\"25\",\"2.0.2-3.0.14\":\"28\",\"2.2.5-0.2.0\":\"3\",\"1.0.8-2.2.5\":\"10\",\"2.0.0-3.0.14\":\"26\",\"2.2.0-3.0.16\":\"200\",\"1.0.14-3.0.7\":\"14\",\"1.0.6-2.2.5\":\"8\",\"2.0.3-3.0.14\":\"29\",\"2.3.0-3.0.16\":\"300\",\"1.0.7-2.2.5\":\"9\",\"0.2.0-1\":\"0\",\"1.0.4-2.2.5\":\"5\"}}"
const defaultDescribeResponse = `{
	"package": {
		"resource": {
			"assets": {
				"uris": {
					"dcos-ui-bundle": "https://frontend-elasticl-11uu7xp48vh9c-805473783.eu-central-1.elb.amazonaws.com/package/resource?url=https://downloads.mesosphere.io/dcos-ui/master%2Bdcos-ui-v2.24.4.tar.gz"
				}
			}
		}
	}}`

const noFileFoundDescribeResponse = `{
		"package": {
			"resource": {
				"assets": {
					"uris": {
						"dcos-ui-bundle": "https://unknown"
					}
				}
			}
		}}`

const noBundleInAssetsDescribeResponse = `{
			"package": {
				"resource": {
					"assets": {
						"uris": {
							"somethingElse": "https://frontend-elasticl-11uu7xp48vh9c-805473783.eu-central-1.elb.amazonaws.com/package/resource?url=https://downloads.mesosphere.io/dcos-ui/master%2Bdcos-ui-v2.24.4.tar.gz"
						}
					}
				}
			}}`

func TestUpdateManagerLoadVersion(t *testing.T) {
	// Use single quote backticks instead of escape
	defaultHandler := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		path := req.URL.Path
		if path == "/package/list-versions" {
			io.WriteString(rw, defaultListResponse)
		}

		if path == "/package/describe" {
			var request PackageDetailRequest

			body, err := ioutil.ReadAll(req.Body)
			if err != nil {
				return
			}
			defer req.Body.Close()
			err = json.Unmarshal(body, &request)
			if err != nil {
				return
			}

			// 2.25.0 => everything is there
			if request.PackageVersion == "2.25.0" {
				io.WriteString(rw, defaultDescribeResponse)
			}

			// 2.25.1 => file not found
			if request.PackageVersion == "2.25.1" {
				io.WriteString(rw, noFileFoundDescribeResponse)
			}

			// 2.25.2 => asset unknown
			if request.PackageVersion == "2.25.2" {
				io.WriteString(rw, noBundleInAssetsDescribeResponse)
			}
		}
	})

	t.Parallel()

	t.Run("throws an error if it can't reach the server", func(t *testing.T) {
		server := httptest.NewServer(defaultHandler)
		// Close the server when test finishes
		defer server.Close()

		loader := UpdateManager{
			Cosmos: CosmosClient{
				Client:      server.Client(),
				UniverseURL: "http://unkonwn",
			},
			Loader: Downloader{
				Client: server.Client(),
			},
			Fs: afero.NewMemMapFs(),
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

		loader := UpdateManager{
			Cosmos: CosmosClient{
				Client:      server.Client(),
				UniverseURL: server.URL,
			},
			Loader: Downloader{
				Client: server.Client(),
			},
			Fs: afero.NewMemMapFs(),
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

		loader := UpdateManager{
			Cosmos: CosmosClient{
				Client:      server.Client(),
				UniverseURL: server.URL,
			},
			Loader: Downloader{
				Client: server.Client(),
			},
			Fs: afero.NewMemMapFs(),
		}

		err := loader.LoadVersion("2.25.0", "/ponies")

		if err == nil {
			t.Fatalf("Expected error, got nil")
		}

		if !strings.Contains(err.Error(), "is no directory") {
			t.Fatalf("Error message should hint that the directory is absent. Instead got error message %q", err.Error())
		}
	})

	t.Run("throws error if one of the file named dcos-ui-bundle can not be found in the assets", func(t *testing.T) {
		server := httptest.NewServer(defaultHandler)
		// Close the server when test finishes
		defer server.Close()

		loader := UpdateManager{
			Cosmos: CosmosClient{
				Client:      server.Client(),
				UniverseURL: server.URL,
			},
			Loader: Downloader{
				Client: server.Client(),
			},
			Fs: afero.NewMemMapFs(),
		}

		err := loader.LoadVersion("2.25.2", "/")

		if err == nil {
			t.Fatalf("Expected error, got nil")
		}

		if !strings.Contains(err.Error(), "Could not find asset with the name") {
			t.Fatalf("Error message should hint that it could not load a file. Instead got error message %q", err.Error())
		}
	})

	t.Run("throws error if one of the files could not be downloaded", func(t *testing.T) {
		server := httptest.NewServer(defaultHandler)
		// Close the server when test finishes
		defer server.Close()

		loader := UpdateManager{
			Cosmos: CosmosClient{
				Client:      server.Client(),
				UniverseURL: server.URL,
			},
			Loader: Downloader{
				Client: server.Client(),
			},
			Fs: afero.NewMemMapFs(),
		}

		err := loader.LoadVersion("2.25.1", "/")

		if err == nil {
			t.Fatalf("Expected error, got nil")
		}

		if !strings.Contains(err.Error(), "Could not load") {
			t.Fatalf("Error message should hint that it could not load a file. Instead got error message %q", err.Error())
		}
	})
}

func TestUpdateManagerGetCurrentVersion(t *testing.T) {
	t.Parallel()

	t.Run("throws error if the VersionPath directory does not exist", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {}))
		// Close the server when test finishes
		defer server.Close()
		fs := afero.NewMemMapFs()

		loader := UpdateManager{
			Cosmos: CosmosClient{
				Client:      server.Client(),
				UniverseURL: server.URL,
			},
			Loader: Downloader{
				Client: server.Client(),
			},
			VersionPath: "/ui-versions",
			Fs:          fs,
		}

		_, err := loader.GetCurrentVersion()

		if err == nil {
			t.Fatalf("Expected error, got nil")
		}
	})

	t.Run("returns empty string if VersionPath directory is empty", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {}))
		// Close the server when test finishes
		defer server.Close()
		fs := afero.NewMemMapFs()

		loader := UpdateManager{
			Cosmos: CosmosClient{
				Client:      server.Client(),
				UniverseURL: server.URL,
			},
			Loader: Downloader{
				Client: server.Client(),
			},
			VersionPath: "/ui-versions",
			Fs:          fs,
		}

		fs.MkdirAll("/ui-versions", 0755)
		ver, err := loader.GetCurrentVersion()

		if err != nil {
			t.Fatalf("returned error when not expecting it %v", err)
		}
		if len(ver) != 0 {
			t.Errorf("returned non-empty string for version. %v", ver)
		}
	})

	t.Run("returns name of the only directory in VersionPath", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {}))
		// Close the server when test finishes
		defer server.Close()
		fs := afero.NewMemMapFs()

		loader := UpdateManager{
			Cosmos: CosmosClient{
				Client:      server.Client(),
				UniverseURL: server.URL,
			},
			Loader: Downloader{
				Client: server.Client(),
			},
			VersionPath: "/ui-versions",
			Fs:          fs,
		}

		fs.MkdirAll("/ui-versions/2.25.3", 0755)
		result, err := loader.GetCurrentVersion()

		if err != nil {
			t.Fatalf("Expected no error, got %#v", err)
		}

		if result != "2.25.3" {
			t.Fatalf("Expected result to be %q, got %q", "2.25.3", result)
		}
	})

	t.Run("returns error if there are more than one directory in VersionPath", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {}))
		// Close the server when test finishes
		defer server.Close()
		fs := afero.NewMemMapFs()

		loader := UpdateManager{
			Cosmos: CosmosClient{
				Client:      server.Client(),
				UniverseURL: server.URL,
			},
			Loader: Downloader{
				Client: server.Client(),
			},
			VersionPath: "/ui-versions",
			Fs:          fs,
		}

		fs.MkdirAll("/ui-versions/2.25.3", 0755)
		fs.MkdirAll("/ui-versions/2.25.7", 0755)
		_, err := loader.GetCurrentVersion()

		if err == nil {
			t.Fatalf("Expected an error, got nil")
		}
	})
}

func TestUpdateManagerGetPathToCurrentVersion(t *testing.T) {
	t.Parallel()

	t.Run("returns path to version", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {}))
		// Close the server when test finishes
		defer server.Close()
		fs := afero.NewMemMapFs()

		loader := UpdateManager{
			Cosmos: CosmosClient{
				Client:      server.Client(),
				UniverseURL: server.URL,
			},
			Loader: Downloader{
				Client: server.Client(),
			},
			VersionPath: "/ui-versions",
			Fs:          fs,
		}

		fs.MkdirAll("/ui-versions/2.25.3", 0755)
		result, err := loader.GetPathToCurrentVersion()

		if err != nil {
			t.Fatalf("Expected no error, got %#v", err)
		}

		if result != "/ui-versions/2.25.3" {
			t.Fatalf("Expected result to be %q, got %q", "/ui-versions/2.25.3", result)
		}
	})

	t.Run("throws error if VersionPath directory is empty", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {}))
		// Close the server when test finishes
		defer server.Close()
		fs := afero.NewMemMapFs()

		loader := UpdateManager{
			Cosmos: CosmosClient{
				Client:      server.Client(),
				UniverseURL: server.URL,
			},
			Loader: Downloader{
				Client: server.Client(),
			},
			VersionPath: "/ui-versions",
			Fs:          fs,
		}

		fs.MkdirAll("/ui-versions", 0755)
		_, err := loader.GetPathToCurrentVersion()

		if err == nil {
			t.Error("did not return an error for an empty versions dir")
		}
	})
}

type MockFileServer struct {
	DocumentRoot string
	Error        error
}

func (mfs *MockFileServer) UpdateDocumentRoot(documentRoot string) error {
	if mfs.Error != nil {
		return mfs.Error
	}
	mfs.DocumentRoot = documentRoot
	return nil
}

func (mfs *MockFileServer) GetDocumentRoot() string {
	return mfs.DocumentRoot
}

func TestUpdateManagerUpdateToVersion(t *testing.T) {
	t.Run("creates update in new dir when no current version exists", func(t *testing.T) {
		urlChan := make(chan string, 3) // because three requests will be made
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			baseURL := <-urlChan
			path := req.URL.Path
			if path == "/package/list-versions" {
				io.WriteString(rw, defaultListResponse)
			} else if path == "/package/describe" {
				io.WriteString(rw, strings.Replace(defaultDescribeResponse, "https://frontend-elasticl-11uu7xp48vh9c-805473783.eu-central-1.elb.amazonaws.com", baseURL, -1))
			} else {
				http.ServeFile(rw, req, "fixtures/release.tar.gz")
			}
		}))
		// because three requests will be made
		urlChan <- server.URL
		urlChan <- server.URL
		urlChan <- server.URL
		// Close the server when test finishes
		defer server.Close()
		fs := afero.NewMemMapFs()
		versionsPath := "/ui-versions"

		loader := UpdateManager{
			Cosmos: CosmosClient{
				Client:      server.Client(),
				UniverseURL: server.URL,
			},
			Loader: Downloader{
				Client: server.Client(),
				Fs:     fs,
			},
			VersionPath: versionsPath,
			Fs:          fs,
		}
		mfs := MockFileServer{
			DocumentRoot: "/opt/public",
			Error:        nil,
		}

		fs.MkdirAll(versionsPath, 0755)
		err := loader.UpdateToVersion("2.25.2", &mfs)

		if err != nil {
			t.Fatalf("Expected no error, got %#v", err)
		}

		newVersionPath := path.Join(versionsPath, "2.25.2")
		newVersionExists, err := afero.DirExists(fs, newVersionPath)

		if !newVersionExists || err != nil {
			t.Fatalf("Expected new directoy to exist, got %t, %#v", newVersionExists, err)
		}

		files, err := afero.ReadDir(fs, versionsPath)
		if err != nil {
			t.Fatalf("Expected no error, got %#v", err)
		}

		var versionDirs []string
		for _, file := range files {
			if file.IsDir() {
				versionDirs = append(versionDirs, file.Name())
			}
		}

		onlyNewVersionExists := len(versionDirs) == 1
		if !onlyNewVersionExists {
			t.Errorf("Expected only new version directory to exist")
		}

		fileServerUpdated := mfs.GetDocumentRoot() == newVersionPath
		if !fileServerUpdated {
			t.Errorf("Expected new version directory to be set as document root")
		}
	})

	t.Run("returns error if it can't upgrade", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			path := req.URL.Path
			if path == "/package/list-versions" {
				io.WriteString(rw, defaultListResponse)
			}

			if path == "/package/describe" {
				io.WriteString(rw, noFileFoundDescribeResponse)
			}
		}))
		// Close the server when test finishes
		defer server.Close()
		fs := afero.NewMemMapFs()

		loader := UpdateManager{
			Cosmos: CosmosClient{
				Client:      server.Client(),
				UniverseURL: server.URL,
			},
			Loader: Downloader{
				Client: server.Client(),
				Fs:     fs,
			},
			VersionPath: "/ui-versions",
			Fs:          fs,
		}
		mfs := MockFileServer{
			DocumentRoot: "/opt/public",
			Error:        nil,
		}

		fs.MkdirAll("/ui-versions/2.25.1", 0755)
		err := loader.UpdateToVersion("2.25.2", &mfs)

		if err == nil {
			t.Fatalf("Expected error, got nil")
		}
	})

	t.Run("removes new version dir if update fails", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			path := req.URL.Path
			if path == "/package/list-versions" {
				io.WriteString(rw, defaultListResponse)
			}

			if path == "/package/describe" {
				io.WriteString(rw, noFileFoundDescribeResponse)
			}
		}))
		// Close the server when test finishes
		defer server.Close()
		fs := afero.NewMemMapFs()

		loader := UpdateManager{
			Cosmos: CosmosClient{
				Client:      server.Client(),
				UniverseURL: server.URL,
			},
			Loader: Downloader{
				Client: server.Client(),
				Fs:     fs,
			},
			VersionPath: "/ui-versions",
			Fs:          fs,
		}
		mfs := MockFileServer{
			DocumentRoot: "/opt/public",
			Error:        nil,
		}

		fs.MkdirAll("/ui-versions/2.25.1", 0755)
		err := loader.UpdateToVersion("2.25.2", &mfs)

		if err == nil {
			t.Fatalf("Expected error, got nil")
		}

		newVersionExists, err := afero.DirExists(fs, "/ui-versions/2.25.2")

		if newVersionExists || err != nil {
			t.Fatalf("Expected new directoy to not exist, got %t, %#v", newVersionExists, err)
		}
	})

	t.Run("creates update in new directory and returns no error", func(t *testing.T) {
		urlChan := make(chan string, 3) // because three requests will be made
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			baseURL := <-urlChan
			path := req.URL.Path
			if path == "/package/list-versions" {
				io.WriteString(rw, defaultListResponse)
			} else if path == "/package/describe" {
				io.WriteString(rw, strings.Replace(defaultDescribeResponse, "https://frontend-elasticl-11uu7xp48vh9c-805473783.eu-central-1.elb.amazonaws.com", baseURL, -1))
			} else {
				http.ServeFile(rw, req, "fixtures/release.tar.gz")
			}
		}))
		// because three requests will be made
		urlChan <- server.URL
		urlChan <- server.URL
		urlChan <- server.URL
		// Close the server when test finishes
		defer server.Close()
		fs := afero.NewMemMapFs()
		versionsPath := "/ui-versions"

		loader := UpdateManager{
			Cosmos: CosmosClient{
				Client:      server.Client(),
				UniverseURL: server.URL,
			},
			Loader: Downloader{
				Client: server.Client(),
				Fs:     fs,
			},
			VersionPath: versionsPath,
			Fs:          fs,
		}
		mfs := MockFileServer{
			DocumentRoot: "/opt/public",
			Error:        nil,
		}

		fs.MkdirAll("/ui-versions/2.25.1", 0755)
		err := loader.UpdateToVersion("2.25.2", &mfs)

		if err != nil {
			t.Fatalf("Expected no error, got %#v", err)
		}

		newVersionExists, err := afero.DirExists(fs, versionsPath+"/2.25.2")

		if !newVersionExists || err != nil {
			t.Fatalf("Expected new directoy to exist, got %t, %#v", newVersionExists, err)
		}

		oldVersionExists, err := afero.DirExists(fs, versionsPath+"/2.25.1")

		if oldVersionExists || err != nil {
			t.Fatalf("Expected old directoy to be removed, got %t, %#v", oldVersionExists, err)
		}
	})

	t.Run("returns error if you can't update the file server to the current version", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			path := req.URL.Path
			if path == "/package/list-versions" {
				io.WriteString(rw, defaultListResponse)
			} else if path == "/package/describe" {
				io.WriteString(rw, defaultDescribeResponse)
			} else {
				http.ServeFile(rw, req, "fixtures/release.tar.gz")
			}
		}))
		// Close the server when test finishes
		defer server.Close()
		fs := afero.NewMemMapFs()
		versionsPath := "/ui-versions"

		loader := UpdateManager{
			Cosmos: CosmosClient{
				Client:      server.Client(),
				UniverseURL: server.URL,
			},
			Loader: Downloader{
				Client: server.Client(),
				Fs:     fs,
			},
			VersionPath: versionsPath,
			Fs:          fs,
		}
		mfs := MockFileServer{
			DocumentRoot: "/opt/public",
			Error:        nil,
		}

		fs.MkdirAll("/ui-versions/2.25.1", 0755)
		err := loader.UpdateToVersion("2.25.1", &mfs)

		if err == nil {
			t.Fatalf("Expected error, got nil")
		}
	})

	t.Run("returns error if file server fails to update", func(t *testing.T) {
		urlChan := make(chan string, 3) // because three requests will be made
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			baseURL := <-urlChan
			path := req.URL.Path
			if path == "/package/list-versions" {
				io.WriteString(rw, defaultListResponse)
			} else if path == "/package/describe" {
				io.WriteString(rw, strings.Replace(defaultDescribeResponse, "https://frontend-elasticl-11uu7xp48vh9c-805473783.eu-central-1.elb.amazonaws.com", baseURL, -1))
			} else {
				http.ServeFile(rw, req, "fixtures/release.tar.gz")
			}
		}))
		// because three requests will be made
		urlChan <- server.URL
		urlChan <- server.URL
		urlChan <- server.URL
		// Close the server when test finishes
		defer server.Close()
		fs := afero.NewMemMapFs()
		versionsPath := "/ui-versions"

		loader := UpdateManager{
			Cosmos: CosmosClient{
				Client:      server.Client(),
				UniverseURL: server.URL,
			},
			Loader: Downloader{
				Client: server.Client(),
				Fs:     fs,
			},
			VersionPath: versionsPath,
			Fs:          fs,
		}
		mfs := MockFileServer{
			DocumentRoot: "/ui-versions/2.25.1",
			Error:        fmt.Errorf("oh no bad stuff"),
		}

		fs.MkdirAll("/ui-versions/2.25.1", 0755)
		err := loader.UpdateToVersion("2.25.2", &mfs)

		if err == nil {
			t.Fatalf("Expected no error, got %#v", err)
		}

		newVersionExists, err := afero.DirExists(fs, versionsPath+"/2.25.2")

		if newVersionExists || err != nil {
			t.Fatalf("Expected new directoy to be removed, got %t, %#v", newVersionExists, err)
		}

		oldVersionExists, err := afero.DirExists(fs, versionsPath+"/2.25.1")

		if !oldVersionExists || err != nil {
			t.Fatalf("Expected old directoy to be removed, got %t, %#v", oldVersionExists, err)
		}
	})
}

func TestUpdateManagerResetVersion(t *testing.T) {
	t.Run("returns error if cant get current version", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {}))
		// Close the server when test finishes
		defer server.Close()
		fs := afero.NewMemMapFs()

		loader := UpdateManager{
			Cosmos: CosmosClient{
				Client:      server.Client(),
				UniverseURL: server.URL,
			},
			Loader: Downloader{
				Client: server.Client(),
			},
			VersionPath: "/ui-versions",
			Fs:          fs,
		}

		fs.MkdirAll("/ui-versions/2.25.3", 0755)
		fs.MkdirAll("/ui-versions/2.25.2", 0755)
		err := loader.ResetVersion()

		if err == nil {
			t.Fatalf("Expected error, got nil")
		}
	})

	t.Run("returns nil if there is not a current version", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {}))
		// Close the server when test finishes
		defer server.Close()
		fs := afero.NewMemMapFs()

		loader := UpdateManager{
			Cosmos: CosmosClient{
				Client:      server.Client(),
				UniverseURL: server.URL,
			},
			Loader: Downloader{
				Client: server.Client(),
			},
			VersionPath: "/ui-versions",
			Fs:          fs,
		}

		fs.MkdirAll("/ui-versions", 0755)
		err := loader.ResetVersion()

		if err != nil {
			t.Errorf("Expect no error, but got error %v", err)
		}
	})

	t.Run("returns error if fails to remove current version", func(t *testing.T) {
		// TODO: Can we test this?
	})

	t.Run("return nil if current version is removed", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {}))
		// Close the server when test finishes
		defer server.Close()
		fs := afero.NewMemMapFs()

		loader := UpdateManager{
			Cosmos: CosmosClient{
				Client:      server.Client(),
				UniverseURL: server.URL,
			},
			Loader: Downloader{
				Client: server.Client(),
			},
			VersionPath: "/ui-versions",
			Fs:          fs,
		}
		currentVersionPath := "/ui-versions/2.25.3"
		fs.MkdirAll(currentVersionPath, 0755)
		err := loader.ResetVersion()

		if err != nil {
			t.Fatalf("Expected nil, got an error %#v", err)
		}

		versionExists, err := afero.DirExists(fs, currentVersionPath)

		if versionExists || err != nil {
			t.Errorf("Expected version dir to not exist, got %t, %#v", versionExists, err)
		}
	})
}
