package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/restic/chunker"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"

	"github.com/urfave/cli/v2"
)

const (
	weddingPort = 2376
	staticPol   = chunker.Pol(0x3DA3358B4DC173)
)

var (
	gitHash string
	gitRef  = "latest"
)

func main() {
	app := &cli.App{
		Name:  "Wedding client",
		Usage: "Make wedding accessible.",
		Commands: []*cli.Command{
			{
				Name:  "run",
				Usage: "Connect to wedding server and set DOCKER_HOST for started process.",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "context",
						Usage:   "Context from kubectl config to use.",
						EnvVars: []string{"WEDDING_CONTEXT"},
					},
					&cli.StringFlag{
						Name:    "namespace",
						Usage:   "Namespace to look for wedding server.",
						EnvVars: []string{"WEDDING_NAMESPACE"},
					},
				},
				Action: run,
			},
			{
				Name:   "version",
				Usage:  "Show the version",
				Action: version,
			},
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}
}

func version(c *cli.Context) error {
	_, err := os.Stdout.WriteString(fmt.Sprintf("version: %s\ngit commit: %s", gitRef, gitHash))
	if err != nil {
		return err
	}

	return nil
}

func run(c *cli.Context) error {
	args := c.Args()
	if args.First() == "" {
		return fmt.Errorf("command missing")
	}

	context := c.String("context")
	namespace := c.String("namespace")

	clientset, config, namespace, err := setupKubernetesClient(context, namespace)
	if err != nil {
		return fmt.Errorf("setup kubernetes client: %v", err)
	}

	pods := clientset.CoreV1().Pods(namespace)

	pod, err := weddingPod(c.Context, pods)
	if err != nil {
		return fmt.Errorf("list pods: %v", err)
	}

	localAddr, stopCh := portForward(pod, config)
	defer close(stopCh)

	localPort, err := localServer("http://" + localAddr)
	if err != nil {
		return fmt.Errorf("parse local address: %v", err)
	}

	err = executeCommand(c.Args(), fmt.Sprintf("127.0.0.1:%d", localPort))
	if err != nil {
		return fmt.Errorf("command failed with %s", err)
	}

	return nil
}

func setupKubernetesClient(context, namespace string) (*kubernetes.Clientset, *rest.Config, string, error) {
	configLoader := clientcmd.NewDefaultClientConfigLoadingRules()
	configPath := configLoader.Precedence[0]

	clientCfg, err := configLoader.Load()
	if err != nil {
		return nil, nil, "", err
	}

	config, err := clientcmd.BuildConfigFromFlags("", configPath)
	if err != nil {
		panic(err.Error())
	}

	if context == "" {
		context = clientCfg.CurrentContext
	}

	if namespace != "" {
		namespace = clientCfg.Contexts[context].Namespace
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, nil, "", err
	}

	return clientset, config, namespace, nil
}

func weddingPod(ctx context.Context, pods corev1.PodInterface) (*v1.Pod, error) {
	for i := 0; i < 60; i++ {
		labelSelector := metav1.LabelSelector{MatchLabels: map[string]string{"app": "wedding"}}
		listOptions := metav1.ListOptions{
			LabelSelector: labels.Set(labelSelector.MatchLabels).String(),
			Limit:         100,
		}
		pods, err := pods.List(ctx, listOptions)
		if err != nil {
			return nil, fmt.Errorf("list pods: %v", err)
		}

	PODS:
		for _, pod := range pods.Items {
			if pod.Status.Phase != v1.PodRunning {
				continue
			}

			for _, conditions := range pod.Status.Conditions {
				if conditions.Status != v1.ConditionTrue {
					continue PODS
				}
			}

			return &pod, nil
		}

		time.Sleep(time.Second)
	}

	return nil, fmt.Errorf("running wedding server not found")
}

func portForward(pod *v1.Pod, cfg *rest.Config) (string, chan struct{}) {
	stopCh := make(chan struct{}, 1)

	readyCh := make(chan struct{})
	addrCh := make(chan string)

	pfr, pfw := io.Pipe()

	go func() {
		scanner := bufio.NewScanner(pfr)
		addr := ""
		for scanner.Scan() {
			ln := scanner.Text()
			if addr == "" {
				addr = extractAddress(ln)
				if addr != "" {
					addrCh <- addr
				}
			}
			fmt.Println(ln)
		}
		if err := scanner.Err(); err != nil {
			log.Printf("reading from port forward logs: %v", err)
		}
	}()

	go func() {
		defer pfw.Close()

		err := portForwardPod(
			pod.ObjectMeta.Namespace,
			pod.ObjectMeta.Name,
			weddingPort,
			cfg,
			stopCh,
			readyCh,
			pfw,
			os.Stderr,
		)

		_, ok := (<-stopCh)
		if !ok {
			return
		}

		if err != nil {
			log.Fatal(err)
		}
	}()

	localAddr := <-addrCh
	<-readyCh

	return localAddr, stopCh
}

func extractAddress(ln string) string {
	re := regexp.MustCompile(`Forwarding from ((127.0.0.1|\[::1\]):[0-9]+) -> [0-9]+`)
	matches := re.FindAllStringSubmatch(ln, -1)
	if len(matches) != 1 {
		return ""
	}
	return matches[0][1]
}

func portForwardPod(
	namespace,
	podName string,
	port int,
	cfg *rest.Config,
	stopCh <-chan struct{},
	readyCh chan struct{},
	stdout io.Writer,
	errout io.Writer,
) error {
	path := fmt.Sprintf("/api/v1/namespaces/%s/pods/%s/portforward", namespace, podName)
	hostIP := strings.TrimLeft(cfg.Host, "htps:/")

	transport, upgrader, err := spdy.RoundTripperFor(cfg)
	if err != nil {
		return err
	}

	dialer := spdy.NewDialer(
		upgrader,
		&http.Client{Transport: transport},
		http.MethodPost,
		&url.URL{Scheme: "https", Path: path, Host: hostIP},
	)
	fw, err := portforward.New(
		dialer,
		[]string{fmt.Sprintf("%d:%d", 0, port)},
		stopCh,
		readyCh,
		stdout,
		errout,
	)
	if err != nil {
		return err
	}
	return fw.ForwardPorts()
}

func executeCommand(args cli.Args, localAddr string) error {
	cmd := exec.Command(args.First(), args.Tail()...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "DOCKER_HOST=tcp://"+localAddr)

	fmt.Printf("Execute command DOCKER_HOST=tcp://%s %v\n", localAddr, cmd)

	err := cmd.Run()
	if err != nil {
		return err
	}

	return nil
}

func localServer(localAddr string) (int, error) {
	targetURL, err := url.Parse(localAddr)
	if err != nil {
		return 0, err
	}

	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	mux := http.NewServeMux()
	mux.HandleFunc("/", uploadContextHandlerFunc(proxy, localAddr))

	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return 0, err
	}

	go http.Serve(listener, mux)

	return listener.Addr().(*net.TCPAddr).Port, nil
}

func uploadContextHandlerFunc(proxy *httputil.ReverseProxy, localAddr string) http.HandlerFunc {
	re := regexp.MustCompile(`^/[^/]+/build$`)
	chunksList := bytes.Buffer{}

	return func(w http.ResponseWriter, r *http.Request) {
		if !re.MatchString(r.URL.Path) {
			proxy.ServeHTTP(w, r)
		}

		chnker := chunker.New(r.Body, staticPol)

		for {
			c, err := chnker.Next(nil)

			if err == io.EOF {
				break
			}

			if err != nil {
				log.Printf("searching next chunk: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			hash, err := hashData(bytes.NewBuffer(c.Data))

			chunksList.Write(hash)

			found, err := chunkExists(r.Context(), localAddr+"/_chunks", hash)
			if err != nil {
				log.Printf("chunk #%d deduplication: %v", c.Cut, err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			if found {
				log.Printf("SKIP uploading %d bytes", c.Length)
				continue
			}

			log.Printf("uploading %d bytes", c.Length)

			err = uploadChunk(r.Context(), localAddr+"/_chunks", bytes.NewBuffer(c.Data))
			if err != nil {
				log.Printf("uploading chunk #%d: %v", c.Cut, err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		}

		r.Body = io.NopCloser(bytes.NewReader(chunksList.Bytes()))
		r.Header.Add("d8s-chunked", "true")

		proxy.ServeHTTP(w, r)
	}
}

func chunkExists(ctx context.Context, target string, hash []byte) (bool, error) {
	client := &http.Client{
		Transport: &http.Transport{
			Dial: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).Dial,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return false, fmt.Errorf("create request for chunk deduplication: %v", err)
	}

	hashHex := make([]byte, hex.EncodedLen(len(hash)))
	hex.Encode(hashHex, hash)

	q := req.URL.Query()
	q.Add("hash", string(hashHex))
	req.URL.RawQuery = q.Encode()

	resp, err := client.Do(req)
	if err != nil {
		return false, fmt.Errorf("chunk deduplication request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return true, nil
	}

	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}

	return false, fmt.Errorf("chunk deduplication request returned status code %d", resp.StatusCode)

}

func hashData(r io.Reader) ([]byte, error) {
	h := sha256.New()

	_, err := io.Copy(h, r)
	if err != nil {
		return []byte{}, err
	}

	return h.Sum(nil), nil
}

func uploadChunk(ctx context.Context, target string, data io.Reader) error {
	client := &http.Client{
		Transport: &http.Transport{
			Dial: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).Dial,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, data)
	if err != nil {
		return fmt.Errorf("create request for chunk upload: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("chunk upload: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("chunk upload returned status code %d", resp.StatusCode)
	}

	return nil
}
