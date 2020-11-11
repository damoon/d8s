package wedding

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (s Service) tagImage(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	args := r.URL.Query()

	from := fmt.Sprintf("wedding-registry:5000/digests@%s", vars["name"])
	to := fmt.Sprintf("wedding-registry:5000/image/%s:%s", args.Get("repo"), args.Get("tag"))

	log.Printf("tag: %s as %s", from, to)

	// TODO add timeout for script
	buildScript := fmt.Sprintf(`
set -euo pipefail

skopeo copy --src-tls-verify=false --dest-tls-verify=false docker://%s docker://%s
`, from, to)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "wedding-tag-",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "skopeo",
					Image: "mrliptontea/skopeo:1.2.0",
					Command: []string{
						"sh",
						"-c",
						buildScript,
					},
				},
			},
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}

	err := s.executePod(r.Context(), pod, os.Stderr)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf("execute tagging: %v", err)))
		log.Printf("execute tagging: %v", err)
		return
	}

	w.WriteHeader(http.StatusCreated)
}
