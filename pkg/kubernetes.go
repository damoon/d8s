package wedding

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (s Service) executePod(ctx context.Context, pod *corev1.Pod, w io.Writer) error {
	podClient := s.kubernetesClient.CoreV1().Pods(s.namespace)

	w.Write([]byte("Creating new pod.\n"))

	pod, err := podClient.Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		streamf(w, "Pod creation failed: %v\n", err)
		return fmt.Errorf("create pod: %v", err)
	}

	streamf(w, "Created pod %v.\n", pod.Name)

	failed := false

	defer func() {
		if failed {
			w.Write([]byte("Pod failed. Skipping cleanup.\n"))
			return
		}

		return

		w.Write([]byte("Deleting pod.\n"))

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		err = podClient.Delete(ctx, pod.Name, metav1.DeleteOptions{})
		if err != nil {
			streamf(w, "Pod deletetion failed: %v\n", err)
			log.Printf("delete pod %s: %v", pod.Name, err)
		}
	}()

	w.Write([]byte("Waiting for pod execution.\n"))

waitRunning:
	pod, err = s.kubernetesClient.CoreV1().Pods(s.namespace).Get(ctx, pod.Name, metav1.GetOptions{})

	if err != nil {
		streamf(w, "Looking up pod: %v.\n", err)
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
	// w.Write([]byte("Streaming logs.\n"))

	podLogs, err := s.kubernetesClient.CoreV1().Pods(s.namespace).
		GetLogs(pod.Name, &corev1.PodLogOptions{Follow: true}).
		Stream(ctx)
	if err != nil {
		streamf(w, "Log streaming failed: %v\n", err)
		return fmt.Errorf("streaming pod %s logs: %v", pod.Name, err)
	}
	defer podLogs.Close()

	buf := make([]byte, 1024)

	for {
		n, err := podLogs.Read(buf)
		if err != nil {
			if err == io.EOF {
				// w.Write([]byte("End of logs reached.\n"))
				if failed {
					return fmt.Errorf("pod %s failed", pod.Name)
				}

				for {
					pod, err = s.kubernetesClient.CoreV1().Pods(s.namespace).Get(ctx, pod.Name, metav1.GetOptions{})
					if err != nil {
						streamf(w, "Looking up pod: %v.\n", err)
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
