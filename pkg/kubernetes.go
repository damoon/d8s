package wedding

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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
