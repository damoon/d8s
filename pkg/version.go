package d8s

import (
	"fmt"
)

var (
	gitHash string
	gitRef  = "latest"
)

func Version() error {
	_, err := fmt.Printf("version: %s\ngit commit: %s", gitRef, gitHash)
	if err != nil {
		return err
	}

	return nil
}
