package main

import (
	"errors"
	"os"
)

func checkCommand() {
	args := []string{"check"}
	_, stderr := genericCommand(args, commandOptions{print: true})
	metrics.Errors.WithLabelValues("all", os.Getenv(hostname)).Set(float64(len(stderr)))
	metrics.Update(metrics.Errors)
	if len(stderr) > 0 {
		commandError = errors.New("There was at least one backup error")
	}
}
