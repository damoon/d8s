package main

/*
	https://restic.readthedocs.io/en/stable/040_backup.html?highlight=password#environment-variables
	https://restic.readthedocs.io/en/stable/030_preparing_a_new_repo.html#minio-server
	https://restic.readthedocs.io/en/stable/100_references.html?highlight=deduplication#backups-and-deduplication
*/

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
	"regexp"
	"strings"
	"time"

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

const weddingPort = 2376

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

	clientset, config, namespace, err := setupKubernetesClient(context)
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

	err = executeCommand(c.Args(), localAddr)
	if err != nil {
		return fmt.Errorf("command failed with %s", err)
	}

	return nil
}

func setupKubernetesClient(context string) (*kubernetes.Clientset, *rest.Config, string, error) {
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

	namespace := clientCfg.Contexts[context].Namespace

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
