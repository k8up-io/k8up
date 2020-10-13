package executor

import "github.com/vshn/k8up/job"

type BackupExecutor struct {
	generic
	name string
	//TODO: list of PVCs
}

func NewBackupExecutor(config job.Config, name string) *BackupExecutor {
	return &BackupExecutor{
		generic: generic{config},
		name:    name,
	}
}

func (b *BackupExecutor) Execute() error {
	job, err := job.GetGenericJob(b.Obj, b.Scheme)
	if err != nil {
		return err
	}
	err = b.Client.Create(b.CTX, job)
	return err
}
