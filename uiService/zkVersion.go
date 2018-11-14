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
	active       bool
	channel      <-chan zk.Event
	disconnected chan struct{}
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

	found, err := zks.client.Exists(zks.versionPath)
	if err != nil {
		return errors.Wrap(err, "Failed to create version in ZK,")
	}
	if found {
		err = zks.client.Set(zks.versionPath, []byte(newVersion))
		if err != nil {
			return errors.Wrap(err, "Failed to create version in ZK,")
		}
	} else {
		err = zks.client.Create(zks.versionPath, []byte(newVersion), zookeeper.PermAll)
		if err != nil {
			return errors.Wrap(err, "Failed to create version in ZK,")
		}
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

func (zks *zkVersionStore) initCurrentVersion() {
	var version UIVersion

	fmt.Println("ZKVersionStore: Getting current ui version from ZK") // TODO: Debug
	found, editChan, err := zks.client.ExistsW(zks.versionPath)
	if err != nil {
		fmt.Printf("ZKVersionStore: Failed to initialize current ui version from zk. Error %#v\n", err) // TODO: Error
		return
	}
	if !found {
		version = PreBundledUIVersion
	} else {
		data, err := zks.client.Get(zks.versionPath)
		switch err {
		default:
			fmt.Printf("ZKVersionStore: Failed to get ui version from zk, Error: %#v\n", err) // TODO: Error
			// Assume Pre-bundled UI?
			version = PreBundledUIVersion
		case nil:
			version = UIVersion(data)
		case zk.ErrNoNode:
			version = PreBundledUIVersion
		}
	}

	zks.updateLocalCurrentVersion(version)

	zks.startVersionWatch(editChan)
}

func (zks *zkVersionStore) broadcastVersionChange() {
	if len(zks.listeners.versionListeners) == 0 {
		// don't bother contining if there are no listeners
		return
	}

	// Don't add new listeners while we are broadcasting
	zks.listeners.Lock()
	// Wait for broadcast to complete before allowing to update the local version again
	zks.currentVersion.Lock()
	defer zks.listeners.Unlock()
	defer zks.currentVersion.Unlock()

	currentVersion := zks.currentVersion.currentVersion
	for _, listener := range zks.listeners.versionListeners {
		go listener(currentVersion)
	}
}

func (zks *zkVersionStore) startVersionWatch(ech <-chan zk.Event) {
	if zks.watchState.active {
		fmt.Println("ZKVersionStore: Tried to start version watch while old watch was active") //TODO: Warning
		return
	}
	if ech == nil {
		fmt.Println("ZKVersionStore: Received nil exist channel for version watch") // TODO: Warning
		return
	}

	zks.watchState.Lock()
	defer zks.watchState.Unlock()
	zks.watchState.active = true
	zks.watchState.channel = ech
	zks.watchState.disconnected = make(chan struct{})

	fmt.Println("ZKVersionStore: Version watch started") // TODO: Info | Debug
	go zks.watchForZKEdits()
}

func (zks *zkVersionStore) startNewVersionWatchChannel() {
	zks.watchState.Lock()
	defer zks.watchState.Unlock()

	_, watchChan, err := zks.client.ExistsW(zks.versionPath)
	if err != nil {
		fmt.Printf("ZKVersionStore: Failed to create new watch for version node. Error: %#v\n", err) // TODO: Warning
		zks.stopVersionWatch("failed to create a new watch")
		go zks.restartWatchAfterError()
		return
	}
	zks.watchState.channel = watchChan
	go zks.watchForZKEdits()
}

func (zks *zkVersionStore) stopVersionWatch(reason string) {
	if !zks.watchState.active {
		fmt.Printf("ZKVersionStore: Tried to stop version watch with no watch active - %s\n", reason) // TODO: Warning
	}
	zks.watchState.Lock()
	defer zks.watchState.Unlock()
	zks.watchState.active = false
	zks.watchState.channel = nil

	fmt.Printf("ZKVersionStore: Version watch stopped - %s\n", reason) // TODO: Info | Debug
}

func (zks *zkVersionStore) watchForZKEdits() {
	select {
	case nodeEvent := <-zks.watchState.channel:
		// We got an event
		if nodeEvent.Err != nil {
			fmt.Printf("ZKVersionStore: Verion watch event returned an error. Error %#v\n", nodeEvent.Err) // TODO: Warning
			zks.stopVersionWatch("received error from watch event")
			go zks.restartWatchAfterError()
			return
		}
		if nodeEvent.State == zk.StateDisconnected {
			zks.stopVersionWatch("zookeeper disconnected")
			return
		}
		zks.handleVersionNodeEvent(nodeEvent)
		// watches from zk are 1 time use, after we get an event we need to create a new watch
		zks.startNewVersionWatchChannel()
		return
	case <-zks.watchState.disconnected:
		// We were disconnected from ZK, stop watch
		zks.stopVersionWatch("zookeeper disconnected")
		return
	}
}

func (zks *zkVersionStore) handleVersionNodeEvent(nodeEvent zk.Event) {
	switch nodeEvent.Type {
	case zk.EventNodeCreated:
		go zks.getVersionFromZKAndUpdateLocal()
	case zk.EventNodeDataChanged:
		go zks.getVersionFromZKAndUpdateLocal()
	case zk.EventNodeDeleted:
		go zks.handleVersionNodeDeleted()
	}
}

func (zks *zkVersionStore) getVersionFromZKAndUpdateLocal() {
	data, err := zks.client.Get(zks.versionPath)
	if err != nil {
		fmt.Printf("ZKVersionStore: Failed to get current ui version value from zk. Error: %#v", err) // TODO: Error
		return
	}
	version := UIVersion(data)
	zks.updateLocalCurrentVersion(version)
}

func (zks *zkVersionStore) handleVersionNodeDeleted() {
	zks.updateLocalCurrentVersion(PreBundledUIVersion)
}

func (zks *zkVersionStore) restartWatchAfterError() {
	b := &backoff.Backoff{
		Min:    1 * time.Second,
		Max:    5 * time.Minute,
		Factor: 2,
		Jitter: false,
	}
	for {
		<-time.After(b.Duration())

		if zks.client.ClientState() != zookeeper.Connected ||
			zks.watchState.active {
			return
		}

		zks.watchState.Lock()
		_, watchChan, err := zks.client.ExistsW(zks.versionPath)
		zks.watchState.Unlock()
		if err == nil {
			zks.startVersionWatch(watchChan)
			return
		}
		fmt.Printf("ZKVersionStore: Failed to create zk watch for version node. Error: %#v\n", err) // TODO: Error
	}
}
