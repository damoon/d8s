package wedding

import (
	"net/http"
)

func unsupported(res http.ResponseWriter, req *http.Request) {
	res.WriteHeader(http.StatusNotImplemented)
}
