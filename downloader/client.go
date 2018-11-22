package downloader

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
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
		return fmt.Errorf("Error unzipping the payload")
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
					return errors.Wrap(err, fmt.Sprintf("error making directory %s", target))
				}
			}

		// if it's a file create it
		case tar.TypeReg:
			logrus.Infof("Extract tar.gz to directory: Creating file - %s", target)
			f, err := d.Fs.OpenFile(target, os.O_CREATE|os.O_RDWR, 0755)
			if err != nil {
				return errors.Wrap(err, fmt.Sprintf("error opening file for writing %s", target))
			}
			defer f.Close()

			// copy over contents
			if _, err := io.Copy(f, tr); err != nil {
				return errors.Wrap(err, fmt.Sprintf("error copying file contents to archive %s", target))
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
		return err
	}
	logrus.WithFields(logrus.Fields{"statusCode": resp.StatusCode}).Info("Download and unpack: response received")
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download package %v", resp.StatusCode)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("could not read response body: %s", err)
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
