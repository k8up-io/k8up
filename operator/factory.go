package operator

import (
	"github.com/spotahome/kooper/client/crd"
	"github.com/spotahome/kooper/operator"
	"github.com/spotahome/kooper/operator/controller"
	"k8s.io/client-go/kubernetes"

	baas8scli "git.vshn.net/vshn/baas/client/k8s/clientset/versioned"
	"git.vshn.net/vshn/baas/log"
)

// New returns pod terminator operator.
func New(cfg Config, baasCLI baas8scli.Interface, crdCli crd.Interface, kubeCli kubernetes.Interface, logger log.Logger) (operator.Operator, error) {

	// Create crd.
	bCRD := newBaasCRD(baasCLI, crdCli, kubeCli)

	// Create handler.
	handler := newHandler(kubeCli, baasCLI, logger)

	// Create controller.
	ctrl := controller.NewSequential(cfg.ResyncPeriod, handler, bCRD, nil, logger)

	// Assemble CRD and controller to create the operator.
	return operator.NewOperator(bCRD, ctrl, logger), nil
}
