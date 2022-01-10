package main

import (
	"log"
	"os"
	"path/filepath"

	d8s "github.com/damoon/d8s/pkg"
	"github.com/urfave/cli/v2"
)

func main() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("look up user home dir: %v", err)
	}

	app := &cli.App{
		Name:  "D8s (dates).",
		Usage: "The client for dinner.",
		Commands: []*cli.Command{
			{
				Name:  "up",
				Usage: "Connect to docker in docker and set DOCKER_HOST for started process.",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "verbose",
						Aliases: []string{"v"},
						Usage:   "Print verbose logs.",
						EnvVars: []string{"D8S_VERBOSE"},
					},
					&cli.StringFlag{
						Name:    "kubeconfig",
						Usage:   "Kubeconfig file to use.",
						EnvVars: []string{"D8S_KUBECONFIG", "KUBECONFIG"},
						Value:   filepath.Join(homeDir, ".kube", "config"),
					},
					&cli.StringFlag{
						Name:    "context",
						Usage:   "Context from kubectl config to use.",
						EnvVars: []string{"D8S_CONTEXT"},
					},
					&cli.StringFlag{
						Name:    "namespace",
						Usage:   "Namespace to look for dinner server.",
						EnvVars: []string{"D8S_NAMESPACE"},
					},
				},
				Action: d8s.Up,
			},
			{
				Name:   "version",
				Usage:  "Show the version",
				Action: d8s.Version,
			},
		},
	}

	err = app.Run(os.Args)
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}
}
