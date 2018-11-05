package http

import net_http "net/http"

import (
	"fmt"
	"strings"

	"github.com/dcos/dcos-go/dcos/http/transport"
	"github.com/dcos/dcos-ui-update-service/config"
)

// Client is a convenience wrapper around an httpClient
type Client struct {
	client     *net_http.Client
	reqHeaders net_http.Header
	hasIAM     bool
}

var (
	authorization = "authorization"
	cookie        = "cookie"

	clientHeaders = []string{authorization, cookie}
)

func includeInRequestHeaders(headerKey string) bool {
	lowerHeaderKey := strings.ToLower(headerKey)
	for _, hdr := range clientHeaders {
		if lowerHeaderKey == hdr {
			return true
		}
	}
	return false
}

func copyHeaders(src *net_http.Header, dest *net_http.Header) {
	for hdrKey, hdrValues := range *src {
		for _, value := range hdrValues {
			dest.Set(hdrKey, value)
		}
	}
}

func (c *Client) Do(req *net_http.Request) (*net_http.Response, error) {
	if !c.hasIAM && len(c.reqHeaders) > 0 {
		copyHeaders(&c.reqHeaders, &req.Header)
	}
	return c.client.Do(req)
}

func (c *Client) SetRequestHeaders(reqHeader net_http.Header) {
	c.reqHeaders = net_http.Header{}
	for hdrKey, hdrValues := range reqHeader {
		if includeInRequestHeaders(hdrKey) {
			for _, value := range hdrValues {
				c.reqHeaders.Set(hdrKey, value)
			}
		}
	}
}

func (c *Client) ClearRequestHeaders() {
	c.reqHeaders = net_http.Header{}
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
		client:     client,
		reqHeaders: net_http.Header{},
		hasIAM:     hasIAM,
	}, nil
}

func NewClient(client *net_http.Client) *Client {
	return &Client{
		client:     client,
		reqHeaders: net_http.Header{},
		hasIAM:     false,
	}
}
