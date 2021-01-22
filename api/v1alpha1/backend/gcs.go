package backend

import "k8s.io/api/core/v1"

type (
	GCSSpec struct {
		Bucket               string                `json:"bucket,omitempty"`
		ProjectIDSecretRef   *v1.SecretKeySelector `json:"projectIDSecretRef,omitempty"`
		AccessTokenSecretRef *v1.SecretKeySelector `json:"accessTokenSecretRef,omitempty"`
	}
)

func (g *GCSSpec) EnvVars(vars map[string]*v1.EnvVarSource) map[string]*v1.EnvVarSource {
	if g.ProjectIDSecretRef != nil {
		vars["GOOGLE_PROJECT_ID"] = &v1.EnvVarSource{
			SecretKeyRef: g.ProjectIDSecretRef,
		}
	}

	if g.AccessTokenSecretRef != nil {
		vars["GOOGLE_ACCESS_TOKEN"] = &v1.EnvVarSource{
			SecretKeyRef: g.AccessTokenSecretRef,
		}
	}

	return vars

}
