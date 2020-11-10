package wedding

import (
	"net/http"
)

func buildPrune(res http.ResponseWriter, req *http.Request) {
	res.Write([]byte(`{
  "CachesDeleted": [],
  "SpaceReclaimed": 0
}
`))
}
