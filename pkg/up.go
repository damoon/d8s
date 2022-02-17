package d8s

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const (
	dindPort = 2375
)

func Up(ctx context.Context, allowContext string, command []string) error {
	// verify kubernetes context in use
	allowed, err := ContextAllowed(ctx, allowContext)
	if err != nil {
		return fmt.Errorf("verify kubernetes context: %v", err)
	}
	if !allowed {
		return fmt.Errorf("kubernetes context not allowed")
	}

	// deploy docker in docker
	err = deployDind(ctx)
	if err != nil {
		return fmt.Errorf("deploy dind: %v", err)
	}

	// port forward
	err = awaitDind(ctx)
	if err != nil {
		return fmt.Errorf("wait for dind to start: %v", err)
	}

	localPort, err := freePort()
	if err != nil {
		return fmt.Errorf("select free local port: %v", err)
	}

	cancel, err := startPortForward(ctx, localPort, dindPort)
	if err != nil {
		return fmt.Errorf("starting port-forward: %v", err)
	}
	defer cancel()

	// execute command
	err = executeCommand(ctx, command, fmt.Sprintf("tcp://127.0.0.1:%d", localPort))
	if err != nil {
		return fmt.Errorf("command failed with %s", err)
	}

	return nil
}

func deployDind(ctx context.Context) error {
	cmd := exec.CommandContext(
		ctx,
		"kubectl",
		"apply",
		"-f-",
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = strings.NewReader(manifest)
	cmd.Env = os.Environ()

	err := cmd.Run()
	if err != nil {
		return err
	}

	return nil
}
