package uiService

import (
	"fmt"
	"path"
	"sync"
	"time"

	"github.com/dcos/dcos-ui-update-service/config"
	"github.com/dcos/dcos-ui-update-service/zookeeper"
	"github.com/jpillora/backoff"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type zkVersionStore struct {
	currentVersion    zkUIVersion
	listeners         versionChangeListeners
	client            zookeeper.ZKClient
	zkClientState     zookeeper.ClientState
	zkBasePath        string
	versionPath       string
	zkPollingInterval time.Duration
	versionWatcher    zookeeper.ValueNodeWatcher
}

type zkUIVersion struct {
	currentVersion UIVersion
	initialized    bool
	sync.Mutex
}

type versionChangeListeners struct {
	versionListeners []VersionChangeListener
	sync.Mutex
}

var (
	ErrZookeeperNotConnected = errors.New("Zookeeper is not currently connected")

	log = logrus.WithFields(logrus.Fields{"package": "ZKVersionStore"})
)

// NewZKVersionStore creates a new zookeeper version store from the config.
// zookeeper connection will be asyncronously initiated.
func NewZKVersionStore(cfg *config.Config) VersionStore {
	store := &zkVersionStore{
		currentVersion: zkUIVersion{
			currentVersion: PreBundledUIVersion,
			initialized:    false,
		},
		zkBasePath:        cfg.ZKBasePath(),
		versionPath:       makeVersionPath(cfg.ZKBasePath()),
		zkPollingInterval: cfg.ZKPollingInterval(),
		versionWatcher:    nil,
	}
	go store.connectAndInitZKAsync(cfg)
	return store
}

// CurrentVersion gets the current UIVersion stored.
func (zks *zkVersionStore) CurrentVersion() (UIVersion, error) {
	return zks.currentVersion.currentVersion, nil
}

// UpdateCurrentVersion sets the UIVersion stored to the newVersion provided
func (zks *zkVersionStore) UpdateCurrentVersion(newVersion UIVersion) error {
	if zks.client == nil || zks.client.ClientState() != zookeeper.Connected {
		return ErrZookeeperNotConnected
	}

	_, err := zks.client.Set(zks.versionPath, []byte(newVersion))
	if err != nil {
		return errors.Wrap(err, "Failed to create version in ZK, not able to set the version node")
	}
	zks.updateLocalCurrentVersion(newVersion)
	return nil
}

// WatchForVersionChange registers the VersionChangeListener provided to be called when changes
// to the stored version are received. Provided listener will be called with the current version
// upon successful registration. VersionChangeListener is called asyncronously and must handle all
// errors internally.
func (zks *zkVersionStore) WatchForVersionChange(listener VersionChangeListener) error {
	zks.listeners.Lock()
	defer zks.listeners.Unlock()

	zks.listeners.versionListeners = append(zks.listeners.versionListeners, listener)
	if zks.currentVersion.initialized {
		go listener(zks.currentVersion.currentVersion)
	}

	return nil
}

func (zks *zkVersionStore) connectAndInitZKAsync(cfg *config.Config) {
	connectionAttempt := 0
	b := &backoff.Backoff{
		Min:    15 * time.Second,
		Max:    5 * time.Minute,
		Factor: 2,
		Jitter: false,
	}
	for {
		connectionAttempt++
		zkClient, err := zookeeper.Connect(cfg)
		if err != nil {
			backoffDuration := b.Duration()
			log.WithError(err).WithFields(logrus.Fields{
				"connectionAttempt": connectionAttempt,
				"backOffDuration":   backoffDuration,
			}).Warning("Failed to connect to ZK")
			<-time.After(backoffDuration)
		} else {
			zks.initZKVersionStore(zkClient)
			if connectionAttempt > 1 {
				log.WithFields(logrus.Fields{
					"connectionAttempt": connectionAttempt,
				}).Info("Successfully connected to ZK after previous failures")
			} else {
				log.Info("Successfully connected to ZK")
			}
			return
		}
	}
}

func (zks *zkVersionStore) initZKVersionStore(client zookeeper.ZKClient) {
	zks.client = client
	client.RegisterListener("zk-version-store-version", func(state zookeeper.ClientState) {
		go zks.handleZKStateChange(state)
	})
}

func makeVersionPath(basePath string) string {
	return path.Join(basePath, "version")
}

func (zks *zkVersionStore) handleZKStateChange(state zookeeper.ClientState) {
	if zks.zkClientState == state {
		return
	}
	oldState := zks.zkClientState
	zks.zkClientState = state
	log.WithFields(logrus.Fields{"state": state}).Info("ZK connection state changed")

	if oldState == zookeeper.Disconnected {
		zks.initCurrentVersion()
	}
}

func (zks *zkVersionStore) updateLocalCurrentVersion(version UIVersion) {
	zks.currentVersion.Lock()
	defer zks.currentVersion.Unlock()
	if zks.currentVersion.currentVersion == version && zks.currentVersion.initialized {
		// If there isn't a change return, unless the version is not initialized
		return
	}

	zks.currentVersion.currentVersion = version

	if !zks.currentVersion.initialized {
		zks.currentVersion.initialized = true
	}

	go zks.broadcastVersionChange()
	log.WithFields(logrus.Fields{"version": version}).Debug("Current UI version cached from ZK")
}

func (zks *zkVersionStore) getVersionFromZK() (UIVersion, error) {
	data, _, err := zks.client.Get(zks.versionPath)
	if err != nil {
		return UIVersion(""), errors.Wrap(err, "unable to get version from zk")
	}
	return UIVersion(data), nil
}

func (zks *zkVersionStore) initCurrentVersion() {
	var version UIVersion

	log.Debug("Getting current ui version from ZK")
	found, _, err := zks.client.Exists(zks.versionPath)
	if err != nil {
		panic(fmt.Sprintf("Error making exists check in zookeeper for ui version node @ '%v'. Error: %v", zks.versionPath, err.Error()))
	}
	if !found {
		err = zks.client.Create(zks.versionPath, []byte(PreBundledUIVersion), zookeeper.PermAll)
		if err != nil {
			panic(fmt.Sprintf("Error creating zookeeper ui version node @ '%v'. Error: %v", zks.versionPath, err.Error()))
		}
		version = PreBundledUIVersion
	} else {
		uiVersion, err := zks.getVersionFromZK()
		if err != nil {
			panic(fmt.Sprintf("Error getting value from zookeeper for ui version node @ '%v'. Error: %v", zks.versionPath, err.Error()))
		}
		version = uiVersion
	}

	zks.updateLocalCurrentVersion(version)

	zks.createVersionWatcher()
}

func (zks *zkVersionStore) broadcastVersionChange() {
	if len(zks.listeners.versionListeners) == 0 {
		// don't bother contining if there are no listeners
		return
	}

	// Don't add new listeners while we are broadcasting
	zks.listeners.Lock()
	// Wait for broadcast to complete before allowing to update the local version again
	defer zks.listeners.Unlock()

	currentVersion := zks.localVersion()
	for _, listener := range zks.listeners.versionListeners {
		go listener(currentVersion)
	}
}

func (zks *zkVersionStore) createVersionWatcher() {
	if zks.versionWatcher != nil {
		return
	}

	watcher, err := zookeeper.CreateValueNodeWatcher(zks.client, zks.versionPath, zks.zkPollingInterval, zks.versionWatcherCallback)
	if err != nil {
		// handle error
		log.WithError(err).Warn("Failed to create ZK node watcher")
		return
	}
	zks.versionWatcher = watcher
}

func (zks *zkVersionStore) versionWatcherCallback(data []byte) {
	version := UIVersion(data)
	currentVersion := zks.localVersion()
	if version != currentVersion {
		zks.updateLocalCurrentVersion(version)
	}
}

func (zks *zkVersionStore) localVersion() UIVersion {
	zks.currentVersion.Lock()
	defer zks.currentVersion.Unlock()
	return zks.currentVersion.currentVersion
}
