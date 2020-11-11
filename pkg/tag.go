package wedding

import (
	"log"
	"net/http"

	"github.com/gorilla/mux"
)

func tagImage(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	args := r.URL.Query()
	log.Printf("Not implemented yet: tag %s as %s:%s", vars["name"], args.Get("repo"), args.Get("tag"))

	w.WriteHeader(http.StatusCreated)
}
