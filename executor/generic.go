// executor contains the logic that is needed to apply the actual k8s job objects to a cluster.
// each job type should implement its own executor that handles its own job creation.
// There are various methods that provide default env vars and batch.job scaffolding.

package executor

import (
	"fmt"
	"sort"

	"github.com/go-logr/logr"
	"github.com/imdario/mergo"
	"github.com/vshn/k8up/constants"
	"github.com/vshn/k8up/job"
	"github.com/vshn/k8up/queue"
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

// NewExecutor will return the right Executor for the given job object.
func NewExecutor(config job.Config) queue.Executor {
	switch config.Obj.GetType() {
	case "backup":
		return NewBackupExecutor(config)
	case "check":
		return NewCheckExecutor(config)
	case "archive":
		return NewArchiveExecutor(config)
	case "restore":
		return NewRestoreExecutor(config)
	}
	return nil
}

// DefaultEnv returns an environment that contains the default values for the fields.
func DefaultEnv(namespace string) EnvVarConverter {
	defaults := NewEnvVarConverter()

	defaults.SetString("STATS_URL", constants.GetGlobalStatsURL())
	defaults.SetString(constants.ResticPasswordEnvName, constants.GetGlobalRepoPassword())
	defaults.SetString(constants.ResticRepositoryEnvName, fmt.Sprintf("s3:%s/%s", constants.GetGlobalS3Endpoint(), constants.GetGlobalS3Bucket()))
	defaults.SetString(constants.ResticPasswordEnvName, constants.GetGlobalRepoPassword())
	defaults.SetString(constants.AwsAccessKeyIDEnvName, constants.GetGlobalAccessKey())
	defaults.SetString(constants.AwsSecretAccessKeyEnvName, constants.GetGlobalSecretAccessKey())
	defaults.SetString("HOSTNAME", namespace)

	return defaults
}

func cleanOldObjects(jobObjects jobObjectList, maxObjects int, config job.Config) error {

	numToDelete := len(jobObjects) - maxObjects

	if numToDelete <= 0 {
		return nil
	}

	sort.Sort(jobObjects)

	for i := 0; i < numToDelete; i++ {
		config.Log.Info("cleaning old job", "namespace", jobObjects[i].GetMetaObject().GetNamespace(), "name", jobObjects[i].GetMetaObject().GetName())
		option := metav1.DeletePropagationForeground
		err := config.Client.Delete(config.CTX, jobObjects[i].GetRuntimeObject(), &client.DeleteOptions{
			PropagationPolicy: &option,
		})
		if err != nil {
			if !errors.IsNotFound(err) {
				return err
			}
		}
	}

	return nil
}

func getKeepJobs(keepJobs *int) int {
	if keepJobs == nil {
		return constants.GetGlobalKeepJobs()
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
