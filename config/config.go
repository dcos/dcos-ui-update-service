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
}

// Default values for config files
const (
	defaultHTTPClientTimeout = 5 * time.Second
	defaultListenNet         = "unix"
	defaultListenAddr        = "/run/dcos/dcos-ui-update-service.sock"
	defaultAssetPrefix       = "/static/"
	defaultUniverseURL       = "http://127.0.0.1:7070"
	defaultDefaultDocRoot    = "/opt/mesosphere/active/dcos-ui/usr"
	defaultVersionsRoot      = "./versions"
	defaultMasterCountFile   = "/opt/mesosphere/etc/master_count"
)

const (
	optAssetPrefix       = "asset-prefix"
	optDefaultDocRoot    = "default-ui-path"
	optHTTPClientTimeout = "http-client-timeout"
	optListenNet         = "listen-net"
	optListenAddress     = "listen-addr"
	optMasterCountFile   = "master-count-file"
	optUniverseURL       = "universe-url"
	optVersionsRoot      = "versions-root"
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
	cliArgs.DurationVar(&cfg.HTTPClientTimeout, optHTTPClientTimeout, cfg.HTTPClientTimeout, "The default http client timeout for requests.")
	cliArgs.Parse(args)

	return cfg
}
