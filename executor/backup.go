package executor

import "github.com/vshn/k8up/job"

type BackupExecutor struct {
	generic
	//TODO: list of PVCs
}

func NewBackupExecutor(config job.Config) *BackupExecutor {
	return &BackupExecutor{
		generic: generic{config},
	}
}

func (b *BackupExecutor) Execute() error {
	job, err := job.GetGenericJob(b.Obj, b.Config)
	if err != nil {
		return err
	}
	return b.Client.Create(b.CTX, job)
}
