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
	"github.com/samuel/go-zookeeper/zk"
)

type zkVersionStore struct {
	currentVersion        zkUIVersion
	versionListeners      []VersionChangeListener
	listeners             versionChangeListeners
	versionListenersMutex sync.Mutex
	client                zookeeper.ZKClient
	zkClientState         zookeeper.ClientState
	zkBasePath            string
	versionPath           string
	watchState            versionWatchState
}

type zkUIVersion struct {
	currentVersion UIVersion
	initialized    bool
	sync.Mutex
}

type versionWatchState struct {
	active          bool
	disconnected    chan struct{}
	pollingInterval time.Duration
	sync.Mutex
}

type versionChangeListeners struct {
	versionListeners []VersionChangeListener
	sync.Mutex
}

var (
	ErrZookeeperNotConnected = errors.New("Zookeeper is not currently connected")
)

// NewZKVersionStore creates a new zookeeper version store from the config.
// zookeeper connection will be asyncronously initiated.
func NewZKVersionStore(cfg *config.Config) VersionStore {
	store := &zkVersionStore{
		currentVersion: zkUIVersion{
			currentVersion: PreBundledUIVersion,
			initialized:    false,
		},
		zkBasePath:  cfg.ZKBasePath,
		versionPath: makeVersionPath(cfg.ZKBasePath),
		watchState: versionWatchState{
			active:          false,
			pollingInterval: cfg.ZKPollingInterval,
		},
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

	err := zks.client.Set(zks.versionPath, []byte(newVersion))
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
			fmt.Printf(
				"ZKVersionStore: Failed to connect to ZK on attempt %v. Error: %v. Will retry again in %s\n",
				connectionAttempt,
				err.Error(),
				backoffDuration) //TODO: Warning
			<-time.After(backoffDuration)
		} else {
			zks.initZKVersionStore(zkClient)
			if connectionAttempt > 1 {
				fmt.Printf("ZKVersionStore: Successfully connected to ZK after %v failed attempts\n", connectionAttempt) //TODO: Info
			} else {
				fmt.Println("ZKVersionStore: Successfully connected to ZK") //TODO: Info
			}
			return
		}
	}
}

func (zks *zkVersionStore) initZKVersionStore(client zookeeper.ZKClient) {
	zks.client = client
	client.RegisterListener(func(state zookeeper.ClientState) {
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
	fmt.Printf("ZKVersionStore: ZK connection state changed to %v\n", state) // TODO: Info

	if oldState == zookeeper.Disconnected {
		zks.initCurrentVersion()
	}
	if state == zookeeper.Disconnected && zks.watchState.active {
		close(zks.watchState.disconnected)
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
	fmt.Printf("ZKVersionStore: Current UI version cached from ZK: %v\n", version) //TODO: Trace | Debug
}

func (zks *zkVersionStore) getVersionFromZK() (UIVersion, error) {
	data, err := zks.client.Get(zks.versionPath)
	if err != nil {
		return UIVersion(""), errors.Wrap(err, "unable to get version from zk")
	}
	return UIVersion(data), nil
}

func (zks *zkVersionStore) initCurrentVersion() {
	var version UIVersion

	fmt.Println("ZKVersionStore: Getting current ui version from ZK") // TODO: Debug
	found, err := zks.client.Exists(zks.versionPath)
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

	zks.startVersionWatch()
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

func (zks *zkVersionStore) startVersionWatch() {
	if zks.watchState.active {
		fmt.Println("ZKVersionStore: Tried to start version watch while old watch was active") //TODO: Warning
		return
	}

	zks.watchState.Lock()
	defer zks.watchState.Unlock()
	zks.watchState.active = true
	zks.watchState.disconnected = make(chan struct{})

	fmt.Println("ZKVersionStore: Version watch started") // TODO: Info | Debug
	go zks.pollForVersionChanges()
}

func (zks *zkVersionStore) stopVersionWatch() {
	if !zks.watchState.active {
		fmt.Print("ZKVersionStore: Tried to stop version watch with no watch active\n") // TODO: Warning
		return
	}
	zks.watchState.Lock()
	defer zks.watchState.Unlock()
	zks.watchState.active = false
	zks.watchState.disconnected = nil

	fmt.Print("ZKVersionStore: Version watch stopped\n") // TODO: Info | Debug
}

func (zks *zkVersionStore) pollForVersionChanges() {
	for {
		select {
		case <-zks.watchState.disconnected:
			zks.stopVersionWatch()
		case <-time.After(zks.watchState.pollingInterval):
			version, err := zks.getVersionFromZK()
			switch err {
			case nil:
				currentVersion := zks.localVersion()
				if version != currentVersion {
					zks.updateLocalCurrentVersion(version)
				}
			case zk.ErrNoNode:
				fmt.Printf("ZKVersionStore: version node not found in ZK (was deleted)\n") // TODO: Error | Fatal?
			default:
				fmt.Printf("ZKVersionStore: version poll get from zk failed. Error: %v\n", err.Error()) // TODO: Warning | Error
			}
		}
	}
}

func (zks *zkVersionStore) localVersion() UIVersion {
	zks.currentVersion.Lock()
	defer zks.currentVersion.Unlock()
	return zks.currentVersion.currentVersion
}
