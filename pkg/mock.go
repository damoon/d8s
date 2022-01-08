package d8s

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

	msg := fmt.Sprintf(`{"stream": %s}`, b)

	_, err = o.w.Write([]byte(msg))
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

	msg := fmt.Sprintf(`{"error": %s, "errorDetail": {"code": %d, "message": %s}}`, b, 1, b)

	_, err = o.w.Write([]byte(msg))
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
