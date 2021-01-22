package backend

import "k8s.io/api/core/v1"

type (
	B2Spec struct {
		Bucket              string                `json:"bucket,omitempty"`
		Path                string                `json:"path,omitempty"`
		AccountIDSecretRef  *v1.SecretKeySelector `json:"accountIDSecretRef,omitempty"`
		AccountKeySecretRef *v1.SecretKeySelector `json:"accountKeySecretRef,omitempty"`
	}
)

func (b *B2Spec) EnvVars(vars map[string]*v1.EnvVarSource) map[string]*v1.EnvVarSource {
	if b.AccountIDSecretRef != nil {
		vars["B2_ACCOUNT_ID"] = &v1.EnvVarSource{
			SecretKeyRef: b.AccountIDSecretRef,
		}
	}

	if b.AccountKeySecretRef != nil {
		vars["B2_ACCOUNT_KEY"] = &v1.EnvVarSource{
			SecretKeyRef: b.AccountKeySecretRef,
		}
	}

	return vars
}
