package wedding

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (s Service) runSkopeoPod(ctx context.Context, w io.Writer, processName, script, dockerJSON string) error {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("wedding-%s-", processName),
			Labels: map[string]string{
				"app": "wedding",
				"job": "skopeo",
			},
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
						script,
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

	if dockerJSON != "" {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "wedding-docker-config-",
			},
			StringData: map[string]string{
				"config.json": dockerJSON,
			},
		}

		secretClient := s.kubernetesClient.CoreV1().Secrets(s.namespace)

		secret, err := secretClient.Create(ctx, secret, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("create docker.json secret: %v", err)
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

		pod.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{
			{
				MountPath: "/root/.docker",
				Name:      "docker-config",
			},
		}
		pod.Spec.Volumes = []corev1.Volume{
			{
				Name: "docker-config",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: secret.Name,
					},
				},
			},
		}
	}

	return s.executePod(ctx, pod, w)
}

func (s Service) executePod(ctx context.Context, pod *corev1.Pod, w io.Writer) error {
	podClient := s.kubernetesClient.CoreV1().Pods(s.namespace)

	pod, err := podClient.Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("create pod: %v", err)
	}

	failed := false

	defer func() {
		// helpful for development: remove all failed pods
		// kubectl get po | grep -E 'wedding-(push|pull|tag|build)' | awk '{ print $1 }' | xargs kubectl delete po
		if failed && os.Getenv("KEEP_FAILED_PODS") != "" {
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		err = podClient.Delete(ctx, pod.Name, metav1.DeleteOptions{})
		if err != nil {
			log.Printf("delete pod %s: %v", pod.Name, err)
		}
	}()

waitRunning:
	pod, err = s.kubernetesClient.CoreV1().Pods(s.namespace).Get(ctx, pod.Name, metav1.GetOptions{})

	if err != nil {
		return fmt.Errorf("look up pod %s: %v", pod.Name, err)
	}

	switch pod.Status.Phase {
	case "Failed":
		failed = true
		fallthrough
	case "Succeeded":
		fallthrough
	case "Running":
		goto printLogs
	default:
		time.Sleep(time.Second)
		goto waitRunning
	}

printLogs:
	podLogs, err := s.kubernetesClient.CoreV1().Pods(s.namespace).
		GetLogs(pod.Name, &corev1.PodLogOptions{Follow: true}).
		Stream(ctx)
	if err != nil {
		return fmt.Errorf("streaming pod %s logs: %v", pod.Name, err)
	}
	defer podLogs.Close()

	buf := make([]byte, 1024)

	for {
		n, err := podLogs.Read(buf)
		if err != nil {
			if err == io.EOF {
				if failed {
					return fmt.Errorf("pod %s failed", pod.Name)
				}

				for {
					pod, err = s.kubernetesClient.CoreV1().Pods(s.namespace).Get(ctx, pod.Name, metav1.GetOptions{})
					if err != nil {
						return fmt.Errorf("look up pod %s: %v", pod.Name, err)
					}

					switch pod.Status.Phase {
					case "Succeeded":
						return nil
					case "Failed":
						return fmt.Errorf("pod %s failed", pod.Name)
					default:
						log.Printf("pod %s phase %s", pod.Name, pod.Status.Phase)
						time.Sleep(time.Second)
					}
				}
			}

			return fmt.Errorf("read pod %s logs: %v", pod.Name, err)
		}

		w.Write([]byte(string(buf[:n])))
	}
}

func (s Service) podStatus(ctx context.Context, podName string) (corev1.PodPhase, error) {
	pod, err := s.kubernetesClient.CoreV1().Pods(s.namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	return pod.Status.Phase, nil
}

func streamf(w io.Writer, message string, args ...interface{}) []byte {
	b, err := json.Marshal(fmt.Sprintf(message, args...))
	if err != nil {
		panic(err) // encode a string to json should not fail
	}

	return []byte(fmt.Sprintf(`{"stream": %s}`, b))
}
