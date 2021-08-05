package cmd

import (
	"github.com/go-logr/logr"
	"github.com/urfave/cli/v2"
	"go.uber.org/zap/zapcore"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func Logger(c *cli.Context, name string) logr.Logger {
	level := zapcore.InfoLevel
	if c.Bool("debug") {
		level = zapcore.DebugLevel
	}
	ctrl.SetLogger(zap.New(zap.UseDevMode(true), zap.Level(level)))
	setupLog := ctrl.Log.WithName(name)

	return setupLog
}
