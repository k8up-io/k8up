package backend

import "k8s.io/api/core/v1"

type (
	AzureSpec struct {
		Container            string                `json:"container,omitempty"`
		AccountNameSecretRef *v1.SecretKeySelector `json:"accountNameSecretRef,omitempty"`
		AccountKeySecretRef  *v1.SecretKeySelector `json:"accountKeySecretRef,omitempty"`
	}
)

func (a *AzureSpec) EnvVars(vars map[string]*v1.EnvVarSource) map[string]*v1.EnvVarSource {
	if a.AccountKeySecretRef != nil {
		vars["AZURE_ACCOUNT_KEY"] = &v1.EnvVarSource{
			SecretKeyRef: a.AccountKeySecretRef,
		}
	}

	if a.AccountNameSecretRef != nil {
		vars["AZURE_ACCOUNT_NAME"] = &v1.EnvVarSource{
			SecretKeyRef: a.AccountNameSecretRef,
		}
	}

	return vars
}
