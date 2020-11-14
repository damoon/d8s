package wedding

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type output struct {
	w io.Writer
}

func (o output) Write(b []byte) (int, error) {
	i := len(b)

	b, err := json.Marshal(string(b))
	if err != nil {
		return 0, err
	}

	_, err = o.w.Write([]byte(fmt.Sprintf(`{"stream": %s}`, b)))
	if err != nil {
		return 0, err
	}

	if f, ok := o.w.(http.Flusher); ok {
		f.Flush()
	} else {
		return 0, fmt.Errorf("stream can not be flushed")
	}

	return i, nil
}

func (o output) Errorf(e string, args ...interface{}) error {
	return o.Error(fmt.Sprintf(e, args...))
}

func (o output) Error(e string) error {
	b, err := json.Marshal(string(e))
	if err != nil {
		return err
	}

	_, err = o.w.Write([]byte(fmt.Sprintf(`{"errorDetail": {"code": %d, "message": %s}, "error": %s}`, 1, b, b)))
	if err != nil {
		return err
	}

	if f, ok := o.w.(http.Flusher); ok {
		f.Flush()
	} else {
		return fmt.Errorf("stream can not be flushed")
	}

	return nil
}

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
