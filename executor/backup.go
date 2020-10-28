package executor

import (
	"path"
	"strconv"

	"github.com/vshn/k8up/job"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	mountPath                      = "/data"
	backupAnnotationDefault        = "foo"
	backupCommandAnnotationDefault = "bar"
	fileExtensionAnnotationDefault = "foobar"
)

type BackupExecutor struct {
	generic
}

func NewBackupExecutor(config job.Config) *BackupExecutor {
	return &BackupExecutor{
		generic: generic{config},
	}
}

func (b *BackupExecutor) Execute() error {

	if b.Obj.GetK8upStatus().Started {
		return nil
	}

	job, err := job.GetGenericJob(b.Obj, b.Config)
	if err != nil {
		return err
	}

	go func() {

		preBackup := NewPreBackupExecutor(b.Config)
		err := preBackup.Execute()
		if err != nil {
			b.Config.Log.Error(err, "error while handling pre backup pods")
			return
		}

		volumes := b.listPVCs(backupAnnotationDefault)

		job.Spec.Template.Spec.Volumes = volumes
		job.Spec.Template.Spec.ServiceAccountName = "pod-executor"
		job.Spec.Template.Spec.Containers[0].VolumeMounts = b.getVolumeMounts(volumes)
		err = b.Client.Create(b.CTX, job)
		if err != nil {
			if !errors.IsAlreadyExists(err) {
				b.Config.Log.Error(err, "could not create job")
				return
			}
		}

	}()

	b.Obj.GetK8upStatus().Started = true

	return b.Client.Status().Update(b.CTX, b.Obj.GetRuntimeObject().DeepCopyObject())
}

func (b *BackupExecutor) listPVCs(annotation string) []corev1.Volume {
	b.Log.Info("Listing all PVCs", "annotation", annotation, "namespace", b.Obj.GetMetaObject().GetNamespace())
	volumes := make([]corev1.Volume, 0)

	claimlist := &corev1.PersistentVolumeClaimList{}

	err := b.Client.List(b.CTX, claimlist, &client.ListOptions{})
	if err != nil {
		return nil
	}

	for _, item := range claimlist.Items {
		annotations := item.GetAnnotations()

		tmpAnnotation, ok := annotations[annotation]

		if !b.containsAccessMode(item.Spec.AccessModes, "ReadWriteMany") && !ok {
			b.Log.Info("PVC isn't RWX", "namespace", item.GetNamespace(), "name", item.GetName())
			continue
		}

		if !ok {
			b.Log.Info("PVC doesn't have annotation, adding to list", "namespace", item.GetNamespace(), "name", item.GetName())
		} else if anno, _ := strconv.ParseBool(tmpAnnotation); !anno {
			b.Log.Info("PVC skipped due to annotation", "namespace", item.GetNamespace(), "name", item.GetName(), "annotation", tmpAnnotation)
			continue
		} else {
			b.Log.Info("Adding to list", "namespace", item.GetNamespace(), "name", item.Name)
		}

		tmpVol := corev1.Volume{
			Name: item.Name,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: item.Name,
				},
			},
		}

		volumes = append(volumes, tmpVol)
	}

	return volumes
}

func (b *BackupExecutor) getVolumeMounts(claims []corev1.Volume) []corev1.VolumeMount {
	mounts := make([]corev1.VolumeMount, len(claims))
	for i, volume := range claims {
		mounts[i] = corev1.VolumeMount{
			Name:      volume.Name,
			MountPath: path.Join(mountPath, volume.Name),
			ReadOnly:  true,
		}
	}
	return mounts
}

func (b *BackupExecutor) containsAccessMode(s []corev1.PersistentVolumeAccessMode, e string) bool {
	for _, a := range s {
		if string(a) == e {
			return true
		}
	}
	return false
}
