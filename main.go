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
}

func NewConfig(assetPrefix, documentRoot string) Config {
	// Don't use keyed literals so we get errors at compile time when new
	// config fields get added.
	return Config{
		assetPrefix,
		documentRoot,
	}
}

// Default values for config files
const (
	defaultAssetPrefix  = "/static/"
	defaultDocumentRoot = "./public"
)

func NewDefaultConfig() Config {
	return NewConfig(
		defaultAssetPrefix,
		defaultDocumentRoot,
	)
}

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
	r := newRouter(cfg.AssetPrefix, cfg.DocumentRoot)
	http.Handle("/", r)
	return http.Serve(l, r)
}

func newRouter(assetPrefix, documentRoot string) *mux.Router {
	r := mux.NewRouter()
	r.HandleFunc("/api/v1/", APIHandler)
	r.PathPrefix(assetPrefix).Handler(StaticHandler(assetPrefix, documentRoot))
	return r
}

// StaticHandler handles requests for static files
func StaticHandler(urlpath, fspath string) http.Handler {
	return http.StripPrefix(urlpath, http.FileServer(http.Dir(fspath)))
}

// APIHandler handles api calls
func APIHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
}
