package wedding

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

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

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	scheduler := s.scheduleInKubernetes
	err = semSkopeo.Acquire(ctx, 1)
	if err == nil {
		log.Printf("push locally %s", to)
		defer semSkopeo.Release(1)
		scheduler = scheduleLocal
	} else {
		log.Printf("push scheduled %s", to)
	}

	o := &output{w: w}
	err = scheduler(r.Context(), o, "push", script, dockerCfg.mustToJSON())
	if err != nil {
		log.Printf("execute push: %v", err)
		o.Errorf("execute push: %v", err)
	}
}
