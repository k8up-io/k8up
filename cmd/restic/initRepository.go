package main

import "fmt"

func initRepository() error {
	_, err := listSnapshots()
	if err == nil {
		return nil
	} else if err.Error() == notInitialisedError {
		fmt.Println("No repository available, initialising...")
		args := []string{"init"}
		genericCommand(args, commandOptions{print: true})
		return commandError
	} else {
		return err
	}
}
