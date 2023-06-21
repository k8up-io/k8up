package gotest

import (
	"os"
	"testing"

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	"github.com/stretchr/testify/require"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

func getClient(t *testing.T) client.Client {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(k8upv1.AddToScheme(scheme))
	c, err := client.New(config.GetConfigOrDie(), client.Options{
		Scheme: scheme,
	})
	require.NoError(t, err)

	return c
}
