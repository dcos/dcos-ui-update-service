package main

import (
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"

	"github.com/coreos/go-systemd/activation"
	"github.com/gorilla/mux"
)

// Config holds the configuration vaules needed for the Application
type Config struct {
	ListenNetProtocol string

	ListenNetAddress string

	UniverseURL string

	// The filesystem path where the cluster pre-bundled UI is stored
	ClusterUIPath string

	// The filesystem path where downloaded versions are stored
	VersionsRoot string

	// The filesystem path where the file determining the master count is
	MasterCountFile string

	APIToken string

	UIHandler *UIFileHandler

	UpdateManager *UpdateManager
}

// NewConfig returns an instance of Config to be used by the Application
func NewConfig(listenNet, listenAddress, universeURL, clusterUIPath, versionsRoot, masterCountFile string) Config {

	// Don't use keyed literals so we get errors at compile time when new
	// config fields get added.
	return Config{
		listenNet,
		listenAddress,
		universeURL,
		clusterUIPath,
		versionsRoot,
		masterCountFile,
		"",
		nil,
		nil,
	}
}

// Default values for config files
const (
	defaultListenNet       = "unix"
	defaultListenAddr      = "/run/dcos/dcos-ui-update-service.sock"
	defaultAssetPrefix     = "/static/"
	defaultUniverseURL     = "https://leader.mesos"
	defaultClusterUIPath   = "/opt/mesosphere/active/dcos-ui/usr"
	defaultVersionsRoot    = "./versions"
	defaultMasterCountFile = "/opt/mesosphere/etc/master_count"
)

// NewDefaultConfig creates a Config from default values
func NewDefaultConfig() Config {
	return NewConfig(
		defaultListenNet,
		defaultListenAddr,
		defaultUniverseURL,
		defaultClusterUIPath,
		defaultVersionsRoot,
		defaultMasterCountFile,
	)
}

func LoadUpdateManager(cfg *Config) {
	updateManager := NewUpdateManager((*cfg).UniverseURL, (*cfg).VersionsRoot, (*cfg).APIToken)
	(*cfg).UpdateManager = &updateManager
}

func LoadUIHandler(assetPrefix string, cfg *Config) {
	documentRoot := (*cfg).ClusterUIPath
	currentVersionPath, err := (*(*cfg).UpdateManager).GetPathToCurrentVersion()
	if err == nil {
		documentRoot = currentVersionPath
	}
	uiHandler := NewUIFileHandler(assetPrefix, documentRoot)
	(*cfg).UIHandler = &uiHandler
}

func loadConfig() *Config {
	cfg := NewDefaultConfig()
	assetPrefix := defaultAssetPrefix
	flag.StringVar(
		&cfg.ListenNetProtocol,
		"listen-net",
		cfg.ListenNetProtocol,
		"The transport type on which to listen for connections. May be one of 'tcp', 'unix'.",
	)
	flag.StringVar(&cfg.ListenNetAddress, "listen-addr", cfg.ListenNetAddress, "The network address at which to listen for connections.")
	flag.StringVar(&assetPrefix, "asset-prefix", assetPrefix, "The URL path at which to host static assets.")
	flag.StringVar(&cfg.UniverseURL, "universe-url", cfg.UniverseURL, "The URL where universe can be reached")
	flag.StringVar(&cfg.ClusterUIPath, "default-ui-path", cfg.ClusterUIPath, "The filesystem path to serve the default UI from (pre-bundled).")
	flag.StringVar(&cfg.VersionsRoot, "versions-root", cfg.VersionsRoot, "The filesystem path where downloaded versions are stored")
	flag.StringVar(&cfg.MasterCountFile, "master-count-file", cfg.MasterCountFile, "The filesystem path to the file determining the master count")
	flag.StringVar(
		&cfg.APIToken,
		"api-token",
		cfg.APIToken,
		"DC/OS API token to use for authentication, this should only be needed for local development.",
	)
	flag.Parse()

	LoadUpdateManager(&cfg)
	LoadUIHandler(assetPrefix, &cfg)

	return &cfg
}

// TODO: think about client timeouts https://blog.cloudflare.com/the-complete-guide-to-golang-net-http-timeouts/
func main() {
	cfg := loadConfig()
	// Use systemd socket activation.
	l, err := activation.Listeners()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot listen for %q connections at address %q: %s", (*cfg).ListenNetProtocol, (*cfg).ListenNetAddress, err.Error())
		os.Exit(1)
	}

	if len(l) == 1 {
		// Run application
		if err := Run(cfg, l[0]); err != nil {
			fmt.Fprintf(os.Stderr, "Application error: %s", err.Error())
			os.Exit(1)
		}
		return
	}

	// Start socket
	if err := StartSocket(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Application error: %s", err.Error())
		os.Exit(1)
	}
}

// StartSocket if systemd did not provide a socket
func StartSocket(cfg *Config) error {
	listenNet := cfg.ListenNetProtocol
	listenAddr := cfg.ListenNetAddress

	l, err := net.Listen(listenNet, listenAddr)
	fmt.Fprintf(os.Stderr, "Starting new socket using net: %q and Addr: %q\n", listenNet, listenAddr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot listen for %q connections at address %q: %s \n", listenNet, listenAddr, err.Error())
		os.Exit(1)
	}
	return Run(cfg, l)
}

// Run the server and listen to provided address
func Run(cfg *Config, l net.Listener) error {
	r := newRouter(cfg)
	http.Handle("/", r)
	return http.Serve(l, r)
}

func newRouter(cfg *Config) *mux.Router {
	r := mux.NewRouter()
	r.HandleFunc("/api/v1/", NotImplementedHandler)
	r.HandleFunc("/api/v1/update/{version}", UpdateHandler(cfg))
	r.PathPrefix((*(*cfg).UIHandler).GetAssetPrefix()).Handler((*cfg).UIHandler)
	return r
}

// NotImplementedHandler writes a HTTP Not Implemented response
func NotImplementedHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
}

// UpdateHandler processes update requests
func UpdateHandler(cfg *Config) func(http.ResponseWriter, *http.Request) {
	dcos := Dcos{
		MasterCountLocation: (*cfg).MasterCountFile,
	}

	isMultiMaster, err := dcos.IsMultiMaster()
	if err != nil {
		fmt.Printf("Error checking for multi master setup: %#v", err)
		return func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}
	}

	// Unsupported multi master setup
	if isMultiMaster {
		return NotImplementedHandler
	}

	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		version := vars["version"]

		// Check for empty version
		if len(version) == 0 {
			w.WriteHeader(http.StatusNotAcceptable)
			return
		}
		err := (*(*cfg).UpdateManager).UpdateToVersion(version, (*cfg).UIHandler)

		if err != nil {
			// This returns locked on every error, it would be better if we would return a boolean if the process is locked
			w.WriteHeader(http.StatusLocked)
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}
