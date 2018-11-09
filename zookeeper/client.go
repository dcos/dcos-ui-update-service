package zookeeper

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/dcos/dcos-ui-update-service/config"
	"github.com/pkg/errors"
	"github.com/samuel/go-zookeeper/zk"
)

type StateListener func(state ClientState)

type Client struct {
	conn        *zk.Conn
	acl         []zk.ACL
	basePath    string
	nodeOwner   string
	nodeSchema  string
	zkState     zk.State
	clientState ClientState
	listeners   []StateListener
	sync.Mutex
}

type ZKClient interface {
	Close()
	ClientState() ClientState
	RegisterListener(listener StateListener)

	Exists(path string) (bool, error)
	ExistsW(path string) (bool, <-chan zk.Event, error)
	Get(path string) ([]byte, error)
	Create(path string, data []byte, perms []int32) error
	Set(path string, data []byte) error
}

var (
	// PermAll grants all permissions
	PermAll = []int32{zk.PermAll}
)

type zkConfig struct {
	BasePath       string
	ZnodeOwner     string
	AuthInfo       string
	Address        string
	SessionTimeout time.Duration
	ConnectTimeout time.Duration
}

// schemaOwner composes a schema and owner
type schemaOwner struct {
	schema string
	owner  string
}

var (
	errSchemaOwnerFormat           = errors.New("format: 'schema:owner'")
	errCouldNotEstablishConnection = errors.New("connection to ZK could not be established")
	defaultZnodeOwner              = &schemaOwner{schema: "world", owner: "anyone"}
	zkNoFlags                      = int32(0)
	zkNoVersion                    = int32(-1)
)

// exported functions
// Connect creates and initializes a zookeeper client
func Connect(cfg *config.Config) (*Client, error) {
	return connect(zkConfig{
		BasePath:       cfg.ZKBasePath,
		ZnodeOwner:     cfg.ZKZnodeOwner,
		AuthInfo:       cfg.ZKAuthInfo,
		Address:        cfg.ZKAddress,
		SessionTimeout: cfg.ZKSessionTimeout,
		ConnectTimeout: cfg.ZKConnectionTimeout,
	})
}

func (c *Client) Close() {
	c.conn.Close()
}

func (c *Client) ClientState() ClientState {
	return c.clientState
}

func (c *Client) Exists(path string) (bool, error) {
	found, _, err := c.conn.Exists(path)
	return found, err
}

func (c *Client) ExistsW(path string) (bool, <-chan zk.Event, error) {
	found, _, watch, err := c.conn.ExistsW(path)
	return found, watch, err
}

func (c *Client) Get(path string) ([]byte, error) {
	data, _, err := c.conn.Get(path)
	return data, err
}

func (c *Client) Create(path string, data []byte, perms []int32) error {
	return c.create(path, data, perms)
}

func (c *Client) Set(path string, data []byte) error {
	_, err := c.conn.Set(path, data, zkNoVersion)
	return err
}

// Delete removes a node at the path provided
func (c *Client) Delete(path string) error {
	return c.conn.Delete(path, zkNoVersion)
}

// RegisterListener adds the specified listener and also sets the current state
func (c *Client) RegisterListener(listener StateListener) {
	c.Lock()
	defer c.Unlock()
	c.listeners = append(c.listeners, listener)
	// always send the client state the first time
	listener(c.clientState)
}

// private functions

func connect(config zkConfig) (*Client, error) {
	basePath, err := parseBasePath(config.BasePath)
	if err != nil {
		return nil, errors.Wrapf(err, "could not parse base path '%s'", basePath)
	}
	znodeOwner, err := parseSchemaOwner(config.ZnodeOwner, defaultZnodeOwner)
	if err != nil {
		return nil, errors.Wrapf(err, "could not parse ZK owner '%s'", config.ZnodeOwner)
	}
	authInfo, err := parseSchemaOwner(config.AuthInfo, nil)
	if err != nil {
		return nil, errors.Wrapf(err, "could not parse ZK auth '%s'", config.AuthInfo)
	}
	sessionEstablished := make(chan struct{})
	client := &Client{
		basePath:   basePath,
		nodeOwner:  znodeOwner.owner,
		nodeSchema: znodeOwner.schema,
		zkState:    zk.StateUnknown,
		acl: []zk.ACL{
			{
				ID:     znodeOwner.owner,
				Scheme: znodeOwner.schema,
				Perms:  zk.PermAll,
			},
		},
	}
	client.conn, _, err = zk.Connect([]string{config.Address},
		config.SessionTimeout,
		zk.WithEventCallback(client.eventCallback(sessionEstablished)))
	if err != nil {
		return nil, errors.Wrapf(err, "could not connect to ZK at '%s'", config.Address)
	}
	err = func() error {
		if authInfo != nil {
			if err := client.conn.AddAuth(authInfo.schema, []byte(authInfo.owner)); err != nil {
				return errors.Wrapf(err, "could not authenticate to ZK using '%s'", config.AuthInfo)
			}
		}
		// wait for the session to be established
		select {
		case <-sessionEstablished:
			fmt.Println("Initial ZK session established")
		case <-time.After(config.ConnectTimeout):
			return errCouldNotEstablishConnection
		}

		if err := client.initialize(); err != nil {
			return errors.Wrap(err, "could not initialize ZK client")
		}
		return nil
	}()

	if err != nil {
		fmt.Printf("Shutting down ZK connection due to failure to initialize\n")
		client.conn.Close()
		return nil, err
	}
	return client, nil
}

func parseBasePath(path string) (string, error) {
	if path == "" {
		return "", errors.New("zk base path must not be blank")
	}
	if !strings.HasSuffix(path, "/") {
		path += "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return path, nil
}

func parseSchemaOwner(input string, ifEmpty *schemaOwner) (*schemaOwner, error) {
	so := &schemaOwner{}
	if input == "" {
		return ifEmpty, nil
	}
	parsed := strings.SplitN(input, ":", 2)
	if len(parsed) != 2 {
		return nil, errSchemaOwnerFormat
	}
	so.schema, so.owner = parsed[0], parsed[1]
	if so.schema == "" || so.owner == "" {
		return nil, errSchemaOwnerFormat
	}
	return so, nil
}

func (c *Client) eventCallback(sessionEstablished chan struct{}) zk.EventCallback {
	once := sync.Once{}
	return func(e zk.Event) {
		c.Lock()
		defer c.Unlock()
		c.zkState = e.State
		// signal that the ZK client has connected and has a session for the first time.
		switch e.State {
		case zk.StateHasSession:
			once.Do(func() {
				close(sessionEstablished)
			})
			c.clientState = Connected
		case zk.StateDisconnected:
			c.clientState = Disconnected
		}
		for _, listener := range c.listeners {
			listener(c.clientState)
		}
		if e.Err != nil {
			fmt.Printf("ZK %s %s %s %s: %s\n",
				e.Type, e.State, e.Path, e.Server, e.Err)
		} else {
			fmt.Printf("ZK %s %s %s %s\n",
				e.Type, e.State, e.Path, e.Server)
		}
	}
}

func (c *Client) initialize() error {
	if err := c.createParents(c.basePath, nil, []int32{zk.PermAll}); err != nil {
		return errors.Wrapf(err, "could not create parent for base path '%s'", c.basePath)
	}
	return nil
}

func (c *Client) create(path string, value []byte, perms []int32) error {
	acls := []zk.ACL{}
	for _, perm := range perms {
		acl := zk.ACL{
			Perms:  perm,
			Scheme: c.nodeSchema,
			ID:     c.nodeOwner,
		}
		acls = append(acls, acl)
	}
	if _, err := c.conn.Create(path, value, zkNoFlags, acls); err != nil {
		return err
	}
	return nil
}

func (c *Client) createParents(path string, value []byte, perms []int32) error {
	nodes := strings.Split(path, "/")
	fullPath := ""
	for i, node := range nodes {
		if strings.TrimSpace(node) == "" {
			continue
		}
		fullPath += "/" + node
		isLast := i == (len(nodes) - 1)
		exists, _, err := c.conn.Exists(fullPath)
		if err != nil {
			return errors.Wrapf(err, "could not check if path '%s' exists", fullPath)
		}
		var znodeValue []byte
		if isLast {
			// only set a non-nil value on the last node
			znodeValue = value
		}
		if !exists {
			if err := c.create(fullPath, znodeValue, perms); err != nil {
				return errors.Wrapf(err, "could not create path '%s'", fullPath)
			}
			continue
		}
		if isLast {
			if _, err := c.conn.Set(fullPath, znodeValue, zkNoVersion); err != nil {
				return errors.Wrapf(err, "could not set value on '%s'", fullPath)
			}
			continue
		}
	}
	return nil
}
