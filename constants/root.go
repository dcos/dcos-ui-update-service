package constants

import (
	"time"
)

const (
	VersionNode       = "version"
	UpdateNode        = "updated"
	ClusterStatusNode = "cluster-status"
	UpdateLeaderNode  = "update-leader"
	NodeStatusNode    = "node-status"

	EmptyClusterState = ""
	IdleClusterState  = "Idle"

	PreBundledUIVersion = ""

	ZKNodeInitMaxRetries  = 10
	ZKNodeWriteMaxRetries = 3
)

var (
	UpdateOperationTimeout   = 6 * time.Minute
	ZKNodeWriteRetryInterval = 100 * time.Millisecond
)
