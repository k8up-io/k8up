package operator

import (
	"github.com/spotahome/kooper/client/crd"
	"github.com/spotahome/kooper/operator"
	"github.com/spotahome/kooper/operator/controller"
	"github.com/spotahome/kooper/operator/resource"
	"github.com/spotahome/kooper/operator/retrieve"
	"k8s.io/client-go/kubernetes"

	baas8scli "git.vshn.net/vshn/baas/client/k8s/clientset/versioned"
	"git.vshn.net/vshn/baas/log"
	"git.vshn.net/vshn/baas/service/backup"
	"git.vshn.net/vshn/baas/service/restore"
)

type options struct {
	cfg     Config
	baasCLI baas8scli.Interface
	crdCli  crd.Interface
	kubeCli kubernetes.Interface
	logger  log.Logger
}

// New returns pod terminator operator.
func New(cfg Config, baasCLI baas8scli.Interface, crdCli crd.Interface, kubeCli kubernetes.Interface, logger log.Logger) (operator.Operator, error) {

	options := options{
		cfg:     cfg,
		baasCLI: baasCLI,
		crdCli:  crdCli,
		kubeCli: kubeCli,
		logger:  logger,
	}

	operators := create(options)

	return operators, nil
}

func create(options options) operator.Operator {
	bCRD := newBackupCRD(options.baasCLI, options.crdCli, options.kubeCli)
	backup := backup.NewBackup(options.kubeCli, options.baasCLI, options.logger)
	backupHandler := newHandler(options.kubeCli, options.baasCLI, options.logger, backup)

	rCRD := newRestoreCRD(options.baasCLI, options.crdCli, options.kubeCli)
	restore := restore.NewRestore(options.kubeCli, options.baasCLI, options.logger)
	restoreHandler := newHandler(options.kubeCli, options.baasCLI, options.logger, restore)

	CRDs := []resource.CRD{
		bCRD,
		rCRD,
	}

	cfg := []controller.Config{
		{
			ProcessingJobRetries: 5,
			ResyncInterval:       options.cfg.ResyncPeriod,
			ConcurrentWorkers:    1,
			Name:                 "backup",
		},
		{
			ProcessingJobRetries: 5,
			ResyncInterval:       options.cfg.ResyncPeriod,
			ConcurrentWorkers:    1,
			Name:                 "restore",
		},
	}

	retr := []retrieve.Resource{
		{
			Object:        bCRD.GetObject(),
			ListerWatcher: bCRD.GetListerWatcher(),
		},
		{
			Object:        rCRD.GetObject(),
			ListerWatcher: rCRD.GetListerWatcher(),
		},
	}

	ctrls := []controller.Controller{}
	backupCtrl := controller.New(&cfg[0], backupHandler, &retr[0], nil, nil, nil, options.logger)
	restoreCtrl := controller.New(&cfg[1], restoreHandler, &retr[1], nil, nil, nil, options.logger)
	ctrls = append(ctrls, backupCtrl, restoreCtrl)
	return operator.NewMultiOperator(CRDs, ctrls, options.logger)
}
