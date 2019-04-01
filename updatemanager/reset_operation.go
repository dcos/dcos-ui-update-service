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

type ResetOperation struct {
	completed     chan struct{}
	nodeStatus    UpdateServiceStatus
	clusterStatus UpdateServiceStatus
	zkNodePath    string
	parent        *Client
	log           *logrus.Entry
	sync.Mutex
}

func (um *Client) newResetOperation(clusterStatus UpdateServiceStatus) (UpdateOperationHandler, <-chan struct{}, error) {
	var nodeStatus UpdateServiceStatus
	switch clusterStatus.State {
	case RequestedOperationState:
		nodeStatus = UpdateServiceStatus{
			Operation: ResetUIOperation,
			State:     RequestedOperationState,
		}
		break
	default:
		// Reset Operation can only be created when cluster is in the Requested state
		nodeStatus = UpdateServiceStatus{
			Operation: ResetUIOperation,
			State:     FailedOperationState,
		}
		break
	}
	myIP, err := um.dcos.DetectIP()
	if err != nil {
		return nil, nil, errors.Wrap(err, "Could not detect master ip for id")
	}
	zkNodePath := path.Join(um.zkClient.BasePath(), constants.NodeStatusNode, myIP.String())

	log := logrus.WithField("package", "updatemanager.reset_operation")
	log.Debug("Reset operation created")
	handler := &ResetOperation{
		completed:     make(chan struct{}),
		nodeStatus:    nodeStatus,
		clusterStatus: clusterStatus,
		parent:        um,
		zkNodePath:    zkNodePath,
		log:           log,
	}
	go handler.initNodeStatus()
	go handler.startTimeout()
	return handler, handler.completed, nil
}

func (rop *ResetOperation) ClusterStatusReceived(status UpdateServiceStatus) {
	rop.Lock()
	defer rop.Unlock()

	rop.log.WithFields(
		logrus.Fields{
			"new-status":     status.String(),
			"current-status": rop.clusterStatus.String(),
		},
	).Trace("Received Cluster Status")
	if status == rop.clusterStatus {
		return
	}
	rop.clusterStatus = status
	newNodeState := rop.newNodeState(rop.nodeStatus.State, status.State)
	if newNodeState != rop.nodeStatus.State {
		// Do State Change
		go rop.doStateChange(newNodeState)
	}
}

func (rop *ResetOperation) initNodeStatus() {
	rop.Lock()
	defer rop.Unlock()
	attempts := 0
	for {
		zkClient := rop.parent.zkClient
		zkNodeValue := []byte(rop.nodeStatus.String())
		exists, _, err := zkClient.Exists(rop.zkNodePath)
		if err == nil {
			rop.log.WithField("node-status", rop.nodeStatus.String()).Debug("Setting node status")
			if !exists {
				err = zkClient.Create(rop.zkNodePath, zkNodeValue, zookeeper.PermAll)
				if err == nil {
					return
				}
			} else {
				_, err = zkClient.Set(rop.zkNodePath, zkNodeValue)
				if err == nil {
					return
				}
			}
		}
		attempts++
		if attempts > constants.ZKNodeInitMaxRetries {
			rop.initFailed(err)
			return
		}
		//init failed, wait then retry init
		<-time.After(constants.ZKNodeWriteRetryInterval)
	}
}

func (rop *ResetOperation) initFailed(err error) {
	rop.log.WithError(err).Warn("Initializing reset operation failed")
	rop.completeOperation()
}

func (rop *ResetOperation) updateNodeStatus() {
	writeAttempts := 0
	for {
		rop.Lock()
		rop.log.WithField("node-status", rop.nodeStatus.String()).Debug("Setting node status")
		zkClient := rop.parent.zkClient
		zkNodeValue := []byte(rop.nodeStatus.String())
		_, err := zkClient.Set(rop.zkNodePath, zkNodeValue)
		rop.Unlock()
		writeAttempts++
		if err == nil {
			return
		}
		rop.log.WithError(err).Warning("Failed to update node status in ZK")
		if writeAttempts > constants.ZKNodeWriteMaxRetries {
			rop.completeOperation()
			return
		}
		<-time.After(constants.ZKNodeWriteRetryInterval)
	}
}

func (rop *ResetOperation) removeNodeStatus() {
	deleteAttempts := 0
	for {
		rop.Lock()
		rop.log.Debug("Removing node status")
		zkClient := rop.parent.zkClient
		err := zkClient.Delete(rop.zkNodePath)
		rop.Unlock()
		if err == nil {
			return
		}
		deleteAttempts++
		if deleteAttempts > constants.ZKNodeWriteMaxRetries {
			rop.log.WithError(err).Error("Failed to remove node status in ZK, giving up after max retries")
			return
		}
		rop.log.WithError(err).Warning("Failed to remove node status in ZK")
		<-time.After(constants.ZKNodeWriteRetryInterval)
	}
}

func (rop *ResetOperation) newNodeState(state, clusterState OperationState) OperationState {
	switch state {
	case RequestedOperationState:
		// Operation is waiting to start
		switch clusterState {
		case InProgressOperationState:
			// Cluster has update to InProgess, we can start reset
			return InProgressOperationState
		case RequestedOperationState:
			// Cluster still waiting
			return RequestedOperationState
		case FailedOperationState:
			// Reset Failed, mostlikely timeout waiting for all masters
			return FailedOperationState
		case IdleOperationState:
			return IdleOperationState
		default:
			// Unexpected cluster state, fail reset
			return FailedOperationState
		}
	case InProgressOperationState:
		// If we're in Progress ignore cluster changes, until we're done
		return state
	case CompleteOperationState:
		// Our Reset has been completed and we're waiting on other master to finish the operation
		switch clusterState {
		case CompleteOperationState:
			// Cluster has gone to completed, we go to idle and clean-up
			return IdleOperationState
		default:
			// still waiting for all resets to complete | fail, there is nothing we can do if cluster "fails" after we've completed
			return CompleteOperationState
		}
	case FailedOperationState:
		// Our Reset has been completed and we're waiting on other master to finish the operation
		switch clusterState {
		case IdleOperationState:
			fallthrough
		case FailedOperationState:
			fallthrough
		case CompleteOperationState:
			// Cluster has gone to idle, we can clean-up
			return IdleOperationState
		default:
			// still waiting for all resets to complete/fail, continue to report we failed our reset
			return FailedOperationState
		}
	default:
		// Unexpected current state
		return state
	}
}

func (rop *ResetOperation) doStateChange(newState OperationState) {
	rop.log.WithField("state", newState).Debug("Reset operation state change")

	rop.Lock()
	rop.nodeStatus.State = newState
	rop.Unlock()
	switch newState {
	case InProgressOperationState:
		//Start reset
		rop.updateNodeStatus()
		go rop.doReset()
		return
	case CompleteOperationState:
		// complete reset
		rop.updateNodeStatus()
		return
	case FailedOperationState:
		rop.updateNodeStatus()
		return
	case IdleOperationState:
		// clean-up
		rop.completeOperation()
		return
	}
}

func (rop *ResetOperation) doReset() {
	um := rop.parent
	currentVersion, err := um.CurrentVersion()
	if err != nil {
		rop.log.WithError(err).Error("Sync reset failed to get current version")
		rop.doStateChange(FailedOperationState)
		return
	}
	rop.log.WithField("CurrentVersion", currentVersion).Debug("Sync Reset Started")
	if currentVersion != constants.PreBundledUIVersion {
		err = um.UpdateServedVersion(um.Config.DefaultDocRoot())
		if err != nil {
			rop.log.WithError(err).Error("Sync reset failed to update served version")
			rop.doStateChange(FailedOperationState)
			return
		}
	}
	err = um.RemoveAllVersionsExcept(constants.PreBundledUIVersion)
	if err != nil {
		rop.log.WithError(err).Error("Sync reset failed to remove all old versions")
		rop.doStateChange(FailedOperationState)
		return
	}
	rop.log.Info("Sync Reset Completed")
	rop.doStateChange(CompleteOperationState)
	// We ignore cluster changes while in progress, so process current cluster
	// status when we complete just in case it changed.
	go rop.ClusterStatusReceived(rop.parent.ClusterStatus())
}

func (rop *ResetOperation) completeOperation() {
	rop.removeNodeStatus()
	close(rop.completed)
	rop.log.Debug("Reset operation cleaned up done")
}

func (rop *ResetOperation) startTimeout() {
	select {
	case <-time.After(constants.UpdateOperationTimeout):
		rop.log.Warn("Reset operation timed out")
		rop.completeOperation()
		return
	case <-rop.completed:
		return
	}
}
