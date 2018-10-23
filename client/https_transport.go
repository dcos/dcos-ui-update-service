package client

import "net/http"

// forces urls to use https:// if necessary
type httpsRoundTripper struct {
	delegate http.RoundTripper
	useHTTPS bool
}

func (h *httpsRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	url := req.URL
	if h.useHTTPS {
		url.Scheme = "https"
	} else {
		url.Scheme = "http"
	}
	return h.delegate.RoundTrip(req)
}
