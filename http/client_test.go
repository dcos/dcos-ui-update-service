package http

import (
	net_http "net/http"
	"net/http/httptest"
	"testing"
)

func makeFakeServer(expectedHeaders map[string]string) *httptest.Server {
	server := httptest.NewServer(net_http.HandlerFunc(func(rw net_http.ResponseWriter, req *net_http.Request) {
		foundAllHeaders := true
		for k, v := range expectedHeaders {
			recHeader := req.Header.Get(k)
			if recHeader != v {
				foundAllHeaders = false
			}
		}

		if foundAllHeaders {
			rw.WriteHeader(net_http.StatusOK)
		} else {
			rw.WriteHeader(net_http.StatusBadRequest)
		}
	}))

	return server
}

func makeHeader(values map[string]string) net_http.Header {
	result := net_http.Header{}
	for k, v := range values {
		result.Set(k, v)
	}
	return result
}

func TestSetRequestHeaders(t *testing.T) {
	t.Parallel()

	t.Run("uses authorization if set", func(t *testing.T) {
		headers := map[string]string{"Authorization": "testing123"}
		server := makeFakeServer(headers)
		defer server.Close()

		client := NewClient(server.Client())
		client.SetRequestHeaders(makeHeader(headers))

		req, err := net_http.NewRequest("GET", server.URL+"/", nil)
		if err != nil {
			t.Fatalf("Expected NewRequest err to be nil, got %v", err)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Expected Do err to be nil, got %v", err)
		}
		if resp.StatusCode != net_http.StatusOK {
			t.Errorf("Expected Auth header didn't match, mock server returned %v", resp.StatusCode)
		}
	})

	t.Run("clears authorization as expected", func(t *testing.T) {
		server := makeFakeServer(map[string]string{"Authorization": ""})
		defer server.Close()

		client := NewClient(server.Client())
		client.SetRequestHeaders(makeHeader(map[string]string{"Authorization": "testing_123"}))
		client.ClearRequestHeaders()

		req, err := net_http.NewRequest("GET", server.URL+"/", nil)
		if err != nil {
			t.Fatalf("Expected NewRequest err to be nil, got %v", err)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Expected Do err to be nil, got %v", err)
		}
		if resp.StatusCode != net_http.StatusOK {
			t.Errorf("Expected Auth header didn't match, mock server returned %v", resp.StatusCode)
		}
	})

	t.Run("uses cookie if set", func(t *testing.T) {
		headers := map[string]string{"Cookie": "testing123"}
		server := makeFakeServer(headers)
		defer server.Close()

		client := NewClient(server.Client())
		client.SetRequestHeaders(makeHeader(headers))

		req, err := net_http.NewRequest("GET", server.URL+"/", nil)
		if err != nil {
			t.Fatalf("Expected NewRequest err to be nil, got %v", err)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Expected Do err to be nil, got %v", err)
		}
		if resp.StatusCode != net_http.StatusOK {
			t.Errorf("Expected header didn't match, mock server returned %v", resp.StatusCode)
		}
	})

	t.Run("uses supports multiple Cookies if set", func(t *testing.T) {
		headers := map[string]string{"Cookie": `ajs_user_id=null; ajs_group_id=null; dcos-acs-auth-cookie=testing123; dcos-acs-info-cookie="this is a test"`}
		server := makeFakeServer(headers)
		defer server.Close()

		client := NewClient(server.Client())
		client.SetRequestHeaders(makeHeader(headers))

		req, err := net_http.NewRequest("GET", server.URL+"/", nil)
		if err != nil {
			t.Fatalf("Expected NewRequest err to be nil, got %v", err)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Expected Do err to be nil, got %v", err)
		}
		if resp.StatusCode != net_http.StatusOK {
			t.Errorf("Expected header didn't match, mock server returned %v", resp.StatusCode)
		}
	})

	t.Run("does not use non-white listed headers", func(t *testing.T) {
		headers := map[string]string{"otherThing": "testing123"}
		server := makeFakeServer(headers)
		defer server.Close()

		client := NewClient(server.Client())
		client.SetRequestHeaders(makeHeader(headers))

		req, err := net_http.NewRequest("GET", server.URL+"/", nil)
		if err != nil {
			t.Fatalf("Expected NewRequest err to be nil, got %v", err)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Expected Do err to be nil, got %v", err)
		}
		if resp.StatusCode != net_http.StatusBadRequest {
			t.Errorf("Expected header didn't match, mock server returned %v", resp.StatusCode)
		}
	})
}
