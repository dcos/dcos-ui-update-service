package updateManager

import (
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path"
	"strings"
	"testing"

	"github.com/dcos/dcos-ui-update-service/cosmos"
	"github.com/dcos/dcos-ui-update-service/downloader"
	"github.com/dcos/dcos-ui-update-service/tests"
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

func TestClientLoadVersion(t *testing.T) {
	// Use single quote backticks instead of escape
	defaultHandler := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		path := req.URL.Path
		if path == "/package/list-versions" {
			io.WriteString(rw, defaultListResponse)
		}

		if path == "/package/describe" {
			var request cosmos.PackageDetailRequest

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
		cosmosURL, _ := url.Parse("http://example.com")
		cosmos := cosmos.NewClient(cosmosURL)
		fs := afero.NewMemMapFs()

		loader := Client{
			Cosmos: cosmos,
			Loader: downloader.New(fs),
			Fs:     fs,
		}

		err := loader.loadVersion("2.25.0", "/")

		tests.H(t).ErrEql(err, ErrCosmosRequestFailure)
	})

	t.Run("throws an error if the requested version is not available", func(t *testing.T) {
		server := httptest.NewServer(defaultHandler)
		// Close the server when test finishes
		defer server.Close()

		cosmosURL, _ := url.Parse(server.URL)
		cosmos := cosmos.NewClient(cosmosURL)
		fs := afero.NewMemMapFs()

		loader := Client{
			Cosmos: cosmos,
			Loader: downloader.New(fs),
			Fs:     fs,
		}

		err := loader.loadVersion("3.25.0", "/")

		tests.H(t).ErrEql(err, ErrRequestedVersionNotFound)
	})

	t.Run("throws error if one of the file named dcos-ui-bundle can not be found in the assets", func(t *testing.T) {
		server := httptest.NewServer(defaultHandler)
		// Close the server when test finishes
		defer server.Close()

		cosmosURL, _ := url.Parse(server.URL)
		cosmos := cosmos.NewClient(cosmosURL)
		fs := afero.NewMemMapFs()

		loader := Client{
			Cosmos: cosmos,
			Loader: downloader.New(fs),
			Fs:     fs,
		}

		err := loader.loadVersion("2.25.2", "/")

		tests.H(t).ErrEql(err, ErrUIPackageAssetNotFound)
	})

	t.Run("throws error if one of the files could not be downloaded", func(t *testing.T) {
		server := httptest.NewServer(defaultHandler)
		// Close the server when test finishes
		defer server.Close()

		cosmosURL, _ := url.Parse(server.URL)
		cosmos := cosmos.NewClient(cosmosURL)
		fs := afero.NewMemMapFs()

		loader := Client{
			Cosmos: cosmos,
			Loader: downloader.New(fs),
			Fs:     fs,
		}

		err := loader.loadVersion("2.25.1", "/")

		tests.H(t).ErrEql(err, downloader.ErrDowloadPackageFailed)
	})
}

func TestClientCurrentVersion(t *testing.T) {
	t.Parallel()

	t.Run("throws error if the VersionPath directory does not exist", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {}))
		// Close the server when test finishes
		defer server.Close()
		fs := afero.NewMemMapFs()

		cosmosURL, _ := url.Parse(server.URL)
		cosmos := cosmos.NewClient(cosmosURL)

		loader := Client{
			Cosmos:      cosmos,
			Loader:      downloader.New(fs),
			VersionPath: "/ui-versions",
			Fs:          fs,
		}

		_, err := loader.CurrentVersion()

		tests.H(t).ErrEql(err, ErrVersionsPathDoesNotExist)
	})

	t.Run("returns empty string if VersionPath directory is empty", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {}))
		// Close the server when test finishes
		defer server.Close()
		fs := afero.NewMemMapFs()

		cosmosURL, _ := url.Parse(server.URL)
		cosmos := cosmos.NewClient(cosmosURL)

		loader := Client{
			Cosmos:      cosmos,
			Loader:      downloader.New(fs),
			VersionPath: "/ui-versions",
			Fs:          fs,
		}

		fs.MkdirAll("/ui-versions", 0755)
		ver, err := loader.CurrentVersion()

		tests.H(t).ErrEql(err, nil)
		tests.H(t).StringEql(ver, "")
	})

	t.Run("returns name of the only directory in VersionPath", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {}))
		// Close the server when test finishes
		defer server.Close()
		fs := afero.NewMemMapFs()

		cosmosURL, _ := url.Parse(server.URL)
		cosmos := cosmos.NewClient(cosmosURL)

		loader := Client{
			Cosmos:      cosmos,
			Loader:      downloader.New(fs),
			VersionPath: "/ui-versions",
			Fs:          fs,
		}

		fs.MkdirAll("/ui-versions/2.25.3", 0755)
		result, err := loader.CurrentVersion()

		tests.H(t).ErrEql(err, nil)
		tests.H(t).StringEql(result, "2.25.3")
	})

	t.Run("returns error if there are more than one directory in VersionPath", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {}))
		// Close the server when test finishes
		defer server.Close()
		fs := afero.NewMemMapFs()

		cosmosURL, _ := url.Parse(server.URL)
		cosmos := cosmos.NewClient(cosmosURL)

		loader := Client{
			Cosmos:      cosmos,
			Loader:      downloader.New(fs),
			VersionPath: "/ui-versions",
			Fs:          fs,
		}

		fs.MkdirAll("/ui-versions/2.25.3", 0755)
		fs.MkdirAll("/ui-versions/2.25.7", 0755)
		_, err := loader.CurrentVersion()

		tests.H(t).ErrEql(err, ErrMultipleVersionFound)
	})
}

func TestClientPathToCurrentVersion(t *testing.T) {
	t.Parallel()

	t.Run("returns path to version", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {}))
		// Close the server when test finishes
		defer server.Close()
		fs := afero.NewMemMapFs()

		cosmosURL, _ := url.Parse(server.URL)
		cosmos := cosmos.NewClient(cosmosURL)

		loader := Client{
			Cosmos:      cosmos,
			Loader:      downloader.New(fs),
			VersionPath: "/ui-versions",
			Fs:          fs,
		}

		fs.MkdirAll("/ui-versions/2.25.3", 0755)
		result, err := loader.PathToCurrentVersion()

		tests.H(t).ErrEql(err, nil)
		tests.H(t).StringEql(result, "/ui-versions/2.25.3/dist")
	})

	t.Run("throws error if VersionPath directory is empty", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {}))
		// Close the server when test finishes
		defer server.Close()
		fs := afero.NewMemMapFs()

		cosmosURL, _ := url.Parse(server.URL)
		cosmos := cosmos.NewClient(cosmosURL)

		loader := Client{
			Cosmos:      cosmos,
			Loader:      downloader.New(fs),
			VersionPath: "/ui-versions",
			Fs:          fs,
		}

		fs.MkdirAll("/ui-versions", 0755)
		_, err := loader.PathToCurrentVersion()

		if err == nil {
			t.Error("did not return an error for an empty versions dir")
		}
	})
}

func successfulUpdateCompleteCallback(s string) error {
	return nil
}

func unsuccessfulUpdateCompleteCallback(s string) error {
	return errors.New("error completing update")
}

func TestClientUpdateToVersion(t *testing.T) {
	t.Parallel()

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
				http.ServeFile(rw, req, "../fixtures/release.tar.gz")
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

		cosmosURL, _ := url.Parse(server.URL)
		cosmos := cosmos.NewClient(cosmosURL)

		loader := Client{
			Cosmos:      cosmos,
			Loader:      downloader.New(fs),
			VersionPath: versionsPath,
			Fs:          fs,
		}

		fs.MkdirAll(versionsPath, 0755)
		err := loader.UpdateToVersion("2.25.2", successfulUpdateCompleteCallback)

		if err != nil {
			t.Fatalf("Expected no error, got %#v", err)
		}

		newVersionPath := path.Join(versionsPath, "2.25.2")
		newVersionExists, err := afero.DirExists(fs, newVersionPath)

		if !newVersionExists || err != nil {
			t.Fatalf("Expected new directory to exist, got %t, %#v", newVersionExists, err)
		}

		files, err := afero.ReadDir(fs, versionsPath)

		tests.H(t).ErrEql(err, nil)

		var versionDirs []string
		for _, file := range files {
			if file.IsDir() {
				versionDirs = append(versionDirs, file.Name())
			}
		}

		onlyNewVersionExists := len(versionDirs) == 1

		tests.H(t).BoolEqlWithMessage(onlyNewVersionExists, true, "Expected only new version directory to exist")
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

		cosmosURL, _ := url.Parse(server.URL)
		cosmos := cosmos.NewClient(cosmosURL)

		loader := Client{
			Cosmos:      cosmos,
			Loader:      downloader.New(fs),
			VersionPath: "/ui-versions",
			Fs:          fs,
		}

		err := loader.UpdateToVersion("2.25.2", successfulUpdateCompleteCallback)

		tests.H(t).ErrEql(err, ErrCouldNotGetCurrentVersion)
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

		cosmosURL, _ := url.Parse(server.URL)
		cosmos := cosmos.NewClient(cosmosURL)

		loader := Client{
			Cosmos:      cosmos,
			Loader:      downloader.New(fs),
			VersionPath: "/ui-versions",
			Fs:          fs,
		}

		loader.UpdateToVersion("2.25.2", successfulUpdateCompleteCallback)

		newVersionExists, _ := afero.DirExists(fs, "/ui-versions/2.25.2")

		tests.H(t).BoolEqlWithMessage(newVersionExists, false, "Expected new directoy to not exist")
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
				http.ServeFile(rw, req, "../fixtures/release.tar.gz")
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

		cosmosURL, _ := url.Parse(server.URL)
		cosmos := cosmos.NewClient(cosmosURL)

		loader := Client{
			Cosmos:      cosmos,
			Loader:      downloader.New(fs),
			VersionPath: versionsPath,
			Fs:          fs,
		}

		fs.MkdirAll("/ui-versions/2.25.1", 0755)
		err := loader.UpdateToVersion("2.25.2", successfulUpdateCompleteCallback)

		tests.H(t).ErrEql(err, nil)

		newVersionExists, _ := afero.DirExists(fs, versionsPath+"/2.25.2")
		oldVersionExists, _ := afero.DirExists(fs, versionsPath+"/2.25.1")

		tests.H(t).BoolEqlWithMessage(newVersionExists, true, "Expected new directoy to exist on failure")
		tests.H(t).BoolEqlWithMessage(oldVersionExists, false, "Expected old directoy to be removed")
	})

	t.Run("return nil if version already exists", func(t *testing.T) {
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
		fs.MkdirAll("/ui-versions/2.25.1", 0755)

		cosmosURL, _ := url.Parse(server.URL)
		cosmos := cosmos.NewClient(cosmosURL)

		loader := Client{
			Cosmos:      cosmos,
			Loader:      downloader.New(fs),
			VersionPath: versionsPath,
			Fs:          fs,
		}

		err := loader.UpdateToVersion("2.25.1", successfulUpdateCompleteCallback)

		tests.H(t).ErrEql(err, nil)
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
		fs.MkdirAll("/ui-versions/2.25.1", 0755)

		cosmosURL, _ := url.Parse(server.URL)
		cosmos := cosmos.NewClient(cosmosURL)

		loader := Client{
			Cosmos:      cosmos,
			Loader:      downloader.New(fs),
			VersionPath: versionsPath,
			Fs:          fs,
		}
		err := loader.UpdateToVersion("2.25.2", unsuccessfulUpdateCompleteCallback)

		tests.H(t).ErrEql(err, downloader.ErrDowloadPackageFailed)

		newVersionExists, _ := afero.DirExists(fs, versionsPath+"/2.25.2")
		oldVersionExists, _ := afero.DirExists(fs, versionsPath+"/2.25.1")

		tests.H(t).BoolEqlWithMessage(newVersionExists, false, "Expected new directoy to be removed on failure")
		tests.H(t).BoolEqlWithMessage(oldVersionExists, true, "Expected old directoy to not be removed")
	})
}

func TestClientResetVersion(t *testing.T) {
	t.Parallel()

	t.Run("returns error if cant get current version", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {}))
		// Close the server when test finishes
		defer server.Close()
		fs := afero.NewMemMapFs()

		cosmosURL, _ := url.Parse(server.URL)
		cosmos := cosmos.NewClient(cosmosURL)

		loader := Client{
			Cosmos:      cosmos,
			Loader:      downloader.New(fs),
			VersionPath: "/ui-versions",
			Fs:          fs,
		}

		fs.MkdirAll("/ui-versions/2.25.3", 0755)
		fs.MkdirAll("/ui-versions/2.25.2", 0755)
		err := loader.ResetVersion()

		tests.H(t).ErrEql(err, ErrCouldNotGetCurrentVersion)
	})

	t.Run("returns nil if there is not a current version", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {}))
		// Close the server when test finishes
		defer server.Close()
		fs := afero.NewMemMapFs()

		cosmosURL, _ := url.Parse(server.URL)
		cosmos := cosmos.NewClient(cosmosURL)

		loader := Client{
			Cosmos:      cosmos,
			Loader:      downloader.New(fs),
			VersionPath: "/ui-versions",
			Fs:          fs,
		}

		fs.MkdirAll("/ui-versions", 0755)
		err := loader.ResetVersion()

		tests.H(t).ErrEql(err, nil)
	})

	t.Run("return nil if current version is removed", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {}))
		// Close the server when test finishes
		defer server.Close()
		fs := afero.NewMemMapFs()

		cosmosURL, _ := url.Parse(server.URL)
		cosmos := cosmos.NewClient(cosmosURL)

		loader := Client{
			Cosmos:      cosmos,
			Loader:      downloader.New(fs),
			VersionPath: "/ui-versions",
			Fs:          fs,
		}
		currentVersionPath := "/ui-versions/2.25.3"
		fs.MkdirAll(currentVersionPath, 0755)
		err := loader.ResetVersion()

		tests.H(t).ErrEql(err, nil)

		versionExists, _ := afero.DirExists(fs, currentVersionPath)

		tests.H(t).BoolEqlWithMessage(versionExists, false, "Expected version dir to not exist")
	})
}
