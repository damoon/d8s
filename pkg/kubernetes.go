package wedding

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (s Service) executePod(ctx context.Context, pod *corev1.Pod, w io.Writer) error {
	podClient := s.kubernetesClient.CoreV1().Pods(s.namespace)

	stream(w, "Creating new pod.\n")

	pod, err := podClient.Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		streamf(w, "Pod creation failed: %v\n", err)
		return fmt.Errorf("create pod: %v", err)
	}

	streamf(w, "Created pod %v.\n", pod.Name)

	failed := false

	defer func() {
		if failed {
			stream(w, "Pod failed. Skipping cleanup.\n")
			return
		}

		stream(w, "Deleting pod.\n")

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		err = podClient.Delete(ctx, pod.Name, metav1.DeleteOptions{})
		if err != nil {
			streamf(w, "Pod deletetion failed: %v\n", err)
			log.Printf("delete pod: %v", err)
		}
	}()

	stream(w, "Waiting for pod execution.\n")

waitRunning:
	pod, err = s.kubernetesClient.CoreV1().Pods(s.namespace).Get(ctx, pod.Name, metav1.GetOptions{})

	if err != nil {
		streamf(w, "Looking up pod: %v.\n", err)
		return fmt.Errorf("look up pod: %v", err)
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
	stream(w, "Streaming logs.\n")

	podLogs, err := s.kubernetesClient.CoreV1().Pods(s.namespace).
		GetLogs(pod.Name, &corev1.PodLogOptions{Follow: true}).
		Stream(ctx)
	if err != nil {
		streamf(w, "Log streaming failed: %v\n", err)
		return fmt.Errorf("streaming logs: %v", err)
	}
	defer podLogs.Close()

	buf := make([]byte, 1024)

	for {
		n, err := podLogs.Read(buf)
		if err != nil {
			if err == io.EOF {
				stream(w, "End of logs reached.\n")
				if failed {
					return fmt.Errorf("pod failed")
				}
				return nil
			}

			return fmt.Errorf("read logs: %v", err)
		}

		stream(w, string(buf[:n]))
	}
}

func (s Service) podStatus(ctx context.Context, podName string) (corev1.PodPhase, error) {
	pod, err := s.kubernetesClient.CoreV1().Pods(s.namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	return pod.Status.Phase, nil
}

func stream(w io.Writer, message string) error {
	b, err := json.Marshal(message)
	if err != nil {
		panic(err) // encode a string to json should not fail
	}

	_, err = w.Write([]byte(fmt.Sprintf(`{"stream": %s}`, b)))
	if err != nil {
		return err
	}

	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	} else {
		return fmt.Errorf("stream can not be flushed")
	}

	return nil
}

func streamf(w io.Writer, message string, args ...interface{}) error {
	return stream(w, fmt.Sprintf(message, args...))
}
