package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/restic/chunker"
	"github.com/urfave/cli/v2"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

const (
	dinnerPort     = 2375
	staticPol      = chunker.Pol(0x3DA3358B4DC173)
	ErrPodNotExist = NotFoundError("pod could not be found")
)

var (
	gitHash string
	gitRef  = "latest"
)

type MutexMap struct {
	mutexes sync.Map
}

func (mm *MutexMap) Lock(key []byte) func() {
	value, _ := mm.mutexes.LoadOrStore(string(key), &sync.Mutex{})
	mu := value.(*sync.Mutex)
	mu.Lock()
	unlock := func() {
		mu.Unlock()
	}
	return unlock
}

func main() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("look up user home dir: %v", err)
	}

	app := &cli.App{
		Name:  "D8s (dates).",
		Usage: "The client for dinner.",
		Commands: []*cli.Command{
			{
				Name:  "up",
				Usage: "Connect to docker in docker and set DOCKER_HOST for started process.",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "verbose",
						Aliases: []string{"v"},
						Usage:   "Print verbose logs.",
						EnvVars: []string{"D8S_VERBOSE"},
					},
					&cli.StringFlag{
						Name:    "kubeconfig",
						Usage:   "Kubeconfig file to use.",
						EnvVars: []string{"D8S_KUBECONFIG", "KUBECONFIG"},
						Value:   filepath.Join(homeDir, ".kube", "config"),
					},
					&cli.StringFlag{
						Name:    "context",
						Usage:   "Context from kubectl config to use.",
						EnvVars: []string{"D8S_CONTEXT"},
					},
					&cli.StringFlag{
						Name:    "namespace",
						Usage:   "Namespace to look for dinner server.",
						EnvVars: []string{"D8S_NAMESPACE"},
					},
				},
				Action: up,
			},
			{
				Name:   "version",
				Usage:  "Show the version",
				Action: version,
			},
		},
	}

	err = app.Run(os.Args)
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}
}

type NotFoundError string

func (e NotFoundError) Error() string {
	return string(e)
}

func version(c *cli.Context) error {
	_, err := os.Stdout.WriteString(fmt.Sprintf("version: %s\ngit commit: %s", gitRef, gitHash))
	if err != nil {
		return err
	}

	return nil
}

func up(c *cli.Context) error {
	args := c.Args()
	if args.First() == "" {
		return fmt.Errorf("command missing")
	}

	verbose := c.Bool("verbose")
	kubeconfig := c.String("kubeconfig")
	context := c.String("context")
	namespace := c.String("namespace")

	clientset, config, context, namespace, err := setupKubernetesClient(kubeconfig, context, namespace)
	if err != nil {
		return fmt.Errorf("setup kubernetes client: %v", err)
	}

	pod, err := findDinnerPod(c.Context, namespace, clientset)
	if err != nil {
		return fmt.Errorf("find dinner pod: %v", err)
	}

	localAddr, stopCh := portForward(pod, config, verbose)
	defer close(stopCh)

	err = executeCommand(c.Args(), localAddr)
	if err != nil {
		return fmt.Errorf("command failed with %s", err)
	}

	return nil
}

// https://stackoverflow.com/questions/50435564/use-kubectl-context-in-kubernetes-client-go
func setupKubernetesClient(kubeconfig, context, namespace string) (*kubernetes.Clientset, *rest.Config, string, string, error) {
	// TODO: verify kubernetes context is ok
	// https://github.com/tilt-dev/tilt/blob/fe386b5cc967383972bf73f8cbe6514c604100f8/internal/k8s/env.go#L38
	// https://github.com/turbine-kreuzberg/dind-nurse/blob/main/Tiltfile#L3

	configLoader := clientcmd.NewDefaultClientConfigLoadingRules()

	configLoader.ExplicitPath = kubeconfig

	clientCfg, err := configLoader.Load()
	if err != nil {
		return nil, nil, "", "", err
	}

	if context == "" {
		context = clientCfg.CurrentContext
	}

	if namespace == "" {
		namespace = clientCfg.Contexts[context].Namespace
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, nil, "", "", err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, nil, "", "", err
	}

	return clientset, config, context, namespace, nil
}

func findDinnerPod(ctx context.Context, namespace string, clientset *kubernetes.Clientset) (*v1.Pod, error) {
	for i := 0; i < 60; i++ {
		pods, err := dinnerPodsInNamespace(ctx, clientset.CoreV1().Pods(namespace))
		if err != nil {
			return nil, err
		}

		pod := filterReady(pods.Items)

		if pod != nil {
			return pod, nil
		}

		select {
		case <-ctx.Done():
			return nil, ErrPodNotExist
		case <-time.After(time.Second):
			// continue
		}
	}

	return nil, ErrPodNotExist
}

func dinnerPodsInNamespace(ctx context.Context, podsAPI corev1.PodInterface) (*v1.PodList, error) {
	labelSelector := metav1.LabelSelector{MatchLabels: map[string]string{"app": "dinner"}}
	listOptions := metav1.ListOptions{
		LabelSelector: labels.Set(labelSelector.MatchLabels).String(),
		Limit:         100,
	}
	pods, err := podsAPI.List(ctx, listOptions)
	if err != nil {
		return nil, fmt.Errorf("list pods: %v", err)
	}

	return pods, nil
}

func filterReady(pods []v1.Pod) *v1.Pod {
PODS:
	for _, pod := range pods {
		if pod.Status.Phase != v1.PodRunning {
			continue
		}

		for _, conditions := range pod.Status.Conditions {
			if conditions.Status != v1.ConditionTrue {
				continue PODS
			}
		}

		return &pod
	}

	return nil
}

func portForward(pod *v1.Pod, cfg *rest.Config, verbose bool) (string, chan struct{}) {
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
			if verbose {
				fmt.Println(ln)
			}
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
			dinnerPort,
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
	cmd.Env = append(cmd.Env, "DOCKER_BUILDKIT=1")

	fmt.Printf("Execute command DOCKER_HOST=tcp://%s DOCKER_BUILDKIT=0 %v\n", localAddr, cmd)

	err := cmd.Run()
	if err != nil {
		return err
	}

	return nil
}
