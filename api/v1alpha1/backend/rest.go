package backend

import "k8s.io/api/core/v1"

type (
	RestServerSpec struct {
		URL               string                `json:"url,omitempty"`
		UserSecretRef     *v1.SecretKeySelector `json:"userSecretRef,omitempty"`
		PasswordSecretReg *v1.SecretKeySelector `json:"passwordSecretReg,omitempty"`
	}
)

func (r *RestServerSpec) EnvVars(vars map[string]*v1.EnvVarSource) map[string]*v1.EnvVarSource {
	if r.PasswordSecretReg != nil {
		vars["PASSWORD"] = &v1.EnvVarSource{
			SecretKeyRef: r.PasswordSecretReg,
		}
	}

	if r.UserSecretRef != nil {
		vars["USER"] = &v1.EnvVarSource{
			SecretKeyRef: r.UserSecretRef,
		}
	}

	return vars
}
