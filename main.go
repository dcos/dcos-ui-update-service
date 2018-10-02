package main

import (
	"net/http"

	"github.com/gorilla/mux"
)

func Greeting() string {
	return "hello World"
}

func main() {

	r := Router()

	http.Handle("/", r)
	http.ListenAndServe(":80", nil)
}

func Router() *mux.Router {
	r := mux.NewRouter()
	r.HandleFunc("/api/v1/", ApiHandler)

	assetPrefix := "/static/"

	r.PathPrefix(assetPrefix).Handler(StaticHandler(assetPrefix))

	return r
}

func StaticHandler(path string) http.Handler {
	return http.StripPrefix(path, http.FileServer(http.Dir("./public/")))
}

func ApiHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
}
