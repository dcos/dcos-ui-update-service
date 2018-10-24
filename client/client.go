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
	client           *http.Client
	clientAuthHeader string
	hasIAM           bool
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

func (h *HTTP) Do(req *http.Request) (*http.Response, error) {
	if !h.hasIAM && h.clientAuthHeader != "" {
		req.Header.Set("Authorization", h.clientAuthHeader)
	}
	return h.client.Do(req)
}

func (h *HTTP) SetClientAuth(clientAuth string) {
	h.clientAuthHeader = clientAuth
}

func (h *HTTP) ClearClientAuth() {
	h.clientAuthHeader = ""
}

// New returns a new http.Client that handles setting the authentication
// header appropriately for the dcos-ui-service account if IAM is configured.
func New(cfg *config.Config) (*HTTP, error) {
	transportOptions := []transport.OptionTransportFunc{}
	if cfg.CACertFile != "" {
		transportOptions = append(transportOptions, transport.OptionCaCertificatePath(cfg.CACertFile))
	}
	if cfg.IAMConfig != "" {
		transportOptions = append(transportOptions, transport.OptionIAMConfigPath(cfg.IAMConfig))
	}
	tr, err := transport.NewTransport(transportOptions...)
	if err != nil {
		return nil, fmt.Errorf("Unable to initialize HTTP transport: %s", err)
	}
	hasIAM := cfg.IAMConfig != ""
	client := &http.Client{
		Transport: tr,
		Timeout:   cfg.HTTPClientTimeout,
	}
	return &HTTP{client, "", hasIAM}, nil
}

func NewClient(client *http.Client) *HTTP {
	return &HTTP{client, "", false}
}
