package d8s

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

var (
	//go:embed kubernetes.yaml
	manifest string
)

func ContextAllowed(envVar string) (bool, error) {
	contextName, err := kubectlContext()
	if err != nil {
		return false, err
	}

	if contextName == envVar {
		return true, nil
	}

	if isTiltDevCluster(contextName) {
		return true, nil
	}

	return false, fmt.Errorf("context %s does not appear to be a development environment", contextName)
}

func kubectlContext() (string, error) {
	cmd := exec.Command(
		"kubectl",
		"config",
		"current-context",
	)
	cmd.Stderr = os.Stderr

	context, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("execute %v: %v", cmd.String(), err)
	}

	return strings.TrimSpace(string(context)), nil
}

// see https://github.com/tilt-dev/tilt/blob/fe386b5cc967383972bf73f8cbe6514c604100f8/internal/k8s/env.go#L38
func isTiltDevCluster(name string) bool {
	return name == "minikube" ||
		name == "docker-for-desktop" ||
		name == "microk8s" ||
		name == "crc" ||
		name == "kind-0.5-" ||
		name == "kind-0.6+" ||
		name == "k3d" ||
		name == "krucible" ||
		name == "rancher-desktop"
}
