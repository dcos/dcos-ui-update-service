package zookeeper

import (
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
