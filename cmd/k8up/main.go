package main

import (
	"fmt"
	"github.com/k8up-io/k8up/v2/cmd/citacloud/fallback"
	"github.com/k8up-io/k8up/v2/cmd/citacloud/switchover"

	"log"
	"os"
	"runtime"
	"time"

	"github.com/go-logr/logr"
	"github.com/urfave/cli/v2"
	"go.uber.org/zap/zapcore"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/k8up-io/k8up/v2/cmd"
	"github.com/k8up-io/k8up/v2/cmd/operator"
	"github.com/k8up-io/k8up/v2/cmd/restic"
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

func before(c *cli.Context) error {
	logger := newLogger("k8up", c.Bool("debug"))
	cmd.SetAppLogger(c, logger)

	logger.WithValues(
		"version", version,
		"date", date,
		"commit", commit,
		"go_os", runtime.GOOS,
		"go_arch", runtime.GOARCH,
		"go_version", runtime.Version(),
		"uid", os.Getuid(),
		"gid", os.Getgid(),
	).Info("Starting k8up…")

	return nil
}

func app() *cli.App {
	cli.VersionPrinter = func(_ *cli.Context) {
		fmt.Printf("version=%s revision=%s date=%s\n", version, commit, date)
	}

	compiled, err := time.Parse(time.RFC3339, date)
	if err != nil {
		compiled = time.Time{}
	}

	return &cli.App{
		Name:      "k8up",
		Version:   version,
		Compiled:  compiled,
		Copyright: "(c) 2021 VSHN AG",

		EnableBashCompletion: true,

		Before: before,
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
			fallback.Command,
			switchover.Command,
		},
	}
}

func newLogger(name string, debug bool) logr.Logger {
	level := zapcore.InfoLevel
	if debug {
		level = zapcore.DebugLevel
	}
	logger := zap.New(zap.UseDevMode(true), zap.Level(level))
	return logger.WithName(name)
}
