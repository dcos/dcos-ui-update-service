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

type Config struct {
	// The URL path at which to host static assets.
	AssetPrefix string
	// The filesystem path from which static assets should be served.
	DocumentRoot string

	UniverseUrl string

	// The filesystem path where downloaded versions are stored
	VersionsRoot string

	// The filesystem path where the file determining the master count is
	MasterCountFile string
}

func NewConfig(assetPrefix, documentRoot, universeUrl, versionsRoot, masterCountFile string) Config {
	// Don't use keyed literals so we get errors at compile time when new
	// config fields get added.
	return Config{
		assetPrefix,
		documentRoot,
		universeUrl,
		versionsRoot,
		masterCountFile,
	}
}

// Default values for config files
const (
	defaultAssetPrefix     = "/static/"
	defaultDocumentRoot    = "./public"
	defaultUniverseUrl     = "https://leader.mesos"
	defaultVersionsRoot    = "./versions"
	defaultMasterCountFile = "https://leader.mesos"
)

func NewDefaultConfig() Config {
	return NewConfig(
		defaultAssetPrefix,
		defaultDocumentRoot,
		defaultUniverseUrl,
		defaultVersionsRoot,
		defaultMasterCountFile,
	)
}

// TODO: think about client timeouts https://blog.cloudflare.com/the-complete-guide-to-golang-net-http-timeouts/
func main() {
	// Parse flags that are used in main only.
	const (
		defaultListenNet  = "unix"
		defaultListenAddr = "/run/dcos/dcos-ui-update-service.sock"
	)
	listenNet := defaultListenNet
	flag.StringVar(&listenNet, "listen-net", listenNet, "The transport type on which to listen for connections. May be one of 'tcp', 'unix'.")
	listenAddr := defaultListenAddr
	flag.StringVar(&listenAddr, "listen-addr", listenAddr, "The network address at which to listen for connections.")
	// Parse flags into config.
	cfg := NewDefaultConfig()
	flag.StringVar(&cfg.AssetPrefix, "asset-prefix", cfg.AssetPrefix, "The URL path at which to host static assets.")
	flag.StringVar(&cfg.DocumentRoot, "document-root", cfg.DocumentRoot, "The filesystem path from which static assets should be served.")
	flag.StringVar(&cfg.UniverseUrl, "universe-url", cfg.UniverseUrl, "The URL where universe can be reached")
	flag.StringVar(&cfg.VersionsRoot, "versions-root", cfg.VersionsRoot, "The filesystem path where downloaded versions are stored")
	flag.StringVar(&cfg.MasterCountFile, "master-count-file", cfg.MasterCountFile, "The filesystem path to the file determening the master count")
	flag.Parse()
	// Use systemd socket activation.
	l, err := activation.Listeners()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot listen for %q connections at address %q: %s", listenNet, listenAddr, err.Error())
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
	if err := StartSocket(cfg, listenNet, listenAddr); err != nil {
		fmt.Fprintf(os.Stderr, "Application error: %s", err.Error())
		os.Exit(1)
	}
}

// StartSocket if systemd did not provide a socket
func StartSocket(cfg Config, listenNet string, listenAddr string) error {
	l, err := net.Listen(listenNet, listenAddr)
	fmt.Fprintf(os.Stderr, "Starting new socket using net: %q and Addr: %q\n", listenNet, listenAddr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot listen for %q connections at address %q: %s \n", listenNet, listenAddr, err.Error())
		os.Exit(1)
	}
	return Run(cfg, l)
}

// Run the server and listen to provided address
func Run(cfg Config, l net.Listener) error {
	r := newRouter(cfg)
	http.Handle("/", r)
	return http.Serve(l, r)
}

func newRouter(cfg Config) *mux.Router {
	r := mux.NewRouter()
	r.HandleFunc("/api/v1/", NotImplementedHandler)
	r.HandleFunc("/api/v1/update/{version}", UpdateHandler(cfg))
	r.PathPrefix(cfg.AssetPrefix).Handler(StaticHandler(cfg.AssetPrefix, cfg.DocumentRoot))
	return r
}

// StaticHandler handles requests for static files
func StaticHandler(urlpath, fspath string) http.Handler {
	return http.StripPrefix(urlpath, http.FileServer(http.Dir(fspath)))
}

func NotImplementedHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
}

func UpdateHandler(cfg Config) func(http.ResponseWriter, *http.Request) {
	dcos := Dcos{
		MasterCountLocation: cfg.MasterCountFile,
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

	updateManager := NewUpdateManager(cfg.UniverseUrl, cfg.VersionsRoot)

	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		version := vars["version"]

		// Check for empty version
		if len(version) == 0 {
			w.WriteHeader(http.StatusNotAcceptable)
			return
		}

		err := updateManager.UpdateToVersion(version)

		if err != nil {
			// This returns locked on every error, it would be better if we would return a boolean if the process is locked
			w.WriteHeader(http.StatusLocked)
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}
