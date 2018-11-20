package config

import (
	"flag"
	"os"
	"time"
)

// Config holds the configuration vaules needed for the Application
type Config struct {
	HTTPClientTimeout time.Duration

	ListenNetProtocol string

	ListenNetAddress string

	UniverseURL string

	StaticAssetPrefix string

	// The filesystem path where the cluster pre-bundled UI is stored
	DefaultDocRoot string

	// The filesystem path where downloaded versions are stored
	VersionsRoot string

	// The filesystem path where the file determining the master count is
	MasterCountFile string

	LogLevel string
	// Zookeeper configuration options
	ZKAddress           string
	ZKBasePath          string
	ZKAuthInfo          string
	ZKZnodeOwner        string
	ZKSessionTimeout    time.Duration
	ZKConnectionTimeout time.Duration
	ZKPollingInterval   time.Duration
}

// Default values for config files
const (
	defaultHTTPClientTimeout = 5 * time.Second
	defaultListenNet         = "unix"
	defaultListenAddr        = "/run/dcos/dcos-ui-update-service.sock"
	defaultAssetPrefix       = "/static/"
	defaultUniverseURL       = "http://127.0.0.1:7070"
	defaultDefaultDocRoot    = "/opt/mesosphere/active/dcos-ui/usr"
	defaultVersionsRoot      = "/opt/mesosphere/active/dcos-ui-service/versions"
	defaultMasterCountFile   = "/opt/mesosphere/etc/master_count"
	defaultLogLevel          = "info"
	defaultZKAddress         = "127.0.0.1:2181"
	defaultZKBasePath        = "/dcos/ui-update"
	defaultZKAuthInfo        = ""
	defaultZKZnodeOwner      = ""
	defaultZKSessionTimeout  = 5 * time.Second
	defaultZKConnectTimeout  = 5 * time.Second
	defaultZKPollingInterval = 30 * time.Second
)

const (
	optAssetPrefix       = "asset-prefix"
	optDefaultDocRoot    = "default-ui-path"
	optHTTPClientTimeout = "http-client-timeout"
	optListenNet         = "listen-net"
	optListenAddress     = "listen-addr"
	optMasterCountFile   = "master-count-file"
	optLogLevel          = "log-level"
	optUniverseURL       = "universe-url"
	optVersionsRoot      = "versions-root"
	optZKAddress         = "zk-addr"
	optZKBasePath        = "zk-base-path"
	optZKAuthInfo        = "zk-auth-info"
	optZKZnodeOwner      = "zk-znode-owner"
	optZKSessionTimeout  = "zk-session-timeout"
	optZKConnectTimeout  = "zk-connect-timeout"
	optZKPollingInterval = "zk-poll-int"
)

func NewDefaultConfig() *Config {
	// Don't use keyed literals so we get errors at compile time when new
	// config fields get added.
	return &Config{
		defaultHTTPClientTimeout,
		defaultListenNet,
		defaultListenAddr,
		defaultUniverseURL,
		defaultAssetPrefix,
		defaultDefaultDocRoot,
		defaultVersionsRoot,
		defaultMasterCountFile,
		defaultLogLevel,
		defaultZKAddress,
		defaultZKBasePath,
		defaultZKAuthInfo,
		defaultZKZnodeOwner,
		defaultZKSessionTimeout,
		defaultZKConnectTimeout,
		defaultZKPollingInterval,
	}
}

func replaceEnvVariables(args []string) []string {
	result := make([]string, len(args))
	for i, arg := range args {
		if arg[0] == '$' {
			result[i] = os.Getenv(arg[1:])
		} else {
			result[i] = arg
		}
	}
	return result
}

func Parse(args []string) *Config {
	cfg := NewDefaultConfig()
	args = replaceEnvVariables(args)

	cliArgs := flag.NewFlagSet("cli-args", flag.ContinueOnError)
	cliArgs.StringVar(
		&cfg.ListenNetProtocol,
		optListenNet,
		cfg.ListenNetProtocol,
		"The transport type on which to listen for connections. May be one of 'tcp', 'unix'.",
	)
	cliArgs.StringVar(&cfg.ListenNetAddress, optListenAddress, cfg.ListenNetAddress, "The network address at which to listen for connections.")
	cliArgs.StringVar(&cfg.StaticAssetPrefix, optAssetPrefix, cfg.StaticAssetPrefix, "The URL path at which to host static assets.")
	cliArgs.StringVar(&cfg.UniverseURL, optUniverseURL, cfg.UniverseURL, "The URL where universe can be reached.")
	cliArgs.StringVar(&cfg.DefaultDocRoot, optDefaultDocRoot, cfg.DefaultDocRoot, "The filesystem path to serve the default UI from (pre-bundled).")
	cliArgs.StringVar(&cfg.VersionsRoot, optVersionsRoot, cfg.VersionsRoot, "The filesystem path where downloaded versions are stored.")
	cliArgs.StringVar(&cfg.MasterCountFile, optMasterCountFile, cfg.MasterCountFile, "The filesystem path to the file determining the master count.")
	cliArgs.StringVar(&cfg.LogLevel, optLogLevel, cfg.LogLevel, "The output logging level.")
	cliArgs.DurationVar(&cfg.HTTPClientTimeout, optHTTPClientTimeout, cfg.HTTPClientTimeout, "The default http client timeout for requests.")
	cliArgs.StringVar(&cfg.ZKAddress, optZKAddress, cfg.ZKAddress, "The Zookeeper address this client will connect to.")
	cliArgs.StringVar(&cfg.ZKBasePath, optZKBasePath, cfg.ZKBasePath, "The path of the root zookeeper znode.")
	cliArgs.StringVar(&cfg.ZKAuthInfo, optZKAuthInfo, cfg.ZKAuthInfo, "Authentication details for zookeeper.")
	cliArgs.StringVar(&cfg.ZKZnodeOwner, optZKZnodeOwner, cfg.ZKZnodeOwner, "The ZK owner of the base path.")
	cliArgs.DurationVar(&cfg.ZKSessionTimeout, optZKSessionTimeout, cfg.ZKSessionTimeout, "ZK session timeout.")
	cliArgs.DurationVar(&cfg.ZKConnectionTimeout, optZKConnectTimeout, cfg.ZKConnectionTimeout, "Timeout to establish initial zookeeper connection.")
	cliArgs.DurationVar(&cfg.ZKPollingInterval, optZKPollingInterval, cfg.ZKPollingInterval, "Interval to check zookeeper node for version updates.")
	cliArgs.Parse(args)

	return cfg
}
