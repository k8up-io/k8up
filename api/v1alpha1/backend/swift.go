package backend

import "k8s.io/api/core/v1"

type (
	SwiftSpec struct {
		Container string `json:"container,omitempty"`
		Path      string `json:"path,omitempty"`
	}
)

func (s *SwiftSpec) EnvVars(vars map[string]*v1.EnvVarSource) map[string]*v1.EnvVarSource {
	return vars
}
