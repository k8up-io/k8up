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
type EnvVarEntry struct {
	StringEnv    string
	EnvVarSource *corev1.EnvVarSource
}

// EnvVarConverter can convert the given map to a []corev1.EnvVar
type EnvVarConverter struct {
	Vars map[string]EnvVarEntry
}

func NewEnvVarConverter() EnvVarConverter {
	return EnvVarConverter{
		Vars: make(map[string]EnvVarEntry),
	}
}

func (e *EnvVarConverter) SetEntry(key string, value EnvVarEntry) {
	e.Vars[key] = value
}

func (e *EnvVarConverter) Convert() []corev1.EnvVar {
	vars := make([]corev1.EnvVar, 0)
	for key, value := range e.Vars {
		envVar := corev1.EnvVar{
			Name: key,
		}
		if value.EnvVarSource != nil {
			envVar.ValueFrom = value.EnvVarSource
		} else {
			envVar.Value = value.StringEnv
		}
		vars = append(vars, envVar)
	}
	return vars
}

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

func NewExecutor(config job.Config) queue.Executor {
	switch config.Obj.GetType() {
	case "backup":
		return NewBackupExecutor(config)
	case "check":
		return NewCheckExecutor(config)
	}
	return nil
}

// DefaultEnv returns an environment that contains the default values for the fields.
func DefaultEnv(namespace string) EnvVarConverter {
	defaults := NewEnvVarConverter()

	defaults.SetEntry("STATS_URL", EnvVarEntry{StringEnv: constants.GetGlobalStatsURL()})
	defaults.SetEntry(constants.ResticPasswordEnvName, EnvVarEntry{StringEnv: constants.GetGlobalRepoPassword()})
	defaults.SetEntry(constants.ResticRepositoryEnvName, EnvVarEntry{StringEnv: fmt.Sprintf("s3:%s/%s", constants.GetGlobalS3Endpoint(), constants.GetGlobalS3Bucket())})
	defaults.SetEntry(constants.ResticPasswordEnvName, EnvVarEntry{StringEnv: constants.GetGlobalRepoPassword()})
	defaults.SetEntry(constants.AwsAccessKeyIDEnvName, EnvVarEntry{StringEnv: constants.GetGlobalAccessKey()})
	defaults.SetEntry(constants.AwsSecretAccessKeyEnvName, EnvVarEntry{StringEnv: constants.GetGlobalSecretAccessKey()})
	defaults.SetEntry("HOSTNAME", EnvVarEntry{StringEnv: namespace})

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
