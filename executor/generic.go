// executor contains the logic that is needed to apply the actual k8s job objects to a cluster.
// each job type should implement its own executor that handles its own job creation.
// There are various methods that provide default env vars and batch.job scaffolding.

package executor

import (
	"fmt"
	"sort"

	"k8s.io/apimachinery/pkg/types"

	k8upv1alpha1 "github.com/vshn/k8up/api/v1alpha1"
	"github.com/vshn/k8up/cfg"
	"github.com/vshn/k8up/job"
	"github.com/vshn/k8up/observer"
	"github.com/vshn/k8up/queue"

	"github.com/go-logr/logr"
	"github.com/imdario/mergo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type generic struct {
	job.Config
}

// EnvVarEntry holds one entry for the EnvVarConverter
type envVarEntry struct {
	stringEnv    *string
	envVarSource *corev1.EnvVarSource
}

// EnvVarConverter can convert the given map to a []corev1.EnvVar. It also provides
// a function to merge another EnvVarConverter instance into itself.
// The merge will overwrite all zero-valued or nor declared entries.
type EnvVarConverter struct {
	Vars map[string]envVarEntry
}

// NewEnvVarConverter returns a new
func NewEnvVarConverter() EnvVarConverter {
	return EnvVarConverter{
		Vars: make(map[string]envVarEntry),
	}
}

// SetString adds a string key and value pair to the environment.
func (e *EnvVarConverter) SetString(key, value string) {
	e.setEntry(key, envVarEntry{stringEnv: &value})
}

// SetEnvVarSource add an EnvVarSource to the environment with the given key.
func (e *EnvVarConverter) SetEnvVarSource(key string, value *corev1.EnvVarSource) {
	e.setEntry(key, envVarEntry{envVarSource: value})
}

func (e *EnvVarConverter) setEntry(key string, value envVarEntry) {
	e.Vars[key] = value
}

// Convert returns a ready-to-use []corev1.EnvVar where all the key value
// pairs have been added according to their type. If string and envVarSource
// are set the string will have precedence.
func (e *EnvVarConverter) Convert() []corev1.EnvVar {
	vars := make([]corev1.EnvVar, 0)
	for key, value := range e.Vars {
		envVar := corev1.EnvVar{
			Name: key,
		}
		if value.envVarSource != nil {
			envVar.ValueFrom = value.envVarSource
		} else if value.stringEnv != nil {
			envVar.Value = *value.stringEnv
		}
		vars = append(vars, envVar)
	}
	return vars
}

// Merge will merge the source into the instance. If there's no entry in the instance
// that exists in the source, the source entry will be added. If there's a zero-valued
// entry, it will also be overwritten.
func (e *EnvVarConverter) Merge(source EnvVarConverter) error {
	return mergo.Merge(&e.Vars, source.Vars)
}

type jobObjectList []job.Object

func (jo jobObjectList) Len() int      { return len(jo) }
func (jo jobObjectList) Swap(i, j int) { jo[i], jo[j] = jo[j], jo[i] }

func (jo jobObjectList) Less(i, j int) bool {

	if jo[i].GetMetaObject().GetCreationTimestamp().Time.Equal(jo[j].GetMetaObject().GetCreationTimestamp().Time) {
		return jo[i].GetMetaObject().GetName() < jo[j].GetMetaObject().GetName()
	}

	return jo[i].GetMetaObject().GetCreationTimestamp().Time.Before(jo[j].GetMetaObject().GetCreationTimestamp().Time)
}
func (g *generic) Logger() logr.Logger {
	return g.Log
}

func (*generic) Exclusive() bool {
	return false
}

func (g *generic) GetRepository() string {
	return g.Repository
}

func (g *generic) GetJobNamespace() string {
	return g.Obj.GetMetaObject().GetNamespace()
}

func (g *generic) GetJobNamespacedName() types.NamespacedName {
	return types.NamespacedName{Namespace: g.Obj.GetMetaObject().GetNamespace(), Name: g.Obj.GetMetaObject().GetName()}
}

func (g *generic) GetJobType() k8upv1alpha1.JobType {
	return g.Obj.GetType()
}

// RegisterJobSucceededConditionCallback registers an observer on the job which updates ConditionJobSucceeded when
// the job succeeds or fails, respectively.
func (g *generic) RegisterJobSucceededConditionCallback() {
	name := g.GetJobNamespacedName()
	observer.GetObserver().RegisterCallback(name.String(), func(event observer.ObservableJob) {
		switch event.Event {
		case observer.Suceeded:
			g.SetFinished(event.Job.Namespace, event.Job.Name)
			g.SetConditionTrueWithMessage(k8upv1alpha1.ConditionCompleted,
				k8upv1alpha1.ReasonSucceeded,
				"the job '%v/%v' ended successfully",
				event.Job.Namespace, event.Job.Name)
		case observer.Failed:
			g.SetFinished(event.Job.Namespace, event.Job.Name)
			g.SetConditionFalseWithMessage(k8upv1alpha1.ConditionCompleted,
				k8upv1alpha1.ReasonFailed,
				"the job '%v/%v' failed, please check its log for details",
				event.Job.Namespace, event.Job.Name)
		}
	})
}

// CreateObjectIfNotExisting tries to create the given object, but ignores AlreadyExistsError.
// If it fails for any other reason, the Ready condition is set to False with the error message and reason.
func (g *generic) CreateObjectIfNotExisting(obj client.Object) error {
	err := g.Client.Create(g.CTX, obj)
	if err != nil && !errors.IsAlreadyExists(err) {
		g.SetConditionFalseWithMessage(k8upv1alpha1.ConditionReady,
			k8upv1alpha1.ReasonCreationFailed,
			"unable to create %v '%v/%v': %v",
			obj.GetObjectKind().GroupVersionKind().Kind,
			obj.GetNamespace(), obj.GetName(), err.Error())
		return err
	}
	return nil
}

// listOldResources retrieves a list of the given resource type in the given namespace and fills the Item property
// of objList. On errors, the error is being logged and the Scrubbed condition set to False with reason RetrievalFailed.
func (g *generic) listOldResources(namespace string, objList client.ObjectList) error {
	err := g.Client.List(g.CTX, objList, &client.ListOptions{
		Namespace: namespace,
	})
	if err != nil {
		g.Log.Error(err, "could not list objects to cleanup old resources", "Namespace", namespace, "kind", objList.GetObjectKind().GroupVersionKind().Kind)
		g.SetConditionFalseWithMessage(k8upv1alpha1.ConditionScrubbed, k8upv1alpha1.ReasonRetrievalFailed, "could not list objects to cleanup old resources: %v", err)
		return err
	}
	return nil
}

// NewExecutor will return the right Executor for the given job object.
func NewExecutor(config job.Config) queue.Executor {
	switch config.Obj.GetType() {
	case k8upv1alpha1.BackupType:
		return NewBackupExecutor(config)
	case k8upv1alpha1.CheckType:
		return NewCheckExecutor(config)
	case k8upv1alpha1.ArchiveType:
		return NewArchiveExecutor(config)
	case k8upv1alpha1.PruneType:
		return NewPruneExecutor(config)
	case k8upv1alpha1.RestoreType:
		return NewRestoreExecutor(config)
	}
	return nil
}

// DefaultEnv returns an environment that contains the default values for the fields.
func DefaultEnv(namespace string) EnvVarConverter {
	defaults := NewEnvVarConverter()

	defaults.SetString("STATS_URL", cfg.Config.GlobalStatsURL)
	defaults.SetString(cfg.ResticRepositoryEnvName, fmt.Sprintf("s3:%s/%s", cfg.Config.GlobalS3Endpoint, cfg.Config.GlobalS3Bucket))
	defaults.SetString(cfg.ResticPasswordEnvName, cfg.Config.GlobalRepoPassword)
	defaults.SetString(cfg.AwsAccessKeyIDEnvName, cfg.Config.GlobalAccessKey)
	defaults.SetString(cfg.AwsSecretAccessKeyEnvName, cfg.Config.GlobalSecretAccessKey)
	defaults.SetString("HOSTNAME", namespace)

	return defaults
}

// cleanOldObjects iterates sorted over the given list and deletes them until maxObjects remain. If a deletion fails,
// the Scrubbed condition will be set to false with the error message, which will also be logged. The func also aborts early.
// If all succeed, the condition is set to "true".
func cleanOldObjects(jobObjects jobObjectList, maxObjects int, config job.Config) {
	numToDelete := len(jobObjects) - maxObjects

	if numToDelete <= 0 {
		return
	}

	sort.Sort(jobObjects)
	for i := 0; i < numToDelete; i++ {
		name := jobObjects[i].GetMetaObject().GetName()
		ns := jobObjects[i].GetMetaObject().GetNamespace()
		config.Log.Info("cleaning old job", "namespace", ns, "name", name)
		option := metav1.DeletePropagationForeground
		err := config.Client.Delete(config.CTX, jobObjects[i].GetRuntimeObject().(client.Object), &client.DeleteOptions{
			PropagationPolicy: &option,
		})
		if err != nil && !errors.IsNotFound(err) {
			config.Log.Error(err, "could not delete old job", "namespace", ns)
			config.SetConditionFalseWithMessage(k8upv1alpha1.ConditionScrubbed, k8upv1alpha1.ReasonDeletionFailed, "could not delete old %s: %v", jobObjects[i].GetType(), err)
			return
		}
	}
	config.SetConditionTrueWithMessage(k8upv1alpha1.ConditionScrubbed, k8upv1alpha1.ReasonSucceeded, "Deleted %d resources", numToDelete)
}

func getKeepJobs(keepJobs *int) int {
	if keepJobs == nil {
		return cfg.Config.GlobalKeepJobs
	}
	return *keepJobs
}

// BuildTagArgs will prepend "--tag " to every element in the given []string
func BuildTagArgs(tagList []string) []string {
	var args []string
	for i := range tagList {
		args = append(args, "--tag", tagList[i])
	}
	return args
}
