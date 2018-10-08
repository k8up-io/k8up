package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"

	"git.vshn.net/vshn/wrestic/kubernetes"
)

type commandOptions struct {
	print bool
	stdin bool
	kubernetes.Params
}

func genericCommand(args []string, options commandOptions) ([]string, []string) {

	// Turn into noop if previous commands failed
	if commandError != nil {
		fmt.Println("Errors occured during previous commands skipping...")
		return nil, nil
	}

	cmd := exec.Command(restic, args...)
	cmd.Env = os.Environ()

	if options.stdin {
		stdout, err := kubernetes.PodExec(options.Params)
		stdin, err := cmd.StdinPipe()
		if err != nil {
			fmt.Println(err)
			commandError = err
			return nil, nil
		}
		if stdout == nil {
			fmt.Println("stdout is nil")
		}
		// This needs to run in a separate thread because
		// cmd.CombinedOutput blocks until the command is finished
		// TODO: this is the place where we could implement some sort of
		// progress bars by wrapping stdin/stdout in a custom reader/writer
		go func() {
			defer stdin.Close()
			_, err := io.Copy(stdin, stdout)
			if err != nil {
				fmt.Println(err)
				commandError = err
				return
			}
		}()
	}

	commandStdout, err := cmd.StdoutPipe()
	commandStderr, err := cmd.StderrPipe()

	finished := make(chan error, 0)

	stdOutput := make([]string, 0)
	stderrOutput := make([]string, 0)

	cmd.Start()

	go func() {
		var collectErr error
		stdOutput, collectErr = collectOutput(commandStdout, options.print)
		finished <- collectErr
	}()

	go func() {
		var collectErr error
		stderrOutput, collectErr = collectOutput(commandStderr, options.print)
		finished <- collectErr
	}()

	collectErr1 := <-finished
	collectErr2 := <-finished
	err = cmd.Wait()

	// Avoid overwriting any errors produced by the
	// copy command
	if commandError == nil {
		if err != nil {
			commandError = err
		}
		if collectErr1 != nil {
			commandError = collectErr1
		}
		if collectErr2 != nil {
			commandError = collectErr2
		}
	}

	return stdOutput, stderrOutput
}

func collectOutput(output io.ReadCloser, print bool) ([]string, error) {
	collectedOutput := make([]string, 0)
	scanner := bufio.NewScanner(output)
	buff := make([]byte, 64*1024*1024)
	scanner.Buffer(buff, 64*1024*1024)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		m := scanner.Text()
		if print {
			fmt.Println(m)
		}
		collectedOutput = append(collectedOutput, m)
	}
	return collectedOutput, scanner.Err()
}

func unlock() {
	fmt.Println("Removing locks...")
	args := []string{"unlock"}
	genericCommand(args, commandOptions{print: true})
}
