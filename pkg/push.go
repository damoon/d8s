package wedding

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/mux"
)

func (s Service) pushImage(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	args := r.URL.Query()

	from := fmt.Sprintf("wedding-registry:5000/images/%s:%s", escapePort(vars["name"]), args.Get("tag"))
	to := fmt.Sprintf("%s:%s", vars["name"], args.Get("tag"))

	dockerCfg, err := xRegistryAuth(r.Header.Get("X-Registry-Auth")).toDockerConfig()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf("extract registry config: %v", err)))
		log.Printf("extract registry config: %v", err)
		return
	}

	// TODO only use --dest-tls-verify=false for local registry
	script := fmt.Sprintf(`skopeo copy --retry-times 3 --src-tls-verify=false --dest-tls-verify=false docker://%s docker://%s`, from, to)

	o := &output{w: w}
	err = s.scheduleInKubernetes(r.Context(), o, "push", script, dockerCfg.mustToJSON())
	if err != nil {
		log.Printf("execute push: %v", err)
		o.Errorf("execute push: %v", err)
	}
}
