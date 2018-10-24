package client

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func makeMockServer(expectedAuthHeader string) *httptest.Server {
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		receivedAuth := req.Header.Get("Authorization")
		if receivedAuth != expectedAuthHeader {
			rw.WriteHeader(http.StatusBadRequest)
		} else {
			rw.WriteHeader(http.StatusOK)
		}
	}))

	return server
}

func TestClientUsesAuth(t *testing.T) {
	t.Run("uses clientAuthHeader if set", func(t *testing.T) {
		authValue := "testing123"
		server := makeMockServer(authValue)
		defer server.Close()

		client := NewClient(server.Client())
		client.SetClientAuth(authValue)

		req, err := http.NewRequest("GET", server.URL+"/", nil)
		if err != nil {
			t.Fatalf("Expected NewRequest err to be nil, got %v", err)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Expected Do err to be nil, got %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected Auth header didn't match, mock server returned %v", resp.StatusCode)
		}
	})

	t.Run("clears clientAuthHeader as expected", func(t *testing.T) {
		authValue := ""
		server := makeMockServer(authValue)
		defer server.Close()

		client := NewClient(server.Client())
		client.SetClientAuth("testing123")
		client.ClearClientAuth()

		req, err := http.NewRequest("GET", server.URL+"/", nil)
		if err != nil {
			t.Fatalf("Expected NewRequest err to be nil, got %v", err)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Expected Do err to be nil, got %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected Auth header didn't match, mock server returned %v", resp.StatusCode)
		}
	})
}
