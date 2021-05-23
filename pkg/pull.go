package wedding

import (
	"fmt"
	"log"
	"net/http"
)

func (s Service) pullImage(w http.ResponseWriter, r *http.Request) {
	args := r.URL.Query()

	fromImage := args.Get("fromImage")
	if fromImage == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("image to pull is missing"))
		return
	}

	pullTag := args.Get("tag")
	if pullTag == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("tag to pull is missing"))
		return
	}

	if args.Get("repo") != "" {
		w.WriteHeader(http.StatusNotImplemented)
		w.Write([]byte("repo is not supported"))
		return
	}

	if args.Get("fromSrc") != "" {
		w.WriteHeader(http.StatusNotImplemented)
		w.Write([]byte("import from a file is not supported"))
		return
	}

	if args.Get("message") != "" {
		w.WriteHeader(http.StatusNotImplemented)
		w.Write([]byte("message is not supported"))
		return
	}

	if args.Get("platform") != "" {
		w.WriteHeader(http.StatusNotImplemented)
		w.Write([]byte("platform is not supported"))
		return
	}

	from := fmt.Sprintf("%s:%s", fromImage, pullTag)
	to := fmt.Sprintf("wedding-registry:5000/images/%s", escapePort(from))

	dockerCfg, err := xRegistryAuth(r.Header.Get("X-Registry-Auth")).toDockerConfig()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf("extract registry config: %v", err)))
		log.Printf("extract registry config: %v", err)
		return
	}

	script := fmt.Sprintf(`skopeo copy --retry-times 3 --dest-tls-verify=false docker://%s docker://%s`, from, to)

	o := &output{w: w}
	err = s.runSkopeoPod(r.Context(), o, "pull", script, dockerCfg.mustToJSON())
	if err != nil {
		log.Printf("execute pull: %v", err)
		o.Errorf("execute pull: %v", err)
	}
}
