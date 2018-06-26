package fake

import (
	v1alpha1 "git.vshn.net/vshn/baas/client/k8s/clientset/versioned/typed/backup/v1alpha1"
	rest "k8s.io/client-go/rest"
	testing "k8s.io/client-go/testing"
)

type FakeAppuioV1alpha1 struct {
	*testing.Fake
}

func (c *FakeAppuioV1alpha1) Backups(namespace string) v1alpha1.BackupInterface {
	return &FakeBackups{c, namespace}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeAppuioV1alpha1) RESTClient() rest.Interface {
	var ret *rest.RESTClient
	return ret
}
