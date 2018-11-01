package main

import (
	"fmt"
	"os"
	"strings"
)

func pruneCommand() {
	// TODO: check for integers
	args := []string{"forget"}

	if last := os.Getenv(keepLastEnv); last != "" {
		args = append(args, keepLastArg, last)
	}

	if hourly := os.Getenv(keepHourlyEnv); hourly != "" {
		args = append(args, keepHourlyArg, hourly)
	}

	if daily := os.Getenv(keepDailyEnv); daily != "" {
		args = append(args, keepDailyArg, daily)
	}

	if weekly := os.Getenv(keepWeeklyEnv); weekly != "" {
		args = append(args, keepWeeklyArg, weekly)
	}

	if monthly := os.Getenv(keepMonthlyEnv); monthly != "" {
		args = append(args, keepMonthlyArg, monthly)
	}

	if yearly := os.Getenv(keepYearlyEnv); yearly != "" {
		args = append(args, keepYearlyArg, yearly)
	}

	fmt.Println("Run forget without prune and update the webhook")
	fmt.Println("forget params: ", strings.Join(args, " "))
	genericCommand(args, commandOptions{print: true})

	updateSnapshots()

	args = []string{"prune"}
	genericCommand(args, commandOptions{print: true})
}
