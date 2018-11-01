package client

import (
	"fmt"
	"net/http"

	"github.com/dcos/dcos-go/dcos/http/transport"
	"github.com/dcos/dcos-ui-update-service/config"
)

// HTTP is a convenience wrapper around an httpClient
type HTTP struct {
	client           *http.Client
	clientAuthHeader string
	hasIAM           bool
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
	return &HTTP{
		client:           client,
		clientAuthHeader: "",
		hasIAM:           hasIAM,
	}, nil
}

func NewClient(client *http.Client) *HTTP {
	return &HTTP{
		client:           client,
		clientAuthHeader: "",
		hasIAM:           false,
	}
}
