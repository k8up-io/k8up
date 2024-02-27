package executor

import (
	"fmt"

	"github.com/imdario/mergo"
	"github.com/k8up-io/k8up/v2/operator/cfg"
	corev1 "k8s.io/api/core/v1"
)

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
	var repositoryVar corev1.EnvVar
	var repositoryVarSet bool
	for key, value := range e.Vars {

		envVar := corev1.EnvVar{
			Name: key,
		}
		if value.envVarSource != nil {
			envVar.ValueFrom = value.envVarSource
		} else if value.stringEnv != nil {
			envVar.Value = *value.stringEnv
		}
		if key == cfg.ResticRepositoryEnvName {
			// set repository env var at the end so the rest backend url
			// can use previous set rest server credentials
			repositoryVar = envVar
			repositoryVarSet = true
		} else {
			vars = append(vars, envVar)
		}
	}
	if repositoryVarSet {
		vars = append(vars, repositoryVar)
	}
	return vars
}

// EnvVarEntry holds one entry for the EnvVarConverter
type envVarEntry struct {
	stringEnv    *string
	envVarSource *corev1.EnvVarSource
}

// Merge will merge the source into the instance. If there's no entry in the instance
// that exists in the source, the source entry will be added. If there's a zero-valued
// entry, it will also be overwritten.
func (e *EnvVarConverter) Merge(source EnvVarConverter) error {
	return mergo.Merge(&e.Vars, source.Vars)
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
