package main

import (
	"errors"
	"os"
)

func checkCommand() {
	args := []string{"check"}
	parseCheckOutput(genericCommand(args, commandOptions{print: true}))
}

func parseCheckOutput(stdout, stderr []string) {
	metrics.Errors.WithLabelValues("all", os.Getenv(hostname)).Set(float64(len(stderr)))
	metrics.Update(metrics.Errors)
	if len(stderr) > 0 {
		commandError = errors.New("There was at least one backup error")
	}
}
