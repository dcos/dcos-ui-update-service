package zookeeper

import (
	"bytes"
	"sync"
	"time"

	"github.com/jpillora/backoff"
	"github.com/samuel/go-zookeeper/zk"
	"github.com/sirupsen/logrus"
)

// ValueNodeWatchListener function signature for value node watcher listner, this is used to invoke a callback when a ZK node changes
type ValueNodeWatchListener func([]byte)
type valueNodeWatcher struct {
	client       ZKClient
	nodePath     string
	pollTimeout  time.Duration
	lastVersion  int32
	value        []byte
	eventChannel <-chan zk.Event
	disconnected chan struct{}
	closed       chan struct{}
	listener     ValueNodeWatchListener
	watchActive  bool
	log          *logrus.Entry
	watchMutex   sync.Mutex
}

// CreateValueNodeWatcher returns a ZK ValueNodeWatcher that will call the provided listener when the node value changes, is created or deleted.
func CreateValueNodeWatcher(client ZKClient, path string, polltimeout time.Duration, listener ValueNodeWatchListener) (ValueNodeWatcher, error) {
	if listener == nil {
		return nil, ErrListenerNotProvided
	}
	// Ensure client is currently connected, otherwise error
	if client.ClientState() == Disconnected {
		return nil, ErrDisconnected
	}
	exists, ver, err := client.Exists(path)
	if err != nil {
		return nil, ErrFailedToReadNode
	}
	var value []byte
	if exists {
		value, ver, err = client.Get(path)
		if err != nil {
			return nil, ErrFailedToReadNode
		}
	} else {
		value = []byte{}
	}

	nw := &valueNodeWatcher{
		client:       client,
		nodePath:     path,
		pollTimeout:  polltimeout,
		lastVersion:  ver,
		value:        value,
		eventChannel: nil,
		disconnected: nil,
		closed:       make(chan struct{}),
		listener:     listener,
		watchActive:  false,
		log: logrus.WithFields(logrus.Fields{
			"package": "zookeeper.value_node_watcher",
			"zk-node": path,
		}),
	}
	nw.log.Debug("Value watcher created.")
	nw.log.Tracef("Value poll timeout %d", int64(polltimeout))
	client.RegisterListener(path, nw.handleZkStateChange)
	return nw, nil
}

func (nw *valueNodeWatcher) Value() []byte {
	nw.watchMutex.Lock()
	defer nw.watchMutex.Unlock()
	return nw.value
}

func (nw *valueNodeWatcher) Path() string {
	return nw.nodePath
}

func (nw *valueNodeWatcher) Close() {
	close(nw.closed)
	nw.client.UnregisterListener(nw.nodePath)
}

func (nw *valueNodeWatcher) handleZkStateChange(state ClientState) {
	nw.watchMutex.Lock()
	defer nw.watchMutex.Unlock()
	if state == Disconnected {
		nw.log.Debug("ZK Disconnected, stopping node watcher")
		close(nw.disconnected)
		nw.disconnected = nil
	}
	if state == Connected {
		if nw.disconnected == nil {
			nw.disconnected = make(chan struct{})
		}
		if !nw.watchActive {
			nw.log.Debug("ZK Connected, starting node watcher")
			go nw.startWatch()
		}
	}
}

func (nw *valueNodeWatcher) startWatch() {
	nw.watchMutex.Lock()

	nw.log.Trace("Creating ZK eventChannel")
	_, _, eventChannel, err := nw.client.existsW(nw.nodePath)
	if err != nil {
		nw.log.WithError(err).Warn("Unable to create ZK eventChannel, will retry")
		nw.watchMutex.Unlock()
		go nw.restartWatchAfterError()
		return
	}
	nw.eventChannel = eventChannel
	if !nw.watchActive {
		nw.watchActive = true
	}
	nw.watchMutex.Unlock()
	nw.waitOrPoll()
}

func (nw *valueNodeWatcher) waitOrPoll() {
	for {
		nw.log.Trace("waiting for node event")
		select {
		// Node event received
		case nodeEvent := <-nw.eventChannel:
			nw.handleNodeEvent(nodeEvent)
			return
		// Connection disconnected
		case <-nw.disconnected:
			nw.handleDisconnected()
			return
		// Watch Closed
		case <-nw.closed:
			nw.handleClosed()
			return
		// Timeout and poll
		case <-time.After(nw.pollTimeout):
			nw.handlePollTimeout()
		}
	}
}

func (nw *valueNodeWatcher) clearWatch() {
	if nw.eventChannel != nil {
		nw.eventChannel = nil
	}
	if nw.watchActive {
		nw.watchActive = false
	}
}

func (nw *valueNodeWatcher) restartWatchAfterError() {
	nw.clearWatch()
	b := &backoff.Backoff{
		Min:    5 * time.Second,
		Max:    5 * time.Minute,
		Factor: 2,
		Jitter: false,
	}
	for {
		// Ensure we're connected, otherwise exit
		if nw.client.ClientState() == Disconnected {
			nw.log.Info("Exiting watch retry because ZK connection was lost")
			return
		}
		nw.watchMutex.Lock()
		exists, ver, eventChannel, err := nw.client.existsW(nw.nodePath)
		if err == nil {
			if exists {
				value, getVersion, err := nw.client.Get(nw.nodePath)
				if err == nil {
					nw.handleValueReceived(value, getVersion)
				}
			} else {
				nw.handleValueReceived(nil, ver)
			}
			nw.log.Info("Watch re-established after error")
			nw.eventChannel = eventChannel
			nw.watchActive = true
			nw.watchMutex.Unlock()
			go nw.waitOrPoll()
		}
		nw.watchMutex.Unlock()
		select {
		case <-nw.disconnected:
			nw.handleDisconnected()
			return
		case <-nw.closed:
			nw.handleClosed()
			return
		case <-time.After(b.Duration()):
			nw.log.Trace("Retrying to create watch after error")
		}
	}
}

func (nw *valueNodeWatcher) handlePollTimeout() {
	nw.log.Debug("Timeout poll of value")
	found, ver, err := nw.client.Exists(nw.nodePath)
	if err != nil {
		nw.log.WithError(err).Warn("Failed to check ZK node exists when polling")
		return
	}
	if found {
		err := nw.getAndUpdateValue()
		if err != nil {
			nw.log.WithError(err).Warn("Failed to get ZK node value when polling")
		}
	} else {
		nw.handleValueReceived([]byte{}, ver)
	}

}

func (nw *valueNodeWatcher) handleDisconnected() {
	nw.log.Debug("Lost ZK Connection")
	nw.clearWatch()
}

func (nw *valueNodeWatcher) handleClosed() {
	nw.log.Debug("Watcher closed")
	nw.clearWatch()
}

func (nw *valueNodeWatcher) handleNodeEvent(event zk.Event) {
	if event.Err != nil {
		nw.log.WithError(event.Err).Warn("Received error from ZK eventChannel")
		go nw.restartWatchAfterError()
		return
	}
	if event.State == zk.StateDisconnected {
		nw.handleDisconnected()
		return
	}
	nw.log.WithField("zk-node-event", event.Type.String()).Debug("Received ZK Node Event")
	switch event.Type {
	case zk.EventNodeCreated:
		fallthrough
	case zk.EventNodeDataChanged:
		err := nw.getAndUpdateValue()
		if err != nil {
			nw.log.WithError(err).Warn("Error getting node value")
			go nw.restartWatchAfterError()
			return
		}
		go nw.startWatch()
		return
	case zk.EventNodeDeleted:
		err := nw.handleNodeDeleted()
		if err != nil {
			go nw.restartWatchAfterError()
			return
		}
		go nw.startWatch()
		return
	}
}

func (nw *valueNodeWatcher) getAndUpdateValue() error {
	value, ver, err := nw.client.Get(nw.nodePath)
	if err != nil {
		return err
	}
	nw.handleValueReceived(value, ver)
	return nil
}

func (nw *valueNodeWatcher) handleNodeDeleted() error {
	exists, ver, err := nw.client.Exists(nw.nodePath)
	if err != nil {
		nw.log.WithError(err).Warn("Failed to check ZK node exists")
		return err
	}
	if exists {
		err = nw.getAndUpdateValue()
		if err != nil {
			nw.log.WithError(err).Warn("Error getting node value")
		}
		return err
	}
	nw.handleValueReceived([]byte{}, ver)
	return nil
}

func (nw *valueNodeWatcher) handleValueReceived(value []byte, version int32) {
	if value != nil {
		nw.log.WithField("zk-node-value", string(value)).Trace("Received ZK Node Value")
	} else {
		nw.log.WithField("zk-node-value", "nil").Trace("Received ZK Node Value")
	}

	nw.watchMutex.Lock()
	defer nw.watchMutex.Unlock()
	if !bytes.Equal(nw.value, value) || nw.lastVersion != version {
		nw.value = value

		nw.log.WithFields(
			logrus.Fields{
				"current-value":        string(nw.value),
				"node-value":           string(value),
				"current-stat-version": nw.lastVersion,
				"stat-version":         version,
			},
		).Debug("Executing listener callback")
		go nw.listener(nw.value)
	} else {
		nw.log.Trace("Value matched current value")
	}
	nw.lastVersion = version
}
