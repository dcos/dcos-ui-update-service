package fileHandler

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestUIFileHandler(t *testing.T) {
	t.Run("serves static files", func(t *testing.T) {
		handler := NewUIFileHandler("/static/", "../public")

		req, err := http.NewRequest("GET", "/static/", nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusOK {
			t.Errorf("handler returned wrong status code: got %v wanted %v",
				status, http.StatusOK)
		}

		expected := `<h1>Test</h1>`
		if rr.Body.String() != expected {
			t.Errorf("handler returned unexpected body: got %v want %v",
				rr.Body.String(), expected)
		}
	})

	t.Run("Can change documentRoot at runtime", func(t *testing.T) {
		handler := NewUIFileHandler("/static/", "../public")

		req1, err := http.NewRequest("GET", "/static/test.html", nil)
		if err != nil {
			t.Fatal(err)
		}

		resp1 := httptest.NewRecorder()
		handler.ServeHTTP(resp1, req1)

		if status := resp1.Code; status != http.StatusNotFound {
			t.Errorf("handler returned wrong status code: got %v wanted %v",
				status, http.StatusNotFound)
		}

		err = handler.UpdateDocumentRoot("../testdata/docroot/public")
		if err != nil {
			t.Errorf("UpdateDocumentRoot returned an err when expecting nil, %v", err)
		}

		req2, err := http.NewRequest("GET", "/static/test.html", nil)
		if err != nil {
			t.Fatal(err)
		}

		resp2 := httptest.NewRecorder()
		handler.ServeHTTP(resp2, req2)

		if status := resp2.Code; status != http.StatusOK {
			t.Errorf("handler returned wrong status code: got %v wanted %v",
				status, http.StatusOK)
		}

		got, err := ioutil.ReadAll(resp2.Body)
		if err != nil {
			t.Fatal(err)
		}
		documentRoot := handler.DocumentRoot()
		exp, err := ioutil.ReadFile(filepath.Join(documentRoot, "test.html"))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != string(exp) {
			t.Errorf("Expected %q but got %q", string(exp), string(got))
		}
	})

	t.Run("Can change documentRoot at runtime", func(t *testing.T) {
		handler := NewUIFileHandler("/static/", "../public")
		err := handler.UpdateDocumentRoot("./does-not-exist")
		if !os.IsNotExist(err) {
			t.Errorf("expected UpdateDocumentRoot to return NotExist err, instead got %v", err)
		}
	})

	t.Run("GetAssetPrefix returns expected value", func(t *testing.T) {
		expAssetPrefix := "/static/"
		handler := NewUIFileHandler("/static/", "../public")
		assetPrefix := handler.AssetPrefix()
		if assetPrefix != expAssetPrefix {
			t.Errorf("GetAssetPrefix returned %v, but expected %v", assetPrefix, expAssetPrefix)
		}
	})

	t.Run("GetDocumentRoot returns expected value", func(t *testing.T) {
		expDocRoot := "../public"
		handler := NewUIFileHandler("/static/", "../public")
		docRoot := handler.DocumentRoot()
		if docRoot != expDocRoot {
			t.Errorf("GetDocumentRoot returned %v, but expected %v", docRoot, expDocRoot)
		}
	})

	t.Run("GetDocumentRoot returns expected value after update", func(t *testing.T) {
		expDocRoot1 := "../public"
		expDocRoot2 := "../testdata/docroot/public"
		handler := NewUIFileHandler("/static/", "../public")
		docRoot := handler.DocumentRoot()
		if docRoot != expDocRoot1 {
			t.Errorf("GetDocumentRoot returned %v, but expected %v", docRoot, expDocRoot1)
		}
		handler.UpdateDocumentRoot("../testdata/docroot/public")
		docRoot = handler.DocumentRoot()
		if docRoot != expDocRoot2 {
			t.Errorf("GetDocumentRoot returned %v, but expected %v", docRoot, expDocRoot2)
		}
	})
}
