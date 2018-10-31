package config

import (
	"flag"
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

	// The filesystem path where the cert file is, for making auth requests to admin router
	CACertFile string

	IAMConfig string
}

// Default values for config files
const (
	defaultHTTPClientTimeout = 5 * time.Second
	defaultListenNet         = "unix"
	defaultListenAddr        = "/run/dcos/dcos-ui-update-service.sock"
	defaultAssetPrefix       = "/static/"
	defaultUniverseURL       = "https://leader.mesos"
	defaultDefaultDocRoot    = "/opt/mesosphere/active/dcos-ui/usr"
	defaultVersionsRoot      = "./versions"
	defaultMasterCountFile   = "/opt/mesosphere/etc/master_count"
)

const (
	optAssetPrefix       = "asset-prefix"
	optCaCert            = "ca-cert"
	optDefaultUIPath     = "default-ui-path"
	optIAMConfig         = "iam-config"
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
		"",
		"",
	}
}

func Parse() *Config {
	cfg := NewDefaultConfig()

	flag.StringVar(
		&cfg.ListenNetProtocol,
		optListenNet,
		cfg.ListenNetProtocol,
		"The transport type on which to listen for connections. May be one of 'tcp', 'unix'.",
	)
	flag.StringVar(&cfg.ListenNetAddress, optListenAddress, cfg.ListenNetAddress, "The network address at which to listen for connections.")
	flag.StringVar(&cfg.StaticAssetPrefix, optAssetPrefix, cfg.StaticAssetPrefix, "The URL path at which to host static assets.")
	flag.StringVar(&cfg.UniverseURL, optUniverseURL, cfg.UniverseURL, "The URL where universe can be reached")
	flag.StringVar(&cfg.DefaultDocRoot, optDefaultUIPath, cfg.DefaultDocRoot, "The filesystem path to serve the default UI from (pre-bundled).")
	flag.StringVar(&cfg.VersionsRoot, optVersionsRoot, cfg.VersionsRoot, "The filesystem path where downloaded versions are stored.")
	flag.StringVar(&cfg.MasterCountFile, optMasterCountFile, cfg.MasterCountFile, "The filesystem path to the file determining the master count.")
	flag.StringVar(&cfg.CACertFile, optCaCert, cfg.CACertFile, "The filesystem path to the certificate authority file.")
	flag.StringVar(&cfg.IAMConfig, optIAMConfig, cfg.IAMConfig, "The filesystem path to identity and access management config.")
	flag.DurationVar(&cfg.HTTPClientTimeout, optHTTPClientTimeout, cfg.HTTPClientTimeout, "The default http client timeout for requests.")
	flag.Parse()

	return cfg
}
