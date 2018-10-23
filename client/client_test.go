package client

import (
	"errors"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSchemeConversion(t *testing.T) {
	for _, tc := range []struct {
		useHTTPS    bool
		url         string
		expectedURL string
	}{
		{
			useHTTPS:    false,
			url:         "http://example.com?x=y",
			expectedURL: "http://example.com?x=y",
		},
		{
			useHTTPS:    false,
			url:         "https://example.com?x=y",
			expectedURL: "http://example.com?x=y",
		},
		{
			useHTTPS:    true,
			url:         "http://example.com?x=y",
			expectedURL: "https://example.com?x=y",
		},
		{
			useHTTPS:    true,
			url:         "http://example.com?x=y",
			expectedURL: "https://example.com?x=y",
		},
	} {
		mockTransport := &mockTransport{}
		rt := &httpsRoundTripper{mockTransport, tc.useHTTPS}
		client := &http.Client{Transport: rt}
		client.Get(tc.url)
		expectedURL, err := url.Parse(tc.expectedURL)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, expectedURL, mockTransport.usedURL,
			"Test: %+v", tc)
	}
}

type mockTransport struct {
	usedURL *url.URL
}

func (f *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	f.usedURL = req.URL
	return nil, errors.New("No implementation needed")
}

//http://example.com?x=y
//http://example.com?x=y
