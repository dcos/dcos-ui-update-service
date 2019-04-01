package updatemanager

import (
	"fmt"
	"strings"

	"github.com/dcos/dcos-ui-update-service/constants"
)

type UpdateServiceOperation string
type OperationState string

type UpdateServiceStatus struct {
	Operation UpdateServiceOperation
	State     OperationState
	Version   string
}

type UpdateOperationHandler interface {
	ClusterStatusReceived(status UpdateServiceStatus)
}

const (
	IdleOperation          UpdateServiceOperation = "Idle"
	UpdateVersionOperation UpdateServiceOperation = "Update"
	ResetUIOperation       UpdateServiceOperation = "Reset"
	UnknownOperation       UpdateServiceOperation = "Unknown"

	RequestedOperationState  OperationState = "Requested"
	InProgressOperationState OperationState = "InProgress"
	CompleteOperationState   OperationState = "Complete"
	FailedOperationState     OperationState = "Failed"
	IdleOperationState       OperationState = "Idle"
	ReadyOperationState      OperationState = "Ready"
	UnknownOperationState    OperationState = "Unknown"

	StatusSeparator = ":"
)

var knownOperations = map[UpdateServiceOperation]bool{
	IdleOperation:          true,
	UpdateVersionOperation: true,
	ResetUIOperation:       true,
}

var knownOperationStates = map[OperationState]bool{
	RequestedOperationState:  true,
	InProgressOperationState: true,
	CompleteOperationState:   true,
	FailedOperationState:     true,
	IdleOperationState:       true,
	ReadyOperationState:      true,
}

func (op UpdateServiceOperation) String() string {
	return string(op)
}

func (os OperationState) String() string {
	return string(os)
}

func (status UpdateServiceStatus) String() string {
	if status.Operation == IdleOperation {
		return IdleOperation.String()
	}
	if len(status.Version) > 0 {
		return fmt.Sprintf("%s:%s:%s", status.Operation, status.State, status.Version)
	}
	return fmt.Sprintf("%s:%s", status.Operation, status.State)
}

func parseStatusValue(status string) UpdateServiceStatus {
	statusParts := strings.Split(status, StatusSeparator)
	numStatusParts := len(statusParts)
	if numStatusParts < 2 {
		if status == constants.EmptyClusterState || status == constants.IdleClusterState {
			return UpdateServiceStatus{
				Operation: IdleOperation,
				State:     IdleOperationState,
			}
		}
		return UpdateServiceStatus{
			Operation: UnknownOperation,
			State:     UnknownOperationState,
		}
	}
	operation := UpdateServiceOperation(statusParts[0])
	state := OperationState(statusParts[1])
	version := ""
	if numStatusParts > 2 {
		version = statusParts[2]
	}
	if !knownOperations[operation] {
		operation = UnknownOperation
	}
	if !knownOperationStates[state] {
		state = UnknownOperationState
	}

	return UpdateServiceStatus{
		Operation: operation,
		State:     state,
		Version:   version,
	}
}

func idleServiceStatus() UpdateServiceStatus {
	return UpdateServiceStatus{
		Operation: IdleOperation,
		State:     IdleOperationState,
	}
}
