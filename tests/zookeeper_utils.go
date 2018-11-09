// nolint
package tests

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

// ZkControl allows testing code to manipulate a running ZK instance.
type ZkControl struct {
	dockerClient *client.Client
	containerID  string
	addr         string
	teardownOnce sync.Once
}

// Addr returns the address of the zookeeper node
func (z *ZkControl) Addr() string {
	return z.addr
}

// Teardown destroys the ZK container
func (z *ZkControl) Teardown() error {
	fmt.Println("Starting requested teardown of ZK container")
	var err error
	z.teardownOnce.Do(func() {
		err = removeContainer(z.dockerClient, z.containerID)
		if err == nil {
			fmt.Println("Successfully removed ZK container")
		}
	})
	if err != nil {
		return errors.Wrap(err, "could not remove ZK container")
	}
	return nil
}

// TeardownPanic destroys the ZK container and panics if unsuccessful
func (z *ZkControl) TeardownPanic() {
	if err := z.Teardown(); err != nil {
		panic(err)
	}
}

// StartZookeeper starts a new zookeeper container
func StartZookeeper() (*ZkControl, error) {
	dcli, err := DockerClient()
	if err != nil {
		return nil, errors.Wrap(err, "could not get docker client")
	}
	image := "docker.io/jplock/zookeeper:3.4.13"
	if err = pullDockerImage(dcli, image); err != nil {
		return nil, err
	}
	zkContainerName := "ui-update-test-zk"
	containerConfig := &container.Config{
		Image:      image,
		Entrypoint: []string{"/opt/zookeeper/bin/zkServer.sh"},
		Cmd:        []string{"start-foreground"},
		ExposedPorts: nat.PortSet{
			"2181/tcp": struct{}{},
		},
	}
	hostConfig := &container.HostConfig{
		PortBindings: nat.PortMap{
			"2181/tcp": []nat.PortBinding{
				{
					HostIP:   "0.0.0.0",
					HostPort: "2181",
				},
			},
		},
	}

	r, err := dcli.ContainerCreate(context.Background(), containerConfig, hostConfig, nil, zkContainerName)
	if err != nil {
		fmt.Printf("error creating zk container, %v\n", err)
		return nil, errors.Wrap(err, "could not create zk container")
	}
	// create a teardown that will be used here to try to tear down the
	// container if anything fails in setup
	cleanup := func() {
		removeContainer(dcli, r.ID)
	}
	// start the container
	if err := dcli.ContainerStart(context.Background(), r.ID, types.ContainerStartOptions{}); err != nil {
		cleanup()
		return nil, errors.Wrap(err, "could not start zk container")
	}
	addr := "127.0.0.1:2181"
	done := make(chan struct{})
	defer close(done)
	connected := make(chan struct{})
	go func() {
		for {
			select {
			case <-done:
				return
			default:
				conn, err := net.Dial("tcp", addr)
				if err != nil {
					time.Sleep(1)
					continue
				}
				fmt.Println("successfully connected to ZK at", addr)
				conn.Close()
				close(connected)
				return
			}
		}
	}()
	timeout := 10 * time.Second
	select {
	case <-connected:
	case <-time.After(timeout):
		cleanup()
		return nil, errors.Errorf("could not connect to zookeeper in %s", timeout)
	}
	control := &ZkControl{
		dockerClient: dcli,
		containerID:  r.ID,
		addr:         addr,
	}
	return control, nil
}
