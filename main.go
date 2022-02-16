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
		EnvVars: []string{"TILT_ALLOW_CONTEXT", "D8S_ALLOW_CONTEXT"},
	}
	app = &cli.App{
		Name:        "D8s (dates).",
		Usage:       "A wrapper for docker in docker doing port-forward.",
		Description: "Example: d8s up docker run hello-world",
		Flags: []cli.Flag{
			allowContext,
		},
		Commands: []*cli.Command{
			{
				Name:   "up",
				Usage:  "Deploy and connect to docker in docker and set DOCKER_HOST for started process.",
				Action: up,
			},
			{
				Name:   "run",
				Usage:  "Connect to docker in docker and set DOCKER_HOST for started process.",
				Action: run,
			},
			{
				Name:   "down",
				Usage:  "Deletes docker in docker deployment.",
				Action: down,
			},
			{
				Name:   "version",
				Usage:  "Show the version",
				Action: Version,
			},
		},
	}
)

func main() {
	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}

func verifyContext(c *cli.Context) error {
	ctx := c.Context
	allowContext := c.String(allowContext.Name)

	allowed, err := d8s.ContextAllowed(ctx, allowContext)
	if err != nil {
		return fmt.Errorf("verify kubernetes context: %v", err)
	}

	if !allowed {
		return fmt.Errorf("kubernetes context not allowed")
	}

	return nil
}

func up(c *cli.Context) error {
	err := verifyContext(c)
	if err != nil {
		return err
	}

	ctx := c.Context
	allowContext := c.String(allowContext.Name)
	args := c.Args()
	if !args.Present() {
		return fmt.Errorf("command missing")
	}
	command := args.Slice()

	return d8s.Up(ctx, allowContext, command)
}

func run(c *cli.Context) error {
	err := verifyContext(c)
	if err != nil {
		return err
	}

	ctx := c.Context
	allowContext := c.String(allowContext.Name)
	args := c.Args()
	if !args.Present() {
		return fmt.Errorf("command missing")
	}
	command := args.Slice()

	return d8s.Run(ctx, allowContext, command)
}

func down(c *cli.Context) error {
	err := verifyContext(c)
	if err != nil {
		return err
	}

	ctx := c.Context
	allowContext := c.String(allowContext.Name)

	return d8s.Down(ctx, allowContext)
}
