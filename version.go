package main

import (
	"fmt"
)

var (
	version string
	commit  string
	date    string
)

func Version() error {
	_, err := fmt.Printf("version: %s\ncommit: %s\built at: %s\n", version, commit, date)
	if err != nil {
		return err
	}

	return nil
}
