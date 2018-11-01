package main

import (
	"fmt"
	"net"
	"net/http"
	"os"

	"github.com/coreos/go-systemd/activation"
	"github.com/dcos/dcos-ui-update-service/client"
	"github.com/dcos/dcos-ui-update-service/config"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

type UIService struct {
	Config *config.Config

	UIHandler *UIFileHandler

	UpdateManager *UpdateManager

	Client *client.HTTP
}

// SetupUIHandler create UIFileHandler for service ui and set default directory to
// the current downloaded version or the default document root
func SetupUIHandler(cfg *config.Config, um *UpdateManager) *UIFileHandler {
	documentRoot := cfg.DefaultDocRoot
	currentVersionPath, err := um.GetPathToCurrentVersion()
	if err == nil {
		documentRoot = currentVersionPath
	}
	return NewUIFileHandler(cfg.StaticAssetPrefix, documentRoot)
}

func setup() *UIService {
	cfg := config.Parse()
	httpClient, err := client.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not build http client: %s", err.Error())
		os.Exit(1)
	}
	updateManager := NewUpdateManager(cfg, httpClient)
	uiHandler := SetupUIHandler(cfg, updateManager)

	return &UIService{
		Config:        cfg,
		UpdateManager: updateManager,
		UIHandler:     uiHandler,
		Client:        httpClient,
	}
}

// TODO: think about client timeouts https://blog.cloudflare.com/the-complete-guide-to-golang-net-http-timeouts/
func main() {
	state := setup()

	// Use systemd socket activation.
	l, err := activation.Listeners()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to activate listeners from systemd, will use configured protocol and address instead, %s", err.Error())
		os.Exit(1)
	}

	if len(l) == 1 {
		// Run application
		if err := Run(state, l[0]); err != nil {
			fmt.Fprintf(os.Stderr, "Application error: %s", err.Error())
			os.Exit(1)
		}
		return
	}

	// Start socket
	if err := StartSocket(state); err != nil {
		fmt.Fprintf(os.Stderr, "Application error: %s", err.Error())
		os.Exit(1)
	}
}

// Run serves the API based on the Config and Listener provided
func Run(state *UIService, l net.Listener) error {
	r := newRouter(state)
	http.Handle("/", r)
	loggedRouter := handlers.LoggingHandler(os.Stdout, r)
	return http.Serve(l, loggedRouter)
}

// StartSocket if systemd did not provide a socket
func StartSocket(state *UIService) error {
	listenNet := state.Config.ListenNetProtocol
	listenAddr := state.Config.ListenNetAddress

	l, err := net.Listen(listenNet, listenAddr)
	fmt.Fprintf(os.Stderr, "Starting new socket using net: %q and Addr: %q\n", listenNet, listenAddr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot listen for %q connections at address %q: %s \n", listenNet, listenAddr, err.Error())
		os.Exit(1)
	}
	return Run(state, l)
}

func newRouter(state *UIService) *mux.Router {
	assetPrefix := state.UIHandler.AssetPrefix()

	r := mux.NewRouter()
	r.HandleFunc("/api/v1/", NotImplementedHandler)
	r.HandleFunc("/api/v1/update/{version}/", UpdateHandler(state))
	r.HandleFunc("/api/v1/reset/", ResetToDefaultUIHandler(state)).Methods("DELETE")
	r.PathPrefix(assetPrefix).Handler(state.UIHandler)

	return r
}

// NotImplementedHandler writes a HTTP Not Implemented response
func NotImplementedHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
}

// UpdateHandler processes update requests
func UpdateHandler(state *UIService) func(http.ResponseWriter, *http.Request) {
	dcos := Dcos{
		MasterCountLocation: state.Config.MasterCountFile,
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
		clientAuth := r.Header.Get("Authorization")
		if clientAuth != "" {
			state.Client.SetClientAuth(clientAuth)
			defer state.Client.ClearClientAuth()
		}

		err := state.UpdateManager.UpdateToVersion(version, state.UIHandler)

		if err != nil {
			// This returns locked on every error, it would be better if we would return a boolean if the process is locked
			w.WriteHeader(http.StatusLocked)
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}

// ResetToDefaultUIHandler processes requests to reset to the default ui
func ResetToDefaultUIHandler(state *UIService) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		// verify we aren't currently serving pre-bundled version
		if state.Config.DefaultDocRoot == state.UIHandler.DocumentRoot() {
			w.WriteHeader(http.StatusOK)
			return
		}
		err := state.UIHandler.UpdateDocumentRoot(state.Config.DefaultDocRoot)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		err = state.UpdateManager.ResetVersion()
		if err != nil {
			// TODO: Log we failed to remove latest
		}
		w.WriteHeader(http.StatusOK)
	}
}
