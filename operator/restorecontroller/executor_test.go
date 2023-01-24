package restorecontroller

import (
	"context"
	"testing"

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	"github.com/k8up-io/k8up/v2/operator/job"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
)

type PVCExpectation struct {
	Expected  bool
	ClaimName string
	ReadOnly  bool
}
type VolumeMountExpectation struct {
	Expected bool
	Name     string
	Path     string
}

func newConfig() *job.Config {
	cfg := job.NewConfig(nil, &k8upv1.Restore{}, "")
	return &cfg
}

func newS3RestoreResource() *k8upv1.Restore {
	return &k8upv1.Restore{
		Spec: k8upv1.RestoreSpec{
			RestoreMethod: &k8upv1.RestoreMethod{
				S3: &k8upv1.S3Spec{
					Endpoint: "http://localhost:9000",
					Bucket:   "test",
					AccessKeyIDSecretRef: &corev1.SecretKeySelector{
						Key: "accessKey",
					},
					SecretAccessKeySecretRef: &corev1.SecretKeySelector{
						Key: "secretKey",
					},
				},
			},
			RunnableSpec: k8upv1.RunnableSpec{
				Backend: &k8upv1.Backend{
					S3: &k8upv1.S3Spec{
						Endpoint: "http://localhost:9000",
						Bucket:   "test-backend",
						AccessKeyIDSecretRef: &corev1.SecretKeySelector{
							Key: "accessKey-backend",
						},
						SecretAccessKeySecretRef: &corev1.SecretKeySelector{
							Key: "secretKey-backend",
						},
					},
				},
			},
		},
	}
}

func newFolderRestoreResource() *k8upv1.Restore {
	return &k8upv1.Restore{
		Spec: k8upv1.RestoreSpec{
			RestoreMethod: &k8upv1.RestoreMethod{
				Folder: &k8upv1.FolderRestore{
					PersistentVolumeClaimVolumeSource: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: "test",
						ReadOnly:  false,
					},
				},
			},
		},
	}
}

func newFilteredFolderRestoreResource() *k8upv1.Restore {
	return &k8upv1.Restore{
		Spec: k8upv1.RestoreSpec{
			RestoreMethod: &k8upv1.RestoreMethod{
				Folder: &k8upv1.FolderRestore{
					PersistentVolumeClaimVolumeSource: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: "test",
						ReadOnly:  false,
					},
				},
			},
			Tags:          []string{"testtag", "another"},
			RestoreFilter: "testfilter",
			Snapshot:      "testsnapshot",
		},
	}
}

func TestRestore_setupEnvVars(t *testing.T) {
	tests := map[string]struct {
		GivenResource         *k8upv1.Restore
		ExpectedEnvVars       map[string]string
		ExpectedSecretKeyRefs map[string]string
	}{
		"givenS3RestoreResource_whenSetupEnvVars_expectCertainEnvVars": {
			GivenResource: newS3RestoreResource(),
			ExpectedEnvVars: map[string]string{
				"HOSTNAME":           "",
				"RESTIC_PASSWORD":    "",
				"RESTIC_REPOSITORY":  "s3:http://localhost:9000/test-backend",
				"RESTORE_S3ENDPOINT": "http://localhost:9000/test",
				"STATS_URL":          "",
			},
			ExpectedSecretKeyRefs: map[string]string{
				"AWS_ACCESS_KEY_ID":       "accessKey-backend",
				"AWS_SECRET_ACCESS_KEY":   "secretKey-backend",
				"RESTORE_ACCESSKEYID":     "accessKey",
				"RESTORE_SECRETACCESSKEY": "secretKey",
			},
		},
		"givenFolderRestoreResource_whenSetupEnvVars_expectCertainEnvVars": {
			GivenResource: newFolderRestoreResource(),
			ExpectedEnvVars: map[string]string{
				"AWS_ACCESS_KEY_ID":     "",
				"AWS_SECRET_ACCESS_KEY": "",
				"HOSTNAME":              "",
				"RESTIC_PASSWORD":       "",
				"RESTIC_REPOSITORY":     "s3:/",
				"RESTORE_DIR":           "/restore",
				"STATS_URL":             "",
			},
			ExpectedSecretKeyRefs: map[string]string{},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			e := NewRestoreExecutor(*newConfig())
			envVars := e.setupEnvVars(context.TODO(), tt.GivenResource)

			actualEnvVars, actualSecretKeyRefs := extractVarsAndSecretRefs(envVars)

			assert.Equal(t, actualEnvVars, tt.ExpectedEnvVars)
			assert.Equal(t, actualSecretKeyRefs, tt.ExpectedSecretKeyRefs)
		})
	}
}

func extractVarsAndSecretRefs(envVars []corev1.EnvVar) (map[string]string, map[string]string) {
	actualVars := make(map[string]string)
	actualSecretRefs := make(map[string]string)
	for _, envVar := range envVars {
		if envVar.ValueFrom != nil && envVar.ValueFrom.SecretKeyRef != nil {
			actualSecretRefs[envVar.Name] = envVar.ValueFrom.SecretKeyRef.Key
		} else {
			actualVars[envVar.Name] = envVar.Value
		}
	}
	return actualVars, actualSecretRefs
}

func TestRestore_volumeConfig(t *testing.T) {
	tests := map[string]struct {
		GivenResource       *k8upv1.Restore
		ExpectedPVC         PVCExpectation
		ExpectedVolumeMount VolumeMountExpectation
	}{
		"givenS3RestoreResource_whenVolumeConfig_expectNoPVCAndNoMount": {
			GivenResource:       newS3RestoreResource(),
			ExpectedPVC:         PVCExpectation{Expected: false},
			ExpectedVolumeMount: VolumeMountExpectation{Expected: false},
		},
		"givenFolderRestoreResource_whenVolumeConfig_expectPVCAndMount": {
			GivenResource: newFolderRestoreResource(),
			ExpectedPVC: PVCExpectation{
				Expected:  true,
				ClaimName: "test",
				ReadOnly:  false,
			},
			ExpectedVolumeMount: VolumeMountExpectation{
				Expected: true,
				Name:     "test",
				Path:     "/restore",
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			e := NewRestoreExecutor(*newConfig())
			volumes, mounts := e.volumeConfig(tt.GivenResource)

			assertVolumes(t, tt.ExpectedPVC, volumes)
			assertVolumeMounts(t, tt.ExpectedVolumeMount, mounts)
		})
	}
}

func assertVolumeMounts(t *testing.T, ex VolumeMountExpectation, mounts []corev1.VolumeMount) {
	if !ex.Expected {
		assert.Len(t, mounts, 0)
		return
	}

	assert.Len(t, mounts, 1)
	volumeMount := mounts[0]

	assert.Equal(t, ex.Name, volumeMount.Name)
	assert.Equal(t, ex.Path, volumeMount.MountPath)
}

func assertVolumes(t *testing.T, ex PVCExpectation, volumes []corev1.Volume) {
	if !ex.Expected {
		assert.Len(t, volumes, 0)
		return
	}

	assert.Len(t, volumes, 1)
	volume := volumes[0]

	assert.Equal(t, ex.ClaimName, volume.PersistentVolumeClaim.ClaimName)
	assert.Equal(t, ex.ReadOnly, volume.PersistentVolumeClaim.ReadOnly)
}

func TestRestore_args(t *testing.T) {
	tests := map[string]struct {
		GivenResource *k8upv1.Restore
		ExpectedArgs  []string
	}{
		"givenS3RestoreResource_whenArgs_expectS3RestoreType": {
			GivenResource: newS3RestoreResource(),
			ExpectedArgs:  []string{"-restore", "-restoreType", "s3"},
		},
		"givenFolderRestoreResource_whenArgs_expectFolderRestoreType": {
			GivenResource: newFolderRestoreResource(),
			ExpectedArgs:  []string{"-restore", "-restoreType", "folder"},
		},
		"givenFolderRestoreResourceWithAdditionalArguments_whenBuildRestoreObject_expectJobResource": {
			GivenResource: newFilteredFolderRestoreResource(),
			ExpectedArgs: []string{
				"-restore",
				"--tag", "testtag",
				"--tag", "another",
				"-restoreFilter", "testfilter",
				"-restoreSnap", "testsnapshot",
				"-restoreType", "folder",
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			e := NewRestoreExecutor(*newConfig())
			args, err := e.args(tt.GivenResource)

			require.NoError(t, err)
			assert.Equal(t, tt.ExpectedArgs, args)
		})
	}
}
