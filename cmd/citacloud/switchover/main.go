package switchover

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-logr/logr"
	"github.com/k8up-io/k8up/v2/cmd"
	"github.com/urfave/cli/v2"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Switchover struct {
	namespace  string
	sourceNode string
	destNode   string
}

var switchover = &Switchover{}

var (
	Command = &cli.Command{
		Name:        "switchover",
		Description: "Execute switchover for CITA node",
		Category:    "cita-cloud",
		Action:      switchoverMain,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "namespace",
				Usage:       "The namespace of chain node",
				Required:    true,
				Destination: &switchover.namespace,
			},
			&cli.StringFlag{
				Name:        "source-node",
				Usage:       "The node of source",
				Required:    true,
				Destination: &switchover.sourceNode,
			},
			&cli.StringFlag{
				Name:        "dest-node",
				Usage:       "The node of dest",
				Required:    true,
				Destination: &switchover.destNode,
			},
		},
	}
)

func switchoverMain(c *cli.Context) error {
	switchoverLog := cmd.AppLogger(c).WithName("switchover")
	switchoverLog.Info("initializing")

	ctx, cancel := context.WithCancel(c.Context)
	cancelOnTermination(cancel, switchoverLog)

	return run(ctx, switchoverLog)
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

func run(ctx context.Context, logger logr.Logger) error {
	k8sClient, err := InitK8sClient(switchover.namespace)
	if err != nil {
		logger.Error(err, "unable to init k8s client")
		return err
	}
	sourceNodeConfigmap, err := getAccountConfigmap(ctx, k8sClient, switchover.namespace, switchover.sourceNode)
	if err != nil {
		return err
	}
	destNodeConfigmap, err := getAccountConfigmap(ctx, k8sClient, switchover.namespace, switchover.destNode)
	if err != nil {
		return err
	}
	err = updateAccountConfigmap(ctx, k8sClient, switchover.namespace, switchover.sourceNode, destNodeConfigmap, logger)
	if err != nil {
		return err
	}
	err = updateAccountConfigmap(ctx, k8sClient, switchover.namespace, switchover.destNode, sourceNodeConfigmap, logger)
	if err != nil {
		return err
	}
	return nil
}

func getAccountConfigmap(ctx context.Context, client client.Client, namespace, name string) (string, error) {
	// find node
	sts := &appsv1.StatefulSet{}
	err := client.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, sts)
	if err != nil {
		return "", err
	}
	volumes := sts.Spec.Template.Spec.Volumes
	for _, vol := range volumes {
		if vol.Name == "node-account" {
			return vol.VolumeSource.ConfigMap.LocalObjectReference.Name, nil
		}
	}
	return "", nil
}

func updateAccountConfigmap(ctx context.Context, client client.Client, namespace, name, newConfigmap string, logger logr.Logger) error {
	logger.Info("update account configmap for node...", "namespace", namespace, "name", name, "configmap", newConfigmap)
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// find node
		sts := &appsv1.StatefulSet{}
		err := client.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, sts)
		if err != nil {
			return err
		}
		volumes := sts.Spec.Template.Spec.Volumes
		for _, vol := range volumes {
			if vol.Name == "node-account" {
				vol.VolumeSource.ConfigMap.LocalObjectReference.Name = newConfigmap
			}
		}
		sts.Spec.Template.Spec.Volumes = volumes
		err = client.Update(ctx, sts)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		logger.Error(err, "update account configmap for node failed", "namespace", namespace, "name", name, "configmap", newConfigmap)
		return err
	}
	logger.Info("update account configmap for node successful", "namespace", namespace, "name", name, "configmap", newConfigmap)
	return nil
}
