package executor

import (
	stderrors "errors"

	"github.com/vshn/k8up/cfg"

	k8upv1alpha1 "github.com/vshn/k8up/api/v1alpha1"
	"github.com/vshn/k8up/job"
	corev1 "k8s.io/api/core/v1"
)

// CheckExecutor will execute the batch.job for checks.
type CheckExecutor struct {
	generic
	check *k8upv1alpha1.Check
}

// NewCheckExecutor will return a new executor for check jobs.
func NewCheckExecutor(config job.Config) *CheckExecutor {
	return &CheckExecutor{
		generic: generic{config},
	}
}

// GetConcurrencyLimit returns the concurrent jobs limit
func (c *CheckExecutor) GetConcurrencyLimit() int {
	return cfg.Config.GlobalConcurrentCheckJobsLimit
}

// Execute creates the actual batch.job on the k8s api.
func (c *CheckExecutor) Execute() error {
	checkObject, ok := c.Obj.(*k8upv1alpha1.Check)
	if !ok {
		return stderrors.New("object is not a check")
	}

	c.check = checkObject

	if c.Obj.GetStatus().Started {
		return nil
	}

	jobObj, err := job.GetGenericJob(c.Obj, c.Config)
	jobObj.GetLabels()[job.K8upExclusive] = "true"
	if err != nil {
		return err
	}

	jobObj.Spec.Template.Spec.Containers[0].Env = c.setupEnvVars()
	jobObj.Spec.Template.Spec.Containers[0].Args = []string{"-check"}
	return c.Client.Create(c.CTX, jobObj)
}

// Exclusive should return true for jobs that can't run while other jobs run.
func (*CheckExecutor) Exclusive() bool {
	return true
}

func (c *CheckExecutor) setupEnvVars() []corev1.EnvVar {
	vars := NewEnvVarConverter()

	if c.check != nil {
		if c.check.Spec.Backend != nil {
			for key, value := range c.check.Spec.Backend.GetCredentialEnv() {
				vars.SetEnvVarSource(key, value)
			}
			vars.SetString(cfg.ResticRepositoryEnvName, c.check.Spec.Backend.String())
		}
	}

	vars.SetString("PROM_URL", cfg.Config.PromURL)

	err := vars.Merge(DefaultEnv(c.Obj.GetMetaObject().GetNamespace()))
	if err != nil {
		c.Log.Error(err, "error while merging the environment variables", "name", c.Obj.GetMetaObject().GetName(), "namespace", c.Obj.GetMetaObject().GetNamespace())
	}

	return vars.Convert()
}
