package main

import "fmt"

func initRepository() {
	if _, err := listSnapshots(); err == nil {
		return
	}

	fmt.Println("No repository available, initialising...")
	args := []string{"init"}
	genericCommand(args, commandOptions{print: true})
}
