package wedding

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	to := fmt.Sprintf("wedding-registry:5000/images/%s", url.PathEscape(from))

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
set -euo pipefail

skopeo copy --dest-tls-verify=false docker://%s docker://%s
`, from, to)

	stream(w, buildScript)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "wedding-pull-",
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

	err = s.executePod(r.Context(), pod, w)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		stream(w, fmt.Sprintf("execute push: %v", err))
		log.Printf("execute push: %v", err)
		return
	}

	w.WriteHeader(http.StatusOK)
}
