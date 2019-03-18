package zookeeper

import (
	"github.com/samuel/go-zookeeper/zk"
)

type FakeZKClient struct {
	ExistsError   error
	GetError      error
	CreateError   error
	SetError      error
	ChildrenError error

	ClientStateResult ClientState
	Listeners         []StateListener
	IDListeners       map[string]StateListener
	ExistsResult      bool
	GetResult         []byte
	GetResults        [][]byte
	GetResultsIndex   int
	ChildrenResults   []string
	EventChannel      chan zk.Event

	CreateCall func(string, []byte, []int32)
	SetCall    func(string, []byte)
}

func NewFakeZKClient() *FakeZKClient {
	return &FakeZKClient{
		IDListeners:  make(map[string]StateListener),
		EventChannel: make(chan zk.Event, 1),
	}
}

func (zkc *FakeZKClient) Close() {}

func (zkc *FakeZKClient) ClientState() ClientState {
	return zkc.ClientStateResult
}

func (zkc *FakeZKClient) RegisterListener(listener StateListener) {
	zkc.Listeners = append(zkc.Listeners, listener)
	listener(zkc.ClientStateResult)
}

func (zkc *FakeZKClient) RegisterListenerWithID(id string, listener StateListener) {
	zkc.IDListeners[id] = listener
	listener(zkc.ClientStateResult)
}

func (zkc *FakeZKClient) UnregisterListener(id string) {
	delete(zkc.IDListeners, id)
}

func (zkc *FakeZKClient) PublishStateChange(newState ClientState) {
	zkc.ClientStateResult = newState
	for _, l := range zkc.Listeners {
		l(newState)
	}
	for _, l := range zkc.IDListeners {
		l(newState)
	}
}

func (zkc *FakeZKClient) Exists(path string) (bool, int32, error) {
	if zkc.ExistsError != nil {
		return false, -1, zkc.ExistsError
	}
	return zkc.ExistsResult, 0, nil
}

func (zkc *FakeZKClient) existsW(path string) (bool, int32, <-chan zk.Event, error) {
	found, ver, err := zkc.Exists(path)
	return found, ver, zkc.EventChannel, err
}

func (zkc *FakeZKClient) Get(path string) ([]byte, int32, error) {
	if zkc.GetError != nil {
		return nil, -1, zkc.GetError
	}
	numGetResults := len(zkc.GetResults)
	if numGetResults > 0 {
		resultIndex := zkc.GetResultsIndex
		result := zkc.GetResults[resultIndex]
		if zkc.GetResultsIndex < (numGetResults - 1) {
			zkc.GetResultsIndex++
		}
		return result, int32(resultIndex), nil
	}
	return zkc.GetResult, 0, nil
}

func (zkc *FakeZKClient) getW(path string) ([]byte, int32, <-chan zk.Event, error) {
	val, ver, err := zkc.Get(path)
	return val, ver, zkc.EventChannel, err
}

func (zkc *FakeZKClient) Create(path string, data []byte, perms []int32) error {
	if zkc.CreateCall != nil {
		zkc.CreateCall(path, data, perms)
	}
	if zkc.CreateError != nil {
		return zkc.CreateError
	}
	return nil
}

func (zkc *FakeZKClient) Set(path string, data []byte) (int32, error) {
	if zkc.SetCall != nil {
		zkc.SetCall(path, data)
	}
	if zkc.SetError != nil {
		return -1, zkc.SetError
	}
	return 0, nil
}

func (zkc *FakeZKClient) Children(path string) ([]string, int32, error) {
	if zkc.ChildrenError != nil {
		return nil, -1, zkc.ChildrenError
	}
	return zkc.ChildrenResults, 0, nil
}

func (zkc *FakeZKClient) childrenW(path string) ([]string, int32, <-chan zk.Event, error) {
	val, ver, err := zkc.Children(path)
	return val, ver, zkc.EventChannel, err
}
