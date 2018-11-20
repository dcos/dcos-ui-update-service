package main

import (
	"fmt"
	"net"
	"net/http"
	"os"

	"github.com/coreos/go-systemd/activation"
	"github.com/dcos/dcos-ui-update-service/config"
	"github.com/dcos/dcos-ui-update-service/dcos"
	"github.com/dcos/dcos-ui-update-service/fileHandler"
	"github.com/dcos/dcos-ui-update-service/uiService"
	"github.com/dcos/dcos-ui-update-service/updateManager"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type UIService struct {
	Config *config.Config

	UIHandler *fileHandler.UIFileHandler

	UpdateManager *updateManager.Client

	MasterCounter dcos.MasterCounter

	versionStore uiService.VersionStore
}

// SetupUIHandler create UIFileHandler for service ui and set default directory to
// the current downloaded version or the default document root
func SetupUIHandler(cfg *config.Config, um *updateManager.Client) *fileHandler.UIFileHandler {
	documentRoot := cfg.DefaultDocRoot
	currentVersionPath, err := um.PathToCurrentVersion()
	if err == nil {
		documentRoot = currentVersionPath
	}
	return fileHandler.NewUIFileHandler(cfg.StaticAssetPrefix, documentRoot)
}

func setup(args []string) (*UIService, error) {
	cfg := config.Parse(args)

	// Set logging level
	lvl, err := logrus.ParseLevel(cfg.LogLevel)
	if err != nil {
		logrus.Fatal(err)
	}
	logrus.SetLevel(lvl)

	updateManager, err := updateManager.NewClient(cfg)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create update manager")
	}
	uiHandler := SetupUIHandler(cfg, updateManager)
	dcos := dcos.DCOS{
		MasterCountLocation: cfg.MasterCountFile,
	}

	version, err := updateManager.CurrentVersion()
	if err != nil {
		logrus.WithFields(logrus.Fields{"err": err}).Warn("Error retrieving the current package version from update manager")
	} else {
		logrus.WithFields(logrus.Fields{"version": version}).Info("Current package version")
	}
	
	versionStore := uiService.NewZKVersionStore(cfg)

	service := &UIService{
		Config:        cfg,
		UpdateManager: updateManager,
		UIHandler:     uiHandler,
		MasterCounter: dcos,
		versionStore:  versionStore,
	}

	return service, nil
}

// TODO: think about client timeouts https://blog.cloudflare.com/the-complete-guide-to-golang-net-http-timeouts/
func main() {
	cliArgs := os.Args[1:]
	service, err := setup(cliArgs)
	if err != nil {
		logrus.WithFields(logrus.Fields{"err": err.Error()}).Fatal("Failed to initiate ui service")
	}
	// Use systemd socket activation.
	l, err := activation.Listeners()
	if err != nil {
		logrus.WithFields(logrus.Fields{"err": err.Error()}).Fatal("Failed to activate listeners from systemd")
	}

	var listener net.Listener
	switch numListeners := len(l); numListeners {
	case 0:
		logrus.Info("Did not receive any listeners from systemd, will start with configured listener instead.")
		listener, err = net.Listen(service.Config.ListenNetProtocol, service.Config.ListenNetAddress)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"connections": service.Config.ListenNetProtocol,
				"address":     service.Config.ListenNetAddress,
				"err":         err.Error(),
			}).Fatal("Cannot listen for connections")
		}
		logrus.WithFields(logrus.Fields{"net": service.Config.ListenNetProtocol, "Addr": service.Config.ListenNetAddress}).Info("Listening")
	case 1:
		listener = l[0]
		logrus.WithFields(logrus.Fields{"socket": listener.Addr()}).Info("Listening on systemd")
	default:
		logrus.Fatal("Found multiple systemd sockets.")
	}

	registerForVersionChanges(service)
	if err := Run(service, listener); err != nil {
		logrus.WithFields(logrus.Fields{"err": err.Error()}).Fatal("Application error")
	}
}

// Run serves the API based on the Config and Listener provided
func Run(service *UIService, l net.Listener) error {
	r := newRouter(service)
	loggedRouter := handlers.LoggingHandler(os.Stdout, r)
	http.Handle("/", loggedRouter)
	return http.Serve(l, loggedRouter)
}

func newRouter(service *UIService) *mux.Router {
	assetPrefix := service.UIHandler.AssetPrefix()

	r := mux.NewRouter()
	r.HandleFunc("/api/v1/", NotImplementedHandler)
	r.HandleFunc("/api/v1/update/{version}/", UpdateHandler(service))
	r.HandleFunc("/api/v1/reset/", ResetToDefaultUIHandler(service)).Methods("DELETE")
	r.PathPrefix(assetPrefix).Handler(service.UIHandler)

	return r
}

// NotImplementedHandler writes a HTTP Not Implemented response
func NotImplementedHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
}

// UpdateHandler processes update requests
func UpdateHandler(service *UIService) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		version := vars["version"]

		// Check for empty version
		if len(version) == 0 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		err := service.UpdateManager.UpdateToVersion(version, func(newVersionPath string) error {
			updateErr := service.UIHandler.UpdateDocumentRoot(newVersionPath)
			if updateErr != nil {
				return errors.Wrap(updateErr, "unable to update the document root to the new version")
			}
			newUIVersion := uiService.UIVersion(version)
			updateErr = service.versionStore.UpdateCurrentVersion(newUIVersion)
			if updateErr != nil {
				return errors.Wrap(updateErr, "unable to save new version to the version store")
			}
			return nil
		})

		if err != nil {
			logrus.WithFields(logrus.Fields{
				"version": version,
				"err":     err,
			}).Error("Update failed")
			// This returns locked on every error, it would be better if we would return a boolean if the process is locked
			w.WriteHeader(http.StatusLocked)
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}

// ResetToDefaultUIHandler processes requests to reset to the default ui
func ResetToDefaultUIHandler(service *UIService) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		// verify we aren't currently serving pre-bundled version
		if service.Config.DefaultDocRoot == service.UIHandler.DocumentRoot() {
			w.WriteHeader(http.StatusOK)
			return
		}
		err := service.UIHandler.UpdateDocumentRoot(service.Config.DefaultDocRoot)
		if err != nil {
			logrus.WithFields(logrus.Fields{"err": err}).Error("Failed to reset to default document root")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		storeErr := service.versionStore.UpdateCurrentVersion(uiService.PreBundledUIVersion)
		if storeErr != nil {
			fmt.Printf("Failed to update the version store the the PreBundledUIVersion. %#v", storeErr)
		}

		err = service.UpdateManager.ResetVersion()
		if err != nil {
			logrus.WithFields(logrus.Fields{"err": err}).Error("Failed to remove current version when resetting to default document root")
		}

		w.WriteHeader(http.StatusOK)
	}
}

func registerForVersionChanges(service *UIService) {
	service.versionStore.WatchForVersionChange(func(newVersion uiService.UIVersion) {
		handleVersionChange(service, string(newVersion))
	})
}

func handleVersionChange(service *UIService, newVersion string) {
	fmt.Printf("received version '%v' from version store.\n", newVersion) //TODO: Info
	currentLocalVersion, err := service.UpdateManager.CurrentVersion()
	if err != nil {
		fmt.Printf("Failed to handle version change, error getting the current local version. Error: %v\n", err.Error())
		return
	}
	if currentLocalVersion != newVersion {
		fmt.Printf(
			"new version '%v' doesn't match current version '%v'. Initiating a version sync.\n",
			newVersion,
			currentLocalVersion) //TODO: Info

		if newVersion == "" {
			// Reset to Pre-bundled version
			err := service.UIHandler.UpdateDocumentRoot(service.Config.DefaultDocRoot)
			if err != nil {
				fmt.Printf("Failed to reset to default document root. %#v", err)
				return
			}

			err = service.UpdateManager.ResetVersion()
			if err != nil {
				fmt.Printf("Failed to removed current version when reseting to default document root. %#v", err)
				return
			}

			fmt.Printf("Successfully reset to default document root from on version sync.")
			return
		}

		err := service.UpdateManager.UpdateToVersion(newVersion, func(newVersionPath string) error {
			updateErr := service.UIHandler.UpdateDocumentRoot(newVersionPath)
			if updateErr != nil {
				return errors.Wrap(updateErr, "unable to update the document root to the new version")
			}
			return nil
		})

		if err != nil {
			fmt.Printf("Version sync to version '%s' failed: %#v\n", newVersion, err)
			return
		}

		fmt.Printf("Version sync to version '%s' completed successfully\n", newVersion)
	}
}
