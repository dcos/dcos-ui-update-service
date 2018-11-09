package zookeeper

import "fmt"

type ClientState int

const (
	// Disconnected means that the client does not have a valid connection.
	// This may be because the session has been lost, the server is
	// unreachable, or any similar conditions.
	Disconnected ClientState = iota

	// Connected means that the client currently has a valid connection
	Connected ClientState = iota
)

// String implements fmt.Stringer
func (c ClientState) String() string {
	switch c {
	case Connected:
		return "Connected"
	case Disconnected:
		return "Disconnected"
	}
	panic(fmt.Errorf("Unknown client state: %v", int(c)))
}
