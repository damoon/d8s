package wedding

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (s Service) inspect(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	image := fmt.Sprintf("wedding-registry:5000/images/%s", escapePort(vars["name"]))
	buildScript := fmt.Sprintf(`
set -euo pipefail
mkdir inspect-image
skopeo copy --quiet --retry-times 3 --src-tls-verify=false --dest-tls-verify=false docker://%s dir://inspect-image
skopeo inspect dir://inspect-image
`, image)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "wedding-inspect-",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "skopeo",
					Image: skopeoImage,
					Command: []string{
						"timeout",
						strconv.Itoa(int(MaxExecutionTime / time.Second)),
					},
					Args: []string{
						"sh",
						"-c",
						buildScript,
					},
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse(skopeoCPU),
							corev1.ResourceMemory: resource.MustParse(skopeoMemory),
						},
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse(skopeoCPU),
							corev1.ResourceMemory: resource.MustParse(skopeoMemory),
						},
					},
				},
			},
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}

	o := &bytes.Buffer{}
	err := s.executePod(r.Context(), pod, o)
	if err != nil {
		log.Printf("execute inspect: %v", err)
		w.WriteHeader(http.StatusNotFound)
	}

	_, err = io.Copy(w, o)
	if err != nil {
		log.Printf("write inspect result: %v", err)
	}
}
