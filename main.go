package main

import (
	"fmt"
	"log"
	"os"

	d8s "github.com/damoon/d8s/pkg"
	"github.com/urfave/cli/v2"
)

var (
	allowContext = &cli.StringFlag{
		Name:    "allow-context",
		Usage:   "Allowed Kubernetes context name.",
		EnvVars: []string{"TILT_ALLOW_CONTEXT"},
	}
)

func main() {
	up := func(c *cli.Context) error {
		ctx := c.Context
		allowContext := c.String(allowContext.Name)
		args := c.Args()
		if !args.Present() {
			return fmt.Errorf("command missing")
		}
		command := args.Slice()

		return d8s.Up(ctx, allowContext, command)
	}
	down := func(c *cli.Context) error {
		ctx := c.Context
		allowContext := c.String(allowContext.Name)

		return d8s.Down(ctx, allowContext)
	}
	version := func(c *cli.Context) error {
		return d8s.Version()
	}

	app := &cli.App{
		Name:  "D8s (dates).",
		Usage: "A wrapper for docker in docker doing port-forward.",
		Flags: []cli.Flag{
			allowContext,
		},
		Action: up,
		Commands: []*cli.Command{
			{
				Name:   "up",
				Usage:  "Connect to docker in docker and set DOCKER_HOST for started process.",
				Action: up,
			},
			{
				Name:   "down",
				Usage:  "Deletes docker in docker deployment.",
				Action: down,
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
		log.Fatal(err)
	}
}
