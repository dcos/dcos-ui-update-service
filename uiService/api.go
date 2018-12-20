package uiService

import (
	"fmt"
	"net/http"

	"github.com/dcos/dcos-ui-update-service/updateManager"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func newRouter(service *UIService) *mux.Router {
	assetPrefix := service.UIHandler.AssetPrefix()

	r := mux.NewRouter()
	r.HandleFunc("/api/v1/", notImplementedHandler)
	r.HandleFunc("/api/v1/update/{version}/", updateHandler(service))
	r.HandleFunc("/api/v1/reset/", resetToDefaultUIHandler(service)).Methods("DELETE")
	r.PathPrefix(assetPrefix).Handler(service.UIHandler)

	return r
}

// NotImplementedHandler writes a HTTP Not Implemented response
func notImplementedHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
}

// UpdateHandler processes update requests
func updateHandler(service *UIService) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		version := vars["version"]

		// Check for empty version
		if len(version) == 0 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		service.Lock()
		if service.updating {
			if version == service.updatingVersion {
				http.Error(w, "Service is currently processing an update request", http.StatusAccepted)
			} else {
				http.Error(
					w,
					fmt.Sprintf("Service is currently processing an update request to %s", service.updatingVersion),
					http.StatusConflict,
				)
			}
			service.Unlock()

			return
		}
		service.updating = true
		service.updatingVersion = version
		service.Unlock()
		defer unlockServiceFromUpdate(service)

		err := service.UpdateManager.UpdateToVersion(version, func(newVersionPath string) error {
			updateErr := service.UIHandler.UpdateDocumentRoot(newVersionPath)
			if updateErr != nil {
				return errors.Wrap(updateErr, "unable to update the document root to the new version")
			}
			newUIVersion := UIVersion(version)
			updateErr = service.VersionStore.UpdateCurrentVersion(newUIVersion)
			if updateErr != nil {
				return errors.Wrap(updateErr, "unable to save new version to the version store")
			}
			return nil
		})

		switch err {
		case nil:
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(fmt.Sprintf("Update to %s completed", version)))
			return
		case updateManager.ErrRequestedVersionNotFound:
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		default:
			logrus.WithFields(logrus.Fields{
				"version": version,
				"err":     err,
			}).Error("Update failed")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}

// ResetToDefaultUIHandler processes requests to reset to the default ui
func resetToDefaultUIHandler(service *UIService) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		// verify we aren't currently serving pre-bundled version
		if service.Config.DefaultDocRoot == service.UIHandler.DocumentRoot() {
			w.WriteHeader(http.StatusOK)
			return
		}
		err := service.UIHandler.UpdateDocumentRoot(service.Config.DefaultDocRoot)
		if err != nil {
			logrus.WithError(err).Error("Failed to reset to default document root")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		storeErr := service.VersionStore.UpdateCurrentVersion(PreBundledUIVersion)
		if storeErr != nil {
			logrus.WithError(storeErr).Error("Failed to update the version store to the PreBundledUIVersion.")
		}

		err = service.UpdateManager.ResetVersion()
		if err != nil {
			logrus.WithError(err).Error("Failed to remove current version when resetting to default document root")
		}

		w.WriteHeader(http.StatusOK)
	}
}
