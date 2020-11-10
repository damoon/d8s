package wedding

import (
	"net/http"
)

func imagesJSON(res http.ResponseWriter, req *http.Request) {
	res.Write([]byte(`[]`))
}
