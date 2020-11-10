package wedding

import (
	"net/http"
)

func ping(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Api-Version", apiVersion)
	res.Header().Set("Docker-Experimental", "false")
	res.WriteHeader(http.StatusOK)
}
