package wedding

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"

	"github.com/gorilla/mux"
)

func (s Service) inspect(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	image := fmt.Sprintf("wedding-registry:5000/images/%s", escapePort(vars["name"]))
	randomID := randStringBytes(16)
	script := fmt.Sprintf(`
set -euo pipefail
mkdir %s
skopeo copy --quiet --retry-times 3 --src-tls-verify=false docker://%s dir://%s
skopeo inspect dir://%s
rm -r %s
`, randomID, image, randomID, randomID, randomID)

	o := &bytes.Buffer{}
	err := s.scheduleInKubernetes(r.Context(), o, "inspect", script, "")
	if err != nil {
		log.Printf("execute inspect: %v", err)
		w.WriteHeader(http.StatusNotFound)
		io.Copy(w, o)
		return
	}

	io.Copy(w, o)
}

func randStringBytes(n int) string {
	letterBytes := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}
