package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRouter(t *testing.T) {

	t.Run("serves static files", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/static/", nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		Router().ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusOK {
			t.Errorf("handler returned wrong status code: got %v want %v",
				status, http.StatusOK)
		}

		expected := `<h1>Test</h1>`
		if rr.Body.String() != expected {
			t.Errorf("handler returned unexpected body: got %v want %v",
				rr.Body.String(), expected)
		}
	})

	t.Run("Status codes", func(t *testing.T) {
		var testCases = []struct {
			name       string
			uri        string
			statusCode int
		}{
			{"returns with 404 on root", "/", http.StatusNotFound},
			{"returns with 200 on static", "/static/", http.StatusOK},
			{"returns with 501 on api/v1", "/api/v1/", http.StatusNotImplemented},
			{"returns with 404 on api", "/api", http.StatusNotFound},
		}

		for _, tt := range testCases {
			t.Run(tt.name, func(t *testing.T) {
				req, err := http.NewRequest("GET", tt.uri, nil)
				if err != nil {
					t.Fatal(err)
				}

				rr := httptest.NewRecorder()
				Router().ServeHTTP(rr, req)

				if rr.Code != tt.statusCode {
					t.Errorf("handler for %v returned unexpected statuscode: got %v want %v",
						tt.uri, rr.Code, tt.statusCode)
				}

			})
		}
	})
}
