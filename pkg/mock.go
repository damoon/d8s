package wedding

import (
	"fmt"
	"net/http"
)

func ping(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Api-Version", apiVersion)
	w.Header().Set("Docker-Experimental", "false")
	w.WriteHeader(http.StatusOK)
}

func version(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(fmt.Sprintf(`{
  "Components": [
    {
      "Name": "Engine",
      "Version": "19.03.8",
      "Details": {
        "ApiVersion": "%s",
        "Experimental": "false",
        "MinAPIVersion": "1.12"
      }
    }
  ],
  "Version": "19.03.8",
  "ApiVersion": "%s",
  "MinAPIVersion": "1.12"
}
`, apiVersion, apiVersion)))
}

func buildPrune(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(`{
  "CachesDeleted": [],
  "SpaceReclaimed": 0
}
`))
}

func containersPrune(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(`{
  "ContainersDeleted": [],
  "SpaceReclaimed": 0
}
`))
}

func imagesJSON(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(`[]`))
}
