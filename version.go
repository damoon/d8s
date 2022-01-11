package main

import (
	"fmt"

	"github.com/urfave/cli/v2"
)

var (
	version string
	commit  string
	date    string
)

func Version(c *cli.Context) error {
	_, err := fmt.Printf("version: %s\ncommit: %s\nbuilt at: %s\n", version, commit, date)
	if err != nil {
		return err
	}

	return nil
}
