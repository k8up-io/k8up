package executor

import "github.com/vshn/k8up/job"

// CheckExecutor will execute the batch.job for checks.
type CheckExecutor struct {
	generic
}

// NewCheckExecutor will return a new executor for check jobs.
func NewCheckExecutor(config job.Config) *CheckExecutor {
	return &CheckExecutor{
		generic: generic{config},
	}
}

// Execute creates the actualy batch.job on the k8s api.
func (c *CheckExecutor) Execute() error {
	jobObj, err := job.GetGenericJob(c.Obj, c.Config)
	jobObj.GetLabels()[job.K8upExclusive] = "true"
	if err != nil {
		return err
	}
	return c.Client.Create(c.CTX, jobObj)
}

// Exclusive should return true for jobs that can't run while other jobs run.
func (*CheckExecutor) Exclusive() bool {
	return true
}
