package wedding

import (
	"net/http"
)

func containersPrune(res http.ResponseWriter, req *http.Request) {
	res.Write([]byte(`{
  "ContainersDeleted": [],
  "SpaceReclaimed": 0
}
`))
}
