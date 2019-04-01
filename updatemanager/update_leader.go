package updatemanager

import (
	"path"
	"sync"
	"time"

	"github.com/dcos/dcos-ui-update-service/constants"
	"github.com/dcos/dcos-ui-update-service/zookeeper"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

var (
	ErrZookeeperNotConnected   = errors.New("Zookeeper is not connected")
	ErrClusterLockNotAvailable = errors.New("Cluster status lock unavailable to start leading")
)

type UpdateOperationLeader struct {
	UpdateComplete chan bool

	status            *UpdateServiceStatus
	zkPollingInterval time.Duration
	nodeID            string
	numberOfNodes     uint64
	client            zookeeper.ZKClient
	nodeStatusWatcher zookeeper.ParentNodeWatcher
	nodeValueWatchers map[string]zookeeper.ValueNodeWatcher
	nodeStatuses      map[string]UpdateServiceStatus
	log               *logrus.Entry
	sync.Mutex
}

func (um *Client) newResetLeader() (*UpdateOperationLeader, error) {
	if um.zkClient == nil || um.zkClient.ClientState() != zookeeper.Connected {
		return nil, ErrZookeeperNotConnected
	}

	client := um.zkClient
	// ensure we can lead a reset
	cstat, _, err := client.Get(path.Join(client.BasePath(), constants.ClusterStatusNode))
	if err != nil {
		return nil, errors.Wrap(err, "Failed to initialze reset leader")
	}
	clusterStatus := string(cstat)

	if clusterStatus == constants.EmptyClusterState || clusterStatus == constants.IdleClusterState {
		myip, err := um.dcos.DetectIP()
		if err != nil {
			return nil, errors.Wrap(err, "Could not detect master ip for id")
		}
		numNodes, err := um.dcos.MasterCount()
		if err != nil {
			return nil, errors.Wrap(err, "Could not detect number of masters")
		}
		log := logrus.WithFields(logrus.Fields{"package": "updatemanager.update_leader", "operation": "reset"})

		log.Debug("Update Leader (Reset) Created")
		status := &UpdateServiceStatus{
			Operation: ResetUIOperation,
			State:     RequestedOperationState,
		}

		return &UpdateOperationLeader{
			UpdateComplete:    make(chan bool),
			status:            status,
			zkPollingInterval: um.Config.ZKPollingInterval(),
			client:            client,
			nodeID:            myip.String(),
			numberOfNodes:     numNodes,
			nodeValueWatchers: make(map[string]zookeeper.ValueNodeWatcher),
			nodeStatuses:      make(map[string]UpdateServiceStatus),
			log:               log,
		}, nil
	}
	return nil, ErrClusterLockNotAvailable
}

func (ul *UpdateOperationLeader) LockClusterForReset() error {
	_, err := ul.client.Set(path.Join(ul.client.BasePath(), constants.UpdateLeaderNode), []byte(ul.nodeID))
	if err != nil {
		return errors.Wrap(err, "Could not set zk update leader node")
	}
	return ul.setClusterStatus()
}

func (ul *UpdateOperationLeader) Cleanup() {
	ul.log.Debug("Starting leader cleanup")
	ul.Lock()
	ul.nodeStatusWatcher.Close()
	for nodePath, watcher := range ul.nodeValueWatchers {
		watcher.Close()
		delete(ul.nodeValueWatchers, nodePath)
	}
	for nodePath := range ul.nodeStatuses {
		delete(ul.nodeStatuses, nodePath)
	}
	idleStatus := idleServiceStatus()
	ul.status = &idleStatus
	ul.Unlock()
	ul.client.Set(path.Join(ul.client.BasePath(), constants.ClusterStatusNode), []byte("Idle"))
	ul.client.Set(path.Join(ul.client.BasePath(), constants.UpdateLeaderNode), []byte(""))

	ul.log.Debug("cleanup complete, update leader done")
}

func (ul *UpdateOperationLeader) setClusterStatus() error {
	ul.Lock()
	clusterStatus := []byte(ul.status.String())
	ul.Unlock()
	_, err := ul.client.Set(path.Join(ul.client.BasePath(), constants.ClusterStatusNode), clusterStatus)
	if err != nil {
		return errors.Wrap(err, "Could not set zk cluster-status node")
	}
	ul.log.WithField("cluster-status", string(clusterStatus)).Debug("Cluster status set")
	return nil
}

func (ul *UpdateOperationLeader) retrysetClusterStatus() {
	failed := true
	for failed {
		<-time.After(200 * time.Millisecond)
		err := ul.setClusterStatus()
		failed = err != nil
	}
}

func (ul *UpdateOperationLeader) SetupNodeStatusWatcher() error {
	nodeStatusPath := path.Join(ul.client.BasePath(), constants.NodeStatusNode)

	parentWatcher, err := zookeeper.CreateParentNodeWatcher(
		ul.client,
		nodeStatusPath,
		ul.zkPollingInterval,
		ul.nodeStatusListener,
	)
	if err != nil {
		return errors.Wrap(err, "Unable to create node status watcher")
	}
	ul.Lock()
	ul.nodeStatusWatcher = parentWatcher
	ul.Unlock()
	return nil
}

func (ul *UpdateOperationLeader) nodeStatusListener(nodePath string, children []string) {
	ul.log.WithField("children", children).Debug("Received children")
	ul.Lock()
	defer ul.Unlock()
	for _, node := range children {
		// create value watcher
		if _, exists := ul.nodeValueWatchers[node]; !exists {
			nodeStatusPath := path.Join(ul.client.BasePath(), constants.NodeStatusNode, node)
			value, _, err := ul.client.Get(nodeStatusPath)
			if err != nil {
				ul.log.WithError(err).Error("Failed to get node status from zookeeper")
				go ul.failUpdate()
				return
			}
			status := parseStatusValue(string(value))
			ul.nodeStatuses[nodeStatusPath] = status
			watcher, err := zookeeper.CreateValueNodeWatcher(
				ul.client,
				nodeStatusPath,
				ul.zkPollingInterval,
				ul.nodeStatusValueListener,
			)
			if err != nil {
				ul.log.WithError(err).Error("Failed to create node status wathcer")
				go ul.failUpdate()
				return
			}

			ul.nodeValueWatchers[node] = watcher
			ul.log.WithField("zk-node", nodeStatusPath).Trace("Status watcher created for update leader")
		}
	}
	go ul.checkForNewState()
}

func (ul *UpdateOperationLeader) nodeStatusValueListener(path string, value []byte) {
	nodeStatusValue := string(value)
	nodeStatus := parseStatusValue(nodeStatusValue)
	ul.Lock()
	ul.nodeStatuses[path] = nodeStatus
	ul.Unlock()
	go ul.checkForNewState()
}

func (ul *UpdateOperationLeader) checkForNewState() {
	ul.log.Trace("Checking for new cluster state")
	ul.Lock()
	clusterStates := make(map[OperationState]uint64)
	for nodePath, nodeStatus := range ul.nodeStatuses {
		if nodeStatus.Operation != IdleOperation && nodeStatus.Operation != ul.status.Operation {
			ul.log.WithFields(logrus.Fields{"node-path": nodePath, "status": nodeStatus.String()}).Debug("Skipping node b/c Operation doesn't match reset")
			continue
		}
		if _, exists := clusterStates[nodeStatus.State]; exists {
			clusterStates[nodeStatus.State]++
		} else {
			clusterStates[nodeStatus.State] = 1
		}
	}
	currentState := ul.status.State
	ul.Unlock()
	ul.log.WithFields(
		logrus.Fields{
			"state":      currentState,
			"nodeStates": clusterStates,
			"nodes":      ul.numberOfNodes,
		},
	).Trace("Current status of cluster states")
	switch ul.status.Operation {
	case ResetUIOperation:
		newState := ul.newResetState(currentState, clusterStates)
		if newState != currentState {
			go ul.doResetStateChange(newState)
		}
		break
	case UpdateVersionOperation:
		break
	}
}

func (ul *UpdateOperationLeader) newResetState(currentState OperationState, clusterStates map[OperationState]uint64) OperationState {
	switch currentState {
	case RequestedOperationState:
		if val, exists := clusterStates[RequestedOperationState]; exists && val >= ul.numberOfNodes {
			return InProgressOperationState
		}
		break
	case InProgressOperationState:
		if val, exists := clusterStates[FailedOperationState]; exists && val > 0 {
			return FailedOperationState
		}
		if val, exists := clusterStates[CompleteOperationState]; exists && val >= ul.numberOfNodes {
			return CompleteOperationState
		}
		break
	case CompleteOperationState:
		if val, exists := clusterStates[IdleOperationState]; exists && val >= ul.numberOfNodes {
			return IdleOperationState
		}
		break
	case FailedOperationState:
		var failedNodes, completeNodes uint64
		if val, exists := clusterStates[FailedOperationState]; exists {
			failedNodes = val
		} else {
			failedNodes = 0
		}
		if val, exists := clusterStates[CompleteOperationState]; exists {
			completeNodes = val
		} else {
			completeNodes = 0
		}
		if (failedNodes + completeNodes) >= ul.numberOfNodes {
			return IdleOperationState
		}
		break
	}
	return currentState
}

func (ul *UpdateOperationLeader) doResetStateChange(newState OperationState) {
	ul.Lock()
	oldState := ul.status.State
	ul.status.State = newState
	ul.log.WithField("state", newState).Trace("State updated")
	ul.Unlock()
	err := ul.setClusterStatus()
	if err != nil {
		go ul.retrysetClusterStatus()
	}

	if newState == IdleOperationState {
		ul.Lock()
		var resetSuccessful bool
		switch oldState {
		case CompleteOperationState:
			resetSuccessful = true
		case FailedOperationState:
			resetSuccessful = false
		}
		ul.UpdateComplete <- resetSuccessful
		ul.Unlock()
	}
}

func (ul *UpdateOperationLeader) failUpdate() {
	ul.Lock()
	ul.UpdateComplete <- false
	ul.Unlock()
}
