package zookeeper

import (
	"sync"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/samuel/go-zookeeper/zk"

	"github.com/dcos/dcos-ui-update-service/tests"
)

func TestValueNodeWatcher(t *testing.T) {
	const (
		pollTimeout      = time.Duration(30 * time.Second)
		shortPollTimeout = time.Duration(500 * time.Millisecond)
	)

	t.Parallel()

	t.Run("returns ErrListenerNotProvided if listener callback is nil", func(t *testing.T) {
		client := NewFakeZKClient()

		_, err := CreateValueNodeWatcher(client, "/foo", pollTimeout, nil)

		tests.H(t).ErrEql(err, ErrListenerNotProvided)
	})

	t.Run("returns ErrDisconnected if client is Disconnected", func(t *testing.T) {
		client := NewFakeZKClient()

		_, err := CreateValueNodeWatcher(client, "/foo", pollTimeout, func(path string, val []byte) {})

		tests.H(t).ErrEql(err, ErrDisconnected)
	})

	t.Run("returns ErrFailedToReadNode if Exists returns error while creating", func(t *testing.T) {
		client := NewFakeZKClient()
		client.ClientStateResult = Connected
		client.ExistsError = errors.New("Boom!!")

		_, err := CreateValueNodeWatcher(client, "/foo", pollTimeout, func(path string, val []byte) {})

		tests.H(t).ErrEql(err, ErrFailedToReadNode)
	})

	t.Run("returns Watcher even if node doesn't exist", func(t *testing.T) {
		helper := tests.H(t)
		client := NewFakeZKClient()
		client.ClientStateResult = Connected
		client.ExistsResult = false

		watcher, err := CreateValueNodeWatcher(client, "/foo", pollTimeout, func(path string, val []byte) {})
		defer watcher.Close()

		helper.IsNil(err)
		helper.NotNil(watcher)
	})

	t.Run("watcher initializes value to empty slice if Node doesn't exist", func(t *testing.T) {
		helper := tests.H(t)
		client := NewFakeZKClient()
		client.ClientStateResult = Connected
		client.ExistsResult = false

		watcher, err := CreateValueNodeWatcher(client, "/foo", pollTimeout, func(path string, val []byte) {})
		defer watcher.Close()

		helper.IsNil(err)
		helper.IntEql(len(watcher.Value()), 0)
	})

	t.Run("CreateValueNodeWatcher sets polling timeout to value given", func(t *testing.T) {
		client := NewFakeZKClient()
		client.ClientStateResult = Connected
		client.ExistsResult = false

		watcher, _ := CreateValueNodeWatcher(client, "/foo", pollTimeout, func(path string, val []byte) {})
		defer watcher.Close()

		valueWatcher, _ := watcher.(*valueNodeWatcher)

		tests.H(t).Int64Eql(valueWatcher.pollTimeout.Nanoseconds(), pollTimeout.Nanoseconds())
	})

	t.Run("Calls listener when Node is created", func(t *testing.T) {
		helper := tests.H(t)
		client := NewFakeZKClient()
		client.ClientStateResult = Connected
		client.ExistsResult = false

		var wg sync.WaitGroup
		var listenerMutex sync.Mutex
		wg.Add(1)
		var listenerCalls [][]byte
		watcher, _ := CreateValueNodeWatcher(client, "/foo", pollTimeout, func(path string, val []byte) {
			listenerMutex.Lock()
			defer listenerMutex.Unlock()
			listenerCalls = append(listenerCalls, val)
			wg.Done()
		})
		defer watcher.Close()

		client.Lock()
		client.ExistsResult = true
		client.GetResult = []byte("bar")
		client.Unlock()

		listenerMutex.Lock()
		client.EventChannel <- zk.Event{
			Type:  zk.EventNodeCreated,
			Err:   nil,
			State: zk.StateConnected,
		}
		listenerMutex.Unlock()

		wg.Wait()

		listenerMutex.Lock()
		defer listenerMutex.Unlock()

		helper.IntEql(len(listenerCalls), 1)
		helper.StringEql(string(listenerCalls[0]), "bar")
		helper.StringEql(string(watcher.Value()), "bar")
	})

	t.Run("Calls listener when Node value is changed PollTimeout", func(t *testing.T) {
		helper := tests.H(t)
		client := NewFakeZKClient()
		client.ClientStateResult = Connected
		client.ExistsResult = false

		var wg sync.WaitGroup
		var listenerMutex sync.Mutex
		wg.Add(1)
		var listenerCalls [][]byte
		watcher, _ := CreateValueNodeWatcher(client, "/foo", shortPollTimeout, func(path string, val []byte) {
			listenerMutex.Lock()
			defer listenerMutex.Unlock()
			listenerCalls = append(listenerCalls, val)
			wg.Done()
		})
		defer watcher.Close()

		client.Lock()
		client.ExistsResult = true
		client.GetResult = []byte("bar")
		client.Unlock()

		wg.Wait()

		listenerMutex.Lock()
		defer listenerMutex.Unlock()

		helper.IntEql(len(listenerCalls), 1)
		helper.StringEql(string(listenerCalls[0]), "bar")
		helper.StringEql(string(watcher.Value()), "bar")
	})

	t.Run("Calls listener when Node value is changed Event", func(t *testing.T) {
		helper := tests.H(t)
		client := NewFakeZKClient()
		client.ClientStateResult = Connected
		client.ExistsResult = true
		client.GetResults = append(client.GetResults, []byte("bar"))
		client.GetResults = append(client.GetResults, []byte("baz"))

		var wg sync.WaitGroup
		var listenerMutex sync.Mutex
		wg.Add(1)
		var listenerCalls [][]byte
		watcher, _ := CreateValueNodeWatcher(client, "/foo", pollTimeout, func(path string, val []byte) {
			listenerMutex.Lock()
			defer listenerMutex.Unlock()
			listenerCalls = append(listenerCalls, val)
			wg.Done()
		})
		defer watcher.Close()
		helper.StringEql(string(watcher.Value()), "bar")

		listenerMutex.Lock()
		client.EventChannel <- zk.Event{
			Type:  zk.EventNodeDataChanged,
			Err:   nil,
			State: zk.StateConnected,
		}
		listenerMutex.Unlock()

		wg.Wait()

		listenerMutex.Lock()
		defer listenerMutex.Unlock()

		helper.IntEql(len(listenerCalls), 1)
		helper.StringEql(string(listenerCalls[0]), "baz")
	})

	t.Run("Calls listener when Node value is changed PollTimeout", func(t *testing.T) {
		helper := tests.H(t)
		client := NewFakeZKClient()
		client.ClientStateResult = Connected
		client.ExistsResult = true
		client.GetResults = append(client.GetResults, []byte("bar"))
		client.GetResults = append(client.GetResults, []byte("baz"))

		var wg sync.WaitGroup
		var listenerMutex sync.Mutex
		wg.Add(1)
		var listenerCalls [][]byte
		callback := func(path string, val []byte) {
			listenerMutex.Lock()
			defer listenerMutex.Unlock()
			listenerCalls = append(listenerCalls, val)
			wg.Done()
		}

		watcher, err := CreateValueNodeWatcher(client, "/foo", shortPollTimeout, callback)

		helper.IsNil(err)
		defer watcher.Close()

		helper.StringEql(string(watcher.Value()), "bar")

		wg.Wait()

		listenerMutex.Lock()
		defer listenerMutex.Unlock()

		helper.IntEql(len(listenerCalls), 1)
		helper.StringEql(string(listenerCalls[0]), "baz")
	})

	t.Run("Calls listener when Node is deleted", func(t *testing.T) {
		helper := tests.H(t)
		client := NewFakeZKClient()
		client.ClientStateResult = Connected
		client.ExistsResult = true
		client.GetResult = []byte("bar")

		var wg sync.WaitGroup
		var listenerMutex sync.Mutex
		wg.Add(1)
		var listenerCalls [][]byte
		watcher, _ := CreateValueNodeWatcher(client, "/foo", pollTimeout, func(path string, val []byte) {
			listenerMutex.Lock()
			defer listenerMutex.Unlock()
			listenerCalls = append(listenerCalls, val)
			wg.Done()
		})
		defer watcher.Close()

		client.Lock()
		client.ExistsResult = false
		client.GetResult = []byte{}
		client.Unlock()

		listenerMutex.Lock()
		client.EventChannel <- zk.Event{
			Type:  zk.EventNodeDeleted,
			Err:   nil,
			State: zk.StateConnected,
		}
		listenerMutex.Unlock()

		wg.Wait()

		listenerMutex.Lock()
		defer listenerMutex.Unlock()

		helper.IntEql(len(listenerCalls), 1)
		helper.IntEql(len(listenerCalls[0]), 0)
		helper.IntEql(len(watcher.Value()), 0)
	})

	t.Run("Calls listener when Node is deleted PollTimeout", func(t *testing.T) {
		helper := tests.H(t)
		client := NewFakeZKClient()
		client.ClientStateResult = Connected
		client.ExistsResult = true
		client.GetResult = []byte("bar")

		var wg sync.WaitGroup
		var listenerMutex sync.Mutex
		wg.Add(1)
		var listenerCalls [][]byte
		watcher, _ := CreateValueNodeWatcher(client, "/foo", shortPollTimeout, func(path string, val []byte) {
			listenerMutex.Lock()
			defer listenerMutex.Unlock()
			listenerCalls = append(listenerCalls, val)
			wg.Done()
		})
		defer watcher.Close()

		client.Lock()
		client.ExistsResult = false
		client.GetResult = []byte{}
		client.Unlock()

		wg.Wait()

		listenerMutex.Lock()
		defer listenerMutex.Unlock()

		helper.IntEql(len(listenerCalls), 1)
		helper.IntEql(len(listenerCalls[0]), 0)
		helper.IntEql(len(watcher.Value()), 0)
	})
}
