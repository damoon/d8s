package d8s

import (
	"fmt"
	"os"

	"github.com/urfave/cli/v2"
)

var (
	gitHash string
	gitRef  = "latest"
)

func Version(c *cli.Context) error {
	_, err := os.Stdout.WriteString(fmt.Sprintf("version: %s\ngit commit: %s", gitRef, gitHash))
	if err != nil {
		return err
	}

	return nil
}
