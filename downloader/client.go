package downloader

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"
)

var (
	// ErrDowloadPackageFailed occurs if we cannot download the package
	ErrDowloadPackageFailed = errors.New("Failed to download package")
	// ErrBadPackageDownloadResponse occurs if we cannot read the response when downloading a package
	ErrBadPackageDownloadResponse = errors.New("Received a bad response when downloading package")
	// ErrUnzippingPackageFailed occurs if we cannot unzip the package after downloading
	ErrUnzippingPackageFailed = errors.New("Error unzipping package")
	// ErrCreatingDirectoryWhileUnpacking occurs if we cannot create a directory while unzipping the package
	ErrCreatingDirectoryWhileUnpacking = errors.New("Could not create directory while unzipping package")
	// ErrCreatingFileWhileUnpacking occurs if we cannot open or copy an archive file while unzipping the package
	ErrCreatingFileWhileUnpacking = errors.New("Failed to create file while unzipping package")
)

// Client is used to download a package from a URL and extract it to the filesystem
type Client struct {
	client *http.Client
	Fs     afero.Fs
}

// ExtractTarGzToDir extracts payload as a tar file, unzips each entry.
// It assumes that the tar file represents a directory and writes any
// file/directory within into dest.
func (d *Client) extractTarGzToDir(dest string, payload []byte) error {
	gzr, err := gzip.NewReader(bytes.NewReader(payload))
	if err != nil {
		logrus.WithError(err).Error("Failed to unzip new version package")
		return ErrUnzippingPackageFailed
	}
	defer gzr.Close()
	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()

		switch {

		// if no more files are found return
		case err == io.EOF:
			logrus.Info("Extract tar.gz to directory: No more files found")
			return nil

		// return any other error
		case err != nil:
			return err

		// if the header is nil, just skip it (not sure how this
		// happens)
		case header == nil:
			logrus.Info("Extract tar.gz to directory: Header is nil, skip")
			continue
		}

		target := filepath.Join(dest, header.Name)

		// check the file type
		switch header.Typeflag {

		// if its a dir and it doesn't exist create it
		case tar.TypeDir:
			logrus.Infof("Extract tar.gz to directory: Creating directory - %s", target)
			if _, err := d.Fs.Stat(target); err != nil {
				if err := d.Fs.MkdirAll(target, 0755); err != nil {
					logrus.WithError(err).Errorf("Failed to make directory while unzipping new version package. Target: %s", target)
					return ErrCreatingDirectoryWhileUnpacking
				}
			}

		// if it's a file create it
		case tar.TypeReg:
			logrus.Infof("Extract tar.gz to directory: Creating file - %s", target)
			f, err := d.Fs.OpenFile(target, os.O_CREATE|os.O_RDWR, 0755)
			if err != nil {
				logrus.WithError(err).Errorf("Error opening file while unpacking new version package. Target: %s", target)
				return ErrCreatingFileWhileUnpacking
			}
			defer f.Close()

			// copy over contents
			if _, err := io.Copy(f, tr); err != nil {
				logrus.WithError(err).Errorf("Failed to copy file contents from archive. Target: %s", target)
				return ErrCreatingFileWhileUnpacking
			}
		}
	}
}

func (d *Client) DownloadAndUnpack(fileURL *url.URL, targetDirectory string) error {
	req, err := http.NewRequest("GET", fileURL.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("content-type", "application/octet-stream")
	resp, err := d.client.Do(req)
	if err != nil {
		logrus.WithError(err).Error("Package download request failed")
		return ErrDowloadPackageFailed
	}
	if resp.StatusCode != http.StatusOK {
		logrus.WithField("statusCode", resp.StatusCode).Error("Download and unpack: non-OK response received")
		return ErrDowloadPackageFailed
	}
	logrus.WithField("statusCode", resp.StatusCode).Info("Download and unpack: response received")
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logrus.WithError(err).Error("Failed to read package download response body")
		return ErrBadPackageDownloadResponse
	}
	err = d.extractTarGzToDir(targetDirectory, body)
	if err != nil {
		return err
	}
	logrus.Info("Download and unpack successful")

	return nil
}

func New(fs afero.Fs) *Client {
	return &Client{
		client: &http.Client{},
		Fs:     fs,
	}
}
