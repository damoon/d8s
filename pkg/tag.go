package wedding

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"

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

	scheduler := scheduleLocal
	// scheduler = s.scheduleInKubernetes

	o := &bytes.Buffer{}
	err := scheduler(r.Context(), o, "tag", script, "")
	if err != nil {
		log.Printf("execute tag: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		io.Copy(w, o)
	}

	w.WriteHeader(http.StatusCreated)
}

func escapePort(in string) string {
	re := regexp.MustCompile(`:([0-9]+/)`)
	escaped := re.ReplaceAll([]byte(in), []byte("_${1}"))
	return string(escaped)
}
