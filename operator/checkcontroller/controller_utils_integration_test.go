//go:build integration

package checkcontroller

import (
	"github.com/k8up-io/k8up/v2/operator/cfg"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/utils/ptr"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
)

const (
	checkTlsVolumeName       = "minio-client-tls"
	checkTlsVolumeSecretName = "minio-client-tls"
	checkTlsVolumeMount      = "/mnt/tls"
	checkTlsCaCertPath       = checkTlsVolumeMount + "/ca.cert"

	checkMutualTlsVolumeName       = "minio-client-mtls"
	checkMutualTlsVolumeSecretName = "minio-client-mtls"
	checkMutualTlsVolumeMount      = "/mnt/mtls"
	checkMutualTlsCaCertPath       = checkMutualTlsVolumeMount + "/ca.cert"
	checkMutualTlsClientCertPath   = checkMutualTlsVolumeMount + "/client.cert"
	checkMutualTlsKeyCertPath      = checkMutualTlsVolumeMount + "/client.key"
)

func (ts *CheckTestSuite) expectACheckJob() (foundJob *batchv1.Job) {
	jobs := new(batchv1.JobList)
	err := ts.Client.List(ts.Ctx, jobs, client.InNamespace(ts.NS))
	ts.Require().NoError(err)

	jobsLen := len(jobs.Items)
	ts.T().Logf("%d Jobs found", jobsLen)
	ts.Require().Len(jobs.Items, 1, "job exists")
	return &jobs.Items[0]
}

func (ts *CheckTestSuite) whenReconciling(object *k8upv1.Check) controllerruntime.Result {
	controller := CheckReconciler{
		Kube: ts.Client,
	}

	result, err := controller.Provision(ts.Ctx, object)
	ts.Require().NoError(err)

	return result
}

func (ts *CheckTestSuite) newCheckTls() *k8upv1.Check {
	return &k8upv1.Check{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "check",
			Namespace: ts.NS,
			UID:       uuid.NewUUID(),
		},
		Spec: k8upv1.CheckSpec{
			RunnableSpec: k8upv1.RunnableSpec{
				Backend: &k8upv1.Backend{
					TLSOptions: &k8upv1.TLSOptions{CACert: checkTlsCaCertPath},
					VolumeMounts: &[]corev1.VolumeMount{
						{
							Name:      checkTlsVolumeName,
							MountPath: checkTlsVolumeMount,
						},
					},
				},
				Volumes: &[]k8upv1.RunnableVolumeSpec{
					{
						Name: checkTlsVolumeName,
						Secret: &corev1.SecretVolumeSource{
							SecretName:  checkTlsVolumeSecretName,
							DefaultMode: ptr.To(corev1.SecretVolumeSourceDefaultMode),
						},
					},
				},
			},
		},
	}
}

func (ts *CheckTestSuite) assertCheckTlsVolumeAndTlsOptions(job *batchv1.Job) {
	expectArgs := []string{"-varDir", cfg.Config.PodVarDir, "-check", "-caCert", checkTlsCaCertPath}
	expectVolumeMount := corev1.VolumeMount{Name: checkTlsVolumeName, MountPath: checkTlsVolumeMount}
	expectVolume := corev1.Volume{
		Name: checkTlsVolumeName,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName:  checkTlsVolumeSecretName,
				DefaultMode: ptr.To(corev1.SecretVolumeSourceDefaultMode),
			},
		},
	}

	jobArguments := job.Spec.Template.Spec.Containers[0].Args
	ts.Assert().Equal(jobArguments, expectArgs, "check tls contains caCert path in job args")
	jobVolumeMounts := job.Spec.Template.Spec.Containers[0].VolumeMounts
	ts.Assert().NotNil(jobVolumeMounts)
	ts.Assert().Contains(jobVolumeMounts, expectVolumeMount, "check ca cert in job volume mount")
	jobVolumes := job.Spec.Template.Spec.Volumes
	ts.Assert().NotNil(jobVolumes)
	ts.Assert().Contains(jobVolumes, expectVolume, "check ca cert in job volume mount")
}

func (ts *CheckTestSuite) newCheckMutualTls() *k8upv1.Check {
	return &k8upv1.Check{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "backup",
			Namespace: ts.NS,
			UID:       uuid.NewUUID(),
		},
		Spec: k8upv1.CheckSpec{
			RunnableSpec: k8upv1.RunnableSpec{
				Backend: &k8upv1.Backend{
					TLSOptions: &k8upv1.TLSOptions{
						CACert:     checkMutualTlsCaCertPath,
						ClientCert: checkMutualTlsClientCertPath,
						ClientKey:  checkMutualTlsKeyCertPath,
					},
					VolumeMounts: &[]corev1.VolumeMount{
						{
							Name:      checkMutualTlsVolumeName,
							MountPath: checkMutualTlsVolumeMount,
						},
					},
				},
				Volumes: &[]k8upv1.RunnableVolumeSpec{
					{
						Name: checkMutualTlsVolumeName,
						Secret: &corev1.SecretVolumeSource{
							SecretName:  checkMutualTlsVolumeSecretName,
							DefaultMode: ptr.To(corev1.SecretVolumeSourceDefaultMode),
						},
					},
				},
			},
		},
	}
}

func (ts *CheckTestSuite) assertCheckMutualTlsVolumeAndMutualTlsOptions(job *batchv1.Job) {
	expectArgs := []string{
		"-varDir", cfg.Config.PodVarDir,
		"-check",
		"-caCert", checkMutualTlsCaCertPath,
		"-clientCert", checkMutualTlsClientCertPath,
		"-clientKey", checkMutualTlsKeyCertPath,
	}
	expectVolumeMount := corev1.VolumeMount{Name: checkMutualTlsVolumeName, MountPath: checkMutualTlsVolumeMount}
	expectVolume := corev1.Volume{
		Name: checkMutualTlsVolumeName,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName:  checkMutualTlsVolumeSecretName,
				DefaultMode: ptr.To(corev1.SecretVolumeSourceDefaultMode),
			},
		},
	}

	jobArguments := job.Spec.Template.Spec.Containers[0].Args
	ts.Assert().Equal(jobArguments, expectArgs, "check tls contains caCert path in job args")
	jobVolumeMounts := job.Spec.Template.Spec.Containers[0].VolumeMounts
	ts.Assert().NotNil(jobVolumeMounts)
	ts.Assert().Contains(jobVolumeMounts, expectVolumeMount, "check ca cert in job volume mount")
	jobVolumes := job.Spec.Template.Spec.Volumes
	ts.Assert().NotNil(jobVolumes)
	ts.Assert().Contains(jobVolumes, expectVolume, "check ca cert in job volume mount")
}
