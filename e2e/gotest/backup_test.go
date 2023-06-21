package gotest

import (
	"context"
	"encoding/json"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client"

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"

	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

func TestBackup(t *testing.T) {
	ctx := context.Background()
	c := getClient(t)

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "test-backup-",
		},
	}
	require.NoError(t, c.Create(ctx, ns))
	defer c.Delete(ctx, ns)

	startDeployment(t, ctx, c, ns.GetName())
	runBackup(t, ctx, c, ns.GetName())
	sid := getSnapshotId(t, ctx, ns.GetName())
	assert.Equal(t, "content", dumpSnapshot(t, ctx, sid))
}

func startDeployment(t *testing.T, ctx context.Context, c client.Client, ns string) {
	dep := appv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "deploy",
			Namespace: ns,
		},
		Spec: appv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "backup",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "backup",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "container",
							Image: "quay.io/prometheus/busybox:latest",
							Args: []string{
								"sh",
								"-c",
								`printf "$BACKUP_FILE_CONTENT" | tee "/data/$BACKUP_FILE_NAME" && \
 echo && \
 ls -la /data && \
 echo "test file /data/$BACKUP_FILE_NAME written, sleeping now" && \
 sleep infinity`,
							},
							Env: []corev1.EnvVar{
								{
									Name:  "BACKUP_FILE_CONTENT",
									Value: "content",
								},
								{
									Name:  "BACKUP_FILE_NAME",
									Value: "name",
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "volume",
									MountPath: "/data",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "volume",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: "pvc",
								},
							},
						},
					},
				},
			},
		},
	}
	require.NoError(t, c.Create(ctx, &dep))
	mode := corev1.PersistentVolumeFilesystem
	pvc := corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pvc",
			Namespace: ns,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteMany,
			},
			Resources: corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceStorage: resource.MustParse("1Gi"),
				},
			},
			StorageClassName: pointer.String("standard"),
			VolumeMode:       &mode,
		},
	}
	require.NoError(t, c.Create(ctx, &pvc))

	assert.Eventually(t, func() bool {
		if c.Get(ctx, client.ObjectKeyFromObject(&dep), &dep) != nil {
			return false
		}
		return dep.Status.ReadyReplicas == 1
	}, 10*time.Second, 100*time.Millisecond)

}

func runBackup(t *testing.T, ctx context.Context, c client.Client, ns string) {
	cred := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "backup-credentials",
			Namespace: ns,
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"username": []byte("myaccesskey"),
			"password": []byte("mysecretkey"),
		},
	}
	repo := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "backup-repo",
			Namespace: ns,
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"password": []byte("myreposecret"),
		},
	}
	backup := k8upv1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "k8up-backup",
			Namespace: ns,
		},
		Spec: k8upv1.BackupSpec{
			FailedJobsHistoryLimit:     pointer.Int(1),
			SuccessfulJobsHistoryLimit: pointer.Int(1),
			RunnableSpec: k8upv1.RunnableSpec{
				Backend: &k8upv1.Backend{
					RepoPasswordSecretRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: repo.GetName(),
						},
						Key: "password",
					},
					S3: &k8upv1.S3Spec{
						Endpoint: "http://minio.minio.svc.cluster.local:9000",
						Bucket:   "backup",
						AccessKeyIDSecretRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: cred.GetName(),
							},
							Key: "username",
						},
						SecretAccessKeySecretRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: cred.GetName(),
							},
							Key: "password",
						},
					},
				},
			},
		},
	}

	require.NoError(t, c.Create(ctx, &cred))
	require.NoError(t, c.Create(ctx, &repo))
	require.NoError(t, c.Create(ctx, &backup))

	assert.Eventually(t, func() bool {
		if c.Get(ctx, client.ObjectKeyFromObject(&backup), &backup) != nil {
			return false
		}
		return backup.Status.Finished
	}, 20*time.Second, 100*time.Millisecond)
}

type snapshot struct {
	Hostname string
	Id       string
}

func getSnapshotId(t *testing.T, ctx context.Context, ns string) string {
	var sid string
	require.Eventuallyf(t, func() bool {
		ok := false
		cmd := exec.Command("restic", "--no-cache", "--repo", "s3:http://localhost:30000/backup", "snapshots", "--json")
		cmd.Env = append(cmd.Env, "AWS_ACCESS_KEY_ID=myaccesskey", "AWS_SECRET_ACCESS_KEY=mysecretkey", "RESTIC_PASSWORD=myreposecret")
		snapshotsRaw, err := cmd.Output()
		if err != nil {
			return false
		}
		snapshots := []snapshot{}
		err = json.Unmarshal(snapshotsRaw, &snapshots)
		if err != nil {
			return false
		}

		for _, s := range snapshots {
			if s.Hostname == ns {
				return true
			}
		}
		return ok
	}, 10*time.Second, 100*time.Millisecond, "no snapshot for %s", ns)
	return sid
}

func dumpSnapshot(t *testing.T, ctx context.Context, sid string) string {
	cmd := exec.Command("restic", "--no-cache", "--repo", "s3:http://localhost:30000/backup", "dump", sid, "/data/pvc/name", "--json")
	cmd.Env = append(cmd.Env, "AWS_ACCESS_KEY_ID=myaccesskey", "AWS_SECRET_ACCESS_KEY=mysecretkey", "RESTIC_PASSWORD=myreposecret")
	dump, err := cmd.Output()
	require.NoError(t, err)
	return string(dump)
}

/*

   export OUT=$(kubectl run "restic-dump" \
     --attach \
     --rm \
     --restart Never \
     --namespace "${NAMESPACE}" \
     --image "${E2E_IMAGE}" \
     --env "AWS_ACCESS_KEY_ID=myaccesskey" \
     --env "AWS_SECRET_ACCESS_KEY=mysecretkey" \
     --env "RESTIC_PASSWORD=myreposecret" \
     --pod-running-timeout 10s \
     --quiet \
     --command -- \
     restic \
     --no-cache \
     --repo "s3:http://minio.minio.svc.cluster.local:9000/backup" \
     "dump" "$SID" "/data/subject-pvc/BACKUP_FILE_NAME"  \
     --json )
   bash -c '[ "$OUT" == "BACKUP_FILE_CONTENT" ]'
*/
