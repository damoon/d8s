package wedding

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "wedding-docker-config-",
		},
		StringData: map[string]string{
			"config.json": dockerCfg.mustToJSON(),
		},
	}

	secretClient := s.kubernetesClient.CoreV1().Secrets(s.namespace)

	secret, err = secretClient.Create(r.Context(), secret, metav1.CreateOptions{})
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		streamf(w, "Secret creation failed: %v\n", err)
		log.Printf("create secret: %v", err)
		return
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		err = secretClient.Delete(ctx, secret.Name, metav1.DeleteOptions{})
		if err != nil {
			streamf(w, "Secret deletetion failed: %v\n", err)
			log.Printf("delete secret: %v", err)
		}
	}()

	// TODO add timeout for script
	buildScript := fmt.Sprintf(`
set -euxo pipefail

skopeo copy --src-tls-verify=false --dest-tls-verify=false docker://%s docker://%s
`, from, to)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "wedding-push-",
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
					VolumeMounts: []corev1.VolumeMount{
						{
							MountPath: "/root/.docker",
							Name:      "docker-config",
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "docker-config",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: secret.Name,
						},
					},
				},
			},
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}

	o := &output{w: w}
	err = s.executePod(r.Context(), pod, o)
	if err != nil {
		log.Printf("execute push: %v", err)
		o.Errorf("execute push: %v", err)
	}
}
