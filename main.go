package main

import (
	"fmt"
	"log"
	"os"

	d8s "github.com/damoon/d8s/pkg"
	"github.com/urfave/cli/v2"
)

var (
	verbose = &cli.BoolFlag{
		Name:    "verbose",
		Aliases: []string{"v"},
		Usage:   "Print verbose logs.",
		EnvVars: []string{"D8S_VERBOSE"},
	}
	allowContext = &cli.StringFlag{
		Name:    "allow-context",
		Usage:   "Allowed Kubernetes context name.",
		EnvVars: []string{"TILT_ALLOW_CONTEXT"},
	}
)

func main() {
	app := &cli.App{
		Name:  "D8s (dates).",
		Usage: "The client for dinner.",
		Commands: []*cli.Command{
			{
				Name:  "up",
				Usage: "Connect to docker in docker and set DOCKER_HOST for started process.",
				Flags: []cli.Flag{
					verbose,
					allowContext,
				},
				Action: func(c *cli.Context) error {
					allowContext := c.String(allowContext.Name)
					verbose := c.Bool(verbose.Name)
					args := c.Args()
					if !args.Present() {
						return fmt.Errorf("command missing")
					}
					command := args.Slice()

					return d8s.Up(allowContext, verbose, command)
				},
			},
			{
				Name:  "version",
				Usage: "Show the version",
				Action: func(c *cli.Context) error {
					return d8s.Version()
				},
			},
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
