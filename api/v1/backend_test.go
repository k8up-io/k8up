package v1

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"

	"github.com/k8up-io/k8up/v2/operator/cfg"
)

var tests = map[string]struct {
	givenBackend             *Backend
	expectedRepositoryString string
	expectedVars             map[string]*corev1.EnvVarSource
}{
	"GivenAzureBackend_ThenExpectAzureContainer": {
		givenBackend: &Backend{
			Azure: &AzureSpec{
				Container:            "container",
				AccountNameSecretRef: newSecretRef("name"),
				AccountKeySecretRef:  newSecretRef("key"),
			},
		},
		expectedVars: map[string]*corev1.EnvVarSource{
			cfg.AzureAccountEnvName:    {SecretKeyRef: newSecretRef("name")},
			cfg.AzureAccountKeyEnvName: {SecretKeyRef: newSecretRef("key")},
		},
		expectedRepositoryString: "azure:container:/",
	},
	"GivenAzureBackendAndPath_ThenExpectAzureContainerWithCustomPath": {
		givenBackend: &Backend{
			Azure: &AzureSpec{
				Container:            "container",
				Path:                 "foo",
				AccountNameSecretRef: newSecretRef("name"),
				AccountKeySecretRef:  newSecretRef("key"),
			},
		},
		expectedVars: map[string]*corev1.EnvVarSource{
			cfg.AzureAccountEnvName:    {SecretKeyRef: newSecretRef("name")},
			cfg.AzureAccountKeyEnvName: {SecretKeyRef: newSecretRef("key")},
		},
		expectedRepositoryString: "azure:container:foo",
	},
	"GivenB2Backend_ThenExpectB2BucketAndPath": {
		givenBackend: &Backend{
			B2: &B2Spec{
				Bucket:              "bucket",
				Path:                "path",
				AccountKeySecretRef: newSecretRef("key"),
				AccountIDSecretRef:  newSecretRef("id"),
			},
		},
		expectedVars: map[string]*corev1.EnvVarSource{
			cfg.B2AccountIDEnvName:  {SecretKeyRef: newSecretRef("id")},
			cfg.B2AccountKeyEnvName: {SecretKeyRef: newSecretRef("key")},
		},
		expectedRepositoryString: "b2:bucket:path",
	},
	"GivenLocalBackend_ThenExpectMountPath": {
		givenBackend: &Backend{
			Local: &LocalSpec{
				MountPath: "mountpath",
			},
		},
		expectedVars:             map[string]*corev1.EnvVarSource{},
		expectedRepositoryString: "mountpath",
	},
	"GivenGcsBackend_ThenExpectGcsBucket": {
		givenBackend: &Backend{
			GCS: &GCSSpec{
				Bucket:               "bucket",
				AccessTokenSecretRef: newSecretRef("token"),
				ProjectIDSecretRef:   newSecretRef("id"),
			},
		},
		expectedVars: map[string]*corev1.EnvVarSource{
			cfg.GcsAccessTokenEnvName: {SecretKeyRef: newSecretRef("token")},
			cfg.GcsProjectIDEnvName:   {SecretKeyRef: newSecretRef("id")},
		},
		expectedRepositoryString: "gs:bucket:/",
	},
	"GivenS3Backend_ThenExpectS3EndpointURLWithBucket": {
		givenBackend: &Backend{
			S3: &S3Spec{
				Bucket:                   "bucket",
				Endpoint:                 "https://endpoint",
				SecretAccessKeySecretRef: newSecretRef("secret"),
				AccessKeyIDSecretRef:     newSecretRef("id"),
			},
		},
		expectedVars: map[string]*corev1.EnvVarSource{
			cfg.AwsAccessKeyIDEnvName:     {SecretKeyRef: newSecretRef("id")},
			cfg.AwsSecretAccessKeyEnvName: {SecretKeyRef: newSecretRef("secret")},
		},
		expectedRepositoryString: "s3:https://endpoint/bucket",
	},
	"GivenSwiftBackend_ThenExpectSwiftBucket": {
		givenBackend: &Backend{
			Swift: &SwiftSpec{
				Container: "container",
				Path:      "path",
			},
		},
		expectedVars:             map[string]*corev1.EnvVarSource{},
		expectedRepositoryString: "swift:container:path",
	},
	"GivenRestBackend_ThenExpectRestUrl": {
		givenBackend: &Backend{
			Rest: &RestServerSpec{
				URL:               "https://server",
				PasswordSecretReg: newSecretRef("password"),
				UserSecretRef:     newSecretRef("user"),
			},
		},
		expectedVars: map[string]*corev1.EnvVarSource{
			cfg.RestPasswordEnvName: {SecretKeyRef: newSecretRef("password")},
			cfg.RestUserEnvName:     {SecretKeyRef: newSecretRef("user")},
		},
		expectedRepositoryString: "rest:https://$(USER):$(PASSWORD)@server",
	},
}

func Test_Backend_String(t *testing.T) {
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			result := tt.givenBackend.String()
			assert.Equal(t, tt.expectedRepositoryString, result)
		})
	}
}

func Test_Backend_GetCredentialEnv(t *testing.T) {
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			result := tt.givenBackend.GetCredentialEnv()
			assert.Equal(t, tt.expectedVars, result)
		})
	}
}

func newSecretRef(name string) *corev1.SecretKeySelector {
	return &corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: name,
		},
	}
}

func TestBackend_IsBackendEqualTo(t *testing.T) {
	tests := map[string]struct {
		givenBackend Backend
		otherBackend *Backend
		expectResult bool
	}{
		"GivenDifferentBackend_WhenComparing_ThenReturnFalse": {
			givenBackend: Backend{
				S3: &S3Spec{
					Endpoint: "https://endpoint",
					Bucket:   "bucket",
				},
			},
			otherBackend: &Backend{
				Azure: &AzureSpec{
					Container: "container",
				},
			},
			expectResult: false,
		},
		"GivenSameBackend_WhenComparingWithDifferentValues_ThenReturnFalse": {
			givenBackend: Backend{
				S3: &S3Spec{
					Endpoint: "https://endpoint",
					Bucket:   "bucket1",
				},
			},
			otherBackend: &Backend{
				S3: &S3Spec{
					Endpoint: "https://endpoint",
					Bucket:   "bucket2",
				},
			},
			expectResult: false,
		},
		"GivenSameBackend_WhenComparingWithSameValues_ThenReturnTrue": {
			givenBackend: Backend{
				S3: &S3Spec{
					Endpoint: "https://endpoint",
					Bucket:   "bucket",
				},
			},
			otherBackend: &Backend{
				S3: &S3Spec{
					Endpoint: "https://endpoint",
					Bucket:   "bucket",
				},
			},
			expectResult: true,
		},
		"GivenBackend_WhenComparingWithNil_ThenReturnFalse": {
			givenBackend: Backend{
				S3: &S3Spec{
					Endpoint: "https://endpoint",
					Bucket:   "bucket",
				},
			},
			otherBackend: nil,
			expectResult: false,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			result := tt.givenBackend.IsBackendEqualTo(tt.otherBackend)
			assert.Equal(t, tt.expectResult, result)
		})
	}
}

func Test_AppendEnvFromToContainer(t *testing.T) {
	t.Run("GivenFromEnv_WhenAppendedToContainer_ThenContainerHasFromEnv", func(t *testing.T) {
		// Given a container with an empty EnvFrom
		var container = &corev1.Container{
			EnvFrom: []corev1.EnvFromSource{},
		}

		// And a Backend config with an EnvFrom, containing a secret reference called
		secretRef := new(corev1.SecretEnvSource)
		secretRef.Name = "my-secret"

		var spec = &RunnableSpec{
			Backend: &Backend{
				EnvFrom: []corev1.EnvFromSource{{
					Prefix:       "test-prefix",
					ConfigMapRef: new(corev1.ConfigMapEnvSource),
					SecretRef:    secretRef,
				}},
			},
			Resources:          corev1.ResourceRequirements{},
			PodSecurityContext: nil,
		}

		// When I Append EnvFrom to the container
		spec.AppendEnvFromToContainer(container)

		// Then My container has the secret reference in the EnvFrom structure
		assert.Equal(t, "test-prefix", container.EnvFrom[0].Prefix)
		assert.Equal(t, "my-secret", container.EnvFrom[0].SecretRef.Name)
	})
}
