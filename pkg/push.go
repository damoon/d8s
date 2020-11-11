package wedding

import (
	"log"
	"net/http"

	"github.com/gorilla/mux"
)

func pushImage(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	args := r.URL.Query()
	log.Printf("Not implemented yet: push %s:%s", vars["name"], args.Get("tag"))

	w.WriteHeader(http.StatusOK)
}
