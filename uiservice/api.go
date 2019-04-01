package uiservice

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/dcos/dcos-ui-update-service/constants"
	"github.com/dcos/dcos-ui-update-service/updatemanager"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func newRouter(service *UIService) *mux.Router {
	r := mux.NewRouter()
	r.HandleFunc("/api/v1/", notImplementedHandler)
	r.HandleFunc("/api/v1/version/", versionHandler(service)).Methods("GET")
	r.HandleFunc("/api/v1/update/{version}/", updateHandler(service)).Methods("POST")
	r.HandleFunc("/api/v1/reset/", leadUIResetHandler(service)).Methods("DELETE")

	return r
}

func notImplementedHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
}

type versionResponse struct {
	Default        bool   `json:"default"`
	PackageVersion string `json:"packageVersion"`
	BuildVersion   string `json:"buildVersion"`
}

func versionHandler(service *UIService) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		version, err := service.UpdateManager.CurrentVersion()
		if err != nil {
			logrus.WithError(err).Error("Could not get current version.")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		buildVersion, err := buildVersionFromUIIndex(service.Config.UIDistSymlink())
		if err != nil {
			logrus.WithError(err).Warn("Failed to read version from UI Dist")
			buildVersion = ""
		}

		var response versionResponse
		if len(version) > 0 {
			response = versionResponse{false, version, buildVersion}
		} else {
			response = versionResponse{true, "Default", buildVersion}
		}
		js, err := json.Marshal(response)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(js)
	}
}

func updateHandler(service *UIService) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		version := vars["version"]
		logrus.WithField("version", version).Debug("Received update request.")

		// Check for empty version
		if len(version) == 0 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if updatingVersion, err := setServiceUpdating(service, version); err != nil {
			if version == updatingVersion {
				http.Error(
					w,
					"Service is currently processing an update request",
					http.StatusAccepted,
				)
			} else {
				http.Error(
					w,
					fmt.Sprintf("Service is currently processing an update request to %s", service.updatingVersion),
					http.StatusConflict,
				)
			}
			return
		}
		defer resetServiceFromUpdate(service)

		err := service.UpdateManager.UpdateToVersion(version, func(newVersionPath string) error {
			updateErr := updateServedVersion(service, newVersionPath)
			if updateErr != nil {
				return errors.Wrap(updateErr, "unable to update the ui dist symlink to the new version")
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
			w.Header().Add("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(fmt.Sprintf("Update to %s completed", version)))
			return
		case updatemanager.ErrRequestedVersionNotFound:
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

func leadUIResetHandler(service *UIService) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if updatingVersion, lockErr := setServiceUpdating(service, ""); lockErr != nil {
			var message string
			if UIVersion(updatingVersion) == PreBundledUIVersion {
				message = "Cannot process reset, another reset is currently in progress."
			} else {
				message = "Cannot process reset, an update is currently in progress."
			}
			logrus.WithError(lockErr).Error(message)

			w.Header().Add("Content-Type", "text/plain")
			w.WriteHeader(http.StatusConflict)
			w.Write([]byte(message))
			return
		}
		defer resetServiceFromUpdate(service)

		logrus.Debug("Received reset request.")
		timeoutChannel := make(chan struct{})
		resetComplete := service.UpdateManager.LeadUIReset(timeoutChannel)
		var response *updatemanager.UpdateResult
		select {
		case response = <-resetComplete:
			logrus.WithField("result", response).Info("UI Reset Result")
			break
		case <-time.After(constants.UpdateOperationTimeout):
			logrus.Info("UI Reset Timedout")
			response = &updatemanager.UpdateResult{
				Operation:  "reset",
				Successful: false,
				Message:    "Reset operation timed out",
			}
			close(timeoutChannel)
			break
		}
		if response.Successful {
			logrus.WithField("message", response.Message).Info("Syncronized reset completed successfully")
			go func() {
				storeErr := service.VersionStore.UpdateCurrentVersion(PreBundledUIVersion)
				if storeErr != nil {
					logrus.WithError(storeErr).Error("Failed to update the version store to the PreBundledUIVersion.")
				}
			}()

			w.Header().Add("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		} else {
			entry := logrus.WithField("message", response.Message)
			if response.Error != nil {
				entry = entry.WithError(response.Error)
			}
			entry.Warning("Syncronized reset failed")

			w.Header().Add("Content-Type", "text/plain")
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(response.Message))
		}
	}
}
