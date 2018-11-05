package http

import net_http "net/http"

import (
	"fmt"

	"github.com/dcos/dcos-go/dcos/http/transport"
	"github.com/dcos/dcos-ui-update-service/config"
)

// Client is a convenience wrapper around an httpClient
type Client struct {
	client           *net_http.Client
	clientAuthHeader string
	hasIAM           bool
}

func (h *Client) Do(req *net_http.Request) (*net_http.Response, error) {
	if !h.hasIAM && h.clientAuthHeader != "" {
		req.Header.Set("Authorization", h.clientAuthHeader)
	}
	return h.client.Do(req)
}

func (h *Client) SetClientAuth(clientAuth string) {
	h.clientAuthHeader = clientAuth
}

func (h *Client) ClearClientAuth() {
	h.clientAuthHeader = ""
}

// New returns a new http.Client that handles setting the authentication
// header appropriately for the dcos-ui-service account if IAM is configured.
func New(cfg *config.Config) (*Client, error) {
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
	client := &net_http.Client{
		Transport: tr,
		Timeout:   cfg.HTTPClientTimeout,
	}
	return &Client{
		client:           client,
		clientAuthHeader: "",
		hasIAM:           hasIAM,
	}, nil
}

func NewClient(client *net_http.Client) *Client {
	return &Client{
		client:           client,
		clientAuthHeader: "",
		hasIAM:           false,
	}
}
