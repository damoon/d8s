package wedding

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

func (s Service) tagImage(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	args := r.URL.Query()

	from := fmt.Sprintf("wedding-registry:5000/digests@%s", vars["name"])
	if !strings.HasPrefix(vars["name"], "sha256:") {
		// from = fmt.Sprintf("wedding-registry:5000/images/%s", url.PathEscape(escapePort(vars["name"])))
		from = fmt.Sprintf("wedding-registry:5000/images/%s", escapePort(vars["name"]))
	}

	tag := args.Get("tag")
	if tag == "" {
		tag = "latest"
	}

	to := fmt.Sprintf(
		"wedding-registry:5000/images/%s",
		escapePort(fmt.Sprintf("%s:%s", args.Get("repo"), tag)),
	)

	script := fmt.Sprintf(`skopeo copy --retry-times 3 --src-tls-verify=false --dest-tls-verify=false docker://%s docker://%s`, from, to)

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	scheduler := s.scheduleInKubernetes
	err := semSkopeo.Acquire(ctx, 1)
	if err == nil {
		log.Printf("tag locally %s to %s", tag, to)
		defer semSkopeo.Release(1)
		scheduler = scheduleLocal
	} else {
		log.Printf("tag scheduled %s to %s", tag, to)
	}

	o := &bytes.Buffer{}
	err = scheduler(r.Context(), o, "tag", script+" || "+script, "")
	if err != nil {
		log.Printf("execute tag: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		io.Copy(w, o)
		return
	}

	w.WriteHeader(http.StatusCreated)
	io.Copy(w, o)
}

func escapePort(in string) string {
	re := regexp.MustCompile(`:([0-9]+/)`)
	escaped := re.ReplaceAll([]byte(in), []byte("_${1}"))
	return string(escaped)
}
