// executor contains the logic that is needed to apply the actual k8s job objects to a cluster.
// each job type should implement its own executor that handles its own job creation.
// There are various methods that provide default env vars and batch.job scaffolding.

package executor

import (
	"fmt"

	"github.com/go-logr/logr"
	"github.com/imdario/mergo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	"github.com/k8up-io/k8up/v2/operator/cfg"
	"github.com/k8up-io/k8up/v2/operator/executor/cleaner"
	"github.com/k8up-io/k8up/v2/operator/job"
)

type Generic struct {
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

// SetStringOrDefault adds a string key and value pair to the environment.
// If value is an empty string, it will use the given default value.
func (e *EnvVarConverter) SetStringOrDefault(key, value, def string) {
	if value == "" {
		value = def
	}
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

func (g *Generic) Logger() logr.Logger {
	return g.Log
}

func (*Generic) Exclusive() bool {
	return false
}

func (g *Generic) GetRepository() string {
	return g.Repository
}

func (g *Generic) GetJobNamespace() string {
	return g.Obj.GetNamespace()
}

func (g *Generic) GetJobNamespacedName() types.NamespacedName {
	return types.NamespacedName{Namespace: g.Obj.GetNamespace(), Name: g.Obj.GetJobName()}
}

func (g *Generic) GetJobType() k8upv1.JobType {
	return g.Obj.GetType()
}

// listOldResources retrieves a list of the given resource type in the given namespace and fills the Item property
// of objList. On errors, the error is being logged and the Scrubbed condition set to False with reason RetrievalFailed.
func (g *Generic) listOldResources(namespace string, objList client.ObjectList) error {
	err := g.Client.List(g.CTX, objList, &client.ListOptions{
		Namespace: namespace,
	})
	if err != nil {
		g.Log.Error(err, "could not list objects to cleanup old resources", "Namespace", namespace, "kind", objList.GetObjectKind().GroupVersionKind().Kind)
		g.SetConditionFalseWithMessage(k8upv1.ConditionScrubbed, k8upv1.ReasonRetrievalFailed, "could not list objects to cleanup old resources: %v", err)
		return err
	}
	return nil
}

type jobObjectList interface {
	client.ObjectList

	GetJobObjects() k8upv1.JobObjectList
}

func (g *Generic) CleanupOldResources(typ jobObjectList, name types.NamespacedName, limits cleaner.GetJobsHistoryLimiter) {
	err := g.listOldResources(name.Namespace, typ)
	if err != nil {
		return
	}

	cl := cleaner.ObjectCleaner{Client: g.Client, Limits: limits, Log: g.Logger()}
	deleted, err := cl.CleanOldObjects(g.CTX, typ.GetJobObjects())
	if err != nil {
		g.SetConditionFalseWithMessage(k8upv1.ConditionScrubbed, k8upv1.ReasonDeletionFailed, "could not cleanup old resources: %v", err)
		return
	}
	g.SetConditionTrueWithMessage(k8upv1.ConditionScrubbed, k8upv1.ReasonSucceeded, "Deleted %v resources", deleted)

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

	if cfg.Config.ResticOptions != "" {
		defaults.SetString(cfg.ResticOptionsEnvName, cfg.Config.ResticOptions)
	}

	return defaults
}

// BuildTagArgs will prepend "--tag " to every element in the given []string
func BuildTagArgs(tagList []string) []string {
	var args []string
	for i := range tagList {
		args = append(args, "--tag", tagList[i])
	}
	return args
}
