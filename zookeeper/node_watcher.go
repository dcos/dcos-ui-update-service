package zookeeper

import (
	"time"

	"github.com/pkg/errors"
)

var (
	// ErrListenerNotProvided error if Create watcher function given a nil listener
	ErrListenerNotProvided = errors.New("A listener callback must be provided to create a node watcher")
	// ErrDisconnected if you attempt to create a watcher while ZK is disconnected
	ErrDisconnected = errors.New("ZK connection is current disconnected, cannot create a node watcher")
	// ErrNodeDoesNotExist if you attempt to create a ParentNodeWatcher for a node that does not exist
	ErrNodeDoesNotExist = errors.New("The node must exist to create a ParentNodeWatcher")
	// ErrFailedToReadNode if the given node cannot be read
	ErrFailedToReadNode = errors.New("Failed to read node from ZK")
)

type ParentNodeWatcher interface {
	Children() []string
	Path() string
	Close()
}

type ValueNodeWatcher interface {
	Value() []byte
	Path() string
	Close()
}

// CreateParentNodeWatcher returns a ZK ParentNodeWatcher that will call the provided listener when the children of the node are modified.
// Will poll the node for changes with the default watcherPollTimeout
func CreateParentNodeWatcher(client ZKClient, path string, pollTimeout time.Duration, listener ParentNodeWatchListener) (ParentNodeWatcher, error) {
	if listener == nil {
		return nil, ErrListenerNotProvided
	}
	// Ensure client is currently connected, otherwise error
	if client.ClientState() == Disconnected {
		return nil, ErrDisconnected
	}
	return createParentWatcher(client, path, pollTimeout, listener)
}

// CreateValueNodeWatcher returns a ZK ValueNodeWatcher that will call the provided listener when the node value changes, is created or deleted.
// Will poll the node for changes with the default watcherPollTimeout
func CreateValueNodeWatcher(client ZKClient, path string, pollTimeout time.Duration, listener ValueNodeWatchListener) (ValueNodeWatcher, error) {
	if listener == nil {
		return nil, ErrListenerNotProvided
	}
	// Ensure client is currently connected, otherwise error
	if client.ClientState() == Disconnected {
		return nil, ErrDisconnected
	}
	return createValueWatcher(client, path, pollTimeout, listener)
}
