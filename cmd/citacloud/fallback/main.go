package fallback

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-logr/logr"
	"github.com/k8up-io/k8up/v2/cmd"
	"github.com/urfave/cli/v2"
	"k8s.io/utils/exec"
)

type Fallback struct {
	BlockHeight int64
	Crypto      string
	Consensus   string
}

var fallback = &Fallback{}

var (
	Command = &cli.Command{
		Name:        "fallback",
		Description: "Execute fallback for CITA node",
		Category:    "cita-cloud",
		Action:      fallbackMain,
		Flags: []cli.Flag{
			&cli.Int64Flag{
				Name:        "block-height",
				Usage:       "The block height you want to recover.",
				Required:    true,
				Destination: &fallback.BlockHeight,
			},
			&cli.StringFlag{
				Name:        "crypto",
				Usage:       "The node of crypto. [sm/eth]",
				Value:       "sm",
				Destination: &fallback.Crypto,
			},
			&cli.StringFlag{
				Name:        "consensus",
				Usage:       "The node of consensus. [bft/raft/overlord]",
				Value:       "bft",
				Destination: &fallback.Consensus,
			},
		},
	}
)

func fallbackMain(c *cli.Context) error {
	fallbackLog := cmd.AppLogger(c).WithName("fallback")
	fallbackLog.Info("initializing")

	_, cancel := context.WithCancel(c.Context)
	cancelOnTermination(cancel, fallbackLog)

	return run(fallbackLog)
}

func cancelOnTermination(cancel context.CancelFunc, mainLogger logr.Logger) {
	mainLogger.Info("setting up a signal handler")
	s := make(chan os.Signal, 1)
	signal.Notify(s, syscall.SIGTERM)
	go func() {
		mainLogger.Info("received signal", "signal", <-s)
		cancel()
	}()
}

func run(logger logr.Logger) error {
	execer := exec.New()
	logger.Info("exec block height fallback...", "height", fallback.BlockHeight)
	err := execer.Command("cloud-op", "recover", fmt.Sprintf("%d", fallback.BlockHeight),
		"--node-root", "/data",
		"--config-path", "/cita-config/config.toml",
		"--crypto", fallback.Crypto,
		"--consensus", fallback.Consensus).Run()
	if err != nil {
		logger.Error(err, "exec block height fallback failed")
		return err
	}
	logger.Info("exec block height fallback successful", "height", fallback.BlockHeight)
	return nil
}
