package d8s

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"strconv"
	"time"
)

func Run(ctx context.Context, allowContext string, command []string) error {
	// verify kubernetes context in use
	allowed, err := ContextAllowed(ctx, allowContext)
	if err != nil {
		return fmt.Errorf("verify kubernetes context: %v", err)
	}
	if !allowed {
		return fmt.Errorf("kubernetes context not allowed")
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

func awaitDind(ctx context.Context) error {
	cmd := exec.CommandContext(
		ctx,
		"kubectl",
		"wait",
		"--for=condition=available",
		"--timeout=600s",
		"deployment/dind",
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Env = os.Environ()

	err := cmd.Run()
	if err != nil {
		return err
	}

	return nil
}

func freePort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer l.Close()

	return l.Addr().(*net.TCPAddr).Port, nil
}

func startPortForward(ctx context.Context, localPort, dindPort int) (func(), error) {
	ctx, cancel := context.WithCancel(ctx)
	done := make(chan interface{})

	go func() {
		defer func() {
			done <- struct{}{}
		}()

		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(10 * time.Millisecond):
				err := portForward(ctx, localPort, dindPort)
				isDone := len(ctx.Done()) > 0 || errors.Is(ctx.Err(), context.Canceled)

				if err != nil && !isDone {
					log.Printf("port forward failed: %v", err)
				}
			}
		}
	}()

	err := awaitPortOpen(ctx, localPort)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("wait for port forward to start: %v", err)
	}

	return func() {
		cancel()
		<-done
	}, nil
}

func portForward(ctx context.Context, localPort, dinnerPort int) error {
	cmd := exec.CommandContext(
		ctx,
		"kubectl",
		"port-forward",
		"deployment/dind",
		fmt.Sprintf("%d:%d", localPort, dinnerPort),
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Env = os.Environ()

	err := cmd.Run()
	if err != nil {
		return err
	}

	return nil
}

func awaitPortOpen(ctx context.Context, localPort int) error {
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("port did not open: %v", ctx.Err())
		case <-time.After(10 * time.Millisecond):
			timeout, cancel := context.WithTimeout(ctx, time.Second)
			defer cancel()

			open := portOpen(timeout, "127.0.0.1", strconv.Itoa(localPort))
			if open {
				return nil
			}
		}
	}
}

func portOpen(ctx context.Context, host string, port string) bool {

	d := net.Dialer{Timeout: time.Second}

	conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(host, port))
	if err != nil {
		return false
	}
	defer conn.Close()

	return true
}

func executeCommand(ctx context.Context, command []string, dockerAddr string) error {
	cmd := exec.CommandContext(
		ctx,
		command[0],
		command[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "DOCKER_HOST="+dockerAddr)
	cmd.Env = append(cmd.Env, "DOCKER_BUILDKIT=1")

	fmt.Printf("Execute command %s\n", cmd.String())

	err := cmd.Run()
	if err != nil {
		return err
	}

	return nil
}
