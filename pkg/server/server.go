package server

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
)

var binaryVersion string
var binaryBuildDate string

func NewRouter(version, buildDate string) *mux.Router {
	binaryVersion = version
	binaryBuildDate = buildDate

	r := mux.NewRouter()
	r.HandleFunc("/healthz", HealthzHandler)
	return r
}

// HealthzHandler always returns Ok for now
func HealthzHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Ok\n%s\n%s\n", binaryVersion, binaryBuildDate)
}
