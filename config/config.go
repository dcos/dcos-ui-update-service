package config

import (
	"flag"
	"time"
)

// Config holds the configuration vaules needed for the Application
type Config struct {
	ListenNetProtocol string

	ListenNetAddress string

	UniverseURL string

	StaticAssetPrefix string

	// The filesystem path where the cluster pre-bundled UI is stored
	ClusterUIPath string

	// The filesystem path where downloaded versions are stored
	VersionsRoot string

	// The filesystem path where the file determining the master count is
	MasterCountFile string

	APIToken string

	// The filesystem path where the cert file is, for making auth requests to admin router
	CACertFile string
}

// Default values for config files
const (
	defaultHTTPClientTimeout = 5 * time.Second
	defaultListenNet         = "unix"
	defaultListenAddr        = "/run/dcos/dcos-ui-update-service.sock"
	defaultAssetPrefix       = "/static/"
	defaultUniverseURL       = "https://leader.mesos"
	defaultClusterUIPath     = "/opt/mesosphere/active/dcos-ui/usr"
	defaultVersionsRoot      = "./versions"
	defaultMasterCountFile   = "/opt/mesosphere/etc/master_count"
	defaultCACertFile        = "/run/dcos/pki/CA/ca-bundle.crt"
)

const (
	optAPIToken          = "api-token"
	optAssetPrefix       = "asset-prefix"
	optCaCert            = "ca-cert"
	optDefaultUIPath     = "default-ui-path"
	optHTTPClientTimeout = "http-client-timeout"
	optListenNet         = "listen-net"
	optListenAddress     = "listen-addr"
	optMasterCountFile   = "master-count-file"
	optUniverseUrl       = "universe-url"
	optVersionsRoot      = "versions-root"
)

func NewConfig(listenNet, listenAddress, universeURL, assetPrefix, clusterUIPath, versionsRoot, masterCountFile, caCertFile string) *Config {
	// Don't use keyed literals so we get errors at compile time when new
	// config fields get added.
	return &Config{
		listenNet,
		listenAddress,
		universeURL,
		assetPrefix,
		clusterUIPath,
		versionsRoot,
		masterCountFile,
		"",
		caCertFile,
	}
}

func NewDefaultConfig() *Config {
	return NewConfig(
		defaultListenNet,
		defaultListenAddr,
		defaultUniverseURL,
		defaultAssetPrefix,
		defaultClusterUIPath,
		defaultVersionsRoot,
		defaultMasterCountFile,
		defaultCACertFile,
	)
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
	flag.StringVar(&cfg.UniverseURL, optUniverseUrl, cfg.UniverseURL, "The URL where universe can be reached")
	flag.StringVar(&cfg.ClusterUIPath, optDefaultUIPath, cfg.ClusterUIPath, "The filesystem path to serve the default UI from (pre-bundled).")
	flag.StringVar(&cfg.VersionsRoot, optVersionsRoot, cfg.VersionsRoot, "The filesystem path where downloaded versions are stored")
	flag.StringVar(&cfg.MasterCountFile, optMasterCountFile, cfg.MasterCountFile, "The filesystem path to the file determining the master count")
	flag.StringVar(
		&cfg.APIToken,
		optAPIToken,
		cfg.APIToken,
		"DC/OS API token to use for authentication, this should only be needed for local development.",
	)
	flag.StringVar(&cfg.CACertFile, optCaCert, cfg.CACertFile, "The filesystem path to the CA Cert file used to make authenticated requests to admin router")
	flag.Parse()

	return cfg
}
