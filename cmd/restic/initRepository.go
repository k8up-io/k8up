package main

import "fmt"

func initRepository() error {
	if _, err := listSnapshots(); err == nil {
		return nil
	}

	fmt.Println("No repository available, initialising...")
	args := []string{"init"}
	genericCommand(args, commandOptions{print: true})
	return commandError
}
