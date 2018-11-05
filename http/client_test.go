package http

import net_http "net/http"

import (
	"net/http/httptest"
	"testing"
)

func makeFakeServer(expectedAuthHeader string) *httptest.Server {
	server := httptest.NewServer(net_http.HandlerFunc(func(rw net_http.ResponseWriter, req *net_http.Request) {
		receivedAuth := req.Header.Get("Authorization")
		if receivedAuth != expectedAuthHeader {
			rw.WriteHeader(net_http.StatusBadRequest)
		} else {
			rw.WriteHeader(net_http.StatusOK)
		}
	}))

	return server
}

func TestClientUsesAuth(t *testing.T) {
	t.Parallel()

	t.Run("uses clientAuthHeader if set", func(t *testing.T) {
		authValue := "testing123"
		server := makeFakeServer(authValue)
		defer server.Close()

		client := NewClient(server.Client())
		client.SetClientAuth(authValue)

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

	t.Run("clears clientAuthHeader as expected", func(t *testing.T) {
		var authValue string
		server := makeFakeServer(authValue)
		defer server.Close()

		client := NewClient(server.Client())
		client.SetClientAuth("testing123")
		client.ClearClientAuth()

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
}
