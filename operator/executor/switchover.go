package executor

import (
	"context"
	stderrors "errors"
	k8upv1 "github.com/k8up-io/k8up/v2/api/v1cita"
	"github.com/k8up-io/k8up/v2/operator/cfg"
	"github.com/k8up-io/k8up/v2/operator/job"
	"github.com/k8up-io/k8up/v2/operator/observer"
	batchv1 "k8s.io/api/batch/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type SwitchoverExecutor struct {
	generic
	switchover *k8upv1.Switchover
}

// NewSwitchoverExecutor returns a new SwitchoverExecutor.
func NewSwitchoverExecutor(config job.Config) *SwitchoverExecutor {
	return &SwitchoverExecutor{
		generic: generic{config},
	}
}

// GetConcurrencyLimit returns the concurrent jobs limit
func (s *SwitchoverExecutor) GetConcurrencyLimit() int {
	return cfg.Config.GlobalConcurrentRestoreJobsLimit
}

// Execute triggers the actual batch.job creation on the cluster.
// It will also register a callback function on the observer so the PreBackupPods can be removed after the backup has finished.
func (s *SwitchoverExecutor) Execute() error {
	switchoverObject, ok := s.Obj.(*k8upv1.Switchover)
	if !ok {
		return stderrors.New("object is not a block height fallback")
	}
	s.switchover = switchoverObject

	if s.Obj.GetStatus().Started {
		s.RegisterJobSucceededConditionCallback() // ensure that completed jobs can complete backups between operator restarts.
		return nil
	}

	err := s.createServiceAccountAndBinding()
	if err != nil {
		return err
	}

	genericJob, err := job.GenerateGenericJob(s.Obj, s.Config)
	if err != nil {
		return err
	}

	return s.startSwitchover(genericJob)
}

func (s *SwitchoverExecutor) createServiceAccountAndBinding() error {
	role, sa, binding := newServiceAccountDefinition(s.switchover.Namespace)
	for _, obj := range []client.Object{&role, &sa, &binding} {
		if err := s.CreateObjectIfNotExisting(obj); err != nil {
			return err
		}
	}
	return nil
}

func (s *SwitchoverExecutor) startSwitchover(job *batchv1.Job) error {
	// stop source node
	sourceNode := NewCITANode(s.CTX, s.Client, s.switchover.Namespace, s.switchover.Spec.SourceNode)
	sourceNodeStopped, err := sourceNode.Stop()
	// stop dest node
	destNode := NewCITANode(s.CTX, s.Client, s.switchover.Namespace, s.switchover.Spec.DestNode)
	destNodeStopped, err := destNode.Stop()
	if err != nil {
		return err
	}
	if !sourceNodeStopped || !destNodeStopped {
		return nil
	}

	s.registerCITANodeCallback()
	s.RegisterJobSucceededConditionCallback()

	//volumes := b.prepareVolumes()
	//
	//job.Spec.Template.Spec.Volumes = volumes
	//job.Spec.Template.Spec.ServiceAccountName = "cita-node-job"
	job.Spec.Template.Spec.ServiceAccountName = cfg.Config.ServiceAccount
	//job.Spec.Template.Spec.Containers[0].VolumeMounts = b.newVolumeMounts()

	args := s.args()
	job.Spec.Template.Spec.Containers[0].Args = args
	job.Spec.Template.Spec.Containers[0].Command = []string{"/usr/local/bin/k8up", "switchover"}

	if err = s.CreateObjectIfNotExisting(job); err == nil {
		s.SetStarted("the job '%v/%v' was created", job.Namespace, job.Name)
	}
	return err
}

func (s *SwitchoverExecutor) args() []string {
	return []string{"--namespace", s.switchover.Namespace,
		"--source-node", s.switchover.Spec.SourceNode,
		"--dest-node", s.switchover.Spec.DestNode}
}

func (s *SwitchoverExecutor) registerCITANodeCallback() {
	name := s.GetJobNamespacedName()
	observer.GetObserver().RegisterCallback(name.String(), func(_ observer.ObservableJob) {
		//b.StopPreBackupDeployments()
		//b.cleanupOldBackups(name)
		s.startCITANode(s.CTX, s.Client, s.switchover.Namespace, s.switchover.Spec.DestNode)
		s.startCITANode(s.CTX, s.Client, s.switchover.Namespace, s.switchover.Spec.SourceNode)
	})
}

func (s *SwitchoverExecutor) startCITANode(ctx context.Context, client client.Client, namespace, name string) {
	NewCITANode(ctx, client, namespace, name).Start()
}
