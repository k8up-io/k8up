package prebackup

import (
	"fmt"

	"k8s.io/apimachinery/pkg/types"

	k8upv1alpha1 "github.com/vshn/k8up/api/v1alpha1"
	"github.com/vshn/k8up/cfg"
	"github.com/vshn/k8up/job"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/utils/pointer"
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
	// ConditionPreBackupPodsReady is True if Deployments for all Container definitions were created and are ready
	ConditionPreBackupPodsReady k8upv1alpha1.ConditionType = "PreBackupPodsReady"

	// ReasonNoPreBackupPodsFound is given when no PreBackupPods are found in the same namespace
	ReasonNoPreBackupPodsFound k8upv1alpha1.ConditionReason = "NoPreBackupPodsFound"
	// ReasonWaiting is given when PreBackupPods are waiting to be started
	ReasonWaiting k8upv1alpha1.ConditionReason = "Waiting"
)

// Start will start the defined pods as deployments.
func (p *PreBackup) Start() error {
	templates, err := p.getPodTemplates()
	if err != nil {
		p.SetConditionFalseWithMessage(ConditionPreBackupPodsReady, k8upv1alpha1.ReasonRetrievalFailed, "error while retrieving container definitions: %v", err.Error())
		return err
	}

	if len(templates.Items) == 0 {
		p.SetConditionTrueWithMessage(ConditionPreBackupPodsReady, ReasonNoPreBackupPodsFound, "no container definitions found")
		return nil
	}

	err = p.CTX.Err()
	if err != nil {
		p.SetConditionFalseWithMessage(ConditionPreBackupPodsReady, k8upv1alpha1.ReasonRetrievalFailed, err.Error())
		return err
	}

	p.SetConditionUnknownWithMessage(ConditionPreBackupPodsReady, ReasonWaiting, "ready to start %d PreBackupPods", len(templates.Items))
	deployments := p.generateDeployments(templates)

	return p.startAllAndWaitForReady(deployments)
}

func (p *PreBackup) getPodTemplates() (*k8upv1alpha1.PreBackupPodList, error) {
	podList := &k8upv1alpha1.PreBackupPodList{}

	err := p.Client.List(p.CTX, podList, client.InNamespace(p.Obj.GetMetaObject().GetNamespace()))
	if err != nil {
		return nil, fmt.Errorf("could not list pod templates: %w", err)
	}

	return podList, nil
}

func (p *PreBackup) generateDeployments(templates *k8upv1alpha1.PreBackupPodList) []appsv1.Deployment {
	deployments := make([]appsv1.Deployment, 0)

	for _, template := range templates.Items {

		template.Spec.Pod.PodTemplateSpec.ObjectMeta.Annotations = map[string]string{
			cfg.Config.BackupCommandAnnotation: template.Spec.BackupCommand,
			cfg.Config.FileExtensionAnnotation: template.Spec.FileExtension,
		}

		podLabels := map[string]string{
			"k8up.syn.tools/backupCommandPod": "true",
			"k8up.syn.tools/preBackupPod":     template.Name,
		}

		template.Spec.Pod.PodTemplateSpec.ObjectMeta.Labels = podLabels

		deployment := appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      template.GetName(),
				Namespace: p.Obj.GetMetaObject().GetNamespace(),
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: pointer.Int32Ptr(1),
				Template: template.Spec.Pod.PodTemplateSpec,
				Selector: &metav1.LabelSelector{
					MatchLabels: podLabels,
				},
			},
		}

		err := controllerutil.SetOwnerReference(p.Config.Obj.GetMetaObject(), &deployment, p.Scheme)
		if err != nil {
			p.Config.Log.Error(err, "cannot set the owner reference", "name", p.Config.Obj.GetMetaObject().GetName(), "namespace", p.Config.Obj.GetMetaObject().GetNamespace())
		}

		deployments = append(deployments, deployment)
	}

	return deployments
}

func (p *PreBackup) startAllAndWaitForReady(deployments []appsv1.Deployment) error {
	clientset, err := p.getClientset()
	if err != nil {
		return fmt.Errorf("could not create pre backup pods: %w", err)
	}

	for _, deployment := range deployments {
		err = p.startOneAndWaitForReady(deployment, clientset)
		if err != nil {
			return err
		}
	}

	p.SetConditionTrue(ConditionPreBackupPodsReady, k8upv1alpha1.ReasonReady)
	return nil
}

func (p *PreBackup) startOneAndWaitForReady(deployment appsv1.Deployment, clientset *kubernetes.Clientset) error {
	err := p.CTX.Err()
	if err != nil {
		p.SetConditionFalseWithMessage(ConditionPreBackupPodsReady, k8upv1alpha1.ReasonRetrievalFailed, "error before starting pre backup pod: %v", err.Error())
		return err
	}

	name := deployment.GetName()
	namespace := deployment.GetNamespace()
	p.Log.Info("starting pre backup pod", "namespace", namespace, "name", name)

	err = p.Client.Create(p.CTX, &deployment)
	deploymentExists := errors.IsAlreadyExists(err)
	if err != nil && !deploymentExists {
		err := fmt.Errorf("error creating pre backup pod '%v/%v': %w", namespace, name, err)
		p.SetConditionFalseWithMessage(ConditionPreBackupPodsReady, k8upv1alpha1.ReasonCreationFailed, err.Error())
		return err
	}

	if deploymentExists {
		err := p.Client.Get(p.CTX, types.NamespacedName{Name: deployment.Name, Namespace: deployment.Namespace}, &deployment)
		if err != nil {
			err := fmt.Errorf("error getting pre backup pod '%v/%v': %w", namespace, name, err)
			p.SetConditionFalseWithMessage(ConditionPreBackupPodsReady, k8upv1alpha1.ReasonRetrievalFailed, err.Error())
			return err
		}

		ready, err := p.isDeploymentReady(&deployment)
		if err != nil {
			err := fmt.Errorf("error checking the readyness of deployment '%s/%s': %s", deployment.Namespace, deployment.Name, err)
			p.SetConditionFalseWithMessage(ConditionPreBackupPodsReady, k8upv1alpha1.ReasonRetrievalFailed, err.Error())
			return err
		}
		if ready {
			p.Log.V(2).Info("pre backup pod already in ready state", "deployment", fmt.Sprintf("%s/%s", deployment.Namespace, deployment.Name), "event type")
			p.SetConditionTrue(ConditionPreBackupPodsReady, k8upv1alpha1.ReasonReady)
			return nil
		}
	}

	watcher, err := clientset.AppsV1().
		Deployments(deployment.GetNamespace()).
		Watch(p.CTX, metav1.SingleObject(deployment.ObjectMeta))
	if err != nil {
		err := fmt.Errorf("could not create watcher for '%v/%v': %w", namespace, name, err)
		p.SetConditionFalseWithMessage(ConditionPreBackupPodsReady, k8upv1alpha1.ReasonCreationFailed, err.Error())
		return err
	}

	err = p.waitForReady(watcher)
	if err != nil {
		err := fmt.Errorf("error during deployment watch of '%v/%v': %w", namespace, name, err)
		p.SetConditionFalseWithMessage(ConditionPreBackupPodsReady, k8upv1alpha1.ReasonFailed, err.Error())
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

func (p *PreBackup) waitForReady(watcher watch.Interface) error {
	defer watcher.Stop()

	p.Log.V(2).Info("waiting on watcher ", "deployment", fmt.Sprintf("%s/%s", p.Obj.GetMetaObject().GetNamespace(), p.Obj.GetMetaObject().GetName()))
	for {
		select {
		case event := <-watcher.ResultChan():
			endWatch, err := p.handleEvent(event)
			if err != nil {
				return err
			}
			if endWatch {
				return nil
			}
		case <-p.CTX.Done():
			p.Log.Error(p.CTX.Err(), "unexpected end during deployment watch ", "deployment", fmt.Sprintf("%s/%s", p.Obj.GetMetaObject().GetNamespace(), p.Obj.GetMetaObject().GetName()))
			return p.CTX.Err()
		}
	}
}

func (p *PreBackup) handleEvent(event watch.Event) (bool, error) {
	deployment, ok := event.Object.(*appsv1.Deployment)
	if !ok {
		p.Log.V(1).Info("unexpected event during deployment watch ", "event source", event.Object, "event type", event.Type)
		return false, nil
	}

	p.Log.V(1).Info("new event during deployment watch ", "deployment", fmt.Sprintf("%s/%s", deployment.Namespace, deployment.Name), "event type", event.Type)

	switch event.Type {
	case watch.Modified:
		ready, err := p.isDeploymentReady(deployment)
		if err != nil || ready {
			return true, err
		}

		p.Log.Info("waiting for command pod to get ready", "deployment", fmt.Sprintf("%s/%s", deployment.Namespace, deployment.Name))

	case watch.Error:
		last := p.getLastDeploymentCondition(deployment)

		var err error
		if last != nil {
			err = fmt.Errorf("there was an error while starting pre backup pod '%v/%v': %v", deployment.Namespace, deployment.Name, last.Message)
		} else {
			err = fmt.Errorf("there was an unknown error while starting pre backup pod '%v/%v'", deployment.Namespace, deployment.Name)
		}
		return true, err

	case watch.Added:
	case watch.Bookmark:
	case watch.Deleted:
		p.Log.V(1).Info("ignoring event", "deployment", fmt.Sprintf("%s/%s", deployment.Namespace, deployment.Name), "event type", event.Type)

	default:
		p.Log.Info("unexpected event during deployment watch ", "deployment", fmt.Sprintf("%s/%s", deployment.Namespace, deployment.Name), "event type", event.Type)
	}
	return false, nil
}

func (p *PreBackup) isDeploymentReady(deployment *appsv1.Deployment) (bool, error) {
	last := p.getLastDeploymentCondition(deployment)

	if last != nil && isDeadlineExceeded(last) {
		return true, fmt.Errorf("error starting pre backup pod %v: %v", deployment.GetName(), last.Message)
	}

	if hasAvailableReplica(deployment) {
		return true, nil
	}

	return false, nil
}

func isDeadlineExceeded(last *appsv1.DeploymentCondition) bool {
	// if the deadline can't be respected https://kubernetes.io/docs/concepts/workloads/controllers/deployment/#progress-deadline-seconds
	return last.Type == "Progressing" && last.Status == "False" && last.Reason == "ProgressDeadlineExceeded"
}

func hasAvailableReplica(deployment *appsv1.Deployment) bool {
	return deployment.Status.AvailableReplicas > 0
}

func (p *PreBackup) getLastDeploymentCondition(deployment *appsv1.Deployment) *appsv1.DeploymentCondition {
	conditions := deployment.Status.Conditions

	if len(conditions) > 0 {
		return &conditions[len(conditions)-1]
	}
	return nil
}

// Stop will remove the deployments.
func (p *PreBackup) Stop() {
	templates, err := p.getPodTemplates()
	if err != nil {
		p.Log.Error(err, "could not fetch pod templates", "name", p.Obj.GetMetaObject().GetName(), "namespace", p.Obj.GetMetaObject().GetNamespace())
		p.SetConditionFalseWithMessage(ConditionPreBackupPodsReady, k8upv1alpha1.ReasonRetrievalFailed, "could not fetch pod templates: %v", err)
		return
	}

	if len(templates.Items) == 0 {
		p.SetConditionTrue(ConditionPreBackupPodsReady, ReasonNoPreBackupPodsFound)
		return
	}

	option := metav1.DeletePropagationForeground

	deployments := p.generateDeployments(templates)
	for _, deployment := range deployments {
		// Avoid exportloopref
		deployment := deployment

		p.Log.Info("removing PreBackupPod deployment", "name", deployment.Name, "namespace", deployment.Namespace)
		err := p.Client.Delete(p.CTX, &deployment, &client.DeleteOptions{
			PropagationPolicy: &option,
		})
		if err != nil && !errors.IsNotFound(err) {
			p.Log.Error(err, "could not delete deployment", "name", p.Obj.GetMetaObject().GetName(), "namespace", p.Obj.GetMetaObject().GetNamespace())
			p.SetConditionFalseWithMessage(ConditionPreBackupPodsReady, k8upv1alpha1.ReasonDeletionFailed, "could not delete deployment: %v", err.Error())
		}
	}

	p.SetConditionTrue(ConditionPreBackupPodsReady, k8upv1alpha1.ReasonReady)
}
