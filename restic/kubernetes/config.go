package kubernetes

import (
	"fmt"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/k8up-io/k8up/restic/cfg"
)

func getClientConfig() (*rest.Config, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		err1 := err
		config, err = clientcmd.BuildConfigFromFlags("", cfg.Config.KubeConfig)
		if err != nil {
			err = fmt.Errorf("InClusterConfig as well as BuildConfigFromFlags Failed. Error in InClusterConfig: %+v\nError in BuildConfigFromFlags: %+v", err1, err)
			return nil, err
		}
	}

	return config, nil
}

func newk8sClient() (*kubernetes.Clientset, error) {
	config, err := getClientConfig()
	if err != nil {
		return nil, fmt.Errorf("can't load k8s config: %v", err)
	}
	k8sclient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("can't create k8s client: %v", err)
	}

	return k8sclient, nil
}
