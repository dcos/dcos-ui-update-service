package uiService

import (
	"net"
	"net/http"
	"os"
	"sync"

	"github.com/dcos/dcos-ui-update-service/config"
	"github.com/dcos/dcos-ui-update-service/dcos"
	"github.com/dcos/dcos-ui-update-service/fileHandler"
	"github.com/dcos/dcos-ui-update-service/updateManager"
	"github.com/gorilla/handlers"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"
)

type UIService struct {
	Config *config.Config

	UIHandler *fileHandler.UIFileHandler

	UpdateManager updateManager.UpdateManager

	MasterCounter dcos.MasterCounter

	VersionStore VersionStore

	updating bool

	updatingVersion string

	sync.Mutex
}

func SetupService(config *config.Config) (*UIService, error) {
	updateManager, err := updateManager.NewClient(config)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create update manager")
	}
	uiHandler := SetupUIHandler(config, updateManager)
	dcos := dcos.DCOS{
		MasterCountLocation: config.MasterCountFile,
	}

	version, err := updateManager.CurrentVersion()
	if err != nil {
		logrus.WithFields(logrus.Fields{"err": err}).Warn("Error retrieving the current package version from update manager")
	} else {
		logrus.WithFields(logrus.Fields{"version": version}).Info("Current package version")
	}

	versionStore := NewZKVersionStore(config)

	service := &UIService{
		Config:        config,
		UpdateManager: updateManager,
		UIHandler:     uiHandler,
		MasterCounter: dcos,
		VersionStore:  versionStore,
	}

	return service, nil
}

// SetupUIHandler create UIFileHandler for service ui and set default directory to
// the current downloaded version or the default document root
func SetupUIHandler(cfg *config.Config, um updateManager.UpdateManager) *fileHandler.UIFileHandler {
	documentRoot := cfg.DefaultDocRoot
	currentVersionPath, err := um.PathToCurrentVersion()
	if err == nil {
		documentRoot = currentVersionPath
	}
	return fileHandler.NewUIFileHandler(cfg.StaticAssetPrefix, documentRoot, afero.NewOsFs())
}

func (service *UIService) Run(l net.Listener) error {
	registerForVersionChanges(service)

	r := newRouter(service)
	loggedRouter := handlers.LoggingHandler(os.Stdout, r)
	http.Handle("/", loggedRouter)
	return http.Serve(l, loggedRouter)
}

func registerForVersionChanges(service *UIService) {
	service.VersionStore.WatchForVersionChange(func(newVersion UIVersion) {
		handleVersionChange(service, string(newVersion))
	})
}

func handleVersionChange(service *UIService, newVersion string) {
	logrus.WithFields(logrus.Fields{"version": newVersion}).Info("Received version change from version store.")
	currentLocalVersion, err := service.UpdateManager.CurrentVersion()
	if err != nil {
		logrus.WithError(err).Error("Failed to handle version change, error getting the current local version.")
		return
	}
	if currentLocalVersion != newVersion {
		logrus.WithFields(logrus.Fields{
			"newVersion":     newVersion,
			"currentVersion": currentLocalVersion,
		}).Info("Initiating a version sync.")

		if newVersion == "" {
			// Reset to Pre-bundled version
			err := service.UIHandler.UpdateDocumentRoot(service.Config.DefaultDocRoot)
			if err != nil {
				logrus.WithError(err).Error("Failed to reset to default document root.")
				return
			}

			err = service.UpdateManager.ResetVersion()
			if err != nil {
				logrus.WithError(err).Error("Failed to removed current version when reseting to default document root.")
				return
			}

			logrus.Info("Successfully reset to default document root from on version sync.")
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
			logrus.WithError(err).WithFields(logrus.Fields{"newVersion": newVersion}).Error("Version sync failed")
			return
		}

		logrus.WithFields(logrus.Fields{"newVersion": newVersion}).Info("Version sync completed successfully")
	}
}

func unlockServiceFromUpdate(service *UIService) {
	service.Lock()
	service.updating = false
	service.updatingVersion = ""
	service.Unlock()
}
