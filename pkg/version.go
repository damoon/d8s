package wedding

import (
	"fmt"
	"net/http"
)

func version(res http.ResponseWriter, req *http.Request) {
	res.Write([]byte(fmt.Sprintf(`{
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
