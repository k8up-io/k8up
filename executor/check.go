package executor

import "github.com/vshn/k8up/job"

type CheckExecutor struct {
	generic
}

func NewCheckExecutor(config job.Config) *CheckExecutor {
	return &CheckExecutor{
		generic: generic{config},
	}
}

func (c *CheckExecutor) Execute() error {
	jobObj, err := job.GetGenericJob(c.Obj, c.Config)
	jobObj.GetLabels()[job.K8upExclusive] = "true"
	if err != nil {
		return err
	}
	return c.Client.Create(c.CTX, jobObj)
}

func (*CheckExecutor) Exclusive() bool {
	return true
}
