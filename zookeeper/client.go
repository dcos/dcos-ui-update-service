package zookeeper

import (
	"strings"
	"sync"
	"time"

	"github.com/dcos/dcos-ui-update-service/config"
	"github.com/pkg/errors"
	"github.com/samuel/go-zookeeper/zk"
	"github.com/sirupsen/logrus"
)

type StateListener func(state ClientState)

type Client struct {
	conn        ZKConnection
	acl         []zk.ACL
	basePath    string
	nodeOwner   string
	nodeSchema  string
	zkState     zk.State
	clientState ClientState
	listeners   map[string]StateListener
	sync.Mutex
}

type ZKClient interface {
	Close()
	ClientState() ClientState
	RegisterListener(id string, listener StateListener)
	UnregisterListener(id string)

	Exists(path string) (bool, int32, error)
	existsW(path string) (bool, int32, <-chan zk.Event, error)
	Get(path string) ([]byte, int32, error)
	getW(path string) ([]byte, int32, <-chan zk.Event, error)
	Create(path string, data []byte, perms []int32) error
	Set(path string, data []byte) (int32, error)
	Children(path string) ([]string, int32, error)
	childrenW(path string) ([]string, int32, <-chan zk.Event, error)
}

type ZKConnection interface {
	AddAuth(scheme string, auth []byte) error
	Children(path string) ([]string, *zk.Stat, error)
	ChildrenW(path string) ([]string, *zk.Stat, <-chan zk.Event, error)
	Close()
	Create(path string, data []byte, flags int32, acl []zk.ACL) (string, error)
	Delete(path string, version int32) error
	Exists(path string) (bool, *zk.Stat, error)
	ExistsW(path string) (bool, *zk.Stat, <-chan zk.Event, error)
	Get(path string) ([]byte, *zk.Stat, error)
	GetW(path string) ([]byte, *zk.Stat, <-chan zk.Event, error)
	Set(path string, data []byte, version int32) (*zk.Stat, error)
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
	log                            = logrus.WithField("package", "zookeeper.client")
)

// exported functions

// Connect creates and initializes a zookeeper client
func Connect(cfg *config.Config) (*Client, error) {
	return connect(zkConfig{
		BasePath:       cfg.ZKBasePath(),
		ZnodeOwner:     cfg.ZKZnodeOwner(),
		AuthInfo:       cfg.ZKAuthInfo(),
		Address:        cfg.ZKAddress(),
		SessionTimeout: cfg.ZKSessionTimeout(),
		ConnectTimeout: cfg.ZKConnectionTimeout(),
	})
}

func (c *Client) Close() {
	c.conn.Close()
}

func (c *Client) ClientState() ClientState {
	return c.clientState
}

func (c *Client) Exists(path string) (bool, int32, error) {
	found, stat, err := c.conn.Exists(path)
	return found, stat.Version, err
}

func (c *Client) existsW(path string) (bool, int32, <-chan zk.Event, error) {
	found, stat, channel, err := c.conn.ExistsW(path)
	return found, stat.Version, channel, err
}

func (c *Client) Get(path string) ([]byte, int32, error) {
	data, stat, err := c.conn.Get(path)
	return data, stat.Version, err
}

func (c *Client) getW(path string) ([]byte, int32, <-chan zk.Event, error) {
	data, stat, channel, err := c.conn.GetW(path)
	return data, stat.Version, channel, err
}

func (c *Client) Create(path string, data []byte, perms []int32) error {
	return c.create(path, data, perms)
}

func (c *Client) Set(path string, data []byte) (int32, error) {
	_, stat, err := c.conn.Get(path)
	if err != nil {
		return zkNoVersion, err
	}
	stat, err = c.conn.Set(path, data, stat.Version)
	return stat.Version, err
}

func (c *Client) Children(path string) ([]string, int32, error) {
	children, stat, err := c.conn.Children(path)
	return children, stat.Cversion, err
}

func (c *Client) childrenW(path string) ([]string, int32, <-chan zk.Event, error) {
	children, stat, channel, err := c.conn.ChildrenW(path)
	return children, stat.Cversion, channel, err
}

// Delete removes a node at the path provided
func (c *Client) Delete(path string) error {
	_, stat, err := c.conn.Get(path)
	if err != nil {
		return err
	}
	return c.conn.Delete(path, stat.Version)
}

// RegisterListener adds the specified listener and also sends the current state to the listener
func (c *Client) RegisterListener(id string, listener StateListener) {
	c.Lock()
	defer c.Unlock()
	if _, ok := c.listeners[id]; ok {
		// Listener with id is already registered, replace listener, but don't call it
		c.listeners[id] = listener
	} else {
		c.listeners[id] = listener
		// always send the client state the first time
		listener(c.clientState)
	}
}

// UnregisterListener removed a registered listener based on its ID
func (c *Client) UnregisterListener(id string) {
	c.Lock()
	defer c.Unlock()
	if _, ok := c.listeners[id]; ok {
		delete(c.listeners, id)
	}
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
		listeners: make(map[string]StateListener),
	}
	client.conn, _, err = zk.Connect([]string{config.Address},
		config.SessionTimeout,
		zk.WithEventCallback(client.eventCallback(sessionEstablished)),
		zk.WithLogger(zookeeperClientLogger()))
	if err != nil {
		return nil, errors.Wrapf(err, "could not connect to ZK at '%s'", config.Address)
	}
	err = func() error {
		if authInfo != nil {
			if addAuthErr := client.conn.AddAuth(authInfo.schema, []byte(authInfo.owner)); addAuthErr != nil {
				return errors.Wrapf(addAuthErr, "could not authenticate to ZK using '%s'", config.AuthInfo)
			}
		}
		// wait for the session to be established
		select {
		case <-sessionEstablished:
			log.Debug("Initial ZK session established")
		case <-time.After(config.ConnectTimeout):
			return errCouldNotEstablishConnection
		}

		if initErr := client.initialize(); initErr != nil {
			return errors.Wrap(initErr, "could not initialize ZK client")
		}
		return nil
	}()

	if err != nil {
		log.Debug("Shutting down ZK connection due to failure to initialize")
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
		stateChange := false
		c.zkState = e.State
		// signal that the ZK client has connected and has a session for the first time.
		switch e.State {
		case zk.StateHasSession:
			once.Do(func() {
				close(sessionEstablished)
			})
			c.clientState = Connected
			stateChange = true
		case zk.StateDisconnected:
			c.clientState = Disconnected
			stateChange = true
		}
		if stateChange {
			for _, listener := range c.listeners {
				go listener(c.clientState)
			}
		}
		if e.Err != nil {
			log.WithError(e.Err).Tracef("ZK event: %s %s %s %s", e.Type, e.State, e.Path, e.Server)
		} else {
			log.Tracef("ZK event: %s %s %s %s", e.Type, e.State, e.Path, e.Server)
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
