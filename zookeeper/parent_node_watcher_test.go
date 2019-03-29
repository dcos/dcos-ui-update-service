package zookeeper

import (
	"sync"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/samuel/go-zookeeper/zk"

	"github.com/dcos/dcos-ui-update-service/tests"
)

func TestParentNodeWatcher(t *testing.T) {
	const (
		pollTimeout      = time.Duration(30 * time.Second)
		shortPollTimeout = time.Duration(500 * time.Millisecond)
	)

	t.Parallel()

	t.Run("returns ErrListenerNotProvided if listener callback is nil", func(t *testing.T) {
		client := NewFakeZKClient()

		_, err := CreateParentNodeWatcher(client, "/foo", pollTimeout, nil)

		tests.H(t).ErrEql(err, ErrListenerNotProvided)
	})

	t.Run("returns ErrDisconnected if client is Disconnected", func(t *testing.T) {
		client := NewFakeZKClient()

		_, err := CreateParentNodeWatcher(client, "/foo", pollTimeout, func(path string, val []string) {})

		tests.H(t).ErrEql(err, ErrDisconnected)
	})

	t.Run("returns ErrFailedToReadNode if Exists returns error while creating", func(t *testing.T) {
		client := NewFakeZKClient()
		client.ClientStateResult = Connected
		client.ExistsError = errors.New("Boom!!")

		_, err := CreateParentNodeWatcher(client, "/foo", pollTimeout, func(path string, val []string) {})

		tests.H(t).ErrEql(err, ErrFailedToReadNode)
	})

	t.Run("returns ErrNodeDoesNotExist if Exists returns false", func(t *testing.T) {
		client := NewFakeZKClient()
		client.ClientStateResult = Connected
		client.ExistsResult = false

		_, err := CreateParentNodeWatcher(client, "/foo", pollTimeout, func(path string, val []string) {})

		tests.H(t).ErrEql(err, ErrNodeDoesNotExist)
	})

	t.Run("returns watcher if node exists", func(t *testing.T) {
		helper := tests.H(t)
		client := NewFakeZKClient()
		client.ClientStateResult = Connected
		client.ExistsResult = true
		client.ChildrenResults = []string{}

		watcher, err := CreateParentNodeWatcher(client, "/foo", pollTimeout, func(path string, val []string) {})
		defer watcher.Close()

		helper.IsNil(err)
		helper.NotNil(watcher)
		helper.IntEql(len(watcher.Children()), 0)
	})

	t.Run("CreateParentNodeWatcher  sets polling timeout to value given", func(t *testing.T) {
		helper := tests.H(t)
		client := NewFakeZKClient()
		client.ClientStateResult = Connected
		client.ExistsResult = true
		client.ChildrenResults = []string{}

		watcher, _ := CreateParentNodeWatcher(client, "/foo", pollTimeout, func(path string, val []string) {})
		defer watcher.Close()

		parentWatcher, _ := watcher.(*parentNodeWatcher)
		helper.Int64Eql(parentWatcher.pollTimeout.Nanoseconds(), pollTimeout.Nanoseconds())
	})

	t.Run("Calls listener when NodeCreated event is received", func(t *testing.T) {
		helper := tests.H(t)
		client := NewFakeZKClient()
		client.ClientStateResult = Connected
		client.ExistsResult = true
		client.ChildrenResults = []string{}

		var wg sync.WaitGroup
		var listenerMutex sync.Mutex
		var listenerCalls [][]string
		wg.Add(1)

		watcher, _ := CreateParentNodeWatcher(client, "/foo", pollTimeout, func(path string, val []string) {
			listenerMutex.Lock()
			defer listenerMutex.Unlock()
			listenerCalls = append(listenerCalls, val)
			wg.Done()
		})
		defer watcher.Close()

		client.Lock()
		client.ChildrenResults = []string{"bar"}
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
		helper.IntEql(len(listenerCalls[0]), 1)
		helper.StringEql(listenerCalls[0][0], "bar")
		helper.IntEql(len(watcher.Children()), 1)
		helper.StringEql(watcher.Children()[0], "bar")
	})

	t.Run("Calls listener when Children change after pollTimeout", func(t *testing.T) {
		helper := tests.H(t)
		client := NewFakeZKClient()
		client.ClientStateResult = Connected
		client.ExistsResult = true
		client.ChildrenResults = []string{}

		var wg sync.WaitGroup
		var listenerMutex sync.Mutex
		var listenerCalls [][]string
		wg.Add(1)

		watcher, _ := CreateParentNodeWatcher(client, "/foo", shortPollTimeout, func(path string, val []string) {
			listenerMutex.Lock()
			defer listenerMutex.Unlock()
			listenerCalls = append(listenerCalls, val)
			wg.Done()
		})
		defer watcher.Close()

		client.Lock()
		client.ChildrenResults = []string{"bar"}
		client.Unlock()

		wg.Wait()

		listenerMutex.Lock()
		defer listenerMutex.Unlock()
		helper.IntEql(len(listenerCalls), 1)
		helper.IntEql(len(listenerCalls[0]), 1)
		helper.StringEql(listenerCalls[0][0], "bar")
		helper.IntEql(len(watcher.Children()), 1)
		helper.StringEql(watcher.Children()[0], "bar")
	})

	t.Run("Calls listener when NodeDeleted event is received", func(t *testing.T) {
		helper := tests.H(t)
		client := NewFakeZKClient()
		client.ClientStateResult = Connected
		client.ExistsResult = true
		client.ChildrenResults = []string{"bar", "baz"}

		var wg sync.WaitGroup
		var listenerMutex sync.Mutex
		var listenerCalls [][]string
		wg.Add(1)

		watcher, _ := CreateParentNodeWatcher(client, "/foo", pollTimeout, func(path string, val []string) {
			listenerMutex.Lock()
			defer listenerMutex.Unlock()
			listenerCalls = append(listenerCalls, val)
			wg.Done()
		})
		defer watcher.Close()
		client.Lock()
		client.ChildrenResults = []string{"baz"}
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
		helper.IntEql(len(listenerCalls[0]), 1)
		helper.StringEql(listenerCalls[0][0], "baz")
		helper.IntEql(len(watcher.Children()), 1)
		helper.StringEql(watcher.Children()[0], "baz")
	})
}
