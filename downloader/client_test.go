package downloader

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/spf13/afero"
)

func TestDownloader(t *testing.T) {
	t.Run("DownloadAndUnpack", func(t *testing.T) {
		t.Run("should download and unpack a single file", func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
				http.ServeFile(rw, req, "../fixtures/release.tar.gz")
			}))
			// Close the server when test finishes
			defer server.Close()
			appFS := afero.NewMemMapFs()

			loader := New(appFS)

			dest, err := ioutil.TempDir("", "downloader_test")
			if err != nil {
				t.Fatalf("Could not create a tmp dir")
			}
			serverURL, _ := url.Parse(server.URL)
			err = loader.DownloadAndUnpack(serverURL, dest)

			if err != nil {
				t.Fatalf("Should not have thrown an error, got %#v", err)
			}

			f, err := appFS.Open(dest + "/README.md")
			defer f.Close()
			if err != nil {
				t.Fatalf("Expected to open the README.md file, but got an error: %#v", err)
			}
		})

		t.Run("should throw if server cannot dowload file", func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
				rw.WriteHeader(http.StatusForbidden)
			}))
			// Close the server when test finishes
			defer server.Close()
			appFS := afero.NewMemMapFs()

			loader := New(appFS)

			dest, err := ioutil.TempDir("", "downloader_test")
			if err != nil {
				t.Fatalf("Could not create a tmp dir")
			}

			downloadURL, _ := url.Parse(server.URL)
			err = loader.DownloadAndUnpack(downloadURL, dest)

			if err == nil {
				t.Fatalf("Should have thrown an error, got none")
			}
		})

		t.Run("should throw if server errors", func(t *testing.T) {
			appFS := afero.NewMemMapFs()

			loader := New(appFS)

			dest, err := ioutil.TempDir("", "downloader_test")
			if err != nil {
				t.Fatalf("Could not create a tmp dir")
			}

			downloadURL, _ := url.Parse("http://unknown")
			err = loader.DownloadAndUnpack(downloadURL, dest)

			if err == nil {
				t.Fatalf("Should have thrown an error, got none")
			}
		})
	})
}
