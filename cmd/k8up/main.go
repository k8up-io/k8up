package main

import (
	"fmt"
	"log"
	"os"
	"runtime"

	"github.com/urfave/cli/v2"

	"github.com/vshn/k8up/cmd"
	"github.com/vshn/k8up/cmd/operator"
	"github.com/vshn/k8up/cmd/restic"
)

// Strings are populated by Goreleaser
var (
	version = "snapshot"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	err := app().Run(os.Args)
	if err != nil {
		log.Fatalf("unable to start k8up: %v", err)
	}
}

func mainAction(c *cli.Context) error {
	setupLog := cmd.Logger(c, "k8up")
	setupLog.WithValues(
		"version", version,
		"date", date,
		"commit", commit,
		"go_os", runtime.GOOS,
		"go_arch", runtime.GOARCH,
		"go_version", runtime.Version(),
	).Info("Starting k8up…")

	return nil
}

func app() *cli.App {
	cli.VersionPrinter = func(c *cli.Context) {
		fmt.Printf("version=%s revision=%s date=%s\n", c.App.Version, commit, date)
	}

	return &cli.App{
		Name:                 "k8up",
		Version:              version,
		Copyright:            "(c) 2021 VSHN AG",
		EnableBashCompletion: true,
		Before:               mainAction,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:        "debug",
				Aliases:     []string{"verbose", "d"},
				Usage:       "sets the log level to debug",
				EnvVars:     []string{"K8UP_DEBUG"},
				DefaultText: "false",
			},
		},
		Commands: []*cli.Command{
			operator.Command,
			restic.Command,
		},
	}
}
