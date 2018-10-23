package client

import (
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/dcos/dcos-go/dcos/http/transport"
	"github.com/dcos/dcos-ui-update-service/config"
	"github.com/pkg/errors"
)

// HTTP is a convenience wrapper around an httpClient
type HTTP struct {
	http.Client
}

func (h *HTTP) Read(resp *http.Response, err error) (*HTTPResult, error) {
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, errors.New("Nil response received")
	}
	defer resp.Body.Close()
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Could not read response body: %s", err)
	}
	return &HTTPResult{
		Code: resp.StatusCode,
		Body: b,
	}, nil
}

// New returns a new http.Client that handles setting the authentication
// header appropriately for the dcos-ui-update-service account. It also sets
// the url scheme to use http vs. https based on whether or not
// config.CACertFile was set.
func New(cfg *config.Config) (*HTTP, error) {
	roundTripper, err := getTransport(cfg)
	if err != nil {
		return nil, errors.Wrap(err, "could not get transport")
	}
	client := http.Client{
		Transport: roundTripper,
		Timeout:   cfg.HTTPClientTimeout,
	}
	return &HTTP{client}, nil
}

func getTransport(cfg *config.Config) (http.RoundTripper, error) {
	useHTTPS := cfg.CACertFile != ""
	transportOptions := []transport.OptionTransportFunc{}
	if useHTTPS {
		transportOptions = append(transportOptions, transport.OptionCaCertificatePath(cfg.CACertFile))
	}
	if cfg.IAMConfig != "" {
		transportOptions = append(transportOptions, transport.OptionIAMConfigPath(cfg.IAMConfig))
	}
	tr, err := transport.NewTransport(transportOptions...)
	if err != nil {
		return nil, fmt.Errorf("Unable to initialize HTTP transport: %s", err)
	}
	return &httpsRoundTripper{tr, useHTTPS}, nil
}
