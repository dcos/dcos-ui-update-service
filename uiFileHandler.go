package main

import (
	"net/http"
	"os"
)

type UIFileHandler struct {
	assetPrefix  string
	documentRoot string
	fileHandler  http.Handler
}

func createFileHandler(assetPrefix, documentRoot string) http.Handler {
	return http.StripPrefix(assetPrefix, http.FileServer(http.Dir(documentRoot)))
}

// NewUIFileHandler create a new ui file handler that serves file for the given prefix and from the documentRoot
func NewUIFileHandler(assetPrefix, documentRoot string) UIFileHandler {
	fileHandler := createFileHandler(assetPrefix, documentRoot)

	return UIFileHandler{
		assetPrefix,
		documentRoot,
		fileHandler,
	}
}

// ServerHTTP handles service HTTP requests
func (ufh *UIFileHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ufh.fileHandler.ServeHTTP(w, r)
}

// UpdateDocumentRoot used to update the documentRoot and serve a new UI version
func (ufh *UIFileHandler) UpdateDocumentRoot(documentRoot string) error {
	// Ignore request if documentRoot matches the current documentRoot
	if documentRoot == ufh.documentRoot {
		return nil
	}
	// Check that new documentRoot exists before proceeding
	if _, err := os.Stat(documentRoot); os.IsNotExist(err) {
		return err
	}

	newFileHander := createFileHandler(ufh.assetPrefix, documentRoot)
	ufh.documentRoot = documentRoot
	ufh.fileHandler = newFileHander

	return nil
}

// GetAssetPrefix return the assetPrefix for this handler
func (ufh *UIFileHandler) GetAssetPrefix() string {
	return ufh.assetPrefix
}

// GetDocumentRoot returns the current documentRoot for this handler
func (ufh *UIFileHandler) GetDocumentRoot() string {
	return ufh.documentRoot
}