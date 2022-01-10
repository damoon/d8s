package d8s

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func Down(ctx context.Context, allowContext string) error {
	// verify kubernetes context in use
	allowed, err := contextAllowed(allowContext)
	if err != nil {
		return fmt.Errorf("verify kubernetes context: %v", err)
	}
	if !allowed {
		return fmt.Errorf("kubernetes context not allowed")
	}

	err = deleteDind()
	if err != nil {
		return fmt.Errorf("delete dind: %v", err)
	}

	return nil
}

func deleteDind() error {
	cmd := exec.Command(
		"kubectl",
		"delete",
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
