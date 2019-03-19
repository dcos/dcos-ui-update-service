package config

import (
	"fmt"
	"os"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// Config holds the configuration vaules needed for the Application
type Config struct {
	viper *viper.Viper
}

// Default values for config files
const (
	defaultConfig             = ""
	defaultHTTPClientTimeout  = 5 * time.Second
	defaultListenNet          = "unix"
	defaultListenAddr         = "/run/dcos/dcos-ui-update-service.sock"
	defaultUniverseURL        = "http://127.0.0.1:7070"
	defaultDefaultDocRoot     = "/opt/mesosphere/active/dcos-ui/usr"
	defaultUIDistSymlink      = "/opt/mesosphere/active/dcos-ui-dist"
	defaultUIDistStageSymlink = "/opt/mesosphere/active/new-dcos-ui-dist"
	defaultVersionsRoot       = "/opt/mesosphere/active/dcos-ui-service/versions"
	defaultMasterCountFile    = "/opt/mesosphere/etc/master_count"
	defaultLogLevel           = "info"
	defaultZKAddress          = "127.0.0.1:2181"
	defaultZKBasePath         = "/dcos/ui-update"
	defaultZKAuthInfo         = ""
	defaultZKZnodeOwner       = ""
	defaultZKSessionTimeout   = 5 * time.Second
	defaultZKConnectTimeout   = 5 * time.Second
	defaultZKPollingInterval  = 30 * time.Second
	defaultInitUIDistSymlink  = false
)

const (
	optConfig             = "config"
	optDefaultDocRoot     = "default-ui-path"
	optUIDistSymlink      = "ui-dist-symlink"
	optUIDistStageSymlink = "ui-dist-stage-symlink"
	optHTTPClientTimeout  = "http-client-timeout"
	optListenNet          = "listen-net"
	optListenAddress      = "listen-addr"
	optMasterCountFile    = "master-count-file"
	optLogLevel           = "log-level"
	optUniverseURL        = "universe-url"
	optVersionsRoot       = "versions-root"
	optZKAddress          = "zk-addr"
	optZKBasePath         = "zk-base-path"
	optZKAuthInfo         = "zk-auth-info"
	optZKZnodeOwner       = "zk-znode-owner"
	optZKSessionTimeout   = "zk-session-timeout"
	optZKConnectTimeout   = "zk-connect-timeout"
	optZKPollingInterval  = "zk-poll-int"
	optInitUIDistSymlink  = "init-ui-dist-symlink"
)

func defineFlags(viper *viper.Viper) (*pflag.FlagSet, error) {
	fs := &pflag.FlagSet{}
	fs.String(optConfig, defaultConfig, "The path to the optional config file")
	fs.String(optListenNet, defaultListenNet, "The transport type on which to listen for connections. May be one of 'tcp', 'unix'.")
	fs.String(optListenAddress, defaultListenAddr, "The network address at which to listen for connections.")
	fs.String(optUniverseURL, defaultUniverseURL, "The URL where universe can be reached.")
	fs.String(optDefaultDocRoot, defaultDefaultDocRoot, "The filesystem path with the default ui distribution (pre-bundled ui).")
	fs.String(optUIDistSymlink, defaultUIDistSymlink, "The filesystem symlink path where the ui distributed files are served from.")
	fs.String(
		optUIDistStageSymlink,
		defaultUIDistStageSymlink,
		"The temporary filesystem symlink path that links to where the ui distribution files are located.",
	)
	fs.String(optVersionsRoot, defaultVersionsRoot, "The filesystem path where downloaded versions are stored.")
	fs.String(optMasterCountFile, defaultMasterCountFile, "The filesystem path to the file determining the master count.")
	fs.String(optLogLevel, defaultLogLevel, "The output logging level.")
	fs.Duration(optHTTPClientTimeout, defaultHTTPClientTimeout, "The default http client timeout for requests.")
	fs.String(optZKAddress, defaultZKAddress, "The Zookeeper address this client will connect to.")
	fs.String(optZKBasePath, defaultZKBasePath, "The path of the root zookeeper znode.")
	fs.String(optZKAuthInfo, defaultZKAuthInfo, "Authentication details for zookeeper.")
	fs.String(optZKZnodeOwner, defaultZKZnodeOwner, "The ZK owner of the base path.")
	fs.Duration(optZKSessionTimeout, defaultZKSessionTimeout, "ZK session timeout.")
	fs.Duration(optZKConnectTimeout, defaultZKConnectTimeout, "Timeout to establish initial zookeeper connection.")
	fs.Duration(optZKPollingInterval, defaultZKPollingInterval, "Interval to check zookeeper node for version updates.")
	fs.Bool(optInitUIDistSymlink, defaultInitUIDistSymlink, "Initialize the UI dist symlink if missing")

	viper.BindEnv(optListenAddress, "DCOS_UI_UPDATE_LISTEN_ADDR")
	viper.BindEnv(optDefaultDocRoot, "DCOS_UI_UPDATE_DEFAULT_UI_PATH")
	viper.BindEnv(optVersionsRoot, "DCOS_UI_UPDATE_VERSIONS_ROOT")
	viper.BindEnv(optUIDistSymlink, "DCOS_UI_UPDATE_DIST_LINK")
	viper.BindEnv(optUIDistStageSymlink, "DCOS_UI_UPDATE_STAGE_LINK")
	viper.BindEnv(optZKAuthInfo, "DCOS_UI_UPDATE_ZK_AUTH_INFO")
	viper.BindEnv(optZKZnodeOwner, "DCOS_UI_UPDATE_ZK_ZKNODE_OWNER")

	if err := viper.BindPFlags(fs); err != nil {
		return nil, errors.Wrap(err, "Could not bind PFlags")
	}

	return fs, nil
}

// NewDefaultConfig returns a default configuration without runtime config used
func NewDefaultConfig() *Config {
	defaults, err := Parse(nil)
	if err != nil {
		// This should never happen
		panic(err)
	}
	return defaults
}

// Parse parses configuration from CLI arguments, environment variables, or config file
func Parse(args []string) (*Config, error) {
	viper := viper.New()
	fs, err := defineFlags(viper)
	if err != nil {
		return nil, err
	}

	if err := fs.Parse(args); err != nil {
		if err == pflag.ErrHelp {
			os.Exit(0)
		}
		return nil, err
	}

	if path := viper.GetString(optConfig); path != "" {
		viper.SetConfigFile(path)
		if err := viper.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("Could not read config file: %s", err)
		}
	}

	return &Config{viper}, nil
}

// ConfigFilePath is the path of the config file to load config settings from
func (c Config) ConfigFilePath() string {
	return c.viper.GetString(optConfig)
}

// HTTPClientTimeout is the default http client timeout for requests
func (c Config) HTTPClientTimeout() time.Duration {
	return c.viper.GetDuration(optHTTPClientTimeout)
}

// ListenNetProtocol is the transport type on which to listen for connections. May be one of 'tcp', 'unix'
func (c Config) ListenNetProtocol() string {
	return c.viper.GetString(optListenNet)
}

// ListenNetAddress is the network address at which to listen for connections
func (c Config) ListenNetAddress() string {
	return c.viper.GetString(optListenAddress)
}

// UniverseURL is the URL where the universe package repository can be reached
func (c Config) UniverseURL() string {
	return c.viper.GetString(optUniverseURL)
}

// DefaultDocRoot is the filesystem path where the cluster pre-bundled UI is stored
func (c Config) DefaultDocRoot() string {
	return c.viper.GetString(optDefaultDocRoot)
}

// UIDistSymlink is the filesystem symlink used to serve dcos-ui files
func (c Config) UIDistSymlink() string {
	return c.viper.GetString(optUIDistSymlink)
}

// UIDistStageSymlink is the filesystem path to use for a temporary symlink when updating the ui dist symlink
func (c Config) UIDistStageSymlink() string {
	return c.viper.GetString(optUIDistStageSymlink)
}

// VersionsRoot is the filesystem path where downloaded versions are stored
func (c Config) VersionsRoot() string {
	return c.viper.GetString(optVersionsRoot)
}

// MasterCountFile is the filesystem path where the file determining the master count is
func (c Config) MasterCountFile() string {
	return c.viper.GetString(optMasterCountFile)
}

// LogLevel is the minimum logging level to output
func (c Config) LogLevel() string {
	return c.viper.GetString(optLogLevel)
}

// ZKAddress is the host:port to which the zookeeper client will connect
func (c Config) ZKAddress() string {
	return c.viper.GetString(optZKAddress)
}

// ZKBasePath is the path of the base zookeeper znode
func (c Config) ZKBasePath() string {
	return c.viper.GetString(optZKBasePath)
}

// ZKAuthInfo contains the details of how to authenticate to zookeeper
func (c Config) ZKAuthInfo() string {
	return c.viper.GetString(optZKAuthInfo)
}

// ZKZnodeOwner is the ZK owner of the base path
func (c Config) ZKZnodeOwner() string {
	return c.viper.GetString(optZKZnodeOwner)
}

// ZKSessionTimeout is the session timeout to ZK
func (c Config) ZKSessionTimeout() time.Duration {
	return c.viper.GetDuration(optZKSessionTimeout)
}

// ZKConnectionTimeout is the timeout to establish the initial ZK connection
func (c Config) ZKConnectionTimeout() time.Duration {
	return c.viper.GetDuration(optZKConnectTimeout)
}

// ZKPollingInterval is the interval used for polling ZK to detect state changes
func (c Config) ZKPollingInterval() time.Duration {
	return c.viper.GetDuration(optZKPollingInterval)
}

// InitUIDistSymlink is whether the UIDistSymlink should be initialized if it doesn't exist, defaults to false and should only be used for local dev
func (c Config) InitUIDistSymlink() bool {
	return c.viper.GetBool(optInitUIDistSymlink)
}
