package uiService

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"sync"

	"github.com/dcos/dcos-ui-update-service/config"
	"github.com/dcos/dcos-ui-update-service/dcos"
	"github.com/dcos/dcos-ui-update-service/updateManager"
	"github.com/gorilla/handlers"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type UIService struct {
	Config *config.Config

	UpdateManager updateManager.UpdateManager

	MasterCounter dcos.MasterCounter

	VersionStore VersionStore

	updating bool

	updatingVersion string

	sync.Mutex
}

func SetupService(cfg *config.Config) (*UIService, error) {
	updateManager, err := updateManager.NewClient(cfg)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create update manager")
	}
	dcos := dcos.DCOS{
		MasterCountLocation: cfg.MasterCountFile,
	}

	versionStore := NewZKVersionStore(cfg)

	service := &UIService{
		Config:        cfg,
		UpdateManager: updateManager,
		MasterCounter: dcos,
		VersionStore:  versionStore,
	}

	checkUIDistSymlink(cfg)
	checkCurrentVersion(updateManager)

	return service, nil
}

func (service *UIService) Run(l net.Listener) error {
	registerForVersionChanges(service)

	r := newRouter(service)
	loggedRouter := handlers.LoggingHandler(os.Stdout, r)
	http.Handle("/", loggedRouter)
	return http.Serve(l, loggedRouter)
}

func checkUIDistSymlink(cfg *config.Config) {
	uiDistTarget, err := os.Readlink(cfg.UIDistSymlink)
	if err != nil {
		if cfg.InitUIDistSymlink {
			logrus.Info("Attempting to initialize UI dist symlink")
			createErr := os.Symlink(cfg.DefaultDocRoot, cfg.UIDistSymlink)
			if createErr != nil {
				logrus.WithError(createErr).Error("Failed to initialize UI dist symlink")
			} else {
				logrus.WithFields(
					logrus.Fields{
						"UIDistSymlink":        cfg.UIDistSymlink,
						"UIDistSymlink-Target": cfg.DefaultDocRoot,
					},
				).Info("Current UI dist symlink target.")
			}
		} else {
			logrus.WithError(err).Error("Failed to read UI dist symlink")
		}
	} else {
		logrus.WithFields(
			logrus.Fields{
				"UIDistSymlink":        cfg.UIDistSymlink,
				"UIDistSymlink-Target": uiDistTarget,
			},
		).Info("Current UI dist symlink target")
	}
}

func checkCurrentVersion(updateManager *updateManager.Client) {
	version, err := updateManager.CurrentVersion()
	if err != nil {
		logrus.WithError(err).Warn("Error retrieving the current package version from update manager")
	} else if len(version) > 0 {
		logrus.WithFields(
			logrus.Fields{"version": version},
		).Info("Current package version")
	} else {
		logrus.WithFields(
			logrus.Fields{"version": "Default"},
		).Info("Current package version")
	}
}

func registerForVersionChanges(service *UIService) {
	service.VersionStore.WatchForVersionChange(func(newVersion UIVersion) {
		handleVersionChange(service, string(newVersion))
	})
}

func handleVersionChange(service *UIService, newVersion string) {
	logrus.WithFields(
		logrus.Fields{"version": newVersion},
	).Info("Received version change from version store.")
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
		_, err := setServiceUpdating(service, newVersion)
		if err != nil {
			logrus.WithError(err).Error("Failed to handle version change, could not lock service for update. ")
			return
		}
		defer resetServiceFromUpdate(service)

		if newVersion == "" {
			// Reset to Pre-bundled version
			err = updateServedVersion(service, service.Config.DefaultDocRoot)
			if err != nil {
				logrus.WithError(err).Error("Failed to reset to default document root.")
				return
			}

			err = service.UpdateManager.RemoveVersion(currentLocalVersion)
			if err != nil {
				logrus.WithError(err).Error("Failed to removed current version when reseting to default document root.")
				return
			}

			logrus.Info("Successfully reset to default document root from on version sync.")
			return
		}

		err = service.UpdateManager.UpdateToVersion(newVersion, completeVersionUpdate(service))

		if err != nil {
			logrus.WithError(err).WithFields(logrus.Fields{"newVersion": newVersion}).Error("Version sync failed")
			return
		}

		logrus.WithFields(logrus.Fields{"newVersion": newVersion}).Info("Version sync completed successfully")
	}
}

func setServiceUpdating(service *UIService, version string) (string, error) {
	service.Lock()
	defer service.Unlock()

	if service.updating {
		return service.updatingVersion, fmt.Errorf(
			"Cannot set service to updating to version %s because another update is already in progress for version: %s",
			version,
			service.updatingVersion,
		)
	}
	service.updating = true
	service.updatingVersion = version

	return version, nil
}

func resetServiceFromUpdate(service *UIService) {
	service.Lock()
	defer service.Unlock()

	service.updating = false
	service.updatingVersion = ""
}

func completeVersionUpdate(service *UIService) func(string) error {
	return func(newVersionPath string) error {
		return updateServedVersion(service, newVersionPath)
	}
}

func updateServedVersion(service *UIService, newVersionPath string) error {
	// Create temporary symlink
	if err := os.Symlink(newVersionPath, service.Config.UIDistStageSymlink); err != nil {
		return errors.Wrap(err, "unable to create temporary staging symlink for new version")
	}
	// Swap serving symlink with temp
	if err := os.Rename(service.Config.UIDistStageSymlink, service.Config.UIDistSymlink); err != nil {
		// remove/unlink temporary symlink
		if removeErr := os.Remove(service.Config.UIDistStageSymlink); removeErr != nil {
			logrus.WithError(removeErr).Error("Failed to remove new version staged symlink, after failing to swap symlinks for an update.")
		}
		// return error
		return errors.Wrap(err, "unable to swap staged new version symlink with dist symlink")
	}
	return nil
}
