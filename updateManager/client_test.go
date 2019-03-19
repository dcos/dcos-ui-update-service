package updateManager

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/dcos/dcos-ui-update-service/config"
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

func setupServingDefault(t *testing.T) {
	t.Log("Setup serving pre-bundled UI")
	os.MkdirAll("../testdata/um-sandbox/ui-versions", 0755)
	os.MkdirAll("../testdata/um-sandbox/dcos-ui", 0755)
	os.Symlink("../testdata/um-sandbox/dcos-ui", "../testdata/um-sandbox/dcos-ui-dist")
}
func setupServingSpecificVersion(t *testing.T, version string) {
	t.Logf("Setup serving UI v%s", version)
	versionPath := path.Join(path.Join("../testdata/um-sandbox/ui-versions/", version), "dist")
	os.MkdirAll(versionPath, 0755)
	os.MkdirAll("../testdata/um-sandbox/dcos-ui", 0755)
	os.Symlink(versionPath, "../testdata/um-sandbox/dcos-ui-dist")
}
func setupServingVersion(t *testing.T) {
	setupServingSpecificVersion(t, "1.0.0")
}

func tearDown(t *testing.T) {
	t.Log("Teardown testdata sandbox")
	os.RemoveAll("../testdata/um-sandbox")
}

func TestClientCurrentVersion(t *testing.T) {
	t.Run("returns empty sting if serving the pre-bundled ui", func(t *testing.T) {
		setupServingDefault(t)
		defer tearDown(t)

		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {}))
		// Close the server when test finishes
		defer server.Close()
		cosmosURL, _ := url.Parse(server.URL)
		cosmos := cosmos.NewClient(cosmosURL)

		cfg, _ := config.Parse([]string{
			"--versions-root", "../testdata/um-sandbox/ui-versions",
			"--ui-dist-symlink", "../testdata/um-sandbox/dcos-ui-dist",
			"--default-ui-path", "../testdata/um-sandbox/dcos-ui",
		})

		fs := afero.NewOsFs()

		loader := Client{
			Cosmos: cosmos,
			Loader: downloader.New(fs),
			Config: cfg,
			Fs:     fs,
		}

		ver, err := loader.CurrentVersion()

		tests.H(t).IsNil(err)
		tests.H(t).StringEql(ver, "")
	})

	t.Run("returns error if the UIDistSymlink does not exist", func(t *testing.T) {
		setupServingDefault(t)
		defer tearDown(t)

		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {}))
		// Close the server when test finishes
		defer server.Close()
		cosmosURL, _ := url.Parse(server.URL)
		cosmos := cosmos.NewClient(cosmosURL)

		cfg, _ := config.Parse([]string{
			"--versions-root", "../testdata/um-sandbox/ui-versions",
			"--ui-dist-symlink", "../testdata/um-sandbox/dcos-ui-bad",
			"--default-ui-path", "../testdata/um-sandbox/dcos-ui",
		})
		fs := afero.NewOsFs()

		loader := Client{
			Cosmos: cosmos,
			Loader: downloader.New(fs),
			Config: cfg,
			Fs:     fs,
		}

		_, err := loader.CurrentVersion()

		tests.H(t).ErrEql(err, ErrUIDistSymlinkNotFound)
	})

	t.Run("return version number if serving a version", func(t *testing.T) {
		setupServingVersion(t)
		defer tearDown(t)

		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {}))
		// Close the server when test finishes
		defer server.Close()
		cosmosURL, _ := url.Parse(server.URL)
		cosmos := cosmos.NewClient(cosmosURL)

		cfg, _ := config.Parse([]string{
			"--versions-root", "../testdata/um-sandbox/ui-versions",
			"--ui-dist-symlink", "../testdata/um-sandbox/dcos-ui-dist",
			"--default-ui-path", "../testdata/um-sandbox/dcos-ui",
		})
		fs := afero.NewOsFs()

		loader := Client{
			Cosmos: cosmos,
			Loader: downloader.New(fs),
			Config: cfg,
			Fs:     fs,
		}

		ver, err := loader.CurrentVersion()

		tests.H(t).IsNil(err)
		tests.H(t).StringEql(ver, "1.0.0")
	})

	t.Run("return error if non-default isn't serving dist folder", func(t *testing.T) {
		// Setup bad version
		os.MkdirAll("../testdata/um-sandbox/ui-versions/1.0.0", 0755)
		os.MkdirAll("../testdata/um-sandbox/dcos-ui", 0755)
		os.Symlink("../testdata/um-sandbox/ui-versions/1.0.0", "../testdata/um-sandbox/dcos-ui-dist")
		defer tearDown(t)

		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {}))
		// Close the server when test finishes
		defer server.Close()
		cosmosURL, _ := url.Parse(server.URL)
		cosmos := cosmos.NewClient(cosmosURL)

		cfg, _ := config.Parse([]string{
			"--versions-root", "../testdata/um-sandbox/ui-versions",
			"--ui-dist-symlink", "../testdata/um-sandbox/dcos-ui-dist",
			"--default-ui-path", "../testdata/um-sandbox/dcos-ui",
		})
		fs := afero.NewOsFs()

		loader := Client{
			Cosmos: cosmos,
			Loader: downloader.New(fs),
			Config: cfg,
			Fs:     fs,
		}

		_, err := loader.CurrentVersion()

		tests.H(t).StringContains(err.Error(), "Expected served version directory to be `dist` but got")
	})

	t.Run("returns version even if its not semver", func(t *testing.T) {
		// Setup bad version
		os.MkdirAll("../testdata/um-sandbox/ui-versions/not_semver/dist", 0755)
		os.MkdirAll("../testdata/um-sandbox/dcos-ui", 0755)
		os.Symlink("../testdata/um-sandbox/ui-versions/not_semver/dist", "../testdata/um-sandbox/dcos-ui-dist")
		defer tearDown(t)

		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {}))
		// Close the server when test finishes
		defer server.Close()
		cosmosURL, _ := url.Parse(server.URL)
		cosmos := cosmos.NewClient(cosmosURL)

		cfg, _ := config.Parse([]string{
			"--versions-root", "../testdata/um-sandbox/ui-versions",
			"--ui-dist-symlink", "../testdata/um-sandbox/dcos-ui-dist",
			"--default-ui-path", "../testdata/um-sandbox/dcos-ui",
		})
		fs := afero.NewOsFs()

		loader := Client{
			Cosmos: cosmos,
			Loader: downloader.New(fs),
			Config: cfg,
			Fs:     fs,
		}

		version, err := loader.CurrentVersion()

		tests.H(t).ErrEql(err, nil)
		tests.H(t).StringEql(version, "not_semver")
	})
}
func TestClientPathToCurrentVersion(t *testing.T) {
	t.Run("returns path to version", func(t *testing.T) {
		defer tearDown(t)
		setupServingVersion(t)

		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {}))
		// Close the server when test finishes
		defer server.Close()
		cosmosURL, _ := url.Parse(server.URL)
		cosmos := cosmos.NewClient(cosmosURL)

		cfg, _ := config.Parse([]string{
			"--versions-root", "../testdata/um-sandbox/ui-versions",
			"--ui-dist-symlink", "../testdata/um-sandbox/dcos-ui-dist",
			"--default-ui-path", "../testdata/um-sandbox/dcos-ui",
		})
		fs := afero.NewOsFs()

		loader := Client{
			Cosmos: cosmos,
			Loader: downloader.New(fs),
			Config: cfg,
			Fs:     fs,
		}

		result, err := loader.PathToCurrentVersion()

		tests.H(t).ErrEql(err, nil)
		tests.H(t).StringEql(result, "../testdata/um-sandbox/ui-versions/1.0.0/dist")
	})

	t.Run("throws error if VersionPath directory is empty", func(t *testing.T) {
		defer tearDown(t)
		setupServingDefault(t)

		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {}))
		// Close the server when test finishes
		defer server.Close()
		cosmosURL, _ := url.Parse(server.URL)
		cosmos := cosmos.NewClient(cosmosURL)

		cfg, _ := config.Parse([]string{
			"--versions-root", "../testdata/um-sandbox/ui-versions",
			"--ui-dist-symlink", "../testdata/um-sandbox/dcos-ui-bad",
			"--default-ui-path", "../testdata/um-sandbox/dcos-ui",
		})
		fs := afero.NewOsFs()

		loader := Client{
			Cosmos: cosmos,
			Loader: downloader.New(fs),
			Config: cfg,
			Fs:     fs,
		}

		_, err := loader.PathToCurrentVersion()

		tests.H(t).ErrEql(err, ErrUIDistSymlinkNotFound)
	})
}
func TestClientRemoveVersion(t *testing.T) {
	t.Parallel()

	t.Run("return nil if current version is removed", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {}))
		// Close the server when test finishes
		defer server.Close()
		fs := afero.NewMemMapFs()
		cfg, _ := config.Parse([]string{
			"--versions-root", "/ui-versions",
		})

		cosmosURL, _ := url.Parse(server.URL)
		cosmos := cosmos.NewClient(cosmosURL)

		loader := Client{
			Cosmos: cosmos,
			Loader: downloader.New(fs),
			Config: cfg,
			Fs:     fs,
		}
		currentVersionPath := "/ui-versions/2.25.3/dist"
		fs.MkdirAll(currentVersionPath, 0755)
		err := loader.RemoveVersion("2.25.3")

		tests.H(t).ErrEql(err, nil)

		versionExists, _ := afero.DirExists(fs, currentVersionPath)

		tests.H(t).BoolEqlWithMessage(versionExists, false, "Expected version dir to not exist")
	})

	t.Run("returns ErrRequestedVersionNotFound is version not found", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {}))
		// Close the server when test finishes
		defer server.Close()
		fs := afero.NewMemMapFs()
		cfg, _ := config.Parse([]string{
			"--versions-root", "/ui-versions",
		})

		cosmosURL, _ := url.Parse(server.URL)
		cosmos := cosmos.NewClient(cosmosURL)

		loader := Client{
			Cosmos: cosmos,
			Loader: downloader.New(fs),
			Config: cfg,
			Fs:     fs,
		}
		currentVersionPath := "/ui-versions/2.25.3/dist"
		fs.MkdirAll(currentVersionPath, 0755)
		err := loader.RemoveVersion("2.25.4")

		tests.H(t).ErrEql(err, ErrRequestedVersionNotFound)
	})
}

func TestClientUpdateToVersion(t *testing.T) {

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
		defer tearDown(t)
		setupServingDefault(t)

		fs := afero.NewOsFs()

		cfg, _ := config.Parse([]string{
			"--versions-root", "../testdata/um-sandbox/ui-versions",
			"--ui-dist-symlink", "../testdata/um-sandbox/dcos-ui-dist",
			"--default-ui-path", "../testdata/um-sandbox/dcos-ui",
		})

		cosmosURL, _ := url.Parse(server.URL)
		cosmos := cosmos.NewClient(cosmosURL)

		loader := Client{
			Cosmos: cosmos,
			Loader: downloader.New(fs),
			Config: cfg,
			Fs:     fs,
		}

		err := loader.UpdateToVersion("2.25.2", successfulUpdateCompleteCallback)

		if err != nil {
			t.Fatalf("Expected no error, got %#v", err)
		}

		newVersionPath := path.Join(cfg.VersionsRoot(), "2.25.2")
		newVersionExists, err := afero.DirExists(fs, newVersionPath)

		if !newVersionExists || err != nil {
			t.Fatalf("Expected new directory to exist, got %t, %#v", newVersionExists, err)
		}

		files, err := afero.ReadDir(fs, cfg.VersionsRoot())

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

	t.Run("returns error if it can't download package", func(t *testing.T) {
		// Setup mock cosmos for test
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

		defer tearDown(t)
		setupServingDefault(t)

		cfg, _ := config.Parse([]string{
			"--versions-root", "../testdata/um-sandbox/ui-versions",
			"--ui-dist-symlink", "../testdata/um-sandbox/dcos-ui-dist",
			"--default-ui-path", "../testdata/um-sandbox/dcos-ui",
		})
		fs := afero.NewOsFs()

		cosmosURL, _ := url.Parse(server.URL)
		cosmos := cosmos.NewClient(cosmosURL)

		loader := Client{
			Cosmos: cosmos,
			Loader: downloader.New(fs),
			Config: cfg,
			Fs:     fs,
		}

		err := loader.UpdateToVersion("2.25.2", successfulUpdateCompleteCallback)

		tests.H(t).ErrEql(err, downloader.ErrDowloadPackageFailed)
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

		defer tearDown(t)
		setupServingDefault(t)

		cfg, _ := config.Parse([]string{
			"--versions-root", "../testdata/um-sandbox/ui-versions",
			"--ui-dist-symlink", "../testdata/um-sandbox/dcos-ui-dist",
			"--default-ui-path", "../testdata/um-sandbox/dcos-ui",
		})
		fs := afero.NewOsFs()

		cosmosURL, _ := url.Parse(server.URL)
		cosmos := cosmos.NewClient(cosmosURL)

		loader := Client{
			Cosmos: cosmos,
			Loader: downloader.New(fs),
			Config: cfg,
			Fs:     fs,
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

		defer tearDown(t)
		setupServingSpecificVersion(t, "2.25.1")

		cfg, _ := config.Parse([]string{
			"--versions-root", "../testdata/um-sandbox/ui-versions",
			"--ui-dist-symlink", "../testdata/um-sandbox/dcos-ui-dist",
			"--default-ui-path", "../testdata/um-sandbox/dcos-ui",
		})
		fs := afero.NewOsFs()

		cosmosURL, _ := url.Parse(server.URL)
		cosmos := cosmos.NewClient(cosmosURL)

		loader := Client{
			Cosmos: cosmos,
			Loader: downloader.New(fs),
			Config: cfg,
			Fs:     fs,
		}

		err := loader.UpdateToVersion("2.25.2", successfulUpdateCompleteCallback)

		tests.H(t).ErrEql(err, nil)

		newVersionExists, _ := afero.DirExists(fs, path.Join(cfg.VersionsRoot(), "/2.25.2"))
		oldVersionExists, _ := afero.DirExists(fs, path.Join(cfg.VersionsRoot(), "/2.25.1"))

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

		defer tearDown(t)
		setupServingSpecificVersion(t, "2.25.1")

		cfg, _ := config.Parse([]string{
			"--versions-root", "../testdata/um-sandbox/ui-versions",
			"--ui-dist-symlink", "../testdata/um-sandbox/dcos-ui-dist",
			"--default-ui-path", "../testdata/um-sandbox/dcos-ui",
		})
		fs := afero.NewOsFs()

		cosmosURL, _ := url.Parse(server.URL)
		cosmos := cosmos.NewClient(cosmosURL)

		loader := Client{
			Cosmos: cosmos,
			Loader: downloader.New(fs),
			Config: cfg,
			Fs:     fs,
		}

		err := loader.UpdateToVersion("2.25.1", successfulUpdateCompleteCallback)

		tests.H(t).ErrEql(err, nil)
	})

	t.Run("returns error if complete callback returns an error", func(t *testing.T) {
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

		defer tearDown(t)
		setupServingSpecificVersion(t, "2.25.1")

		cfg, _ := config.Parse([]string{
			"--versions-root", "../testdata/um-sandbox/ui-versions",
			"--ui-dist-symlink", "../testdata/um-sandbox/dcos-ui-dist",
			"--default-ui-path", "../testdata/um-sandbox/dcos-ui",
		})
		fs := afero.NewOsFs()

		cosmosURL, _ := url.Parse(server.URL)
		cosmos := cosmos.NewClient(cosmosURL)

		loader := Client{
			Cosmos: cosmos,
			Loader: downloader.New(fs),
			Config: cfg,
			Fs:     fs,
		}
		err := loader.UpdateToVersion("2.25.2", unsuccessfulUpdateCompleteCallback)

		tests.H(t).ErrEql(err, downloader.ErrDowloadPackageFailed)

		newVersionExists, _ := afero.DirExists(fs, path.Join(cfg.VersionsRoot(), "/2.25.2"))
		oldVersionExists, _ := afero.DirExists(fs, path.Join(cfg.VersionsRoot(), "/2.25.1"))

		tests.H(t).BoolEqlWithMessage(newVersionExists, false, "Expected new directoy to be removed on failure")
		tests.H(t).BoolEqlWithMessage(oldVersionExists, true, "Expected old directoy to not be removed")
	})
}

func successfulUpdateCompleteCallback(s string) error {
	return nil
}

func unsuccessfulUpdateCompleteCallback(s string) error {
	return errors.New("error completing update")
}
