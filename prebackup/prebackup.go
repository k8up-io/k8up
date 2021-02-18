package prebackup

import (
	"fmt"

	batchv1 "k8s.io/api/batch/v1"

	k8upv1alpha1 "github.com/vshn/k8up/api/v1alpha1"
	"github.com/vshn/k8up/cfg"
	"github.com/vshn/k8up/job"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// PreBackup defines a preBackup.
type PreBackup struct {
	job.Config
}

// NewPrebackup returns a new PreBackup. Although it is not a direct job that is being
// triggered, it takes the same config type as the other job types.
func NewPrebackup(config job.Config) *PreBackup {
	return &PreBackup{
		Config: config,
	}
}

const (
	// ConditionPreBackupJobsReady is True if Deployments for all Container definitions were created and are ready
	ConditionPreBackupJobsReady k8upv1alpha1.ConditionType = "PreBackupJobsReady"
	// ReasonNoPreBackupJobsFound is given when no PreBackupPods are found in the same namespace
	ReasonNoPreBackupJobsFound k8upv1alpha1.ConditionReason = "NoPreBackupJobsFound"
	// ReasonWaiting is given when PreBackupPods are waiting to be started
	ReasonWaiting k8upv1alpha1.ConditionReason = "Waiting"
	// PrebackupJobLabel is set on all jobs that are triggered as a prebackup job
	PrebackupJobLabel = "k8up.syn.tools/preBackupJob"
)

// Start will start the defined pods as deployments.
func (p *PreBackup) Start() error {
	templates, err := p.GetPodTemplates()
	if err != nil {
		p.SetConditionFalseWithMessage(ConditionPreBackupJobsReady, k8upv1alpha1.ReasonRetrievalFailed, "error while retrieving container definitions: %v", err.Error())
		return err
	}

	if len(templates.Items) == 0 {
		p.SetConditionTrueWithMessage(ConditionPreBackupJobsReady, ReasonNoPreBackupJobsFound, "no container definitions found")
		return nil
	}

	err = p.CTX.Err()
	if err != nil {
		p.SetConditionFalseWithMessage(ConditionPreBackupJobsReady, k8upv1alpha1.ReasonRetrievalFailed, err.Error())
		return err
	}

	p.SetConditionUnknownWithMessage(ConditionPreBackupJobsReady, ReasonWaiting, "ready to start %d PreBackupPods", len(templates.Items))
	jobs := p.generateJobs(templates)

	return p.startAll(jobs)
}

// GetPodTemplates returns all pod templates found in the namespace
func (p *PreBackup) GetPodTemplates() (*k8upv1alpha1.PreBackupPodList, error) {
	podList := &k8upv1alpha1.PreBackupPodList{}

	err := p.Client.List(p.CTX, podList, client.InNamespace(p.Obj.GetMetaObject().GetNamespace()))
	if err != nil {
		return nil, fmt.Errorf("could not list pod templates: %w", err)
	}

	return podList, nil
}

func (p *PreBackup) generateJobs(templates *k8upv1alpha1.PreBackupPodList) []batchv1.Job {
	jobs := make([]batchv1.Job, 0)

	for _, template := range templates.Items {

		template.Spec.Pod.PodTemplateSpec.ObjectMeta.Annotations = map[string]string{
			cfg.Config.BackupCommandAnnotation: template.Spec.BackupCommand,
			cfg.Config.FileExtensionAnnotation: template.Spec.FileExtension,
		}

		podLabels := map[string]string{
			"k8up.syn.tools/backupCommandPod": "true",
			PrebackupJobLabel:                 template.Name,
		}

		template.Spec.Pod.PodTemplateSpec.ObjectMeta.Labels = podLabels

		job := batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name:      template.GetName(),
				Namespace: p.Obj.GetMetaObject().GetNamespace(),
			},
			Spec: batchv1.JobSpec{
				Template: template.Spec.Pod.PodTemplateSpec,
			},
		}

		err := controllerutil.SetOwnerReference(p.Config.Obj.GetMetaObject(), &job, p.Scheme)
		if err != nil {
			p.Config.Log.Error(err, "cannot set the owner reference", "name", p.Config.Obj.GetMetaObject().GetName(), "namespace", p.Config.Obj.GetMetaObject().GetNamespace())
		}

		jobs = append(jobs, job)
	}

	return jobs
}

func (p *PreBackup) startAll(jobs []batchv1.Job) error {
	clientset, err := p.getClientset()
	if err != nil {
		return fmt.Errorf("could not create pre backup pods: %w", err)
	}

	for _, job := range jobs {
		err = p.startOne(job, clientset)
		if err != nil {
			return err
		}
	}

	p.SetConditionTrue(ConditionPreBackupJobsReady, k8upv1alpha1.ReasonSucceeded)
	return nil
}

func (p *PreBackup) startOne(job batchv1.Job, clientset *kubernetes.Clientset) error {
	err := p.CTX.Err()
	if err != nil {
		p.SetConditionFalseWithMessage(ConditionPreBackupJobsReady, k8upv1alpha1.ReasonRetrievalFailed, "error before starting pre backup pod: %v", err.Error())
		return err
	}

	name := job.GetName()
	namespace := job.GetNamespace()
	p.Log.Info("starting pre backup job", "namespace", namespace, "name", name)

	err = p.Client.Create(p.CTX, &job)
	deploymentExists := errors.IsAlreadyExists(err)
	if err != nil && !deploymentExists {
		err := fmt.Errorf("error creating pre backup job '%v/%v': %w", namespace, name, err)
		p.SetConditionFalseWithMessage(ConditionPreBackupJobsReady, k8upv1alpha1.ReasonCreationFailed, err.Error())
		return err
	}

	return nil
}

func getKubeConfig() (*rest.Config, error) {
	kubeconfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		&clientcmd.ConfigOverrides{},
	)
	return kubeconfig.ClientConfig()
}

func (p *PreBackup) getClientset() (*kubernetes.Clientset, error) {
	config, err := getKubeConfig()
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("could not create clientset: %w", err)
	}

	return clientset, nil
}

func (p *PreBackup) getLastJobCondition(job *batchv1.Job) *batchv1.JobCondition {
	conditions := job.Status.Conditions

	if len(conditions) > 0 {
		return &conditions[len(conditions)-1]
	}
	return nil
}

// Stop will remove the deployments.
func (p *PreBackup) Stop() {
	templates, err := p.GetPodTemplates()
	if err != nil {
		p.Log.Error(err, "could not fetch pod templates", "name", p.Obj.GetMetaObject().GetName(), "namespace", p.Obj.GetMetaObject().GetNamespace())
		p.SetConditionFalseWithMessage(ConditionPreBackupJobsReady, k8upv1alpha1.ReasonRetrievalFailed, "could not fetch pod templates: %v", err)
		return
	}

	if len(templates.Items) == 0 {
		p.SetConditionTrue(ConditionPreBackupJobsReady, ReasonNoPreBackupJobsFound)
		return
	}

	option := metav1.DeletePropagationForeground

	jobs := p.generateJobs(templates)
	for _, job := range jobs {
		// Avoid exportloopref
		job := job

		p.Log.Info("removing PreBackupPod deployment", "name", job.Name, "namespace", job.Namespace)
		err := p.Client.Delete(p.CTX, &job, &client.DeleteOptions{
			PropagationPolicy: &option,
		})
		if err != nil && !errors.IsNotFound(err) {
			p.Log.Error(err, "could not delete job", "name", p.Obj.GetMetaObject().GetName(), "namespace", p.Obj.GetMetaObject().GetNamespace())
			p.SetConditionFalseWithMessage(ConditionPreBackupJobsReady, k8upv1alpha1.ReasonDeletionFailed, "could not delete job: %v", err.Error())
		}
	}

	p.SetConditionTrue(ConditionPreBackupJobsReady, k8upv1alpha1.ReasonSucceeded)
}
