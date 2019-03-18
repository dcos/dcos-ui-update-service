package zookeeper

import (
	"time"

	"github.com/jpillora/backoff"
	"github.com/samuel/go-zookeeper/zk"
	"github.com/sirupsen/logrus"
)

// ParentNodeWatchListener function signature for parent node watcher listner, this is used to invoke a callback when a ZK node changes
type ParentNodeWatchListener func([]string)

type parentNodeWatcher struct {
	client       ZKClient
	nodePath     string
	pollTimeout  time.Duration
	lastVersion  int32
	children     []string
	eventChannel <-chan zk.Event
	disconnected chan struct{}
	closed       chan struct{}
	listener     ParentNodeWatchListener
	watchActive  bool
	log          *logrus.Entry
}

func (nw *parentNodeWatcher) Children() []string {
	return nw.children
}

func (nw *parentNodeWatcher) Path() string {
	return nw.nodePath
}

func (nw *parentNodeWatcher) Close() {
	close(nw.closed)
	nw.client.UnregisterListener(nw.nodePath)
}

func createParentWatcher(client ZKClient, path string, polltimeout time.Duration, listener ParentNodeWatchListener) (ParentNodeWatcher, error) {
	// For parent node ensure it exists
	exists, _, err := client.Exists(path)
	if err != nil {
		return nil, ErrFailedToReadNode
	}
	if !exists {
		return nil, ErrNodeDoesNotExist
	}

	// Get current value
	children, ver, err := client.Children(path)
	if err != nil {
		return nil, ErrFailedToReadNode
	}

	nw := &parentNodeWatcher{
		client:       client,
		nodePath:     path,
		pollTimeout:  polltimeout,
		lastVersion:  ver,
		children:     children,
		eventChannel: nil,
		disconnected: nil,
		closed:       make(chan struct{}),
		listener:     listener,
		watchActive:  false,
		log: logrus.WithFields(logrus.Fields{
			"package": "zookeeper.parent_node_watcher",
			"zk-node": path,
		}),
	}
	nw.log.Debug("Parent watcher created.")
	nw.log.Tracef("Parent poll timeout %d", int64(polltimeout))
	client.RegisterListenerWithID(path, nw.handleZkStateChange)
	return nw, nil
}

func (nw *parentNodeWatcher) handleZkStateChange(state ClientState) {
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

func (nw *parentNodeWatcher) startWatch() {
	nw.log.Trace("Creating ZK eventChannel")
	children, ver, eventChannel, err := nw.client.childrenW(nw.nodePath)
	if err != nil {
		nw.log.WithError(err).Warn("Unable to create ZK eventChannel, will retry")
		go nw.restartWatchAfterError()
		return
	}
	nw.handleChildrenReceived(children, ver)
	nw.watchChannelCreated(eventChannel)
}

func (nw *parentNodeWatcher) watchChannelCreated(channel <-chan zk.Event) {
	nw.log.Trace("ZK eventChannel created")
	nw.eventChannel = channel
	if !nw.watchActive {
		nw.watchActive = true
	}
	nw.waitOrPoll()
}

func (nw *parentNodeWatcher) waitOrPoll() {
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

func (nw *parentNodeWatcher) restartWatchAfterError() {
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
		children, stat, eventChannel, err := nw.client.childrenW(nw.nodePath)
		if err == nil {
			nw.handleChildrenReceived(children, stat)
			nw.log.Info("Watch re-established after error")
			nw.watchChannelCreated(eventChannel)
			return
		}
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

func (nw *parentNodeWatcher) handlePollTimeout() {
	nw.log.Debug("Timeout poll of value")
	err := nw.getAndUpdateChildren()
	if err != nil {
		nw.log.WithError(err).Warn("Failed to get ZK node value when polling")
	}
}

func (nw *parentNodeWatcher) handleDisconnected() {
	nw.log.Debug("Lost ZK Connection")
	nw.clearWatch()
}

func (nw *parentNodeWatcher) handleClosed() {
	nw.log.Debug("Watcher closed")
	nw.clearWatch()
}

func (nw *parentNodeWatcher) clearWatch() {
	if nw.eventChannel != nil {
		nw.eventChannel = nil
	}
	if nw.watchActive {
		nw.watchActive = false
	}
}

func (nw *parentNodeWatcher) handleNodeEvent(event zk.Event) {
	if event.Err != nil {
		nw.log.WithError(event.Err).Warn("Received error from ZK eventChannel")
		go nw.restartWatchAfterError()
		return
	}
	if event.State == zk.StateDisconnected {
		nw.handleDisconnected()
		return
	}
	go nw.startWatch()
}

func (nw *parentNodeWatcher) getAndUpdateChildren() error {
	children, stat, err := nw.client.Children(nw.nodePath)
	if err != nil {
		return err
	}
	nw.handleChildrenReceived(children, stat)
	return nil
}

func (nw *parentNodeWatcher) handleChildrenReceived(children []string, version int32) {
	nw.log.WithField("zk-node-children", children).Trace("Received ZK Node Children")
	if !childrenEqual(nw.children, children) || nw.lastVersion != version {
		nw.children = children
		nw.log.WithFields(
			logrus.Fields{
				"current-children":     nw.children,
				"zk-node-children":     children,
				"current-stat-version": nw.lastVersion,
				"stat-version":         version,
			},
		).Debug("Executing listener callback")
		go nw.listener(nw.children)
	} else {
		nw.log.Trace("Value matched current value")
	}
	nw.lastVersion = version
}

func childrenEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}
