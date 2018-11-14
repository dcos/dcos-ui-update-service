package uiService

import (
	"errors"
	"testing"
	"time"

	"github.com/dcos/dcos-ui-update-service/tests"
	"github.com/dcos/dcos-ui-update-service/zookeeper"
	"github.com/samuel/go-zookeeper/zk"
)

func makeZKStore(version string) (*zkVersionStore, *fakeZKClient) {
	fakeClient := newFakeZKClient()
	fakeClient.ClientStateResult = zookeeper.Connected
	return &zkVersionStore{
		currentVersion: zkUIVersion{
			currentVersion: UIVersion(version),
			initialized:    false,
		},
		client:      fakeClient,
		zkBasePath:  "/dcos/ui-service-test",
		versionPath: "/dcos/ui-service-test/version",
	}, fakeClient
}

func TestZKVersionStore(t *testing.T) {
	t.Parallel()

	t.Run("CurrentVersion() returns cached current version", func(t *testing.T) {
		expectedVersion := "1.0.0"

		store, _ := makeZKStore(expectedVersion)

		cv, _ := store.CurrentVersion()

		tests.H(t).StringEql(string(cv), expectedVersion)
	})

	t.Run("CurrentVersion() doesn't return an error", func(t *testing.T) {
		store, _ := makeZKStore("1.0.0")

		_, err := store.CurrentVersion()
		tests.H(t).IsNil(err)
	})

	t.Run("UpdateCurrentVersion() updates the version", func(t *testing.T) {
		expectedVersion := "1.1.0"
		store, _ := makeZKStore("1.0.0")

		err := store.UpdateCurrentVersion(UIVersion(expectedVersion))
		tests.H(t).IsNil(err)

		cv, _ := store.CurrentVersion()
		tests.H(t).StringEql(string(cv), expectedVersion)
	})

	t.Run("UpdateCurrentVersion() creates a new zk Node if it doesn't exist", func(t *testing.T) {
		store, client := makeZKStore("1.0.0")
		client.ClientStateResult = zookeeper.Connected

		var createCalled bool
		client.CreateCall = func(path string, data []byte, perms []int32) {
			createCalled = true
		}

		err := store.UpdateCurrentVersion(UIVersion("1.1.0"))
		tests.H(t).IsNil(err)
		tests.H(t).BoolEql(createCalled, true)
	})

	t.Run("UpdateCurrentVersion() populates new zk Node with string version", func(t *testing.T) {
		store, client := makeZKStore("1.0.0")

		var createData []byte
		client.CreateCall = func(path string, data []byte, perms []int32) {
			createData = data
		}
		expectedVersion := "1.1.0"

		err := store.UpdateCurrentVersion(UIVersion(expectedVersion))
		tests.H(t).IsNil(err)

		tests.H(t).StringEql(string(createData), expectedVersion)
	})

	t.Run("UpdateCurrentVersion() creates a new zk Node with PermAll", func(t *testing.T) {
		store, client := makeZKStore("1.0.0")

		var newNodePerms []int32
		client.CreateCall = func(path string, data []byte, perms []int32) {
			newNodePerms = perms
		}

		err := store.UpdateCurrentVersion(UIVersion("1.1.0"))
		tests.H(t).IsNil(err)

		newNodePermsMatch := true
		if len(newNodePerms) != len(zookeeper.PermAll) {
			newNodePermsMatch = false
		} else {
			for i, _ := range zookeeper.PermAll {
				if newNodePerms[i] != zookeeper.PermAll[i] {
					newNodePermsMatch = false
				}
			}
		}

		if !newNodePermsMatch {
			t.Errorf("Expected version node to have PermAll, got %v", newNodePerms)
		}
	})

	t.Run("UpdateCurrentVersion() sets the zk Node if it does exist", func(t *testing.T) {
		store, client := makeZKStore("1.0.0")

		var setCalled bool
		client.ExistsResult = true
		client.SetCall = func(path string, data []byte) {
			setCalled = true
		}

		err := store.UpdateCurrentVersion(UIVersion("1.1.0"))
		tests.H(t).IsNil(err)
		tests.H(t).BoolEql(setCalled, true)
	})

	t.Run("UpdateCurrentVersion() populates the zk Node with string version", func(t *testing.T) {
		store, client := makeZKStore("1.0.0")

		var setData []byte
		client.ExistsResult = true
		client.SetCall = func(path string, data []byte) {
			setData = data
		}
		expectedVersion := "1.1.0"

		err := store.UpdateCurrentVersion(UIVersion(expectedVersion))
		tests.H(t).IsNil(err)
		tests.H(t).StringEql(string(setData), expectedVersion)
	})

	t.Run("UpdateCurrentVersion() calls registered listeners with new version", func(t *testing.T) {
		store, _ := makeZKStore("1.0.0")

		listenerCalledWith := UIVersion("not called")
		listenerCalled := make(chan struct{})

		store.WatchForVersionChange(func(newVersion UIVersion) {
			listenerCalledWith = newVersion
			close(listenerCalled)
		})

		expectedVersion := "1.1.0"

		err := store.UpdateCurrentVersion(UIVersion(expectedVersion))
		tests.H(t).IsNil(err)

		select {
		case <-listenerCalled:
		case <-time.After(20 * time.Millisecond):
		}

		if string(listenerCalledWith) != expectedVersion {
			t.Errorf("version watch not called with expected value, got %v instead of %v", listenerCalledWith, expectedVersion)
		}
	})

	t.Run("UpdateCurrentVersion() fails if zk is disconnected", func(t *testing.T) {
		store, client := makeZKStore("")
		client.ClientStateResult = zookeeper.Disconnected

		err := store.UpdateCurrentVersion("1.0.0")
		tests.H(t).NotNil(err)

		tests.H(t).StringContains(err.Error(), ErrZookeeperNotConnected.Error())
	})

	t.Run("UpdateCurrentVersion() fails if zk.Exists errors", func(t *testing.T) {
		expectedError := errors.New("ZK Exist failure")
		store, client := makeZKStore("1.0.0")
		client.ExistsError = expectedError

		err := store.UpdateCurrentVersion(UIVersion("1.1.0"))
		tests.H(t).NotNil(err)

		tests.H(t).StringContains(err.Error(), expectedError.Error())
	})

	t.Run("UpdateCurrentVersion() fails if zk.Create errors", func(t *testing.T) {
		expectedError := errors.New("ZK Create failure")
		store, client := makeZKStore("1.0.0")
		client.CreateError = expectedError

		err := store.UpdateCurrentVersion(UIVersion("1.1.0"))
		tests.H(t).NotNil(err)

		tests.H(t).StringContains(err.Error(), expectedError.Error())
	})

	t.Run("UpdateCurrentVersion() fails if zk.Set errors", func(t *testing.T) {
		expectedError := errors.New("ZK Set failure")
		store, client := makeZKStore("1.0.0")
		client.ExistsResult = true
		client.SetError = expectedError

		err := store.UpdateCurrentVersion(UIVersion("1.1.0"))
		tests.H(t).NotNil(err)

		tests.H(t).StringContains(err.Error(), expectedError.Error())
	})

	t.Run("WatchForVersionChange() calls listener immediately if current version initialized", func(t *testing.T) {
		store, _ := makeZKStore("1.0.0")
		store.currentVersion.initialized = true

		var watcherCallCount int
		callWait := make(chan struct{})
		store.WatchForVersionChange(func(version UIVersion) {
			watcherCallCount++
			close(callWait)
		})

		select {
		case <-callWait:
		case <-time.After(10 * time.Millisecond):
		}

		tests.H(t).IntEql(watcherCallCount, 1)
	})

	t.Run("WatchForVersionChange() doesn't call listener if current version isn't initialized", func(t *testing.T) {
		store, _ := makeZKStore("1.0.0")

		var watcherCallCount int
		callWait := make(chan struct{})
		store.WatchForVersionChange(func(version UIVersion) {
			watcherCallCount++
			close(callWait)
		})

		select {
		case <-callWait:
		case <-time.After(10 * time.Millisecond):
		}

		tests.H(t).IntEql(watcherCallCount, 0)
	})

	t.Run("handleZKStateChange() updates current version when connecting", func(t *testing.T) {
		store, client := makeZKStore("")
		store.zkClientState = zookeeper.Disconnected
		client.ExistsResult = true
		client.GetResult = []byte("2.0.0")

		store.handleZKStateChange(zookeeper.Connected)

		cv, _ := store.CurrentVersion()
		tests.H(t).StringEql(string(cv), "2.0.0")
	})

	t.Run("handleZKStateChange() sets to PreBundledUI if node doesn't exist", func(t *testing.T) {
		store, client := makeZKStore("1.0.0")
		store.zkClientState = zookeeper.Disconnected
		client.ExistsResult = false

		store.handleZKStateChange(zookeeper.Connected)

		cv, _ := store.CurrentVersion()
		tests.H(t).StringEql(string(cv), "")
	})

	t.Run("handleZKStateChange() starts watching zk for changes when connected", func(t *testing.T) {
		store, client := makeZKStore("1.0.0")
		store.zkClientState = zookeeper.Disconnected
		client.ExistsResult = false

		store.handleZKStateChange(zookeeper.Connected)

		tests.H(t).BoolEql(store.watchState.active, true)
	})

	t.Run("handleZKStateChange() stops watching zk if disconnected", func(t *testing.T) {
		store, client := makeZKStore("1.0.0")
		store.zkClientState = zookeeper.Disconnected
		client.ExistsResult = false

		store.handleZKStateChange(zookeeper.Connected)

		//ensure initial watch is started
		tests.H(t).BoolEql(store.watchState.active, true)

		<-time.After(10 * time.Millisecond)

		// disconnect
		store.handleZKStateChange(zookeeper.Disconnected)

		// wait a little for async calls
		<-time.After(10 * time.Millisecond)

		// ensure we stopped watching
		tests.H(t).BoolEql(store.watchState.active, false)
	})

	t.Run("WatchForVersionChange() listener is called when zk version node is created", func(t *testing.T) {
		store, client := makeZKStore("")
		store.zkClientState = zookeeper.Disconnected
		client.ExistsResult = false
		client.GetResult = []byte("1.2.0")

		// register for version updates
		var versionCalls []UIVersion
		store.WatchForVersionChange(func(newVersion UIVersion) {
			versionCalls = append(versionCalls, newVersion)
		})

		// connect store to start watch
		store.handleZKStateChange(zookeeper.Connected)

		// wait a little for async calls
		<-time.After(10 * time.Millisecond)

		client.ExistsWatch <- zk.Event{
			Type:  zk.EventNodeCreated,
			State: zk.StateConnected,
		}

		// wait a little for async calls
		<-time.After(10 * time.Millisecond)

		// called once when we register then again for the event
		tests.H(t).IntEql(len(versionCalls), 2)

		tests.H(t).StringEql(string(versionCalls[0]), "")
		tests.H(t).StringEql(string(versionCalls[1]), "1.2.0")
	})

	t.Run("WatchForVersionChange() listener is called when zk version updates", func(t *testing.T) {
		store, client := makeZKStore("")
		store.zkClientState = zookeeper.Disconnected
		client.ExistsResult = true
		client.GetResult = []byte("1.0.0")

		// register for version updates
		var versionCalls []UIVersion
		store.WatchForVersionChange(func(newVersion UIVersion) {
			versionCalls = append(versionCalls, newVersion)
		})

		// connect store to start watch
		store.handleZKStateChange(zookeeper.Connected)

		// wait a little for async calls
		<-time.After(10 * time.Millisecond)

		client.GetResult = []byte("1.1.0")

		client.ExistsWatch <- zk.Event{
			Type:  zk.EventNodeDataChanged,
			State: zk.StateConnected,
		}

		// wait a little for async calls
		<-time.After(10 * time.Millisecond)

		// called once when we register then again for the event
		tests.H(t).IntEql(len(versionCalls), 2)

		tests.H(t).StringEql(string(versionCalls[0]), "1.0.0")
		tests.H(t).StringEql(string(versionCalls[1]), "1.1.0")
	})

	t.Run("version watch sets to PreBundledUI if zk node is deleted", func(t *testing.T) {
		store, client := makeZKStore("1.2.0")
		store.zkClientState = zookeeper.Disconnected
		client.ExistsResult = true
		client.GetResult = []byte("1.2.0")

		// register for version updates
		var versionCalls []UIVersion
		store.WatchForVersionChange(func(newVersion UIVersion) {
			versionCalls = append(versionCalls, newVersion)
		})

		// connect store to start watch
		store.handleZKStateChange(zookeeper.Connected)

		// wait a little for async calls
		<-time.After(10 * time.Millisecond)

		client.ExistsWatch <- zk.Event{
			Type:  zk.EventNodeDeleted,
			State: zk.StateConnected,
		}

		// wait a little for async calls
		<-time.After(10 * time.Millisecond)

		// called once when we register then again for the event
		tests.H(t).IntEql(len(versionCalls), 2)

		tests.H(t).StringEql(string(versionCalls[0]), "1.2.0")
		tests.H(t).StringEql(string(versionCalls[1]), "")
	})
}

type fakeZKClient struct {
	ExistsError error
	GetError    error
	CreateError error
	SetError    error

	ClientStateResult zookeeper.ClientState
	Listeners         []zookeeper.StateListener
	ExistsResult      bool
	ExistsWatch       chan zk.Event
	GetResult         []byte

	CreateCall func(string, []byte, []int32)
	SetCall    func(string, []byte)
}

func newFakeZKClient() *fakeZKClient {
	return &fakeZKClient{}
}

func (zkc *fakeZKClient) Close() {}

func (zkc *fakeZKClient) ClientState() zookeeper.ClientState {
	return zkc.ClientStateResult
}

func (zkc *fakeZKClient) RegisterListener(listener zookeeper.StateListener) {
	zkc.Listeners = append(zkc.Listeners, listener)
}

func (zkc *fakeZKClient) PublishStateChange(newState zookeeper.ClientState) {
	zkc.ClientStateResult = newState
	for _, l := range zkc.Listeners {
		l(newState)
	}
}

func (zkc *fakeZKClient) Exists(path string) (bool, error) {
	if zkc.ExistsError != nil {
		return false, zkc.ExistsError
	}
	return zkc.ExistsResult, nil
}

func (zkc *fakeZKClient) ExistsW(path string) (bool, <-chan zk.Event, error) {
	if zkc.ExistsError != nil {
		return false, nil, zkc.ExistsError
	}
	if zkc.ExistsWatch == nil {
		zkc.ExistsWatch = make(chan zk.Event)
	}
	return zkc.ExistsResult, zkc.ExistsWatch, nil
}

func (zkc *fakeZKClient) Get(path string) ([]byte, error) {
	if zkc.GetError != nil {
		return nil, zkc.GetError
	}
	return zkc.GetResult, nil
}

func (zkc *fakeZKClient) Create(path string, data []byte, perms []int32) error {
	if zkc.CreateCall != nil {
		zkc.CreateCall(path, data, perms)
	}
	if zkc.CreateError != nil {
		return zkc.CreateError
	}
	return nil
}

func (zkc *fakeZKClient) Set(path string, data []byte) error {
	if zkc.SetCall != nil {
		zkc.SetCall(path, data)
	}
	if zkc.SetError != nil {
		return zkc.SetError
	}
	return nil
}
