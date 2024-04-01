package kubernetes

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/firepear/qsplit/v2"
	"github.com/go-logr/logr"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/remotecommand"

	"github.com/k8up-io/k8up/v2/restic/logging"
)

type ExecData struct {
	Reader *io.PipeReader
	Done   chan bool
}

// PodExec sends the command to the specified pod
// and returns a bytes buffer with the stdout
func PodExec(pod BackupPod, log logr.Logger) (*ExecData, error) {
	execLogger := log.WithName("k8sExec")
	config, _ := getClientConfig()
	k8sclient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("can't create k8s for exec: %w", err)
	}

	req := k8sclient.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(pod.PodName).
		Namespace(pod.Namespace).
		SubResource("exec")
	scheme := runtime.NewScheme()
	if err := apiv1.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("can't add runtime scheme: %w", err)
	}

	command := qsplit.ToStrings([]byte(pod.Command))
	execLogger.Info("executing command", "command", strings.Join(command, ", "), "namespace", pod.Namespace, "pod", pod.PodName)
	parameterCodec := runtime.NewParameterCodec(scheme)
	req.VersionedParams(&apiv1.PodExecOptions{
		Command:   command,
		Container: pod.ContainerName,
		Stdin:     false,
		Stdout:    true,
		Stderr:    true,
		TTY:       false,
	}, parameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		return nil, err
	}

	var stdoutReader, stdoutWriter = io.Pipe()
	done := make(chan bool, 1)
	go func() {
		err = exec.StreamWithContext(context.Background(), remotecommand.StreamOptions{
			Stdin:  nil,
			Stdout: stdoutWriter,
			Stderr: logging.NewErrorWriter(log.WithName(pod.PodName)),
			Tty:    false,
		})

		defer stdoutWriter.Close()
		done <- true

		if err != nil {
			execLogger.Error(err, "streaming data failed", "namespace", pod.Namespace, "pod", pod.PodName)
			return
		}
	}()

	data := &ExecData{
		Done:   done,
		Reader: stdoutReader,
	}

	return data, nil
}
