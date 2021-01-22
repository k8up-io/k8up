package backend

import "k8s.io/api/core/v1"

type (
	LocalSpec struct {
		MountPath string `json:"mountPath,omitempty"`
	}
)

func (l *LocalSpec) EnvVars(vars map[string]*v1.EnvVarSource) map[string]*v1.EnvVarSource {
	return vars
}
